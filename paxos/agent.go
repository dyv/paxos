package paxos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Lshortfile)
}

// RoundValue is a pair used by Agents to store the value that they have for a
// given round.
type RoundValue struct {
	Round float64 `json:"round"`
	Val   Value   `json:"value"`
}

func iMapToRV(m map[string]interface{}) RoundValue {
	return RoundValue{m["round"].(float64), m["value"].(Value)}
}

// Agent collapses the multiple roles in Paxos into a single role.
// It plays the role of the Proposer, Acceptor, and Learner.
type Agent struct {
	Peer
	udpaddr    *net.UDPAddr
	round      float64
	peers      []Peer
	addrToPeer map[string]Peer
	// votes is the number of votes we have recieved in this round
	// for each given value
	votes   map[Value]int
	voted   map[Peer]bool
	myvalue Value
	// history is a list of our history of accepted values
	history       []RoundValue
	acceptedRound bool
	promisedRound bool
	// naccepted is the number of peers that have accepted the
	// current proposal
	naccepted int
	// channel to indicate when the reuqest's proposal is accepted
	accepted       chan bool
	done           chan bool
	servingClients bool
	servingAgents  bool
	AcceptedValue  Value
}

// getAddress gets the localhosts IPv4 address.
func getAddress() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		log.Print("Error Resolving Hostname:", err)
		return "", err
	}
	as, err := net.LookupHost(name)
	if err != nil {
		log.Print("Error Looking Up Host:", err)
		return "", err
	}
	return as[0], nil
}

// NewAgent creates a new agent that is located at the given port on this
// machine.
func NewAgent(port string) (*Agent, error) {
	var err error
	a := &Agent{}
	addr, err := getAddress()
	if err != nil {
		log.Println("Error Getting Address:", err)
		return nil, errors.New("Cannot Resolve Local IP Address")
	}
	a.addr = addr
	a.port = port
	a.udpaddr, err = net.ResolveUDPAddr("udp", net.JoinHostPort(a.addr, port))
	if err != nil {
		log.Print("Error resolving UDP address")
		return nil, err
	}
	a.round = -1
	a.peers = make([]Peer, 0, 10)
	a.addrToPeer = make(map[string]Peer)
	a.accepted = make(chan bool)
	a.Connect(a.addr, a.port)
	a.done = make(chan bool)
	return a, nil
}

func (a *Agent) Close() {
	log.Println("Closing")
	if a.servingAgents {
		a.done <- true
	}
	if a.servingClients {
		a.done <- true
	}
	log.Println("Closed Agent")
}

// newRound initializes a new round for this proposer. It increments the round
// count to indicate we have a new round and clears the recorded votes and
// accepted.
func (a *Agent) newRound() {
	a.voted = make(map[Peer]bool)
	a.votes = make(map[Value]int)
	a.naccepted = 0
}

// Connect connects to a Paxos Agent at a given address and port.
func (a *Agent) Connect(addr, port string) error {
	p, err := NewPeer(addr, port)
	if err != nil {
		log.Print("Error Creating Peer")
		return err
	}
	udp, err := net.ResolveUDPAddr("udp", net.JoinHostPort(addr, port))
	if err != nil {
		log.Print("Error Resolving UDP addr")
		return err
	}
	a.addrToPeer[udp.String()] = *p
	a.peers = append(a.peers, *p)
	log.Println("Added Peer:", a.peers)
	return nil
}

// Run starts this Paxos node serving clients and agents alike. It runs the
// node in the background
func (a *Agent) Run() error {
	err := a.ServeClients()
	if err != nil {
		log.Print("Failed to Serve Clients")
		return err
	}
	err = a.ServeAgents()
	if err != nil {
		log.Print("Failed to Serve Agents")
		return err
	}
	log.Print("Server is Running at: ", net.JoinHostPort(a.addr, a.port))
	return nil
}

