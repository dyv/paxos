package paxos

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestClient(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	t.Log("TESTING NEW CLIENT")
	a, err := NewAgent("36807")
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	defer a.Close()
	a.Run()
	c := NewClient()
	addr, err := getAddress()
	if err != nil {
		t.Error("Error getting address:", err)
		return
	}
	c.AddServer(addr, "36807")
	err = c.Connect(c.Servers[0])
	if err != nil {
		t.Error("Error Connecting With Server:", err)
		return
	}
	resp, err := c.Request("Request 1")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	t.Log("Response:", resp)
	if resp != "Request 1" {
		t.Error("Server Failed")
	}
	resp, err = c.Request("Request 2")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	if resp != "Request 1" {
		t.Error("Server Failed")
	}

	t.Log("TESTED NEW CLIENT")
}
