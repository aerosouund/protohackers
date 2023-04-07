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
	buf := make([]byte, 0, 50)

	// Read the incoming connection into the buffer
	for {
		// Read the incoming connection into the remaining buffer space
		// starting from the current length of the buffer
		n, err := conn.Read(buf[len(buf):cap(buf)])
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err.Error())
			}
			return
		}

		// Update the length of the buffer to include the newly read bytes
		buf = buf[:len(buf)+n]

		// Print out the incoming data and the current buffer contents
		fmt.Println(string(buf[:n]), buf)
	}
}
