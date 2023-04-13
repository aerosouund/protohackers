package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
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
	fmt.Fprintf(conn, "Welcome to budgetchat! What shall I call you?\n")

	// Read the client's name
	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	name := scanner.Text()

	fmt.Printf("'%s' has joined the room\n", name)

	if len(clients) != 0 {
		greetingMsg := "* This room contains "
		format := strings.Repeat("%s ", len(clients)) + "\n"

		var clientNames []interface{}
		for _, client := range clients {
			clientNames = append(clientNames, client.name+",")
		}

		fmt.Fprintf(conn, greetingMsg+format, clientNames...)
	}

	// Add the client to the list of active connections
	client := &client{conn, name}
	clientsMu.Lock()
	clients = append(clients, client)
	clientsMu.Unlock()

	// Send a message to all connected clients
	message := fmt.Sprintf("'%s' has joined", name)
	sendToAll(message)

	// Keep the connection alive
	for {
		// Read the client's input
		scanner.Scan()
		input := scanner.Text()

		// if err := scanner.Err(); err != nil {
		// 	fmt.Println("Error:", err)
		// 	break
		// }

		// If the client sends "quit", close the connection
		if err := scanner.Err(); err != nil {
			conn.Close()
			fmt.Printf("'%s' has left the room\n", name)
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
			message := fmt.Sprintf("'%s' has left the room\n", name)
			sendToAll(message)
			return
		}
		if len(input) != 0 {
			message := fmt.Sprintf("[%s] %s", name, input)
			// fmt.Println(message)
			sendToAll(message)
		}
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
			// continue
		}
		fmt.Printf("New connection from %s\n", conn.RemoteAddr().String())
		go handleConnection(conn)
	}
}
