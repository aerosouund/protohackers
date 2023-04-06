package main

import (
	"fmt"
	"io"
	"net"
)

func main() {
	// Listen on TCP port 8080
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return
	}
	defer listener.Close()

	fmt.Println("Listening on port 8000...")

	for {
		// Wait for a connection
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			continue
		}

		// Handle the connection in a new goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	// Make a buffer to hold incoming data
	buf := make([]byte, 1024)

	// Read the incoming connection into the buffer
	for {
		// Read the incoming connection into the buffer
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err.Error())
			}
			return
		}

		// Print out the incoming data
		fmt.Println(buf[:n])
	}
}
