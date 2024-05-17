package main

import "sync"

type Database struct {
	Detections        map[string][]Plate
	Dispatchers       map[uint16]*Session
	Tickets           map[string][]uint16
	LostTicketsChan   chan *Ticket
	NewDispatcherChan chan *Session
	PlateChan         chan *Plate
}

func NewDatabase() *Database {
	return &Database{
		Detections:        make(map[string][]Plate),
		Dispatchers:       make(map[uint16]*Session),
		Tickets:           make(map[string][]uint16),
		LostTicketsChan:   make(chan *Ticket),
		NewDispatcherChan: make(chan *Session),
		PlateChan:         make(chan *Plate),
	}
}

var db *Database = nil
var mutex sync.Mutex

func RegisterDispatcher(session *Session) {
	mutex.Lock()
	defer mutex.Unlock()

	for i := 0; i < int(session.DispatcherInfo.NumRoads); i++ {
		Db().Dispatchers[session.DispatcherInfo.Roads[i]] = session
	}
	Db().NewDispatcherChan <- session
}

func InsertPlate(plate Plate) {
	mutex.Lock()
	defer mutex.Unlock()

	plates := Db().Detections[plate.PlateNumber]
	if plates == nil {
		plates = make([]Plate, 0)
	}

	plates = append(plates, plate)
	Db().Detections[plate.PlateNumber] = plates
}

func DeleteTicket(plateNumber string, ticketIndex int) {
	mutex.Lock()
	defer mutex.Unlock()

	detections := Db().Detections[plateNumber]
	newDetections := make([]Plate, len(detections)-1)
	copy(newDetections, detections[:ticketIndex])
	copy(newDetections[ticketIndex:], detections[ticketIndex+1:])
}

func InsertTicket(ticket *Ticket) {
	mutex.Lock()
	defer mutex.Unlock()

	days := calculateDays(ticket.Timestamp1, ticket.Timestamp2)
	tickets := Db().Tickets[ticket.PlateNumber]
	tickets = append(tickets, days...)
	Db().Tickets[ticket.PlateNumber] = tickets
}

func GetSessionByRoad(road uint16) *Session {
	mutex.Lock()
	defer mutex.Unlock()
	return Db().Dispatchers[road]
}

func GetPlates(plateNumber string) []Plate {
	mutex.Lock()
	defer mutex.Unlock()

	return Db().Detections[plateNumber]
}

func GetTickets(plateNumber string) []uint16 {
	mutex.Lock()
	defer mutex.Unlock()

	return Db().Tickets[plateNumber]
}

func Db() *Database {
	if db == nil {
		db = NewDatabase()
	}

	return db
}
