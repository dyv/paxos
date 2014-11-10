package paxos

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

type Peer struct {
	addr   string
	port   string
	client net.Conn
}

func NewPeer(address, port string) (*Peer, error) {
	client, err := net.Dial("udp", address+":"+port)
	if err != nil {
		log.Println("dialing:", err)
		return nil, err
	}
	return &Peer{address, port, client}, nil
}

func (p Peer) Send(m Msg) error {
	enc := gob.NewEncoder(p.client)
	return enc.Encode(m)
}

type Value interface{}

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
	// votes maps each round number with a map that maps
	// each recieved value to a count
	votes            map[int]map[Value]int // number of votes proposal i has
	nvotes           map[int]int
	accepted         []RoundValue // map proposal number to Value
	nPeersAccepted   map[int]int
	proposalAccepted chan bool
}

func NewAgent(address, port string) (*Agent, error) {
	var err error
	a := Agent{}
	a.addr = address
	a.port = port
	a.udpaddr, err = net.ResolveUDPAddr("udp", address+":"+port)
	if err != nil {
		return nil, err
	}
	a.round = 0
	a.peers = make([]Peer, 0, 10)
	a.addrToPeer = make(map[string]Peer)
	a.votes = make(map[int]map[Value]int)
	a.nvotes = make(map[int]int)
	a.accepted = make([]RoundValue, 0)
	a.nPeersAccepted = make(map[int]int)
	a.proposalAccepted = make(chan bool)
	return &a, nil
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
			case Nack:
			case Accept:
			case AcceptRequest:
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

func (a *Agent) Request(value Value, reply *string) {
	args := make([]interface{}, 1)
	args[0] = a.round
	for _, p := range a.peers {
		p.Send(Msg{Prepare, args})
	}
	a.round += 1
	a.votes[a.round] = make(map[Value]int)
	a.votes[a.round][value]++
	a.nvotes[a.round]++
	<-a.proposalAccepted
	*reply = "proposal accepted"
}

func (a *Agent) LastAccepted() RoundValue {
	l := len(a.accepted)
	if l == 0 {
		return RoundValue{-1, -1}
	}
	return a.accepted[l-1]
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

func (a *Agent) Promise(n int, la RoundValue) {
	if la.Round != -1 {
		a.votes[n][la.Val]++
		a.nvotes[a.round]++
	}
	// if we have crossed the quorum threshold
	if a.Quorum(a.nvotes[n]) {
		// get the value that has the most votes
		mv := la.Val
		nv := 0
		for k, v := range a.votes[n] {
			if v > nv {
				mv = k
				nv = v
			}
		}
		// accept my own
		a.accepted = append(a.accepted, RoundValue{n, mv})
		args := make([]interface{}, 2)
		args[0] = n
		args[1] = mv
		for _, p := range a.peers {
			p.Send(Msg{AcceptRequest, args})
		}
	}
}

func (a *Agent) Nack(n int) {}

func (a *Agent) AcceptRequest(r int, v Value, p Peer) {
	if a.round == r {
		args := make([]interface{}, 2)
		args[0] = r
		args[1] = v
		p.Send(Msg{Accepted, args})
		for _, p := range a.peers {
			p.Send(Msg{Accepted, args})
		}
	}
}

func (a *Agent) Accepted(r int, v Value) {
	a.nPeersAccepted[r]++
	if a.Quorum(a.nPeersAccepted[r]) {
		a.accepted = append(a.accepted, RoundValue{r, v})
		a.proposalAccepted <- true
	}
}
