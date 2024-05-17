package main

import (
	"fmt"
	"log"
	"net"
	"strings"
)

type Request int

const (
	INSERT Request = iota
	RETRIEVE
)

func ReadFromBuffer(buf []byte) string {
	for i := 0; i < len(buf); i++ {
		if buf[i] == 0x00 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func main() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: 3000,
	})
	if err != nil {
		log.Fatalln(err)
	}

	db := make(map[string]string)
	db["version"] = "Ken's Key-Value Store 1.0"

	for {
		buf := make([]byte, 1000)
		_, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
			continue
		}
		msg := ReadFromBuffer(buf)

		requestType := GetRequestType(msg)
		log.Printf("Message from %s: %v\n", addr.String(), msg)

		switch requestType {
		case INSERT:
			key, value := GetKeyValueFromMessage(msg)
			if key == "version" {
				continue
			}

			db[key] = value
			log.Printf("DATABASE UPDATE: %s=%s", key, value)

		case RETRIEVE:
			if value, ok := db[msg]; ok {
				log.Printf("RETRIVE: %s=%s", msg, value)
				_, err := conn.WriteToUDP([]byte(fmt.Sprintf("%s=%s", msg, value)), addr)
				if err != nil {
					log.Println(err)
				}

			} else {
				log.Println("Failed to find value", msg)
			}
		}
	}
}

func GetKeyValueFromMessage(msg string) (string, string) {
	key := ""
	for i := 0; i < len(msg); i++ {
		if msg[i] == '=' {
			value := msg[i+1:]
			return key, value
		}
		key += string(msg[i])
	}
	return key, ""
}

func GetRequestType(msg string) Request {
	if strings.ContainsRune(msg, '=') {
		return INSERT
	}
	return RETRIEVE
}
