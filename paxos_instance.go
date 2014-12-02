package paxos

import "sync"

// PaxosInstance is a specific instance of the Paxos protocol. It is associated
// with specific entries in the log of values and is managed on a per entry
// basis.
type PaxosInstance struct {
	sync.Mutex
	round         int
	votes         map[Value]int
	voted         map[Peer]bool
	myvalue       Value
	history       []roundValue
	acceptedRound bool
	promisedRound bool
	naccepted     int
	accepted      chan bool
	AcceptedValue Value
	resultset     chan bool
	Result        Value
	waiting       bool // indicates whether someone is waiting for this
}

func NewPaxosInstance() *PaxosInstance {
	pi := &PaxosInstance{}
	pi.votes = make(map[Value]int)
	pi.voted = make(map[Peer]bool)
	pi.history = make([]roundValue, 0)
	pi.accepted = make(chan bool)
	pi.resultset = make(chan bool)
	pi.round = -1
	return pi
}
