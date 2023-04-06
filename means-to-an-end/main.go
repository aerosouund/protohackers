package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sort"
)

type Message struct {
	messageType rune
	firstNum    int32
	secondNum   int32
}

var store []Message

/*
TO-DO:
1- a listener that recieves data
2- a message parser
3- message objects (structs) with the parsed data
4- a storage method (session separation)
*/

func handleMessage(conn net.Conn) {
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err.Error())
			}
			return
		}
		for i := 0; i < len(buf[:n]); i += 9 {
			var m Message
			m.messageType = rune(buf[i])

			m.firstNum = int32(binary.BigEndian.Uint32(buf[i+1 : i+4]))
			m.secondNum = int32(binary.BigEndian.Uint32(buf[i+5 : i+8]))
			fmt.Printf("Parsed message with following values: %s, %s, %s", string(m.messageType), m.firstNum, m.secondNum)
			if string(m.messageType) == "I" {
				handleInsert(m)
			}
			if string(m.messageType) == "Q" {
				handleQuery(m)
			}
		}

		// Print out the incoming data
		fmt.Println(buf[:n])
	}
}

func handleInsert(m Message) {
	store = append(store, m)

	if len(store) < 2 {
		return
	}

	sort.Slice(store, func(i, j int) bool {
		return store[i].firstNum < store[j].firstNum
	})
}

func handleQuery(m Message) float32 {
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
		return 0
	}

	var sum float32 = 0.0
	for _, elem := range store[startIndex:endIndex] {
		firstNumFloat := float32(elem.firstNum)
		sum += firstNumFloat
	}

	// Compute the average of the Num field values
	avg := sum / float32(len(store[startIndex:endIndex]))
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
		go handleMessage(conn)
	}
}
