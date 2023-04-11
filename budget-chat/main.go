package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)

type client struct {
	conn net.Conn
	name string
}

var (
	clients   []*client
	clientsMu sync.Mutex
)

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send the "What is your name?" question to the client
	fmt.Fprintf(conn, "What is your name?\n")

	// Read the client's name
	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	name := scanner.Text()

	fmt.Printf("Client '%s' has joined\n", name)

	// Add the client to the list of active connections
	client := &client{conn, name}
	clientsMu.Lock()
	clients = append(clients, client)
	clientsMu.Unlock()

	// Send a message to all connected clients
	message := fmt.Sprintf("Client '%s' has joined", name)
	sendToAll(message)

	// Keep the connection alive
	for {
		// Read the client's input
		scanner.Scan()
		input := scanner.Text()

		// If the client sends "quit", close the connection
		if input == "quit" {
			fmt.Printf("Client '%s' has left\n", name)
			clientsMu.Lock()
			// Remove the client from the list of active connections
			for i, c := range clients {
				if c.conn == conn {
					clients = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			clientsMu.Unlock()
			// Send a message to all connected clients
			message := fmt.Sprintf("Client '%s' has left", name)
			sendToAll(message)
			return
		}

		// Echo the client's input back to them
		fmt.Fprintf(conn, "You said: %s\n", input)
	}
}

func sendToAll(message string) {
	clientsMu.Lock()
	for _, client := range clients {
		fmt.Fprintf(client.conn, "%s\n", message)
	}
	clientsMu.Unlock()
}

func main() {
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server started. Listening on port 1234...")

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