func (a *Agent) handleClientRequest(conn net.Conn) {
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var msg Msg
		err := dec.Decode(&msg)
		if err != nil {
			log.Print("Error Decoding Request: ", err)
			enc.Encode("Error Decoding from Connection: " + err.Error())
			return
		}
		var resp Msg
		switch msg.Type {
		case ClientRequest:
			err = a.Request(msg.Args[0].(Value))
			if err != nil {
				resp.Type = Error
				resp.Args = []interface{}{err.Error()}
			} else {
				resp.Type = ClientResponse
				resp.Args = []interface{}{a.AcceptedValue}
			}
		default:
			err = fmt.Errorf("Invalid Message Type: %v", msg.Type)
			resp.Type = Error
			resp.Args = []interface{}{err.Error()}
		}

		err = enc.Encode(resp)
		if err != nil {
			log.Print("Error Encoding Response")
		}
	}
}

// ServeClients serves Clients. It uses RPC to allow these users to execute
// Requests on the Paxos system. These are TCP connections between client and
// a single Paxos node which becomes it's proposer.
func (a *Agent) ServeClients() error {
	l, err := net.Listen("tcp", net.JoinHostPort(a.addr, a.port))
	if err != nil {
		log.Println("Listen Error:", err)
		return err
	}
	// setup Conection Closer
	go func(a *Agent, l net.Listener) {
		<-a.done
		l.Close()
	}(a, l)
	go func(l net.Listener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Print("Error Accepting Connection:", err)
				return
			}
			go a.handleClientRequest(conn)
		}
	}(l)
	return nil
}

// ServeAgents serves other Paxos Agents. It listens on its port for other
// Paxos agents to send it requests. These requests are encoded using json and
// each request is able to fit into a single UDP packet. This essentially
// emulates an RPC server which does not wait for results. Instead it just
// accepts one-off UDP messages and sends back a UDP message when it is done.
func (a *Agent) ServeAgents() error {
	conn, err := net.ListenUDP("udp", a.udpaddr)
	if err != nil {
		return err
	}
	a.servingAgents = true
	by := make([]byte, 64000)
	go func(a *Agent, conn *net.UDPConn, by []byte) {
		defer conn.Close()
		for {
			conn.SetDeadline(time.Now().Add(time.Millisecond * 100))
			select {
			case <-a.done:
				log.Println("Killing Agents")
				return
			default:
			}
			n, _, err := conn.ReadFromUDP(by)
			if err != nil {
				es := err.Error()
				// if the error is a timeout just try again
				if len(es) >= 8 && es[len(es)-8:] == "timeout" {
					continue
				}

			}
			if n == 0 {
				continue
			}
			buf := bytes.NewBuffer(by[0:n])
			dec := json.NewDecoder(buf)
			var m Msg
			err = dec.Decode(&m)
			if err != nil {
				log.Println("Error Decoding:", err)
				continue
			}
			p, ok := a.addrToPeer[net.JoinHostPort(m.Address, m.Port)]
			if !ok {
				log.Print("From Address is not on peers list")
				log.Print(net.JoinHostPort(m.Address, m.Port))
				log.Print(a.addrToPeer)
				continue
			}
			// TODO(dyv): Typesafe way of doing this.
			switch m.Type {
			case Prepare:
				if len(m.Args) != 1 {
					log.Println("Prepare Message: Not Enough Arguments")
					break
				}
				a.Prepare(m.Args[0].(float64), p)
			case Promise:
				if len(m.Args) != 2 {
					log.Println("Promise Message: Not Enough Arguments")
					break
				}
				a.Promise(m.Args[0].(float64), iMapToRV(m.Args[1].(map[string]interface{})), p)
			case Nack:
				if len(m.Args) != 2 {
					log.Println("Nack Message: Not Enough Arguments")
					break
				}
				a.Nack(m.Args[0].(float64), iMapToRV(m.Args[1].(map[string]interface{})))
			case AcceptRequest:
				if len(m.Args) != 2 {
					log.Println("AcceptRequest Message: Not Enough Arguments")
					break
				}
				a.AcceptRequest(m.Args[0].(float64), m.Args[1].(Value), p)
			case Accepted:
				if len(m.Args) != 2 {
					log.Println("Accpeted Message: Not Enough Arguments")
					break
				}
				a.Accepted(m.Args[0].(float64), m.Args[1].(Value))
			default:
				log.Println("Received Message With Bad Type")
			}
		}
	}(a, conn, by)
	return nil

}

// Quorum returns whether the number given signifies exactly one Quorum for
// this Agent/Proposer.
func (a *Agent) Quorum(n int) bool {
	return n == len(a.peers)/2+1
}

