package main

import (
	"chandy-lamport/src/testCode.go"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	RPCPort      int
	NumProcesses int
)

func init() {
	NumProcesses, _ = strconv.Atoi(os.Getenv("NUM_PROCESS"))
	RPCPort, _ = strconv.Atoi(os.Getenv("RPC_PORT"))
}

func main() {
	var wg sync.WaitGroup
	var RPCPortStr string
	for i := 1; i < NumProcesses+1; i++ {
		RPCPortStr = strconv.Itoa(RPCPort)
		i := i
		wg.Add(1)
		go func() {
			err := testCode.RunTest(RPCPortStr, i, &wg)
			if err != nil {
				log.Fatalf("Error running test: %v", err)
			}
		}()
	}

	// Aspetto che tutte le goroutine finiscano
	wg.Wait()
	time.Sleep(120 * time.Second)
	fmt.Println("All goroutines completed")
}
