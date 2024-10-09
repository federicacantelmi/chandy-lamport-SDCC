package main

import (
	"chandy-lamport/src/process"
	"chandy-lamport/src/snapshot"
	"chandy-lamport/src/utils"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var (
	NumProcesses    int
	TcpStartingPort int
	RPCPort         int
	MaxRetries      int
)

type Node struct {
	process         *process.ProcessStruct
	snapshotProcess *snapshot.SnapshotProcess
	channels        utils.NodeChannels
}

type RPCServer struct {
	node *Node
}

// SendMessage Metodo RPC per inviare un messaggio
func (s *RPCServer) SendMessage(args *utils.SendMessageArgs, reply *string) error {
	if args.Content < 0 {
		*reply = fmt.Sprintf("Value to be sent needs to be > 0\n")
		return fmt.Errorf("Value to be sent needs to be > 0\n")
	}
	replyChan := make(chan error)
	// Invio un comando SendMessage al canale del processo
	s.node.channels.ProcessChannel <- utils.Command{Name: "SendMessage", Payload: utils.Message{Sender: 0, Receiver: args.ReceiverID, Content: args.Content}, ReplyChannel: replyChan}
	replyVal := <-replyChan
	if replyVal == nil {
		*reply = fmt.Sprintf("Message '%d' sent to process %d successfully.\n", args.Content, args.ReceiverID)
	} else {
		*reply = fmt.Sprintf("Error in sending message '%d' to process %d: %s\n", args.Content, args.ReceiverID, replyVal)
	}
	return nil
}

// StartSnapshot Metodo RPC per avviare uno snapshot
func (s *RPCServer) StartSnapshot(args *utils.SendMessageArgs, reply *string) error {
	replyChan := make(chan error)
	// Invio un comando StartSnapshot al processo
	s.node.channels.SnapshotProcessChannel <- utils.Command{Name: "StartSnapshot", Payload: utils.Message{}, ReplyChannel: replyChan}
	replyVal := <-replyChan
	if replyVal == nil {
		*reply = fmt.Sprintf("Snapshot executed successfully.\n")
	} else {
		*reply = fmt.Sprintf("Error in executing the snapshot\n")
	}

	return nil
}

func NewNode(processID int, tcpStartingPort int, processNumber int, initialBalance int) *Node {
	var node Node

	// Creo canali per la comunicazione tra le tre parti della dimensione specificata
	node.channels = utils.CreateNodeChannels(10)

	// Calcolo indirizzo TCP sommando al numero di porta di partenza l'id del processo
	TCPAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: tcpStartingPort + processID, Zone: ""}

	// Creo lista dei processi attivi nel sistema -> servirà per tenere traccia dei loro indirizzi TCP
	utils.ProcessList = make(map[int]*net.TCPAddr)
	for i := 1; i < processNumber+1; i++ {
		utils.ProcessList[i] = &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: tcpStartingPort + i,
			Zone: "",
		}
		// fmt.Println("Processo " + strconv.Itoa(i) + " con tcp addr: " + strconv.Itoa(tcpStartingPort+i) + " " + utils.ProcessList[i].IP.String())
	}

	fmt.Println("Inizializzazione processo " + strconv.Itoa(processID))

	// Avvio processo
	node.process = process.StartProcess(processID, processNumber, TCPAddr, initialBalance, node.channels, MaxRetries)

	// Avvio snapshot process
	node.snapshotProcess = snapshot.CreateSnapshotProcess(node.process, node.channels)

	return &node
}

func termination(node *Node) {
	sigChan := make(chan os.Signal, 1)
	// Catturo i segnali SIGINT (Ctrl+C) e SIGTERM (terminazione)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Canale per indicare che il programma può terminare
	done := make(chan bool, 1)

	// Goroutine che aspetta il segnale
	go func() {
		sig := <-sigChan // Blocca finché non riceve un segnale
		fmt.Printf("\nSignal received: %s\n", sig)

		// Avverto process che deve stampare il valore del balance
		node.channels.ProcessChannel <- utils.Command{Name: "ForcedTermination", Payload: utils.Message{}}

		// Dico al programma di terminare
		done <- true
	}()

	// Blocco l'esecuzione finché non ricevo il segnale
	<-done
}
func init() {
	/*err := godotenv.Load("./app/.env")
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}*/
	NumProcesses, _ = strconv.Atoi(os.Getenv("NUM_PROCESS"))
	TcpStartingPort, _ = strconv.Atoi(os.Getenv("TCP_PORT"))
	RPCPort, _ = strconv.Atoi(os.Getenv("RPC_PORT"))
	MaxRetries, _ = strconv.Atoi(os.Getenv("MAX_RETRIES"))
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <processID> <initialBalance>", os.Args[0])
	}

	processID, _ := strconv.Atoi(os.Args[1])

	rpcPort := strconv.Itoa(RPCPort)
	initialBalance, _ := strconv.Atoi(os.Args[2])

	// Creo nuovo nodo
	node := NewNode(processID, TcpStartingPort, NumProcesses, initialBalance)

	// Creo server RPC
	rpcServer := &RPCServer{node: node}
	server := rpc.NewServer()

	// Registro nome del servizio
	err := server.RegisterName("Chandy-Lamport-Service", rpcServer)
	if err != nil {
		log.Fatal("Error registering chandy-lamport node:", err)
	}

	lis, err := net.Listen("tcp", ":"+rpcPort)
	if err != nil {
		log.Fatal("Error listening on tcp port:", err)
	}

	// Avvio go routine che si occupa di gestire la terminazione del codice
	go termination(node)

	// Avvio go routine che si occupa di ricevere invocazioni dei metodi RPC
	go server.Accept(lis)
	defer func(lis net.Listener) {
		err := lis.Close()
		if err != nil {
			log.Printf("Error closing listener: %v", err)
		}
	}(lis)
	select {}
}
