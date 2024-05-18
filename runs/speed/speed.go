package main

import (
	"encoding/binary"
	"io"
	"log"
	"math"
	"net"
	"slices"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", ":3000")
	if err != nil {
		log.Fatalln("Failed to start listening: ", err)
	}
	log.Println("Listening for tcp connections on port 3000")

	go HandleLostTickets(Db().LostTicketsChan, Db().NewDispatcherChan)
	go PlateScanner(Db().PlateChan)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Error accepting connection: ", err)
		}

		session := NewSession(conn)

		go session.HandleConnection()
	}
}

// HandleConnection is the main func for each connection
// It reads the first byte from Conn, determines the MessageType and calls the next func accordingly
func (s *Session) HandleConnection() {
	defer func() {
		s.Conn.Close()
		s.Cancel()
	}()

	for {
		firstByte, err := s.Reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return
			}
		}

		msgType := MessageType(firstByte)

		switch msgType {
		case WANT_HEARTBEAT:
			s.logf("Recieved Heartbeat request")
			s.HandleWantHeartBeat()
		case I_AM_CAMERA:
			s.logf("Recieved IAmCamera request")
			s.IAmCamera()
		case I_AM_DISPATCHER:
			s.logf("Recieved IAmDispatcher request")
			s.IAmDispatcher()
		case PLATE:
			s.logf("Recieved Plate request")
			err := s.HandlePlate()
			if err != nil {
				s.logf("Err: %s", err)
			}
		case TICKET, HEARTBEAT, ERROR:
			_ = s.SendError("you cannot send Server -> Client messages")
		}
	}
}

// ScanPlate accepts a *Plate as a paramater and returns a *Ticket
// It scans the given plate for a potential tickets by getting past plates and calculating average speed between those 2 Plates
// it also makes sure it didn't receive a ticket within the same days range.
func ScanPlate(p *Plate) *Ticket {
	log.Println("Started scanning plate: ", p.String())
	plates := GetPlates(p.PlateNumber)
	log.Println(plates)
	var pIndex int
	for i, plate := range plates {
		if p.Cam.Road != plate.Cam.Road || p.Timestamp == plate.Timestamp {
			if p.Timestamp == plate.Timestamp {
				pIndex = i
			}
			log.Println("Skipping road")
			continue
		}

		distance := math.Abs(float64(plate.Cam.Mile) - float64(p.Cam.Mile))

		time := math.Abs(float64(plate.Timestamp) - float64(p.Timestamp))
		time = time / 3600

		averageSpeed := distance / time
		speedLimit := float64(p.Cam.Limit) + 0.5
		if speedLimit <= averageSpeed {

			days := calculateDays(p.Timestamp, plate.Timestamp)
			if DidRecieveTicket(p.PlateNumber, days) {
				log.Println("Already recieved a ticket on one of days: ", days)
				continue
			}
			speed := uint16(math.Round(averageSpeed))
			ticket := NewTicket(p, &plate, pIndex, i, speed)
			log.Println("Created a ticket: ", ticket)
			return ticket
		}
	}
	return nil
}

// calculateDay accepts a single uint32, a unix timestamp
// returns the current day within the timestamp
// a day is calculated by floor(timestamp / 86400)
func calculateDay(timestamp uint32) uint16 {
	floatDay := float64(timestamp / 86400)
	return uint16(floatDay)
}

// calculateDays accepts two uint32 paramaters t1, t2 which are unix timestamps.
// it returns a slice of uint16 which is all the days that are between those 2 timestamps
func calculateDays(t1, t2 uint32) []uint16 {
	if t2 < t1 {
		t2, t1 = t1, t2
	}
	firstDay := calculateDay(t1)
	lastDay := calculateDay(t2)
	days := make([]uint16, lastDay-firstDay+1)

	for i := range days {
		day := firstDay + uint16(i)
		days[i] = day
	}
	return days
}

