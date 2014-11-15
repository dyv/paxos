package paxos

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestClientOneRequest(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	t.Log("TESTING NEW CLIENT")
	a, err := NewAgent("36807", false)
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
	resp, err := c.Request("Request1")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	t.Log("Response:", resp)
	if resp != "Request1" {
		t.Error("Server Failed")
	}
	resp, err = c.Request("Request2")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	if resp != "Request1" {
		t.Error("Server Failed")
	}

	t.Log("TESTED NEW CLIENT")
}
func TestClientNewRequest(t *testing.T) {
	//log.SetOutput(ioutil.Discard)
	t.Log("TESTING NEW CLIENT")
	a, err := NewAgent("36820", false)
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
	c.AddServer(addr, "36820")
	err = c.Connect(c.Servers[0])
	if err != nil {
		t.Error("Error Connecting With Server:", err)
		return
	}
	resp, err := c.NewRequest("Request1")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	t.Log("Response:", resp)
	if resp != "Request1" {
		t.Error("Response (0) != Request 1")
		return
	}
	resp, err = c.NewRequest("Request2")
	if err != nil {
		t.Error("Request Reqno 2 != Request 1", err)
		return
	}
	if resp != "Request2" {
		t.Error("Response (1) !=  Request2")
		return
	}
	resp, err = c.RequestId(0, "Request2")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	if resp != "Request1" {
		t.Error("Error Requesting From Paxos Node: ", resp)
		return
	}
	resp, err = c.RequestId(1, "Request1")
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	if resp != "Request2" {
		t.Error("Request Reqno 1 != Request2")
		return
	}

	t.Log("TESTED NEW CLIENT")
}
