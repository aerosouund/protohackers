package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"jobs/types"
	"net"
	"os"
)

var qm *types.QueueManager

func handleConnection(conn net.Conn, clientExitChan chan struct{}) {
	defer close(clientExitChan)
	clientAddr := conn.RemoteAddr().String()
	respCh := make(chan map[string]any)

	for s := bufio.NewScanner(conn); s.Scan(); {
		b := s.Bytes()
		err := validateMessage(b)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			go handleMessage(clientExitChan, clientAddr, b, respCh)
			go writer(conn, respCh, clientExitChan)
			// if err != nil {
			// 	fmt.Println(err)
			// }
			// r := <-respCh
			// responseJson, _ := json.Marshal(r)
			// fmt.Fprintf(conn, string(responseJson)+"\n")
		}
	}
}

func writer(conn net.Conn, respCh chan map[string]any, clientExitChan chan struct{}) {
	for {
		select {
		case <-clientExitChan:
		case r := <-respCh:
			responseJson, _ := json.Marshal(r)
			fmt.Fprintf(conn, string(responseJson)+"\n")
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server started. Listening on port 8000...")
	qm = types.NewQueueManager()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			continue
		}
		fmt.Printf("New connection from %s\n", conn.RemoteAddr().String())
		// clientAddr := conn.RemoteAddr().String()

		clientExitChan := make(chan struct{})

		// go checkAlive(conn, clientExitChan)
		// go aborter(clientAddr, clientExitChan)
		go handleConnection(conn, clientExitChan)
	}
}
