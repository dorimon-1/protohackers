package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"
)

type MessageType uint8

const (
	ERROR           MessageType = 0x10
	PLATE           MessageType = 0x20
	TICKET          MessageType = 0x21
	WANT_HEARTBEAT  MessageType = 0x40
	HEARTBEAT       MessageType = 0x41
	I_AM_CAMERA     MessageType = 0x80
	I_AM_DISPATCHER MessageType = 0x81
)

type ClientType int

const (
	NONE ClientType = iota
	CAMERA
	DISPATCHER
)

type Session struct {
	Conn           net.Conn
	Reader         *bufio.Reader
	KeepAliveRate  time.Duration
	Ctx            context.Context
	Cancel         context.CancelFunc
	ClientType     ClientType
	CameraInfo     *Camera
	DispatcherInfo *Dispatcher
}

func NewSession(conn net.Conn) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		Conn:           conn,
		Reader:         bufio.NewReader(conn),
		KeepAliveRate:  0,
		Ctx:            ctx,
		Cancel:         cancel,
		ClientType:     NONE,
		CameraInfo:     nil,
		DispatcherInfo: nil,
	}
}

type Plate struct {
	PlateNumber string
	Timestamp   uint32
	Cam         *Camera
}

func (p Plate) String() string {
	return fmt.Sprintf("PlateID: %s, Timestamp: %v, CameraInfo: %v", p.PlateNumber, p.Timestamp, p.Cam.String())
}

type Ticket struct {
	PlateNumber string
	Road        uint16
	Mile1       uint16
	Mile2       uint16
	Index1      int
	Timestamp1  uint32
	Timestamp2  uint32
	Index2      int
	Speed       uint16
}

func NewTicket(plate1, plate2 *Plate, pIndex1, pIndex2 int, speed uint16) *Ticket {
	if plate1.Timestamp > plate2.Timestamp {
		plate1, plate2 = plate2, plate1
		pIndex1, pIndex2 = pIndex2, pIndex1

	}
	return &Ticket{
		PlateNumber: plate1.PlateNumber,
		Road:        plate1.Cam.Road,
		Mile1:       plate1.Cam.Mile,
		Index1:      pIndex1,
		Mile2:       plate2.Cam.Mile,
		Index2:      pIndex2,
		Timestamp1:  plate1.Timestamp,
		Timestamp2:  plate2.Timestamp,
		Speed:       speed,
	}

}

type Camera struct {
	Road  uint16
	Mile  uint16
	Limit uint16
}

func (c Camera) String() string {
	return fmt.Sprintf("Road: %v, Mile: %v", c.Road, c.Mile)
}

type Dispatcher struct {
	NumRoads uint8
	Roads    []uint16
}
