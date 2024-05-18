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

var (
	db    *Database = nil
	mutex sync.Mutex
	once  sync.Once
)

// RegisterDispatcher accepts a *Session, it adds all the dispatcher's roads to the database
// It also signals the session to NewDispatcherChan which the looks for a potential lost tickets for this specific Dispatcher's road.
func RegisterDispatcher(session *Session) {
	mutex.Lock()
	defer mutex.Unlock()

	for i := 0; i < int(session.DispatcherInfo.NumRoads); i++ {
		Db().Dispatchers[session.DispatcherInfo.Roads[i]] = session
	}
	Db().NewDispatcherChan <- session
}

// InsertPlate accepts a Plate, it inserts it into the db.
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

// DeletePlate accepts a plateNumber string and a ticketIndex int, it removes this specific detection of a plate.
// DeletePlate is used after giving out a ticket to not use the same detection twice.
func DeletePlate(plateNumber string, plateIndex int) {
	mutex.Lock()
	defer mutex.Unlock()

	detections := Db().Detections[plateNumber]
	newDetections := make([]Plate, len(detections)-1)
	copy(newDetections, detections[:plateIndex])
	copy(newDetections[plateIndex:], detections[plateIndex+1:])
}

// InsertTicket accepts a *Ticket, it calcualtes the days of this specific ticket and appends it to the given tickets of the plate.
func InsertTicket(ticket *Ticket) {
	mutex.Lock()
	defer mutex.Unlock()

	days := calculateDays(ticket.Timestamp1, ticket.Timestamp2)
	tickets := Db().Tickets[ticket.PlateNumber]
	tickets = append(tickets, days...)
	Db().Tickets[ticket.PlateNumber] = tickets
}

// GetSessionByRoad accepts a road uint16 and returns a *Session.
// Used to find a dispatcher's session for a specific road.
func GetSessionByRoad(road uint16) *Session {
	mutex.Lock()
	defer mutex.Unlock()
	return Db().Dispatchers[road]
}

// GetPlates accepts a plateNumber string and returns a all detections for this specific plateNumber.
func GetPlates(plateNumber string) []Plate {
	mutex.Lock()
	defer mutex.Unlock()

	return Db().Detections[plateNumber]
}

// GetPlates accepts a plateNumber string and returns a given tickets for the specific plateNumber.
func GetTickets(plateNumber string) []uint16 {
	mutex.Lock()
	defer mutex.Unlock()

	return Db().Tickets[plateNumber]
}

func Db() *Database {
	once.Do(func() {
		db = NewDatabase()
	})
	return db
}
