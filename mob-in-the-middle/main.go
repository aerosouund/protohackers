package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
)

func handleConnection(conn net.Conn) {
	upstreamConn, err := net.Dial("tcp", "chat.protohackers.com:16963")
	if err != nil {
		fmt.Printf("Error in creating upstream: %s\n", err)
		return
	}

	upstreamScanner := bufio.NewScanner(upstreamConn)
	upstreamScanner.Scan()
	initialGreeting := upstreamScanner.Text() + "\n"

	fmt.Fprintf(conn, initialGreeting)

	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	name := scanner.Text() + "\n"

	if len(name) != 0 {
		if len(name) >= 16 {
			fmt.Fprintf(conn, "Invalid name")
			conn.Close()
			return
		}
		fmt.Fprintf(upstreamConn, name)

		upstreamScanner.Scan()
		whosInTheRoom := upstreamScanner.Text() + "\n"

		fmt.Fprintf(conn, whosInTheRoom)

	} else {
		return
	}

	go proxy(upstreamConn, conn)
	go proxy(conn, upstreamConn)
}

func proxy(in net.Conn, out net.Conn) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		data := scanner.Text() + "\n"

		pattern := `\b7\w{25,34}\b`
		re := regexp.MustCompile(pattern)
		var message string

		if re.MatchString(data) {
			modified := re.ReplaceAllString(data, "7YWHMfk9JZe0LM0g1ZauHuiSxhI")
			message = fmt.Sprintf("%s", modified)
		} else {
			message = fmt.Sprintf("%s", data)
		}
		fmt.Fprintf(out, message)
	}
	in.Close()
	out.Close()
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
