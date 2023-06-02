package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/exp/constraints"
)

func getRoad(roadNum int) (*Road, bool) {
	// handle road doesnt exist
	for _, road := range listOfRoads {
		if road.RoadNum == roadNum {
			fmt.Println("found road")
			return road, true
		}
	}
	return &Road{}, false
}

func sendHeartBeat[T constraints.Unsigned](conn net.Conn, interval T) {
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

func checkExceededSpeed(limit int, o Observation, lastObs Observation) (bool, float64) {

	if o.Timestamp < lastObs.Timestamp {
		d := float64(lastObs.CameraDistance - o.CameraDistance)
		t := float64(lastObs.Timestamp-o.Timestamp) / 60 / 60
		v := d / t
		if v > float64(limit) {
			return true, v
		}
		return false, v

	} else {
		d := float64(o.CameraDistance - lastObs.CameraDistance)
		t := float64(o.Timestamp-lastObs.Timestamp) / 60 / 60
		v := d / t
		if v > float64(limit) {
			return true, v
		}
		return false, v
	}
}

func handleCamera(conn net.Conn) {
	fmt.Println("Handling a camera")
	buffer, err := readBytesFromConn(conn, 6)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
	}

	num := int(binary.BigEndian.Uint16(buffer[0:2]))
	dist := int(binary.BigEndian.Uint16(buffer[2:4]))
	limit := int(binary.BigEndian.Uint16(buffer[4:6]))
	fmt.Println("handling camera with values", num, dist, limit)

	// check if road is in the list of roads
	r, found := getRoad(num)
	if !found {
		r := &Road{
			RoadNum:      num,
			Limit:        limit,
			Observations: make(map[string][]Observation),
			Dispatchers:  make([]Dispatcher, 0),
		}
		listOfRoads = append(listOfRoads, r)
		fmt.Println("appended road to list of roads")
	}

	// fmt.Println("RoaDDD", r.RoadNum, &conn)

	// requestedHeartbeat := false

	for {
		messageType, err := readBytesFromConn(conn, 1)

		if err != nil {
			break
		}

		switch messageType[0] {
		case 0x40:
			// check heartbeat request
			fmt.Println("handling a heartbeat")

			buf, _ := readBytesFromConn(conn, 4)
			interval := binary.BigEndian.Uint16(buf[0:4])
			// requestedHeartbeat = true
			go sendHeartBeat(conn, interval)

		case 0x20:
			fmt.Println("handling an observation")
			strLen := make([]byte, 1)
			_, err := conn.Read(strLen)
			if err != nil && err != io.EOF {
				fmt.Println(err)
			}

			buf, _ := readBytesFromConn(conn, int(strLen[0]+4))
			o := Observation{
				Plate:          string(buf[:strLen[0]]),
				Timestamp:      int(binary.BigEndian.Uint32(buf[strLen[0] : strLen[0]+4])),
				CameraDistance: dist,
				RoadNum:        num,
			}
			fmt.Println("observation timestamp", o.Timestamp)
			go handleObservation(r, o)

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

func getLastObservation(r *Road, o Observation) Observation {
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

func insertObservation(r *Road, o Observation) {
	// may have bugs and missing edge cases
	if len(r.Observations[o.Plate]) == 1 {
		if r.Observations[o.Plate][0].Timestamp > o.Timestamp {
			newObservations := []Observation{o, r.Observations[o.Plate][0]}
			r.Observations[o.Plate] = newObservations
			fmt.Println("observation inserted before")
			return
		}
		r.Observations[o.Plate] = append(r.Observations[o.Plate], o)
		fmt.Println("observation inserted after")
		return

	}
	for i, _ := range r.Observations[o.Plate] {
		if i+1 > len(r.Observations[o.Plate]) {
			// insert at the end
			r.Observations[o.Plate] = append(r.Observations[o.Plate], o)
		}
		if r.Observations[o.Plate][i].Timestamp <= o.Timestamp && r.Observations[o.Plate][i+1].Timestamp > o.Timestamp {
			// insert at that index
			var newObservations []Observation
			copy(r.Observations[o.Plate][:i], newObservations)
			newObservations[i] = o
			copy(r.Observations[o.Plate][i+1:], newObservations)

			r.Observations[o.Plate] = newObservations
			return
		}
	}
}

func handleObservation(r *Road, o Observation) {
	fmt.Println("handling observation for", o.Plate)
	fmt.Println("workign with road", r)
	_, ok := r.Observations[o.Plate]
	if ok {
		// get last observation
		lastObservation := getLastObservation(r, o)
		exceeded, speed := checkExceededSpeed(r.Limit, o, lastObservation)
		insertObservation(r, o)

		if exceeded {
			fmt.Println("car exceeded speed, creating ticket")
			createTicket(int(speed), o, lastObservation)
		}

	} else {
		if r.Observations == nil {
			fmt.Println("The map is nil!!!")
		}
		r.Observations[o.Plate] = make([]Observation, 0)
		r.Observations[o.Plate] = append(r.Observations[o.Plate], o)
		fmt.Println("created plate key")
	}
}

func createTicket(speed int, o Observation, lastObs Observation) {
	fmt.Println("creating ticket")
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
	fmt.Println("Creating ticket for road", r.RoadNum)

	// if there isnt dispatchers
	if len(r.Dispatchers) == 0 {
		unsentTickets = append(unsentTickets, t)
	}

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
	var arrayLength int = len(t.Plate) + 16
	var serializedTicket = make([]byte, arrayLength, arrayLength)
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

	numRoadsBytes, err := readBytesFromConn(conn, 1)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
	}

	numRoads := int(numRoadsBytes[0])

	roadsBytes, err := readBytesFromConn(conn, numRoads*2)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
	}

	roads := make([]int, len(roadsBytes)/2)

	for i := 0; i < len(roadsBytes)-1; i++ {
		roads[i] = int(binary.BigEndian.Uint16(roadsBytes[i*2 : i*2+2]))
	}

	fmt.Println("handling a dispatcher with values", numRoads, roads)
	var requestedHeartbeat bool

	d := Dispatcher{
		Conn:  conn,
		Roads: roads,
	}

	// for i, t := range unsentTickets {
	// 	for _, r := range d.Roads {
	// 		if t.RoadNum == r {
	// 			unsentTickets = append(unsentTickets[:i], unsentTickets[i+1:]...)
	// 			// encode to bytes first
	// 			st := serializeTicket(t)
	// 			_, err := conn.Write(st)
	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 		}
	// 	}
	// }

	for _, roadNum := range d.Roads {
		r, _ := getRoad(roadNum)
		r.Dispatchers = append(r.Dispatchers, d)
		fmt.Println("appended dispatcher to road number", r.RoadNum)
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
