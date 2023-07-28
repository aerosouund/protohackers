package main

import (
	"bufio"
	"fmt"
	"jobs/types"
	"net"
	"os"
)

var qm *types.QueueManager

func handleConnection(conn net.Conn, clientExitChan chan struct{}) {

	clientAddr := conn.RemoteAddr().String()
	respCh := make(chan map[string]any)
	go aborter(conn.RemoteAddr().String(), clientExitChan)

	for s := bufio.NewScanner(conn); s.Scan(); {
		b := s.Bytes()
		err := validateMessage(b)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			handleMessage(clientExitChan, clientAddr, b, respCh, conn)
		}
	}
	close(clientExitChan)
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

		clientExitChan := make(chan struct{})
		go handleConnection(conn, clientExitChan)
	}
}
