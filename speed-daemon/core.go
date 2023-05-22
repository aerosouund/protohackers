package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"time"
)

func getRoad(roadNum int) (Road, bool) {
	// handle road doesnt exist
	for _, road := range listOfRoads {
		if road.RoadNum == roadNum {
			return road, true
		}
	}
	return Road{}, false
}

func sendHeartBeat(conn net.Conn, interval uint16) {
	if interval == 0 {
		return
	}
	data, err := hex.DecodeString("41") // decode hex string to byte
	if err != nil {
		panic(err)
	}

	for {
		_, err = conn.Write(data)
		if err != nil {
			return
		}
		time.Sleep(time.Duration(interval/10) * time.Second)
	}
}

func checkExceededSpeed(limit int, observation Observation, lastObservation Observation) (bool, int) {
	speed := (observation.CameraDistance - lastObservation.CameraDistance) / (observation.Timestamp - lastObservation.Timestamp)
	if speed > limit {
		return true, speed
	}
	return false, speed
}

func handleCamera(conn net.Conn) {
	fmt.Println("Handling a camera")
	buffer := make([]byte, 32)

	// Read bytes from the connection into the buffer
	_, err := conn.Read(buffer)
	if err != nil {
		return
	}

	num := int(binary.BigEndian.Uint16(buffer[0:2]))
	dist := int(binary.BigEndian.Uint16(buffer[2:4]))
	limit := int(binary.BigEndian.Uint16(buffer[4:6]))
	fmt.Println(num, dist, limit)
	fmt.Println("all the rest ", buffer[6:])
	var r Road
	found := false

	// check if road is in the list of roads
	for i, _ := range listOfRoads {
		if listOfRoads[i].RoadNum == num {
			r = listOfRoads[i]
			found = true
		}
	}
	if !found {
		r = Road{
			RoadNum:      num,
			Limit:        limit,
			Observations: map[string][]Observation{},
			Dispatchers:  []Dispatcher{},
		}
		listOfRoads = append(listOfRoads, r)
	}

	requestedHeartbeat := false

	for {
		messageType, err := readBytesFromConn(conn, 1)
		fmt.Println(messageType)
		if err != nil {
			break
		}

		if messageType[0] == 0x40 && !requestedHeartbeat {
			fmt.Println("handling a heartbeat")

			buf, _ := readBytesFromConn(conn, 4)
			interval := binary.BigEndian.Uint16(buf[0:4])
			requestedHeartbeat = true
			go sendHeartBeat(conn, interval)
		}

		if messageType[0] == 0x20 {
			fmt.Println("handling an observation")
			strLen := make([]byte, 1)
			_, err := conn.Read(strLen)
			if err != nil && err != io.EOF {
				fmt.Println(err)
			}
			fmt.Printf("going to parse %d bytes", strLen)
			buf, _ := readBytesFromConn(conn, int(strLen[0]+4))
			observation := Observation{
				Plate:          string(buf[:strLen[0]]),
				Timestamp:      int(binary.BigEndian.Uint32(buf[strLen[0] : strLen[0]+4])),
				CameraDistance: dist,
				RoadNum:        num,
			}
			go handleObservation(r, observation)
		}
	}
}

func readBytesFromConn(conn net.Conn, n int) ([]byte, error) {
	buffer := make([]byte, n, n)
	bytesRead := 0

	for bytesRead < n {
		numBytesRead, err := conn.Read(buffer[bytesRead:])
		if err != nil {
			return nil, err
		}
		bytesRead += numBytesRead
		if bytesRead >= n {
			break
		}
	}

	return buffer, nil
}

func getLastObservation(r Road, o Observation) Observation {
	// may have bugs and missing edge cases
	lastObs := Observation{Timestamp: 0}
	if len(r.Observations[o.Plate]) == 1 {
		lastObs = r.Observations[o.Plate][0]
	}
	for i, _ := range r.Observations[o.Plate] {
		if r.Observations[o.Plate][i].Timestamp < o.Timestamp && r.Observations[o.Plate][i].Timestamp > lastObs.Timestamp {
			lastObs = r.Observations[o.Plate][i]
		}
	}
	return lastObs
}

func insertObservation(r Road, o Observation) {
	// may have bugs and missing edge cases
	if len(r.Observations[o.Plate]) == 1 {
		//handle length of 1

	}
	for i, _ := range r.Observations[o.Plate] {
		if i+1 > len(r.Observations[o.Plate]) {
			r.Observations[o.Plate] = append(r.Observations[o.Plate], o)
			// insert at the end
		}
		if r.Observations[o.Plate][i].Timestamp <= o.Timestamp && r.Observations[o.Plate][i+1].Timestamp > o.Timestamp {
			var newObservations []Observation
			copy(r.Observations[o.Plate][:i], newObservations)
			newObservations[i] = o
			copy(r.Observations[o.Plate][i+1:], newObservations)

			r.Observations[o.Plate] = newObservations

			// insert at that index
		}
	}
}

func handleObservation(r Road, o Observation) {
	fmt.Println("handling observation for", o.Plate)
	_, ok := r.Observations[o.Plate]
	if ok {
		// get last observation
		lastObservation := getLastObservation(r, o)
		exceeded, speed := checkExceededSpeed(r.Limit, o, lastObservation)
		insertObservation(r, o)

		if exceeded {
			createTicket(speed, o, lastObservation)
		}

	} else {
		r.Observations[o.Plate] = []Observation{}
		r.Observations[o.Plate] = append(r.Observations[o.Plate], o)
	}
}

