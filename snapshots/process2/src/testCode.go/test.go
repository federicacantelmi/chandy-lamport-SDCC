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

	time.Sleep(2000 * time.Millisecond)

	var err error
	if processId == 1 {
		err = clientNode.SendMessage(1, 3, 100)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		time.Sleep(1000 * time.Millisecond)
		err = clientNode.SendMessage(1, 5, 20)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		err = clientNode.StartSnapshot(1)
		if err != nil {
			log.Printf("Errore starting snapshot: %v", err)
		}
		err = clientNode.SendMessage(1, 2, 310)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
	}
	if processId == 2 {
		time.Sleep(2000 * time.Millisecond)
		err = clientNode.SendMessage(2, 4, 600)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		time.Sleep(1000 * time.Millisecond)
		err = clientNode.SendMessage(2, 4, 20)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		time.Sleep(1000 * time.Millisecond)
		err = clientNode.SendMessage(2, 5, 10)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
	}
	if processId == 3 {
		err = clientNode.SendMessage(3, 2, 10)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
		err = clientNode.StartSnapshot(3)
		if err != nil {
			log.Printf("Errore starting snapshot: %v", err)
		}
		time.Sleep(1000 * time.Millisecond)
		err = clientNode.SendMessage(3, 5, 300)
		if err != nil {
			log.Printf("Errore sending message: %v", err)
		}
	}
	if processId == 4 {
		err = clientNode.StartSnapshot(4)
		if err != nil {
			log.Printf("Errore starting snapshot: %v", err)
		}
		time.Sleep(1000 * time.Millisecond)
		err = clientNode.SendMessage(4, 1, 100)
		if err != nil {
			log.Printf("Errore invio messaggio: %v", err)
		}
	}

	if processId == 5 {
		err = clientNode.SendMessage(5, 3, 20)
		if err != nil {
			log.Printf("Errore invio messaggio: %v", err)
		}
		err = clientNode.SendMessage(5, 1, 30)
		if err != nil {
			log.Printf("Errore invio messaggio: %v", err)
		}
		err = clientNode.SendMessage(5, 4, 10)
		if err != nil {
			log.Printf("Errore invio messaggio: %v", err)
		}
	}
	return nil
}
