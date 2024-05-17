package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"time"

	"github.com/dorimon-1/protohackers"
)

func main() {
	protohackers.NewProtoListener(HandleConnection)
}

const (
	INSERT = 'I'
	QUERY  = 'Q'
)

type Session struct {
	timestamps []int32
	database   []Price
}

type Price struct {
	Timestamp int32
	Price     int32
}

type QueryRequest struct {
	MinTime int32
	MaxTime int32
}

func NewSession() *Session {
	return &Session{
		timestamps: make([]int32, 0),
		database:   make([]Price, 0),
	}
}
func HandleConnection(conn net.Conn) {
	defer func() {
		log.Println("Closing Connection")
		conn.Close()
	}()
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))

	data := make([]byte, 9)
	session := NewSession()
	for {
		_, err := io.ReadFull(conn, data)
		if err != nil {
			break
		}

		pktbuf := bytes.NewReader(data)

		var msgtype byte
		msgtype, err = pktbuf.ReadByte()
		if err != nil {
			break
		}

		if msgtype == INSERT {
			log.Println("Performing INSERT")
			session.HandleInsert(pktbuf)
		} else if msgtype == QUERY {
			log.Println("Performing QUERY")
			means := session.HandleQuery(pktbuf)
			err := binary.Write(conn, binary.BigEndian, means)
			if err != nil {
				continue
			}
		}
	}
}

func (s *Session) HandleQuery(r *bytes.Reader) int32 {
	var query QueryRequest
	binary.Read(r, binary.BigEndian, &query)

	var sum, count int64

	for _, price := range s.database {
		if price.Timestamp >= query.MinTime && price.Timestamp <= query.MaxTime {
			count++
			sum += int64(price.Price)
		}
	}

	if count == 0 {
		return 0
	}
	return int32(sum / count)
}

func (s *Session) HandleInsert(r *bytes.Reader) {
	var price Price
	binary.Read(r, binary.BigEndian, &price)

	s.database = append(s.database, price)
	log.Printf("TIMESTAMP: %d, PRICE: %d\n", price.Timestamp, price.Price)
}

func ReadSignedInt(r *bytes.Reader) (int32, int32) {

	var (
		firstInt  int32
		secondInt int32
	)
	err := binary.Read(r, binary.BigEndian, &firstInt)
	err = binary.Read(r, binary.BigEndian, &secondInt)
	if err != nil {
		return 0, 0
	}

	return firstInt, secondInt
}
