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

	fmt.Fprintf(conn, "Welcome to budgetchat! What shall I call you?\n")

	// Read the client's name
	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	name := scanner.Text()

	if len(name) != 0 {
		fmt.Printf("'%s' has joined the room\n", name)

		greetingMsg := "* This room contains "
		format := strings.Repeat("%s ", len(clients)) + "\n"

		var clientNames []interface{}
		for _, client := range clients {
			clientNames = append(clientNames, client.name+",")
		}

		fmt.Fprintf(conn, greetingMsg+format, clientNames...)

		// Add the client to the list of active connections
		message := fmt.Sprintf("* '%s' has joined", name)
		sendToAll(message)

		client := &client{conn, name}
		clientsMu.Lock()
		clients = append(clients, client)
		clientsMu.Unlock()

	} else {
		return
	}

	// Keep the connection alive
	for {
		// Read the client's input
		res := scanner.Scan()
		input := scanner.Text()

		if res == false {
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
			message := fmt.Sprintf("'%s' has left the room\n", name)
			sendToAll(message)
			return
		}
		if len(input) != 0 {
			message := fmt.Sprintf("[%s] %s", name, input)
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
			continue
		}
		fmt.Printf("New connection from %s\n", conn.RemoteAddr().String())
		go handleConnection(conn)
	}
}
