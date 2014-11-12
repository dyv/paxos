package paxos

import "testing"

func TestPaxosWithOneAgent(t *testing.T) {
	t.Log("Testing Paxos with One Agent")
	a, err := NewAgent("36808")
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
	t.Log("Testing Paxos with Three Agent")
	a1, err := NewAgent("36809")
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a2, err := NewAgent("36810")
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a3, err := NewAgent("36811")
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
	t.Log("Testing Paxos with Three Agent")
	a1, err := NewAgent("36812")
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a2, err := NewAgent("36813")
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a3, err := NewAgent("36814")
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
