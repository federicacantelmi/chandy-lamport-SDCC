package process

import (
	"chandy-lamport/src/utils"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	maxRetries int
)

// Struttura temporanea per decodificare solo il campo "type"
type BaseMessage struct {
	Type int `json:"Type"` // Campo "type" usato per identificare il tipo di messaggio
}

type ProcessStruct struct {
	ProcessID           int                // Id del processo
	Balance             int                // Bilancio
	processNumber       int                // Numero di processi nel sistema
	TCPAddr             *net.TCPAddr       // Indirizzo TCP per identificare il processo
	InternStateMutex    sync.Mutex         // Mutex usato per sincronizzare l'accesso allo stato interno quando viene avviato uno snapshot e si deve prelevare lo stato del processo
	channels            utils.NodeChannels // Canali usati per comunicare con le altre due parti
	sendingMarkersMutex sync.Mutex
	inMsgQueue          []utils.Message
	inMsgQueueMutex     sync.Mutex
}

func StartProcess(processID int, processNumber int, TCPAddr *net.TCPAddr, initialBalance int, channels utils.NodeChannels, maxRet int /*, procList map[int]*net.TCPAddr */) *ProcessStruct {

	p := &ProcessStruct{

		ProcessID:           processID,
		Balance:             initialBalance,
		processNumber:       processNumber,
		TCPAddr:             TCPAddr,
		InternStateMutex:    sync.Mutex{},
		channels:            channels,
		sendingMarkersMutex: sync.Mutex{},
	}
	maxRetries = maxRet
	fmt.Printf("Starting process %d with balance: %d\n", p.ProcessID, p.Balance)

	// Avvio la funzione di ascolto per ricevere comandi e la funzione di ascolto per ricevere connessioni da parte di altri processi
	go p.listenOnChannel()
	go p.runCommand()

	return p
}

func (p *ProcessStruct) runCommand() {
	for {
		select {
		case cmd := <-p.channels.ProcessChannel:
			switch cmd.Name {
			case "SendMessage":
				if msg, ok := cmd.Payload.(utils.Message); ok {
					fmt.Printf("Process %d command: received command to send message to process %d: %d\n", p.ProcessID, msg.Receiver, msg.Content)
					go func(msg utils.Message) {
						p.sendingMarkersMutex.Lock()
						if err := p.sendMessage(msg); err != nil {
							cmd.ReplyChannel <- err
						} else {
							cmd.ReplyChannel <- nil
						}
						p.sendingMarkersMutex.Unlock()
					}(msg)
				}
			case "SendMarkers":
				if msg, ok := cmd.Payload.(utils.Message); ok {
					go func(msg utils.Message) {
						if err := p.sendMarkers(msg.SnapshotID); err != nil {
							log.Printf("Error in sending markers: %v", err)
						}
					}(msg)
				}
			case "MutexRelease":
				go func() {
					p.sendingMarkersMutex.Unlock()
				}()
			case "StartingSnapshot":
				p.sendingMarkersMutex.Lock()
			case "ForcedTermination":
				fmt.Printf("Process %d balance: %d\n", p.ProcessID, p.Balance)
			default:
				log.Printf("Process %d command: Unknown command %s.\n", p.ProcessID, cmd.Name)
			}
		}
	}
}

// Funzione che gestisce l'invio di messaggi
func (p *ProcessStruct) sendMessage(msg utils.Message) error {
	if (p.Balance - msg.Content) < 0 {
		return fmt.Errorf("Balance not enough\n")
	}
	msg.Type = utils.MESSAGE
	msg.Sender = p.ProcessID
	receiver := msg.Receiver

	tcpAddress := fmt.Sprintf("process%d:8081", msg.Receiver)
	var conn net.Conn
	var err error
	for i := 0; i < maxRetries; i++ {
		conn, err = net.Dial("tcp", tcpAddress)
		if err != nil {
			if i == maxRetries-1 {
				// Se ho raggiunto il numero massimo di tentativi ritorno con errore
				return fmt.Errorf("Error connecting to process %d: %s\n", receiver, err)
			}
			// Attendo un po' prima del prossimo tentativo
			time.Sleep(1 * time.Second)
			continue
		}
		// Se la connessione è riuscita, esco dal ciclo
		break
	}

	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Error closing connection to process %d: %s\n", p.ProcessID, err)
		}
	}(conn)

	encoder := json.NewEncoder(conn)
	err = encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("Error encoding message to JSON: %s\n", err)
	}

	p.InternStateMutex.Lock()
	balance := p.Balance - msg.Content
	p.Balance = balance
	p.InternStateMutex.Unlock()

	fmt.Printf("Process %d: sent message to %d and added message to internal state: %d -> now internal state: %d\n", p.ProcessID, msg.Receiver, msg.Content, p.Balance)

	return nil
}

