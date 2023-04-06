package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sort"
)

type Message struct {
	messageType string
	firstNum    int32
	secondNum   int32
}

type dataStore []*Message

var store dataStore

/*
TO-DO:
1- a listener that recieves data
2- a message parser
3- message objects (structs) with the parsed data
4- a storage method (session separation)
*/

func parseMessage(conn net.Conn) (rune, []int32) {
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err.Error())
			}
			return nil, nil
		}

		// Print out the incoming data
		fmt.Println(buf[:n])
	}

	// Extract the remaining bytes as int32 values
	var numbers []int32
	for i := 0; i < len(buf[:n]); i += 9 {
		var m Message
		m.messageType = rune(buf[i])

		m.firstNum = int32(binary.BigEndian.Uint32(buf[i+1 : i+4]))
		m.secondNum = int32(binary.BigEndian.Uint32(buf[i+5 : i+8]))
		numbers = append(numbers, number1)
	}

	fmt.Printf("First character: %c\n", firstChar)
	fmt.Printf("Numbers: %v\n", numbers)

	return firstChar, numbers
}

func createMessage(ch string, nums []int32) {
	var m Message
	m.firstNum = nums[0]
	m.secondNum = nums[1]
	switch ch {
	case "I":
		// logic for insert
		m.messageType = "Insert"
	case "Q":
		// logic for query
		m.messageType = "Query"
	default:
		fmt.Println("Unknown message type!")
	}
	return *m
}

func handleInsert(message *Message) {
	// append(store, message)

	if len(store) < 2 {
		return
	}

	sort.Slice(store, func(i, j int) bool {
		return store[i].firstNum < store[j].firstNum
	})
}

func handleQuery(m *Message) {
	startIndex := sort.Search(len(store), func(i int) bool {
		return store[i].firstNum >= m.firstNum
	})
	endIndex := sort.Search(len(store), func(i int) bool {
		return store[i].firstNum > m.secondNum
	})

	if endIndex > len(store)-1 {
		endIndex = len(store)
	}

	if startIndex >= endIndex {
		fmt.Println("No elements found in the range")
		return
	}

	sum := 0.0
	for _, elem := range store[startIndex:endIndex] {
		sum += elem.firstNum
	}

	// Compute the average of the Num field values
	avg := sum / float64(len(store[startIndex:endIndex]))
	return avg
}

func main() {
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
		go parseMessage(conn)
	}
}
