package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Message struct {
	messageType string
	firstNum    int32
	secondNum   int32
}

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
		panic(err)
	}
	b := string(httpHexStr)

	fmt.Printf("The body coming from protohackers: %s\n", b)
}

func main() {
	http.HandleFunc("/", parseMessage)
	fmt.Println("Server starting")

	err := http.ListenAndServe(":8000", nil)

	if err != nil {
		log.Fatalln("ListenAndServe:", err)
	}
}
