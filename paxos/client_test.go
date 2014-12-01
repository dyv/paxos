package paxos

import "testing"

func TestClientOneRequest(t *testing.T) {
	//log.SetOutput(ioutil.Discard)
	t.Log("TESTING NEW CLIENT")
	a, err := NewAgent("36807", false)
	if err != nil {
		t.Fatal("Error Creating Agent:", err)
		return
	}
	defer a.Close()
	a.Run()
	c := NewClient()
	addr, err := GetAddress()
	if err != nil {
		t.Fatal("Error getting address:", err)
		return
	}
	c.AddServer(addr, "36807")
	err = c.ConnectFirst()

	if err != nil {
		t.Fatal("Error Connecting With Server:", err)
		return
	}
	resp_sr, err := c.Request(NewStrReq("Request1"))
	if err != nil {
		t.Fatal("Error Requesting from Paxos Node:", err)
		return
	}
	resp := resp_sr.(*StringRequest).String()
	if resp != "Request1" {
		t.Fatal("Server Failed: ", resp)
	}
	resp_sr, err = c.Request(NewStrReq("Request2"))
	if err != nil {
		t.Fatal("Error Requesting from Paxos Node:", err)
		return
	}
	resp = resp_sr.(*StringRequest).String()
	if resp != "Request1" {
		t.Fatal("Server Failed: ", resp)
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
	addr, err := GetAddress()
	if err != nil {
		t.Error("Error getting address:", err)
		return
	}
	c.AddServer(addr, "36820")
	err = c.ConnectFirst()
	if err != nil {
		t.Error("Error Connecting With Server:", err)
		return
	}
	resp_sr, err := c.NewRequest(NewStrReq("Request1"))
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	resp := resp_sr.(*StringRequest).String()
	t.Log("Response:", resp)
	if resp != "Request1" {
		t.Error("Response (0) != Request 1")
		return
	}
	resp_sr, err = c.NewRequest(NewStrReq("Request2"))
	if err != nil {
		t.Error("Request Reqno 2 != Request 1", err)
		return
	}
	resp = resp_sr.(*StringRequest).String()
	if resp != "Request2" {
		t.Error("Response (1) !=  Request2")
		return
	}
	resp_sr, err = c.RequestId(0, NewStrReq("Request2"))
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	resp = resp_sr.(*StringRequest).String()
	if resp != "Request1" {
		t.Error("Error Requesting From Paxos Node: ", resp)
		return
	}
	resp_sr, err = c.RequestId(1, NewStrReq("Request1"))
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	resp = resp_sr.(*StringRequest).String()
	if resp != "Request2" {
		t.Error("Request Reqno 1 != Request2")
		return
	}

	t.Log("TESTED NEW CLIENT")
}