// PlateScanner is used as goroutine and is started at the beginning of the program
// Every plate that is registered is channeled to this goroutine and is checked whether it should receive a ticket or not
// The reason for a single routine to handle the scans is to avoid double scans when receiving many Plates with the same PlateNumber
func PlateScanner(plateChan chan *Plate) {
	for plate := range plateChan {
		ticket := ScanPlate(plate)
		if ticket == nil {
			continue
		}

		DeletePlate(ticket.PlateNumber, ticket.Index1)
		DeletePlate(ticket.PlateNumber, ticket.Index2)

		dispatcherSession := GetSessionByRoad(ticket.Road)
		if dispatcherSession == nil {
			log.Println("Couldn't find dispatcher for road: ", ticket.Road)
			Db().LostTicketsChan <- ticket
			continue
		}

		if err := dispatcherSession.SendTicket(ticket); err != nil {
			log.Println("error sending ticket: ", err)
			continue
		}

		InsertTicket(ticket)
		log.Println("ticket issued for ", ticket.PlateNumber)
	}
}

// HandlePlate is called whenever a client sends a MessageType PLATE
// Plate is built with a plateNumber string, timestamp uint32
// After inserting the plate into the database it is channeled through the PlateChan for to a PlateScanner
// Client must be a camera otherwise it sends the client an error and closes the connection
func (s *Session) HandlePlate() error {
	if s.ClientType != CAMERA {
		return s.SendError("you are not a camera!")
	}

	plateNumber, err := s.ReadString()
	if err != nil {
		return err
	}

	timestamp, err := s.ReadUint32()
	if err != nil {
		return err
	}

	plate := Plate{
		PlateNumber: plateNumber,
		Timestamp:   timestamp,
		Cam:         s.CameraInfo,
	}

	InsertPlate(plate)
	Db().PlateChan <- &plate
	return nil
}

// HandleLostTickets accepts a chan *Ticket and chan *Session.
// Also when a ticket is lost, it is channeled through ticketChan and is saved to a lostTickets map.
// When a dispatcher registers it is channeled through dispatcherChan to check whether it has a LostTickets it should handle.
// After sending all lost tickets for a specific road, those tickets are removed from the lostTickets
func HandleLostTickets(ticketChan chan *Ticket, dispatcherChan chan *Session) {
	lostTickets := make(map[uint16][]*Ticket)
	for {
		select {
		case ticket := <-ticketChan:
			if _, ok := lostTickets[ticket.Road]; !ok {
				lostTickets[ticket.Road] = make([]*Ticket, 0)
			}
			lostTickets[ticket.Road] = append(lostTickets[ticket.Road], ticket)
			log.Println("Lost ticket has been registered for road: ", ticket.Road)
			log.Println(lostTickets)
		case newDispatcher := <-dispatcherChan:
			log.Println(lostTickets)
			for _, road := range newDispatcher.DispatcherInfo.Roads {
				if tickets, ok := lostTickets[road]; ok {
					log.Println("Looking for a lost ticket for road: ", road)
					for _, ticket := range tickets {
						if DidRecieveTicket(ticket.PlateNumber, calculateDays(ticket.Timestamp1, ticket.Timestamp2)) {
							log.Println("Already recieved a ticket today")
							continue
						}
						if err := newDispatcher.SendTicket(ticket); err != nil {
							newDispatcher.logln("error sending ticket", err)
							continue
						}

						InsertTicket(ticket)
						log.Println("ticket issued for ", ticket.PlateNumber)
					}
				}
				lostTickets[road] = nil
			}
		}
	}
}

// SendTicket accepts a *Ticket as a paramater, writes it into a buffer and sends it out to a fitting Dispatcher
// It returns the returned error from the SendMessage
func (s *Session) SendTicket(t *Ticket) error {
	buf := CreateString(t.PlateNumber)

	buf = binary.BigEndian.AppendUint16(buf, t.Road)
	buf = binary.BigEndian.AppendUint16(buf, t.Mile1)
	buf = binary.BigEndian.AppendUint32(buf, t.Timestamp1)
	buf = binary.BigEndian.AppendUint16(buf, t.Mile2)
	buf = binary.BigEndian.AppendUint32(buf, t.Timestamp2)
	buf = binary.BigEndian.AppendUint16(buf, t.Speed*100)

	return s.SendMessage(TICKET, buf)
}

