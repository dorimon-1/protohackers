package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"

	"github.com/dorimon-1/protohackers"
)

type Response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

func newResponse(method string, isPrime bool) *Response {
	return &Response{Method: method, Prime: isPrime}
}

type request struct {
	Method string   `json:"method"`
	Number *float64 `json:"number"`
}

func (r *request) String() string {
	return fmt.Sprintf("method: %s, number: %f", r.Method, *r.Number)
}

const MAX_REQUESTS = 5000

func main() {
	protohackers.NewProtoListener(handleConnetions)
}

func handleConnetions(conn net.Conn) {
	defer func() {
		log.Println("Closing connection from:", conn.RemoteAddr().String())
		conn.Close()
	}()

	requests := 0
	for scanner := bufio.NewScanner(conn); scanner.Scan() && requests < MAX_REQUESTS; requests++ {
		data := scanner.Text()

		var req request
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			fmt.Println("failed to unmarshal", err)
			return
		}

		resp, err := validateRequest(req)
		if err != nil {
			log.Println("failed to marshal", err)
			return
		}

		marshledData, err := json.Marshal(*resp)
		if err != nil {
			log.Println("failed to marshal", err)
			return
		}

		marshledData = append(marshledData, byte('\n'))
		conn.Write(marshledData)
	}
	if requests == MAX_REQUESTS {
		log.Println("Too many requests from", conn.RemoteAddr())
	}
}

func validateRequest(req request) (*Response, error) {
	if req.Method != "isPrime" || req.Number == nil {
		return nil, errors.New("bad request")
	}

	isPrime := false
	if float64(int(*req.Number)) == *req.Number {
		isPrime = big.NewInt(int64(*req.Number)).ProbablyPrime(11)
	}
	return newResponse(req.Method, isPrime), nil
}
