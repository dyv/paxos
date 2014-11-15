package paxos

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestNewAgent(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	a, err := NewAgent("36804", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	t.Log("Agent Created:", a)
}

func TestConnect(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	a, err := NewAgent("36804", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	t.Log("Agent Created:", a)
	err = a.Connect("127.0.0.1", "36805")
	if err != nil {
		t.Error("Failed to Connect with Peer:", err)
	}
	found := false
	for _, p := range a.peers {
		if p.addr == "127.0.0.1" && p.port == "36805" {
			found = true
			if p.client == nil {
				t.Error("Client is nil")
			}
		}
	}
	if !found {
		t.Error("Failed to Add Peer to Peer's List")
	}
}

func TestRunAgent(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	a, err := NewAgent("36804", false)
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	t.Log("Agent Created:", a)
	defer a.Close()
	a.Run()
}