func createTicket(speed int, o Observation, lastObs Observation) {
	t := Ticket{
		Plate:           o.Plate,
		RoadNum:         o.RoadNum,
		CameraPosition1: lastObs.CameraDistance,
		CameraPosition2: o.CameraDistance,
		Timestamp1:      lastObs.Timestamp,
		Timestamp2:      o.Timestamp,
		Speed:           speed,
	}
	r, _ := getRoad(o.RoadNum)

	// if there isnt dispatchers
	if len(r.Dispatchers) == 0 {
		unsentTickets = append(unsentTickets, t)
	}
	// if road has no dispatchers {

	//}
	d := r.Dispatchers[0]

	// encode the ticket to bytes before
	st := serializeTicket(t)
	_, err := d.Conn.Write(st)
	if err != nil {
		panic(err)
	}

}

func serializeTicket(t Ticket) []byte {
	// order: Ticket{plate: "UN1X", road: 123, mile1: 8, timestamp1: 0, mile2: 9, timestamp2: 45, speed: 8000}
	// will panic because of zero length
	var serializedTicket []byte
	serializedTicket = append(serializedTicket, []byte(t.Plate)...)
	binary.BigEndian.PutUint16(serializedTicket, uint16(t.RoadNum))
	binary.BigEndian.PutUint16(serializedTicket, uint16(t.CameraPosition1))
	binary.BigEndian.PutUint32(serializedTicket, uint32(t.Timestamp1))
	binary.BigEndian.PutUint16(serializedTicket, uint16(t.CameraPosition2))
	binary.BigEndian.PutUint32(serializedTicket, uint32(t.Timestamp2))
	binary.BigEndian.PutUint16(serializedTicket, uint16(t.Speed))

	return serializedTicket
}

func handleDispatcher(conn net.Conn) {
	fmt.Println("Handling a dispatcher")
	buffer := make([]byte, 32)
	_, err := conn.Read(buffer)
	if err != nil {
		panic(err)
	}

	numRoads := int(buffer[0])
	roads := []int{}
	for i := 1; i < numRoads-2; i += 2 {
		r := int(binary.BigEndian.Uint16(buffer[i : i+2]))
		roads = append(roads, r)
	}
	var requestedHeartbeat bool

	d := Dispatcher{
		Conn:  conn,
		Roads: roads,
	}

	for i, t := range unsentTickets {
		for _, r := range d.Roads {
			if t.RoadNum == r {
				unsentTickets = append(unsentTickets[:i], unsentTickets[i+1:]...)
				// encode to bytes first
				st := serializeTicket(t)
				_, err := conn.Write(st)
				if err != nil {
					panic(err)
				}
			}
		}
	}

	for roadNum, _ := range d.Roads {
		r, _ := getRoad(roadNum)
		r.Dispatchers = append(r.Dispatchers, d)
	}

	for {
		messageType := make([]byte, 24)

		// Read bytes from the connection into the buffer
		_, err := conn.Read(messageType)
		if err != nil {
			return
		}
		if messageType[0] == 0x40 && !requestedHeartbeat {
			interval := binary.BigEndian.Uint16(messageType[3:5])
			requestedHeartbeat = true
			go sendHeartBeat(conn, interval)
		}
	}
}

func checkTicketSentToday() {

}

// function that sends heartbeat

/*

   func handleDispatcher(conn){
   	numroads := int(bytesOfNumRoadsField)
   	roads := []int(bytesOfRoadsField)
   	var requestedHeartBeat bool

   	d := Dispatcher {
   		conn
   		roads
   	}

   	for ticket in unsentTickets {
   		if ticket.road in d.roads {
   			t := ticket
   			unsentTickets.pop(ticket)
   			fmt.Fprintf(conn, ticket)
   		}
   	}

   	for roadNum in roads {
   		r := getRoad(roadNum)
   		r.dispatchers.append(d)

   	if wantHeartBeat & !requestedHeartBeat {
   		requestedHeartBeat = true
   		sendHeartBeat()
   	}
   }

   // handle tickets in the same day

   func handleObservation(road, observation) {
   	if observation.car in road.observations.keys {
   		// get last observation
   		exceeded := checkExceededSpeed(speedLimit, observation, lastObservation)
   		insertObservation(road, observation)

   		if exceeded {
   			createTicket(observation, lastObservation)
   		}

   	} else {
   		road.observations[observation.car] = []
   		road.observations[observation.car].append(observation)
   	}
   }

   func checkExceededSpeed(limit int, observation Observation, lastObservation Observation) {
   	speed := (observation.cameraDist - lastObservation.cameraDist) / (observation.timestamp - lastObservation.timestamp)
   	if speed > limit {
   		return true, speed
   	}
   	return false, speed
   }

   func createTicket(speed, observation Observation, lastObservation Observation) {
   	t := Ticket{
   		car observation.Car
   		road observation.Road
   		cameraPosition1 lastObservation.cameraDist
   		cameraPosition2 observation.cameraDist
   		timestamp1 lastObservation.timestamp
   		timestamp2 observation.timestamp
   		speed speed
   	}
   	r := getRoad(observation.RoadId)
   	// if road has no dispatchers {
   		upsentTickets.append(t)
   	}
   	d := r.dispatchers[0]
   	fmt.Fprint(d.conn, ticket)
   }

*/
