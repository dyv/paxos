package paxos

import "testing"

func TestPaxosWithOneAgent(t *testing.T) {
	a, err := NewAgent("36904")
	if err != nil {
		t.Error("Error Creating Agent:", err)
		return
	}
	t.Log("Agent Created:", a)
}
