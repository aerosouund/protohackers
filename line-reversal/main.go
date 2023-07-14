package main

import (
	"fmt"
	"net"
	"os"
)

var SS *SessionStore

func main() {

	udpAddr, err := net.ResolveUDPAddr("udp", ":8000")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("Server started. Listening on port 8000...")

	messagechan := make(chan MessageDispatch)
	eventchan := make(chan interface{})
	go startWriter(messagechan, conn)
	go manageSessionLifecycle(eventchan)

	SS = NewSessionStore()

	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			continue
		}
		fmt.Printf("New packet from %s\n", addr.String())
		message, err := parseMessage(buffer[:n])
		if err == nil {
			go handleMessage(message, *conn, messagechan, addr, eventchan)
		}
	}

}
