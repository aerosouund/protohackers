package main

import (
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

func validateMessage(message string) error {
	messageFields := strings.Split(message, "/")
	log.Printf("split %v into %d fields \n", message, len(messageFields))
	messageFields = messageFields[:len(messageFields)-1]
	if len(messageFields) <= 1 {
		return errInvalidMessage
	}

	if string(message[len(message)-1]) != "/" {
		return errInvalidMessage
	}

	switch messageFields[1] {
	case "connect":
		if len(messageFields) > 3 {
			return errInvalidMessage
		}
	case "ack":
		if len(messageFields) > 4 {
			return errInvalidMessage
		}
	case "data":
		if len(messageFields) > 6 {
			remainingString := strings.Join(messageFields[5:], "/")
			if string(remainingString[0]) == "/" {
				return errInvalidMessage
			}
			for i, _ := range remainingString {
				if i != 0|len(remainingString) && string(remainingString[i]) == "/" {
					if string(remainingString[i-1]) != "\\" {
						return errInvalidMessage
					}
				}
			}
		}
		if len(messageFields) < 5 {
			return errInvalidMessage
		}
	case "close":
		if len(messageFields) > 3 {
			return errInvalidMessage
		}
	default:
		return errInvalidMessage
	}
	return nil
}

func parseMessage(buffer []byte) (interface{}, error) {
	messageContent := string(buffer)

	err := validateMessage(messageContent)
	if err == nil {
		log.Printf("message content received: %v\n", messageContent)
		messageFields := strings.Split(messageContent, "/")

		switch messageFields[1] {
		case "connect":
			return ConnectMessage{
				SessionID: messageFields[2],
			}, nil
		case "data":
			p, _ := strconv.Atoi(messageFields[3])
			// if there are several parts in the message after splitting by slash
			var d string

			// check if the message after disassembly had / characters and set the resulting data based on that
			if len(messageFields[4:]) > 2 {
				d = strings.Join(messageFields[4:], "/")
			} else {
				d = messageFields[4]
			}
			return DataMessage{
				SessionID: messageFields[2],
				Position:  p,
				Data:      d,
			}, nil
		case "ack":
			l, _ := strconv.Atoi(messageFields[3])
			return AckMessage{
				SessionID: messageFields[2],
				Length:    l,
			}, nil
		case "close":
			return CloseMessage{
				SessionID: messageFields[2],
			}, nil
		}
	}
	return nil, err
}

func handleMessage(message interface{}, conn net.UDPConn, messageChan chan<- MessageDispatch, writeAddr *net.UDPAddr, eventChan chan<- interface{}) {
	sid, _ := GetField(message, "SessionID")
	s, ok := getSession(sid)

	switch message.(type) {
	case ConnectMessage:
		// if this token doesn't already exist
		if !ok {
			cm := message.(ConnectMessage)
			log.Println("instantiating new session for", cm.SessionID)

			s := NewSession(cm.SessionID, writeAddr)
			SS.sessions = append(SS.sessions, s)
			ack := AckMessage{
				SessionID: s.SessionID,
				Length:    0,
			}
			serializedMessage := serializeMessage(ack)
			md := newMessageDispatch(serializedMessage, writeAddr)
			messageChan <- md
			eventChan <- KeepAlive{SessionID: cm.SessionID}
		}

	case DataMessage:
		// message for non existing session
		if !ok {
			close := CloseMessage{
				SessionID: sid,
			}
			serializedMessage := serializeMessage(close)
			md := newMessageDispatch(serializedMessage, writeAddr)
			messageChan <- md
			return
		}

		dm := message.(DataMessage)

		// don't process the message if its trying to override previously sent data
		if dm.Position <= s.LastReceivedPosition && len(s.Data) != 0 {
			return
		}

		s.LastReceivedPosition = dm.Position
		appendSessionData(s, dm)

		ack := AckMessage{
			SessionID: s.SessionID,
			Length:    len(s.Data),
		}

		resp := DataMessage{
			SessionID: s.SessionID,
			Position:  dm.Position,
			Data:      reverse(strings.ReplaceAll(dm.Data, "\n", "")),
		}

		// serialize and send the ack and the response string
		serializedMessage := serializeMessage(ack)
		md := newMessageDispatch(serializedMessage, writeAddr)
		messageChan <- md

		serializedResponse := serializeMessage(resp)
		md = newMessageDispatch(serializedResponse, writeAddr)
		messageChan <- md

		// send a keep alive message
		eventChan <- KeepAlive{SessionID: dm.SessionID}
		go startRetransmitter(s, messageChan)

	case AckMessage:
		// message for non existing session
		if !ok {
			close := CloseMessage{
				SessionID: sid,
			}
			serializedMessage := serializeMessage(close)
			md := newMessageDispatch(serializedMessage, writeAddr)
			messageChan <- md
			return
		}
		am := message.(AckMessage)
		log.Printf("received an ack %v and the data length is %d\n", am, len(s.Data))
		if am.Length > len(s.Data) {
			// client is acknowledging data that doesnt exist
			close := CloseMessage{
				SessionID: sid,
			}
			serializedMessage := serializeMessage(close)
			md := newMessageDispatch(serializedMessage, writeAddr)
			messageChan <- md
			return
		}
		s.LastAcked = am.Length
		log.Println("last acked is now", s.LastAcked)

	case CloseMessage:
		// message for non existing session
		if !ok {
			return
		}
		close := CloseMessage{
			SessionID: sid,
		}
		serializedMessage := serializeMessage(close)
		md := newMessageDispatch(serializedMessage, writeAddr)

		eventChan <- Kill{SessionID: sid}
		messageChan <- md
		err := SS.deleteSession(s.SessionID)

		if err != nil {
			log.Println(err.Error())
		}
	}
}

func startRetransmitter(s *Session, messageChan chan<- MessageDispatch) {
	for s.LastAcked < len(s.Data) && s.Alive {
		m := DataMessage{
			SessionID: s.SessionID,
			Data:      reverse(string(s.Data[s.LastAcked:])),
			Position:  s.LastReceivedPosition,
		}
		serializedMessage := serializeMessage(m)
		md := newMessageDispatch(serializedMessage, s.Address)
		timer := time.NewTimer(3 * time.Second)
		<-timer.C
		messageChan <- md
	}
}

func startWriter(messageChan <-chan MessageDispatch, conn *net.UDPConn) {
	for {
		m := <-messageChan
		log.Println("sending", string(m.Message))
		_, err := conn.WriteToUDP(m.Message, m.WriteAddr)
		if err != nil {
			log.Printf("Error sending UDP response: %s\n", err)
			return
		}
	}
}

func manageSessionLifecycle(eventChan chan interface{}) {
	var timer *time.Timer
	timerActive := false

	for {
		e := <-eventChan
		sid, _ := GetField(e, "SessionID")
		s, _ := getSession(sid)
		switch e.(type) {
		case KeepAlive:
			if timerActive {
				timer.Stop()
			}

			timer = time.NewTimer(60 * time.Second)
			timerActive = true
		case Kill:
			s.Alive = false
			SS.deleteSession(s.SessionID)
			timerActive = false
		}

		if timerActive {
			go func() {
				<-timer.C

				s.Alive = false
				SS.deleteSession(s.SessionID)
				timerActive = false
			}()
		}
	}
}

func serializeMessage(message interface{}) []byte {
	sid, _ := GetField(message, "SessionID")
	switch message.(type) {
	case ConnectMessage:
		messageString := "/connect/" + sid + "/"
		return []byte(messageString)
	case DataMessage:
		data, _ := GetField(message, "Data") // strip the data from slashes
		messageString := "/data/" + sid + "/" + strconv.Itoa(message.(DataMessage).Position) + "/" + data + "/"
		return []byte(messageString)
	case AckMessage:
		s, _ := getSession(sid)
		messageString := "/ack/" + sid + "/" + strconv.Itoa(len(s.Data)) + "/"
		return []byte(messageString)
	case CloseMessage:
		messageString := "/close/" + sid + "/"
		return []byte(messageString)
	default:
		log.Println("unsupported message")
		return []byte{}
	}
}

func (ss *SessionStore) deleteSession(id string) error {
	ss.mu.Lock()
	for i := range ss.sessions {
		if ss.sessions[i].SessionID == id {
			ss.sessions = append(ss.sessions[:i], ss.sessions[i:]...)
			ss.mu.Unlock()
			return nil
		}
	}
	ss.mu.Unlock()
	return errSessionNotFound
}

func appendSessionData(s *Session, m DataMessage) {
	s.Data = append(s.Data, []byte(m.Data)...)
}

func getSession(id string) (*Session, bool) {
	for _, s := range SS.sessions {
		if s.SessionID == id {
			return s, true
		}
	}
	return nil, false
}

func reverse(str string) string {
	byte_str := []rune(str)
	for i, j := 0, len(byte_str)-1; i < j; i, j = i+1, j-1 {
		byte_str[i], byte_str[j] = byte_str[j], byte_str[i]
	}

	// if string(byte_str[0]) == "\n" {
	// 	byte_str = byte_str[1:]
	// }
	byte_str = append(byte_str, '\n')

	return string(byte_str)
}
