package client

import (
	"chandy-lamport/src/utils"
	"log"
	"net/rpc"
)

// ClientNode rappresenta il client RPC
type ClientNode struct {
	serverAddress string
	RpcClient     *rpc.Client
}

// NewClientNode crea una nuova istanza di ClientNode
func NewClientNode(serverAddress string) *ClientNode {
	return &ClientNode{serverAddress: serverAddress}
}

// Connect si connette al server RPC
func (c *ClientNode) Connect() error {
	client, err := rpc.Dial("tcp", c.serverAddress)
	if err != nil {
		log.Fatal("Error in dialing: ", err)
	}
	c.RpcClient = client
	return nil
}

// StartSnapshot invoca il metodo StartSnapshot sul server RPC
func (c *ClientNode) StartSnapshot(processID int) error {
	var reply string
	err := c.RpcClient.Call("Chandy-Lamport-Service.StartSnapshot", utils.SendMessageArgs{
		SenderID:   processID,
		ReceiverID: 0,
		Content:    0,
	}, &reply)
	if err != nil {
		log.Fatalf("Errore avvio snapshot: %v", err)
	}
	log.Printf("%s", reply)
	return nil
}

// SendMessage invia un messaggio a un altro processo
func (c *ClientNode) SendMessage(senderID int, receiverID int, content int) error {
	var reply string
	err := c.RpcClient.Call("Chandy-Lamport-Service.SendMessage", utils.SendMessageArgs{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
	}, &reply)
	if err != nil {
		log.Fatalf("Errore in sending message: %v", err)
	}
	log.Printf("%s", reply)
	return nil
}

// Close chiude la connessione RPC
func (c *ClientNode) Close() {
	if c.RpcClient != nil {
		c.RpcClient.Close()
	}
}