// HandleWantHeartBeat is called when the MessageType is WANT_HEARTBEAT
// It reads the interval and calculates the KeepAliveRate and calls a HandleHeartBeat in a new goroutine
func (s *Session) HandleWantHeartBeat() {
	if s.KeepAliveRate != 0 {
		if err := s.SendError("Too many keepalives"); err != nil {
			return
		}
	}
	interval, err := s.ReadUint32()
	if err != nil {
		s.logln(err)
		return
	}

	if interval == 0 {
		return
	}

	s.KeepAliveRate = (time.Second * time.Duration(interval)) / 10

	s.logf("Started keepalive routine for each %d", s.KeepAliveRate)

	timer := time.NewTimer(s.KeepAliveRate)
	go s.HandleHeartbeat(timer)
}

// HandleHeartbeat is used as a goroutine to send keepalives to each client.
// Each client determines its own KeepAliveRate when HandlwWantHeartNeat is called
func (s *Session) HandleHeartbeat(timer *time.Timer) {
	for {
		select {
		case <-timer.C:
			err := s.SendMessage(HEARTBEAT, nil)
			if err != nil {
				return
			}

			timer.Reset(s.KeepAliveRate)
		case <-s.Ctx.Done():
			return
		}
	}
}

// IAmCamera called when a client reports itself as a Camera.
// It Sends an error to the client if he already reported itself as a Dispatcher or a Camera
// IAmCamera sent from the client with 3 fields: road uint16, mile uint16, limit uint16
// It is not required to register the camera as we only read.
func (s *Session) IAmCamera() {
	if s.ClientType != NONE {
		_ = s.SendError("Client type is NONE")
		return
	}

	s.ClientType = CAMERA

	road, err := s.ReadUint16()
	if err != nil {
		s.logf("error reading values from CAMERA client")
		return
	}

	mile, err := s.ReadUint16()
	if err != nil {
		s.logf("error reading values from CAMERA client")
		return
	}

	limit, err := s.ReadUint16()
	if err != nil {
		s.logf("error reading values from CAMERA client")
		return
	}

	s.CameraInfo = &Camera{
		Road:  road,
		Mile:  mile,
		Limit: limit,
	}

}

// IAmDispatcher is called when a client reports itself as a Dispatcher.
// It Sends an error to the client if he already reported itself as a Dispatcher or a Camera
// IAmDispatcher is sent from the client with 2 fields: numroads uint8, roads []uint16
// After reading all the fields and performing validations it registers the Dispatcher in the database
func (s *Session) IAmDispatcher() {
	if s.ClientType != NONE {
		_ = s.SendError("Client type is NONE")
		return
	}

	s.ClientType = DISPATCHER

	numOfRoads, err := s.ReadUint8()
	if err != nil {
		s.logf("error reading values from CAMERA client")
		return
	}

	roads := make([]uint16, numOfRoads)
	for i := 0; i < int(numOfRoads); i++ {
		road, err := s.ReadUint16()
		if err != nil {
			s.logf("error reading values from CAMERA client")
			return
		}
		roads[i] = road
	}

	s.DispatcherInfo = &Dispatcher{
		NumRoads: numOfRoads,
		Roads:    roads,
	}

	RegisterDispatcher(s)
	s.logln("registered dispatcher for roads: ", roads)
}

// DidRecieveTicket reports whether the given plateNumber received a ticket in the given days
func DidRecieveTicket(plateNumber string, days []uint16) bool {
	tickets := GetTickets(plateNumber)
	log.Println("Tickets: ", tickets)
	for _, ticket := range tickets {
		if slices.Contains(days, ticket) {
			return true
		}
	}

	return false
}
