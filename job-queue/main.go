package main

import (
	"bufio"
	"fmt"
	"jobs/types"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

var qm *types.QueueManager

func handleConnection(conn net.Conn) {
	defer conn.Close()
	disconnected := false
	rand.Seed(time.Now().UnixNano())
	id := rand.Intn(100000000000000)

	clientAddr := strconv.Itoa(id)

	for s := bufio.NewScanner(conn); s.Scan(); {
		b := s.Bytes()
		err := validateMessage(b)
		if err != nil {
			fmt.Println(err.Error())
			write(conn, types.ErrMap, string(b))
			return
		} else {
			handleMessage(clientAddr, b, conn, disconnected)
		}
	}
	disconnected = true
	abort(clientAddr)
}

func main() {
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server started. Listening on port 8000...")
	qm = types.NewQueueManager()

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
