package main

import (
	"net"
	"sync"
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

type TicketTracker struct {
	mu          sync.RWMutex
	SentTickets map[string]int
}

func NewTracker() *TicketTracker {
	return &TicketTracker{
		mu:          sync.RWMutex{},
		SentTickets: make(map[string]int),
	}
}

type Road struct {
	RoadNum      int
	Limit        int
	Observations map[string][]Observation // create a way to sort a list of obervation
	Dispatchers  []*Dispatcher
}

type SortedObservations []Observation

func (so SortedObservations) Len() int {
	return len(so)
}

func (so SortedObservations) Less(i, j int) bool {
	return so[j].Timestamp > so[i].Timestamp
}

func (so SortedObservations) Swap(i, j int) {
	so[i], so[j] = so[j], so[i]
}
