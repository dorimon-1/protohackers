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

		if float64(p.Cam.Limit)+0.5 <= averageSpeed {

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

func calculateDay(timestamp uint32) uint16 {
	floatDay := float64(timestamp / 86400)
	return uint16(floatDay)
}

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

func PlateScanner(plateChan chan *Plate) {
	for plate := range plateChan {
		ticket := ScanPlate(plate)
		if ticket == nil {
			continue
		}

		DeleteTicket(ticket.PlateNumber, ticket.Index1)
		DeleteTicket(ticket.PlateNumber, ticket.Index2)

		dispatcherSession := GetSessionByRoad(ticket.Road)
		if dispatcherSession == nil {
			log.Println("Couldn't find dispatcher for road: ", ticket.Road)
			Db().LostTicketsChan <- ticket
			continue
		}

		if err := dispatcherSession.GiveTicket(ticket); err != nil {
			log.Println("error sending ticket: ", err)
			continue
		}

		InsertTicket(ticket)
		log.Println("ticket issued for ", ticket.PlateNumber)
	}
}

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
						if err := newDispatcher.GiveTicket(ticket); err != nil {
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

func (s *Session) GiveTicket(t *Ticket) error {
	buf := CreateString(t.PlateNumber)

	buf = binary.BigEndian.AppendUint16(buf, t.Road)
	buf = binary.BigEndian.AppendUint16(buf, t.Mile1)
	buf = binary.BigEndian.AppendUint32(buf, t.Timestamp1)
	buf = binary.BigEndian.AppendUint16(buf, t.Mile2)
	buf = binary.BigEndian.AppendUint32(buf, t.Timestamp2)
	buf = binary.BigEndian.AppendUint16(buf, t.Speed*100)

	return s.SendMessage(TICKET, buf)
}

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
