package main

import (
	"fmt"
	"io"
	"net"

	"github.com/dorimon-1/protohackers"
)

func main() {
	protohackers.NewProtoListener(handleConnection)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	if _, err := io.Copy(conn, conn); err != nil {
		fmt.Println(err)
	}
}
