package main

import (
	"bufio"
	"fmt"
	"jobs/types"
	"net"
	"os"
)

var qm *types.QueueManager

// STORE A SINGLE REFERENCE TO THE CLIENT
// INVESTIGATE IF A MAX HEAP IS FASTER

func handleConnection(conn net.Conn) {
	disconnected := false

	clientAddr := conn.RemoteAddr().String()
	respCh := make(chan map[string]any)

	for s := bufio.NewScanner(conn); s.Scan(); {
		b := s.Bytes()
		err := validateMessage(b)
		if err != nil {
			fmt.Println(err.Error())
			write(conn, types.ErrMap, string(b))
			continue
		} else {
			handleMessage(clientAddr, b, respCh, conn, disconnected)
		}
	}
	disconnected = true
	abort(conn.RemoteAddr().String())
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
		go handleConnection(conn)
	}
}