func (p *ProcessStruct) sendMarkers(snapshotID int) error {
	// Creo marker
	marker := utils.Message{Type: utils.MARKER, Sender: p.ProcessID, SnapshotID: snapshotID}

	var tcpAddress string

	// Prelevo per ogni processo attivo nel sistema l'indirizzo associato e apro connessione
	for i := 0; i < len(utils.ProcessList); i++ {
		if i+1 == p.ProcessID {
			// Salto iterazione in cui invierei marker a me stesso
			continue
		}
		tcpAddress = "process" + strconv.Itoa(i+1) + ":8081"
		var conn net.Conn
		var err error
		for i := 0; i < maxRetries; i++ {
			conn, err = net.Dial("tcp", tcpAddress)
			if err != nil {
				if i == maxRetries-1 {
					// Se ho raggiunto il numero massimo di tentativi ritorno con errore
					return fmt.Errorf("Error connecting to process %d: %s\n", p.ProcessID, err)
				}
				// Attendo un po' prima del prossimo tentativo
				time.Sleep(1 * time.Second)
				continue
			}
			// Se la connessione è riuscita, esco dal ciclo
			break
		}

		// Invio marker
		encoder := json.NewEncoder(conn)
		err = encoder.Encode(marker)
		if err != nil {
			return fmt.Errorf("Error in sendind JSON data: %s\n", err)
		}
		err = conn.Close()
		if err != nil {
			log.Println("Error closing connection:", err)
		}

		fmt.Printf("Process %d: sent marker to %d for snapshot %d\n", p.ProcessID, i+1, snapshotID)
	}
	//p.sendingMarkersMutex.Lock()
	//p.sendingMarkers = false
	//p.sendingMarkersCond.Signal()
	p.sendingMarkersMutex.Unlock()
	return nil
}

func (p *ProcessStruct) handleMessage(msg utils.Message) {
	// Aggiorno balance del processo
	p.InternStateMutex.Lock()
	balance := p.Balance + msg.Content
	p.Balance = balance
	p.InternStateMutex.Unlock()

	fmt.Printf("Process %d: received message from %d and updated balance with %d: %d\n", p.ProcessID, msg.Sender, msg.Content, p.Balance)

}

func (p *ProcessStruct) handleConnection(conn net.Conn) error {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Println("Error closing connection:", err)
		}
	}(conn)

	var msg utils.Message

	// Decodifico il messaggio dalla connessione
	err := json.NewDecoder(conn).Decode(&msg)
	if err != nil {
		return fmt.Errorf("Error decoding JSON: %s\n", err)
	}
	p.inMsgQueueMutex.Lock()
	p.inMsgQueue = append(p.inMsgQueue, msg)
	p.inMsgQueueMutex.Unlock()
	return nil
}

func (p *ProcessStruct) processMessageQueue() {
	for {
		p.inMsgQueueMutex.Lock()
		if len(p.inMsgQueue) == 0 {
			p.inMsgQueueMutex.Unlock()
			time.Sleep(1 * time.Second)
		} else {
			// Estraggo primo elemento della inMsgQueue
			msg := p.inMsgQueue[0]
			fmt.Printf("%v+", msg)
			// Elimino elemento appena estratto dalla inMsgQueue
			p.inMsgQueue = p.inMsgQueue[1:]
			p.inMsgQueueMutex.Unlock()
			// Verifico tipo di messaggio ricevuto
			switch msg.Type {
			case utils.MESSAGE:
				p.sendingMarkersMutex.Lock()
				p.handleMessage(msg)
				p.channels.SnapshotProcessChannel <- utils.Command{Name: "MessageReceived", Payload: msg}
				p.sendingMarkersMutex.Unlock()
			case utils.MARKER:
				p.sendingMarkersMutex.Lock()
				p.channels.SnapshotProcessChannel <- utils.Command{Name: "MarkerReceived", Payload: msg}
			default:
				fmt.Errorf("Unknown message type: %d\n", msg.Type)
			}
		}
	}
}

// Funzione che rimane in attesa sulla porta per ricevere messaggi da altri processi
func (p *ProcessStruct) listenOnChannel() {
	go p.processMessageQueue()
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatalf("Error during listening process: %s", err)
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			log.Println("Error closing connection:", err)
		}
	}(listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error during connection:", err)
			continue
		}
		// log.Printf("Process %d: connection accepted\n", p.ProcessID)
		go func() {
			err := p.handleConnection(conn)
			if err != nil {
				log.Printf("Error handling connection: %s", err)
			}
		}()
	}
}

// getter per l'id del processo
func (p *ProcessStruct) GetProcessID() int {
	return p.ProcessID
}

// getter per lo stato interno del processo
func (p *ProcessStruct) GetBalance() int {
	return p.Balance
}
