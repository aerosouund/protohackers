package main

import (
	"net"
)

type Observation struct {
	Plate          string
	RoadNum        int
	CameraDistance int
	Timestamp      int
}

type Dispatcher struct {
	Conn  net.Conn
	Roads []int
}

type Ticket struct {
	Plate           string
	RoadNum         uint16
	CameraPosition1 uint16
	CameraPosition2 uint16
	Timestamp1      uint32
	Timestamp2      uint32
	Speed           uint16
}

type Road struct {
	RoadNum      int
	Limit        int
	Observations map[string][]Observation
	Dispatchers  []*Dispatcher
}
