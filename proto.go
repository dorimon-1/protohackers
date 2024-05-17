package protohackers

import (
	"fmt"
	"net"
)

func NewProtoListener(handleConnection func(conn net.Conn)) {
	ln, err := net.Listen("tcp", ":3000")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handleConnection(conn)
	}
}