var RequestTimeout error = errors.New("paxos: request timed out")

// Request is an RPC call that client applications can call. When a Paxos Agent
// receives a Request RPC it takes the role of the proposer. As proposer it
// sends preparation messages to a Quorum of Acceptors.
func (a *Agent) Request(value Value) error {
	a.StartRequest(a.round+1, value)
	// wait for this proposal to be accepted or a timeout to occur
	timeout := make(chan bool, 10)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()
	select {
	case <-a.accepted:
		return nil
	case <-timeout:
		return RequestTimeout
	}
}

func (a *Agent) StartRequest(round float64, value Value) {
	args := make([]interface{}, 1)
	a.newRound()
	args[0] = round
	a.myvalue = value
	a.round = round
	a.promisedRound = false
	a.acceptedRound = false
	for _, p := range a.peers {
		p.Send(Msg{Prepare, a.addr, a.port, args})
	}
}

var NoValue RoundValue = RoundValue{-1, -1}

// LastAccepted is a function that returns the last RoundValue that has been
// accepted by this agent and commited to its history. If no value was
// previously committed to this agent's history it returns a NoValue.
func (a *Agent) LastAccepted() RoundValue {
	l := len(a.history)
	if l == 0 {
		return NoValue
	}
	return a.history[l-1]
}

// Prepare sends an either a Promise or a Nack to the peer that sent the
// Prepare request. If it corresponds with this round and this Agent has not
// accepted anything else, then it sends a Promise, which reserves this value
// to be the one that the Proposer requests. Otherwise the Agent sends a Nack
// response to the Proposer signifying that this slot has already been taken.
func (a *Agent) Prepare(n float64, p Peer) {
	log.Print("PREPARE")
	args := make([]interface{}, 2)
	args[0] = n
	args[1] = a.LastAccepted()
	if n > a.round || (n == a.round && a.promisedRound == false) {
		a.round = n
		a.promisedRound = true
		a.acceptedRound = false
		p.Send(Msg{Promise, a.addr, a.port, args})
	} else {
		log.Printf("%v, %v, %v", n, a.round, a.promisedRound)
		p.Send(Msg{Nack, a.addr, a.port, args})
	}
}

// The Proposer receives several promises for this round
func (a *Agent) Promise(r float64, la RoundValue, p Peer) {
	log.Print("PROMISE")
	// only accept promises for the round we are currently on
	if r != a.round {
		log.Printf("r != a.round: %v != %v", r, a.round)
		return
	}
	a.round = r
	if la.Round < 0 {
		a.votes[a.myvalue]++
		a.voted[p] = true
	} else {
		a.votes[la.Val]++
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
			log.Print("Sending Accept Request Message")
			p.Send(Msg{AcceptRequest, a.addr, a.port, args})
		}
		return
	}
	log.Print("Not at Quorum: ", a.voted)
}

func (a *Agent) Nack(r float64, rv RoundValue) {
	log.Print("NACK")
	// if we have recieved a nack for a greater round than this
	if rv.Round > a.round {
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		a.StartRequest(rv.Round+1, rv.Val)
	}
	if r < a.round {
		return
	}
	a.newRound()
	args := make([]interface{}, 1)
	args[0] = a.round
	for _, p := range a.peers {
		p.Send(Msg{Prepare, a.addr, a.port, args})
	}
}

func (a *Agent) AcceptRequest(r float64, v Value, p Peer) {
	log.Print("ACCEPTREQUEST")
	if r != a.round {
		return
	}
	if a.acceptedRound && a.history[len(a.history)-1].Val != v {
		return
	}
	args := make([]interface{}, 2)
	args[0] = r
	args[1] = v
	a.acceptedRound = true
	a.AcceptedValue = v
	a.history = append(a.history, RoundValue{r, v})
	p.Send(Msg{Accepted, a.addr, a.port, args})
}

func (a *Agent) Accepted(r float64, v Value) {
	log.Print("ACCEPTED")
	if r != a.round {
		return
	}
	a.naccepted++
	if a.Quorum(a.naccepted) {
		a.AcceptedValue = v
		a.history = append(a.history, RoundValue{r, v})
		a.accepted <- true
	}
}
