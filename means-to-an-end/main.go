package main

import (
	"fmt"
	"net/http"
	"log"
	"encoding/binary"
	"io/ioutil"
	"strconv"
)

type Message struct {
	messageType string
	firstNum int32
	secondNum int32
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

func parseMessage(w http.ResponseWriter, r *http.Request) {
	httpHexStr, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	asciiStr, err := strconv.Unquote(`"` + string(httpHexStr) + `"`)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Parse the input data
	data := []byte(asciiStr)

	// Extract the first byte as a character
	firstChar := rune(data[0])

	// Extract the remaining bytes as int32 values
	var numbers []int32
	for i := 1; i < len(data); i += 4 {
		fmt.Println(data[i : i+4])
		number := int32(binary.BigEndian.Uint32(data[i : i+4]))
		numbers = append(numbers, number)
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
	store.append(message)

	if len(store) < 2 {
		return
	}

	sort.Slice(store, func(i, j int) bool {
		return store[i].firstNum < arr[j].firstNum
	})
}

func handleQuery(m *Message) {

}

func main() {
	http.HandleFunc("/", parseMessage)

	err := http.ListenAndServe("localhost:8000", nil)

	if err != nil {
		log.Fatalln("ListenAndServe:", err)
	}
}