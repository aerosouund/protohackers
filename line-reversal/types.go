package main

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
)

type Session struct {
	SessionID            string
	Data                 []byte
	LastAcked            int
	Alive                bool
	LastReceivedPosition int
	Address              *net.UDPAddr
}

func NewSession(id string, addr *net.UDPAddr) *Session {
	return &Session{
		SessionID: id,
		Alive:     true,
		Address:   addr,
	}
}

type SessionStore struct {
	mu       sync.Mutex
	sessions []*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{}
}

type DataMessage struct {
	SessionID string
	Data      string
	Position  int
}

type ConnectMessage struct {
	SessionID string
}

type AckMessage struct {
	SessionID string
	Length    int
}

type CloseMessage struct {
	SessionID string
}

type MessageDispatch struct {
	Message   []byte
	WriteAddr *net.UDPAddr
}

type KeepAlive struct {
	SessionID string
}

type Kill struct {
	SessionID string
}

func newMessageDispatch(m []byte, waddr *net.UDPAddr) MessageDispatch {
	return MessageDispatch{
		Message:   m,
		WriteAddr: waddr,
	}
}

func GetField(m interface{}, fieldName string) (string, error) {
	value := reflect.ValueOf(m)

	if value.Kind() != reflect.Struct {
		return "", fmt.Errorf("data is not a struct")
	}

	fieldValue := value.FieldByName(fieldName)
	if !fieldValue.IsValid() {
		return "", fmt.Errorf("field '%s' does not exist", fieldName)
	}

	return fieldValue.Interface().(string), nil
}

var errInvalidMessage = errors.New("invalid message packet, not going to process")
var errSessionNotFound = errors.New("session not found")
