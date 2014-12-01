package paxos_test

import (
	"io/ioutil"
	"log"
	"os/exec"
	"testing"

	"github.com/dyv/distfs/paxos"
	"github.com/dyv/distfs/paxos/db_app/db_backend"
)

func TestPaxosApp(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	// First build db_client
	exec.Command("go", "install", "./db_app/db_app").Run()
	t.Log("Testing Paxos with Three Agent")
	a1, err := paxos.NewAgent("36115", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a1.RegisterCmd("db_app")
	a2, err := paxos.NewAgent("36116", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a2.RegisterCmd("db_app")
	a3, err := paxos.NewAgent("36117", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	a3.RegisterCmd("db_app")
	addr, err := paxos.GetAddress()
	if err != nil {
		t.Error("Error Getting Address:", err)
		return
	}
	err = a1.Connect(addr, "36116")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a1.Connect(addr, "36117")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36115")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a2.Connect(addr, "36117")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36115")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	err = a3.Connect(addr, "36116")
	if err != nil {
		t.Error("Error Connecting:", err)
		return
	}
	defer a2.Close()
	defer a3.Close()
	a1.Run()
	a2.Run()
	a3.Run()
	c := paxos.NewClient()
	c.AddServer(addr, "36115")
	c.AddServer(addr, "36116")
	c.AddServer(addr, "36117")
	err = c.ConnectNew()
	if err != nil {
		t.Error("Error Connecting With Server:", err)
		return
	}
	resp_sr, err := c.NewRequest(db_backend.PutOp("key1", "value1"))
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	resp := resp_sr.(*paxos.StringRequest).String()
	t.Log("Response:", resp)
	if resp != "value1" {
		t.Error("Server Failed: Put: resp != value1")
		return
	}
	resp_sr, err = c.NewRequest(db_backend.GetOp("key1"))
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	resp = resp_sr.(*paxos.StringRequest).String()
	t.Log("Response:", resp)
	if resp != "value1" {
		t.Error("Server Failed: Get: resp != value1")
		return
	}
	resp_sr, err = c.NewRequest(db_backend.PutOp("key1", "value2"))
	if err != nil {
		t.Error("Error Requesting From Paxos Node:", err)
		return
	}
	resp = resp_sr.(*paxos.StringRequest).String()
	t.Log("Response3:", resp)
	if resp != "value2" {
		t.Error("Server Failed: Put: resp != value2")
		return
	}
	resp_sr, err = c.NewRequest(db_backend.GetOp("key1"))
	if err != nil {
		t.Error("Error Requesting From Paxos Node:", err)
		return
	}
	resp = resp_sr.(*paxos.StringRequest).String()
	t.Log("Response4:", resp)
	if resp != "value2" {
		t.Error("Server Failed: Put: resp != value2")
		return
	}
	a1.Close()
	err = c.ConnectNew()
	if err != nil {
		t.Error("Failed to Connect with New Paxos Node")
		return
	}
	resp_sr, err = c.NewRequest(db_backend.GetOp("key1"))
	if err != nil {
		t.Error("Error Requesting from Paxos Node:", err)
		return
	}
	resp = resp_sr.(*paxos.StringRequest).String()
	t.Log("Response:", resp)
	if resp != "value1" {
		t.Error("Server Failed: Put: resp != value1")
		return
	}

}
