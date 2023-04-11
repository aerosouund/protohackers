package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

// TO DO
/*
1- on connection call the authorize function
2- once authed add to an array of users
3- implement send message
4-

*/

func main() {
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return
	}
	defer listener.Close()

	fmt.Println("Listening on port 8000...")

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("Error accepting connection:", err.Error())
	}
	defer conn.Close()
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	// Store the user's name in a variable
	name = name[:len(name)-1] // remove the trailing newline character
	fmt.Println("Received name:", name)

	// Send the user's name to the client
	conn.Write([]byte("Hello " + name))
}
