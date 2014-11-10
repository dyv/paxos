package paxos

import (
	"bytes"
	"encoding/gob"
	"errors"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"time"
)

type RoundValue struct {
	Round int
	Val   Value
}

// Agent collapses the multiple roles in Paxos into a single role
type Agent struct {
	Peer
	udpaddr    *net.UDPAddr
	round      int
	peers      []Peer
	addrToPeer map[string]Peer
	// votes is the number of votes we have recieved in this round
	// for each given value
	votes   map[Value]int
	voted   map[Peer]bool
	myvalue Value
	// history is a list of our history of accepted values
	history []RoundValue
	// naccepted is the number of peers that have accepted the
	// current proposal
	naccepted int
	// channel to indicate when the reuqest's proposal is accepted
	accepted chan bool
}

func NewAgent(address, port string) (*Agent, error) {
	var err error
	a := &Agent{}
	a.addr = address
	a.port = port
	a.udpaddr, err = net.ResolveUDPAddr("udp", address+":"+port)
	if err != nil {
		return nil, err
	}
	a.round = -1
	a.peers = make([]Peer, 0, 10)
	a.peers = append(a.peers, Peer{a.addr, a.port, nil})
	a.addrToPeer = make(map[string]Peer)
	a.addrToPeer[a.udpaddr.String()] = Peer{a.addr, a.port, nil}
	a.accepted = make(chan bool)
	return a, nil
}

func (a *Agent) newRound() {
	a.round++
	a.voted = make(map[Peer]bool)
	a.votes = make(map[Value]int)
	a.naccepted = 0
}

func (a *Agent) AddPeer(addr, port string) error {
	p, err := NewPeer(addr, port)
	if err != nil {
		return err
	}
	udp, err := net.ResolveUDPAddr("udp", addr+":"+port)
	if err != nil {
		return err
	}
	a.addrToPeer[udp.String()] = *p
	a.peers = append(a.peers, *p)
	return nil
}

func (a *Agent) Run() {
	a.ServeClients()
	a.ServeAgents()
}

// ServeClients serves Clients. It uses RPC to allow these
// users to execute server commands
func (a *Agent) ServeClients() error {
	rpc.Register(a)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":"+a.port)
	if err != nil {
		log.Println("Listen Error:", err)
		return err
	}
	go http.Serve(l, nil)
	return nil
}

// ServeAgents serves other Agents. It uses UDP connections for speed
func (a *Agent) ServeAgents() error {
	conn, err := net.ListenUDP("udp", a.udpaddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	by := make([]byte, 64000)
	go func(a *Agent, conn *net.UDPConn, by []byte) {
		for {
			n, addrfrom, err := conn.ReadFromUDP(by)
			p := a.addrToPeer[addrfrom.String()]
			buf := bytes.NewBuffer(by[0:n])
			dec := gob.NewDecoder(buf)
			if err != nil {
				log.Println("Error Reading From Connection:", addrfrom)
				a.ServeClients()
			}
			var m Msg
			err = dec.Decode(&m)
			if err != nil {
				log.Println("Error Decoding:", err)
			}
			log.Println("Got Response:", m)
			switch m.Type {
			case Prepare:
				a.Prepare(m.Args[0].(int), p)
			case Promise:
				a.Promise(m.Args[0].(int), m.Args[1].(RoundValue), p)
			case Nack:
				a.Nack(m.Args[0].(int))
			case AcceptRequest:
				a.AcceptRequest(m.Args[0].(int), m.Args[1].(Value), p)
			case Accepted:
				a.Accepted(m.Args[0].(int), m.Args[1].(Value))
			default:
				log.Println("Received Message With Bad Type")
			}
		}
	}(a, conn, by)
	return nil

}

// only return true if it is exactly a quorum
func (a *Agent) Quorum(n int) bool {
	return n == len(a.peers)/2+1
}

var RequestTimeout error = errors.New("paxos: request timed out")

func (a *Agent) Request(value Value, reply *string) error {
	args := make([]interface{}, 1)
	a.newRound()
	args[0] = a.round
	a.myvalue = value
	for _, p := range a.peers {
		p.Send(Msg{Prepare, args})
	}
	// wait for this proposal to be accepted or a timeout to occur
	timeout := make(chan bool, 10)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()
	select {
	case <-a.accepted:
		*reply = "proposal accepted"
		return nil
	case <-timeout:
		return RequestTimeout
	}
}

func (a *Agent) LastAccepted() RoundValue {
	l := len(a.history)
	if l == 0 {
		return RoundValue{-1, -1}
	}
	return a.history[l-1]
}

// In Prepare a leader creates a proposal identified with a number N
// this number N must be greater than any previous proposal number used
// by this Agent
func (a *Agent) Prepare(n int, p Peer) {
	args := make([]interface{}, 2)
	args[0] = n
	args[1] = a.LastAccepted()
	if n >= a.round {
		p.Send(Msg{Promise, args})
	} else {
		p.Send(Msg{Nack, args})
	}
}

func (a *Agent) Promise(r int, la RoundValue, p Peer) {
	// if this isn't a promise for this round ignore it
	if r != a.round {
		return
	}
	if la.Round != -1 {
		a.votes[a.myvalue]++
		a.voted[p] = true
	}
	// if we have crossed the quorum threshold
	if a.Quorum(len(a.voted)) {
		// get the value that has the most votes
		mv := la.Val
		nv := 0
		for k, v := range a.votes {
			if v > nv {
				mv = k
				nv = v
			}
		}
		// accept my own
		a.history = append(a.history, RoundValue{r, mv})
		args := make([]interface{}, 2)
		args[0] = r
		args[1] = mv
		for _, p := range a.peers {
			p.Send(Msg{AcceptRequest, args})
		}
	}
}

func (a *Agent) Nack(r int) {
	if r != a.round {
		return
	}
	a.newRound()
	args := make([]interface{}, 1)
	args[0] = a.round
	for _, p := range a.peers {
		p.Send(Msg{Prepare, args})
	}
}

func (a *Agent) AcceptRequest(r int, v Value, p Peer) {
	if a.round != r {
		return
	}
	args := make([]interface{}, 2)
	args[0] = r
	args[1] = v
	p.Send(Msg{Accepted, args})
	for _, p := range a.peers {
		p.Send(Msg{Accepted, args})
	}
}

func (a *Agent) Accepted(r int, v Value) {
	if a.round != r {
		return
	}
	a.naccepted++
	if a.Quorum(a.naccepted) {
		a.history = append(a.history, RoundValue{r, v})
		a.accepted <- true
	}
}
