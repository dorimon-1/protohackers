package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
)

const (
	CHAT_ADDRESS string = "chat.protohackers.com:16963"
	TONY_ADDRESS string = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"
)

func main() {
	ln, err := net.Listen("tcp", ":3000")
	if err != nil {
		log.Fatalln(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
		}

		server, err := net.Dial("tcp", CHAT_ADDRESS)
		if err != nil {
			conn.Close()
			continue
		}

		go ServerToClient(conn, server)
		go ClientToServer(conn, server)
	}
}

func ServerToClient(conn net.Conn, server net.Conn) {
	reader := bufio.NewReader(server)
	for {
		msg, err := reader.ReadString('\n')
		msg = strings.TrimSuffix(msg, "\n")
		log.Printf("[Server]Message Received: %s", msg)
		switch {
		case err == io.EOF || err == io.ErrUnexpectedEOF:
			log.Println("[Server]Error reading string: ", err)
			return
		case err != nil:
			log.Println("[Server]Error reading string: ", err)
			return
		default:
			err := SendMessage(RewriteMessage(msg), conn)
			if err != nil {
				log.Println("[Server]Error sending message: ", err)
			}
		}
	}
}
func ClientToServer(conn net.Conn, server net.Conn) {
	defer func() {
		server.Close()
		conn.Close()
	}()

	reader := bufio.NewReader(conn)
	for {
		msg, err := reader.ReadString('\n')
		msg = strings.TrimSuffix(msg, "\n")
		log.Printf("[Client] Message Received: %s", msg)
		switch {
		case err == io.EOF || err == io.ErrUnexpectedEOF:
			log.Println("[Client]Error reading string: ", err)
			return
		case err != nil:
			log.Println("[Client]Error reading string: ", err)
			return
		default:
			err := SendMessage(RewriteMessage(msg), server)
			if err != nil {
				log.Println("[Client]Error sending message: ", err)
			}
		}
	}
}

func SendMessage(msg string, conn net.Conn) error {
	msg = msg + string('\n')
	log.Printf("Writing from %s to %s: %v\n", conn.LocalAddr().String(), conn.RemoteAddr().String(), msg)
	_, err := conn.Write([]byte(msg))
	return err
}

func RewriteMessage(msg string) string {
	slicedMsg := strings.Split(msg, " ")
	var caught = false

	for i, subStr := range slicedMsg {
		if isBoguscoin(subStr) {
			slicedMsg[i] = TONY_ADDRESS
			caught = true
		}
	}

	// For optimized messagew write
	if caught {
		return strings.Join(slicedMsg, " ")
	}

	return msg
}

func isBoguscoin(substr string) bool {
	if strlen := len(substr); strlen >= 26 && strlen <= 35 {
		if substr[0] != '7' || !verifyNumericOnly(substr) {
			return false
		}
		return true
	}
	return false
}

func verifyNumericOnly(str string) bool {
	for _, char := range str {
		if char >= 65 && char <= 90 || char >= 97 && char <= 122 || char >= 48 && char <= 57 {
			continue
		}
		return false
	}
	return true
}
