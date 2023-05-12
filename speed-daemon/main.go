package main

import (
	"fmt"
	"net"
	"os"
)

// validate message and if invalid send err
var listOfRoads []Road
var unsentTickets []Ticket

func handleConnection(conn net.Conn) {
	// check what the first byte is
	buffer := make([]byte, 1)

	// Read bytes from the connection into the buffer
	_, err := conn.Read(buffer)
	if err != nil {
		panic(err)
	}

	// Parse the bytes into integers
	startingInt := int(int8(buffer[0]))
	if startingInt == 80 {
		handleCamera(conn)
	}
	if startingInt == 81 {
		handleDispatcher(conn)
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
