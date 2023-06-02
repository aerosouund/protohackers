package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

// validate message and if invalid send err
var listOfRoads []*Road
var unsentTickets []Ticket

func handleConnection(conn net.Conn) {
	// check what the first byte is
	buffer, err := readBytesFromConn(conn, 1)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
	}

	// Parse the bytes into integers
	startingInt := buffer[0]

	switch startingInt {
	case 0x80:
		handleCamera(conn)
	case 0x81:
		handleDispatcher(conn)
	case 0x40:
		fmt.Println("handling heartbeat")

		b, err := readBytesFromConn(conn, 4)
		if err != nil {
			if err != io.EOF {
				panic(err)
			}
		}

		interval := binary.BigEndian.Uint16(b[2:4])
		sendHeartBeat(conn, interval)
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
