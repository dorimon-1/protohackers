package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
)

const (
	WELCOME_MESSAGE = "Welcome to budgetchat! What shall I call you?"
	ERROR_MESSAGE   = "Invalid Username - Must consist only alphabetical chars and numbers"
	BLUE_COLOR      = "\033[34m"
	RESET_COLOR     = "\033[0m"
)

type Session struct {
	Id          int
	Username    string
	Conn        net.Conn
	MsgChan     chan Message
	ConnectChan chan int
	QuitChan    chan int
}

func main() {
	ln, err := net.Listen("tcp", ":3000")
	if err != nil {
		panic(err)
	}
	msgChan := make(chan Message)
	sessions := make([]*Session, 0)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		connectChan := make(chan int)
		quitChan := make(chan int)

		session := NewSession(conn, len(sessions), msgChan, connectChan, quitChan)
		sessions = append(sessions, session)

		go MessageListener(&sessions, msgChan, connectChan, quitChan)
		go session.HandleConnection(msgChan)

	}
}

func MessageListener(sessions *[]*Session, msgChan chan Message, connectChan chan int, quitChan chan int) {
	for {
		select {
		case msg := <-msgChan:
			log.Println("Received MSG from ", msg.Sender.Username)
			HandleMessage(*sessions, &msg)
		case connectId := <-connectChan:
			log.Println("Received Connect MSG", (*sessions)[connectId].Username)
			SendConnectMessage((*sessions)[connectId], *sessions)
		case quitId := <-quitChan:
			log.Println("Received Quit MSG", (*sessions)[quitId].Username)
			SendQuitMessage((*sessions)[quitId], *sessions)
			(*sessions)[quitId] = nil
		}
	}
}

func BroadcastMessage(s *Session, sessions []*Session, msg string) {
	for i := 0; i < len(sessions); i++ {
		if sessions[i] != nil {
			if i != s.Id && sessions[i].Username != "" {
				SendLine(sessions[i].Conn, msg)
			}
		}
	}
}

func SendLine(conn net.Conn, msg string) error {
	log.Printf("Sending: %s%s%s TO %s", BLUE_COLOR, msg, RESET_COLOR, conn.RemoteAddr().String())
	data := []byte(fmt.Sprintf("%s\n", msg))
	_, err := conn.Write(data)
	return err
}

func SendQuitMessage(s *Session, sessions []*Session) {
	BroadcastMessage(s, sessions, fmt.Sprintf("* %s has left the room", s.Username))
}

func SendConnectMessage(s *Session, sessions []*Session) {
	usernamesString := ""
	for i := 0; i < len(sessions); i++ {
		if sessions[i] != nil {
			if i != s.Id && sessions[i].Username != "" {
				usernamesString = fmt.Sprintf("%s%s, ", usernamesString, sessions[i].Username)
			}
		}
	}
	connectMessage := fmt.Sprintf("* The room contains: %s", strings.TrimSuffix(usernamesString, ", "))
	SendLine(s.Conn, connectMessage)

	BroadcastMessage(s, sessions, fmt.Sprintf("* %s has entered the room", s.Username))
}

func HandleMessage(sessions []*Session, msg *Message) {
	BroadcastMessage(msg.Sender, sessions, string(msg.Msg))
}

func NewSession(conn net.Conn, id int, msgChan chan Message, connectChan chan int, quitChan chan int) *Session {
	return &Session{
		Conn:        conn,
		Id:          id,
		MsgChan:     msgChan,
		ConnectChan: connectChan,
		QuitChan:    quitChan,
	}
}

type Message struct {
	Msg    []byte
	Sender *Session
}

func NewMessage(sender *Session, msg []byte) *Message {
	return &Message{Sender: sender, Msg: msg}
}
func (s *Session) HandleConnection(msgChan chan Message) {
	defer func() {
		log.Println("Closing connection from: ", s.Conn.RemoteAddr().String())
		if s.Username != "" {
			s.QuitChan <- s.Id
		}
		s.Conn.Close()
	}()
	log.Println("New connection from: ", s.Conn.RemoteAddr().String())
	SendLine(s.Conn, WELCOME_MESSAGE)

	reader := bufio.NewReader(s.Conn)
	line, _, err := reader.ReadLine()
	if err != nil {
		log.Println("Couldn't read line: ", err)
		return
	}

	username, err := verifyUsername(line)
	if err != nil {
		log.Println("Bad Username")
		SendLine(s.Conn, ERROR_MESSAGE)
		return
	}
	s.Username = username

	log.Printf("%s has set his name to %s", s.Conn.RemoteAddr().String(), username)

	s.ConnectChan <- s.Id

	for {
		message, _, err := reader.ReadLine()
		if err != nil {
			return
		}
		s.SendChatMessage(string(message))

	}
}

func (s *Session) SendChatMessage(msg string) {
	msgStr := fmt.Sprintf("[%s] %s", s.Username, msg)
	message := NewMessage(s, []byte(msgStr))
	log.Println(msgStr)
	s.MsgChan <- *message
}

func verifyUsername(username []byte) (string, error) {
	for _, char := range username {
		if char >= 65 && char <= 90 || char >= 97 && char <= 122 || char >= 48 && char <= 57 {
			continue
		}
		return "", errors.New("Invalid Charachter")
	}
	return string(username), nil
}
