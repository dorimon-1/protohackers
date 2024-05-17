package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	str := `{"number":37551482,"method":"isPrime"}`
	var resp Response
	err := json.Unmarshal([]byte(str), &resp)
	if err != nil {
		t.Errorf(err.Error())
	}
	fmt.Println(resp)
}
