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

func handleMessage(conn net.Conn) {
	buf := make([]byte, 0, 1024)

	// Read the incoming connection into the buffer
	for {
		// Read the incoming connection into the remaining buffer space
		// starting from the current length of the buffer
		n, err := conn.Read(buf[len(buf):cap(buf)])
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err.Error())
			}
			break
		}

		// Update the length of the buffer to include the newly read bytes
		buf = buf[:len(buf)+n]

		if len(buf)%9 == 0 {
			var m Message
			m.messageType = rune(buf[len(buf)-9])
			// fmt.Println(buf[len(buf)-9 : len(buf)])

			m.firstNum = int32(binary.BigEndian.Uint32(buf[len(buf)-8 : len(buf)-4]))
			m.secondNum = int32(binary.BigEndian.Uint32(buf[len(buf)-4 : len(buf)]))
			fmt.Println(m.secondNum)
			fmt.Printf("Parsed message with following values: %s, %d, %d \n", string(m.messageType), m.firstNum, m.secondNum)
			if string(m.messageType) == "I" {
				handleInsert(m)
			}
			if string(m.messageType) == "Q" {
				a := handleQuery(m)
				returnBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(returnBuf, uint32(a))
				fmt.Println(returnBuf)

				// Send the binary data as the TCP response
				if _, err := conn.Write(returnBuf); err != nil {
					return
				}
			}
		}
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

	if startIndex > endIndex {
		fmt.Println("No elements found in the range")
		return 0
	}

	if startIndex == endIndex {
		return float32(store[startIndex].secondNum)
	}

	var sum float32 = 0.0
	for _, elem := range store[startIndex:endIndex] {
		secondNumFloat := float32(elem.secondNum)
		sum += secondNumFloat
	}

	// Compute the average of the Num field values
	avg := sum / float32(len(store[startIndex:endIndex]))
	fmt.Println(avg)
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
