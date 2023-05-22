package main

import "net"

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
	RoadNum         int
	CameraPosition1 int
	CameraPosition2 int
	Timestamp1      int
	Timestamp2      int
	Speed           int
}

type Road struct {
	RoadNum      int
	Limit        int
	Observations map[string][]Observation
	Dispatchers  []Dispatcher
}
