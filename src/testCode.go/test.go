package testCode

import (
	"chandy-lamport/src/client"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

func RunTest(RPCPort string, processId int, wg *sync.WaitGroup) error {
	// Defer Done a quando la goroutine finisce
	defer wg.Done()
	// Avvio il client RPC e lo connetto al server
	procId := strconv.Itoa(processId)
	clientNode := client.NewClientNode("process" + procId + ":" + RPCPort)
	if err := clientNode.Connect(); err != nil {
		return fmt.Errorf("Error connecting to server RPC: %v", err)
	}
	defer clientNode.Close()

	time.Sleep(1000 * time.Millisecond)

	var err error
	if processId == 1 {
		err = clientNode.StartSnapshot(1)
		if err != nil {
			log.Printf("Errore starting snapshot: %v", err)
		}
		err = clientNode.SendMessage(1, 3, 200)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
	}
	if processId == 2 {
		err = clientNode.SendMessage(2, 1, 600)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		err = clientNode.StartSnapshot(2)
		if err != nil {
			log.Printf("Errore starting snapshot: %v", err)
		}
	}
	if processId == 3 {
		err = clientNode.SendMessage(3, 2, 10)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		err = clientNode.StartSnapshot(3)
		if err != nil {
			log.Printf("Errore starting snapshot: %v", err)
		}
	}
	return nil
}
