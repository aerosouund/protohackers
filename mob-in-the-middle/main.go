package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
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

func handleUpstream(conn net.Conn, initialChannel chan string, messagesUpstream <-chan string, messagesDownstream chan<- string) {
	defer conn.Close()

	initialGreeting := make([]byte, 1024)

	n, err := conn.Read(initialGreeting)
	if err != nil {
		if err != io.EOF {
			fmt.Println("Error reading:", err.Error())
		}
	}

	initialChannel <- string(initialGreeting[:n])

	name := <-initialChannel

	fmt.Fprintf(conn, name)

	whosInTheRoom := make([]byte, 1024)
	n, err = conn.Read(whosInTheRoom)
	if err != nil {
		if err != io.EOF {
			fmt.Println("Error reading additional data from connection:", err.Error())
			return
		}
	}
	fmt.Println("the incoming greeting data :", string(whosInTheRoom[:n]))
	initialChannel <- string(whosInTheRoom[:n]) + "\n"

	joinAnnouncement := make([]byte, 1024)
	n, err = conn.Read(joinAnnouncement)
	if err != nil {
		if err != io.EOF {
			fmt.Println("Error reading additional data from connection:", err.Error())
			return
		}
	}
	fmt.Println("the incoming greeting data :", string(joinAnnouncement[:n]))
	initialChannel <- string(joinAnnouncement[:n]) + "\n"

	for {

		messagesComingFromClient := <-messagesUpstream
		fmt.Fprintf(conn, messagesComingFromClient)

		incomingData := make([]byte, 1024)
		n, err := conn.Read(incomingData)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading additional data from connection:", err.Error())
				return
			}
		}
		input := string(incomingData[:n])

		messagesDownstream <- input

	}
}

func handleConnection(conn net.Conn, initialChannel chan string, messagesUpstream chan<- string, messagesDownstream <-chan string) {

	defer conn.Close()
	greeting := <-initialChannel
	fmt.Println("first greeting ", greeting)

	fmt.Fprintf(conn, greeting)

	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	name := scanner.Text()

	if len(name) != 0 {
		if len(name) >= 16 {
			fmt.Fprintf(conn, "Invalid name")
			conn.Close()
			return
		}
		initialChannel <- name + "\n"
		whosInTheRoom := <-initialChannel
		fmt.Println("whos in the room ", whosInTheRoom)

		fmt.Fprintf(conn, whosInTheRoom)

		joinAnnouncement := <-initialChannel
		fmt.Println("join announcement ", joinAnnouncement)

		// Add the client to the list of active connections
		// message := fmt.Sprintf("* '%s' has joined the room", name)

		client := &client{conn, name}
		clientsMu.Lock()
		clients = append(clients, client)
		clientsMu.Unlock()

		sendToAll(joinAnnouncement)

	} else {
		return
	}

	// Keep the connection alive
	for {
		// Read the client's input
		res := scanner.Scan()

		messagesUpstream <- scanner.Text() + "\n"
		input := <-messagesDownstream

		// handle connection closure
		if res == false {
			conn.Close()
			fmt.Printf("* '%s' has left the room\n", name)
			clientsMu.Lock()
			// Remove the client from the list of active connections
			for i, c := range clients {
				if c.conn == conn {
					clients = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			clientsMu.Unlock()
			message := fmt.Sprintf("* '%s' has left the room\n", name)

			sendToAllExceptSender(conn, message)
			return
		}
		pattern := `\b7\w{25,34}\b`
		re := regexp.MustCompile(pattern)
		var message string

		if re.MatchString(input) {
			modified := re.ReplaceAllString(input, "7YWHMfk9JZe0LM0g1ZauHuiSxhI")
			message = fmt.Sprintf("%s", modified)
		} else {
			message = fmt.Sprintf("%s", input)
		}
		// send downstream
		sendToAllExceptSender(conn, message)
	}
}

func sendToAllExceptSender(conn net.Conn, message string) {
	clientsMu.Lock()
	for _, client := range clients {
		if client.conn != conn {
			fmt.Fprintf(client.conn, "%s\n", message)
		}
	}
	clientsMu.Unlock()
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
	messagesUpstream := make(chan string)
	messagesDownstream := make(chan string)
	initialChannel := make(chan string)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			continue
		}
		fmt.Printf("New connection from %s\n", conn.RemoteAddr().String())

		upstreamConn, err := net.Dial("tcp", "localhost:8001")
		// upstreamConn, err := net.Dial("tcp", "chat.protohackers.com:16963")
		if err != nil {
			fmt.Printf("Error in creating upstream: %s\n", err)
			continue
		}

		go handleConnection(conn, initialChannel, messagesUpstream, messagesDownstream)
		go handleUpstream(upstreamConn, initialChannel, messagesUpstream, messagesDownstream)
	}
}
