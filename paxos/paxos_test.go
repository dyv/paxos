package paxos

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestPaxosWithOneAgent(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	t.Log("Testing Paxos with One Agent")
	a, err := NewAgent("36808", false)
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
	c.AddServer(addr, "36808")
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

func TestPaxosWithThreeAgents(t *testing.T) {
	//log.SetOutput(ioutil.Discard)
	t.Log("Testing Paxos with Three Agent")
	a1, err := NewAgent("36809", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a2, err := NewAgent("36810", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a3, err := NewAgent("36811", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	addr, err := getAddress()
	if err != nil {
		t.Error("Error Getting Address:", err)
		return
	}
	err = a1.Connect(addr, "36810")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a1.Connect(addr, "36811")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36809")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36811")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36809")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36810")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	defer a1.Close()
	defer a2.Close()
	defer a3.Close()
	a1.Run()
	a2.Run()
	a3.Run()
	c := NewClient()
	c.AddServer(addr, "36809")
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
}

func TestPaxosWithThreeAgentsAndAFailure(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	t.Log("Testing Paxos with Three Agent")
	a1, err := NewAgent("36812", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a2, err := NewAgent("36813", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a3, err := NewAgent("36814", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	addr, err := getAddress()
	if err != nil {
		t.Error("Error Getting Address:", err)
		return
	}
	err = a1.Connect(addr, "36813")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a1.Connect(addr, "36814")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36812")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36814")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36812")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36813")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	defer a1.Close()
	defer a2.Close()
	a1.Run()
	a2.Run()
	a3.Run()
	a3.Close()
	c := NewClient()
	c.AddServer(addr, "36812")
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
}

func TestPaxosRedirect(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	t.Log("Testing Paxos with Three Agent")
	a1, err := NewAgent("36915", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a2, err := NewAgent("36916", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a3, err := NewAgent("36917", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	addr, err := getAddress()
	if err != nil {
		t.Error("Error Getting Address:", err)
		return
	}
	err = a1.Connect(addr, "36916")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a1.Connect(addr, "36917")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36915")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36917")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36915")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36916")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	defer a1.Close()
	defer a2.Close()
	a1.Run()
	a2.Run()
	a3.Run()
	c := NewClient()
	c.AddServer(addr, "36915")
	c.AddServer(addr, "36916")
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
	err = c.Connect(c.Servers[1])
	if err != nil {
		t.Error("Error Connecting With Server:", err)
		return
	}
	if c.leaderAddr != c.Servers[0].addr || c.leaderPort != c.Servers[0].port {
		t.Error("Was not reconnected with leader")
	}
}
