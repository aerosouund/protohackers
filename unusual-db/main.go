package main

import (
	"fmt"
	"net"
	"os"
)

func handlePacket(db map[string]string, buffer []byte, addr *net.UDPAddr, conn *net.UDPConn) {

	input := string(buffer)

	if string(input[len(input)-1]) == "\n" {
		input = input[:len(input)-1]
	}

	fmt.Println("the main input", input)

	for i, c := range input {
		if string(c) == "=" {
			fmt.Printf("Found equal sign at index %d\n", i)
			if input[:i] != "version" {
				db[input[:i]] = input[i+1:]
			}
			return
		}
	}

	val := db[input]

	data := fmt.Sprintf("%s=%s", input, val)
	fmt.Println("the data", data)
	_, err := conn.WriteToUDP([]byte(data), addr)
	if err != nil {
		fmt.Printf("Error sending UDP response: %s\n", err)
		return
	}
}

func main() {
	db := make(map[string]string)
	db["version"] = "Ken's Key-Value Store 1.0\n"

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

	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			continue
		}
		fmt.Printf("New packet from %s\n", addr.String())
		go handlePacket(db, buffer[:n], addr, conn)
	}
}
