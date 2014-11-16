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
	"os/exec"
	"path"
	"regexp"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Lshortfile)
}

// RoundValue is a pair used by Agents to store the value that they have for a
// given round.
type RoundValue struct {
	Round int   `json:"round"`
	Val   Value `json:"value"`
}

// a client should have a sequence of requests that are dispersed
// throughout the total number of requests. We need the client to give us a
// request number so we know whether we are receiving a request that has
// already been processed or one that hasn't. This means that we have to
// have a means of telling, given a client's id, and a entry number,
// whether this entry has been accepted (an index into values). If this entry
// has been accepted, then we need only return the index into the values
// that it corresponds with

type ClientInfo struct {
	id      int
	reqno   int
	request map[int]int // map the reqno to the Agent Values entry it is in
	conn    net.Conn
}

var LeaderTimeout time.Duration = 1 * time.Second
var MaxLog int = 200000

// Agent collapses the multiple roles in Paxos into a single role.
// It plays the role of the Proposer, Acceptor, and Learner.
type Agent struct {
	Peer
	udpaddr    *net.UDPAddr
	peers      []Peer // Deprecate
	addrToPeer map[string]Peer
	// Each of the following members are slices so that way we have history for
	// each log entry we are trying to fill.
	// votes is the number of votes we have recieved in this round
	// for each given value
	instances      []PaxosInstance
	servingClients bool
	servingAgents  bool
	// leader election for efficiency and multi-paxos
	leaderLock    *sync.Mutex
	isLeader      bool             // is this current node the leader
	leader        *Peer            // who is the current leader (for redirects)
	leaderCurrent bool             // is the leader current (every T seconds check)
	leaderTimeout <-chan time.Time // how long before the leader is invalidated
	heartbeat     <-chan time.Time // how long before leader should send another message
	done          chan bool
	// Logging Functionality
	// log for remembering whether we have committed something to an entry in
	// the log, whether we have promised it, or whether we have prepared
	// something for it, or whether
	Messages   *MsgLog
	Values     *ValueLog
	entry      int
	clientLock *sync.Mutex
	clientId   int                // the next id to assign
	clients    map[int]ClientInfo // map client id to client info
	// App: The Client Application Command to Run when we become the leader
	Cmd *exec.Cmd
}

var ipv4Reg = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)

// getAddress gets the localhosts IPv4 address.
func getAddress() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		log.Print("Error Resolving Hostname:", err)
		return "", err
	}
	log.Print("name: ", name)
	as, err := net.LookupHost(name)
	if err != nil {
		log.Print("Error Looking Up Host:", err)
		return "", err
	}
	addr := ""
	for _, a := range as {
		log.Printf("a = %+v", a)
		if ipv4Reg.MatchString(a) {
			log.Print("matches")
			addr = a
		}
	}
	if addr == "" {
		err = errors.New("No IPv4 Address")
	}
	log.Print("address: ", addr)
	return addr, err
}

// NewAgent creates a new agent that is located at the given port on this
// machine.
func NewAgent(port string, try_recover bool) (*Agent, error) {
	var err error
	a := &Agent{}
	addr, err := getAddress()
	if err != nil {
		log.Println("Error Getting Address:", err)
		return nil, errors.New("Cannot Resolve Local IP Address")
	}
	a.leaderLock = &sync.Mutex{}
	a.clientLock = &sync.Mutex{}
	a.addr = addr
	a.port = port
	a.udpaddr, err = net.ResolveUDPAddr("udp", net.JoinHostPort(a.addr, port))
	if err != nil {
		log.Print("Error resolving UDP address")
		return nil, err
	}
	a.peers = make([]Peer, 0, 10)
	a.addrToPeer = make(map[string]Peer)
	// vote metadata
	a.done = make(chan bool)
	a.instances = make([]PaxosInstance, 0)
	a.Connect(a.addr, a.port)
	a.Messages, err = NewMsgLog(1000, path.Join("logs", net.JoinHostPort(a.addr, port)), a, try_recover)
	if err != nil {
		log.Print("Error Initializing Message Log: ", err)
		return nil, err
	}
	a.Values = NewValueLog(1000)
	a.clients = make(map[int]ClientInfo)
	return a, nil
}

func (a *Agent) RegisterExecCmd(cmd *exec.Cmd) {
	a.Cmd = cmd
}

func (a *Agent) RegisterCmd(name string, arg ...string) {
	a.RegisterExecCmd(exec.Command(name, arg...))
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

// newRound initializes a new round for this proposer. It clears the recorded
// votes and accepted.
func (a *Agent) newRound(entry int) {
	a.instances[entry].voted = make(map[Peer]bool)
	a.instances[entry].votes = make(map[Value]int)
	a.instances[entry].naccepted = 0
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
		case LogRequest:
			for {
				for entry, v := range a.Values.Log {
					// if it is not committed get it first
					if !v.Committed {
						a.Request(v, RequestInfo{Entry: entry, NoSet: true})
					}
					enc.Encode(Msg{Type: LogResponse, Value: v})
				}
				enc.Encode(Msg{Type: Done})
			}
		case ClientRequest:
			// if I am not the leader and there is a leader I defer to
			// then redirect the client to them
			a.leaderLock.Lock()
			if !a.isLeader && a.leader != nil {
				// if there is a leader send a redirect to that peer
				log.Print("Redirecting Client to Leader")
				resp.Type = ClientRedirect
				resp.LeaderAddress = a.leader.addr
				resp.LeaderPort = a.leader.port
				a.leaderLock.Unlock()
				enc.Encode(resp)
				return
			}
			a.leaderLock.Unlock()
			cid := msg.Request.Id
			cno := msg.Request.No
			a.clientLock.Lock()
			ci := a.clients[cid]
			if conn != ci.conn {

				a.clientLock.Unlock()
				log.Println("Malicious Client: Sending with Wrong IDs")
				resp.Type = Error
				resp.Error = "Malicious Client: Wrong ID"
				break
			}
			log.Printf("HISTORY: %+v, client no: %+v", ci, cno)
			if vi, ok := ci.request[cno]; ok {
				log.Print("Passing Back Previous Response: ", ci.request[cno])
				log.Printf("Index: %+v", vi)
				log.Printf("Client Number: %+v", cno)
				log.Printf("Client Request: %+v", ci.request[cno])
				log.Printf("Value Associated: %+v", a.Values.Log[vi])
				a.clientLock.Unlock()
				// if we have handled this request before
				resp.Type = ClientResponse
				resp.Value = a.Values.Log[vi].Val
				break
			}
			a.clientLock.Unlock()
			a.Messages.Append(msg)
			msg.Request.Entry = a.entry
			a.entry++
			err = a.Request(msg.Value, msg.Request)
			if err != nil {
				resp.Type = Error
				resp.Error = err.Error()
			} else {
				vi := ci.request[cno]
				log.Print("Generated Entry")
				log.Printf("Index: %+v", vi)
				log.Printf("Client Number: %+v", cno)
				log.Printf("Client Request: %+v", ci.request[cno])
				log.Printf("Value Associated: %+v", a.Values.Log[vi])
				log.Printf("Log: %+v", a.Values.Log)
				// if we have handled this request before
				resp.Type = ClientResponse
				resp.Value = a.Values.Log[vi].Val
				break
			}
		default:
			err = fmt.Errorf("Invalid Message Type: %v", msg.Type)
			resp.Type = Error
			resp.Error = err.Error()
		}
		resp.Request = msg.Request
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
			enc := json.NewEncoder(conn)
			var resp Msg
			// if there is a leader then redirect to that leader
			a.leaderLock.Lock()
			if !a.isLeader && a.leader != nil {
				resp.Type = ClientRedirect
				resp.LeaderAddress = a.leader.addr
				resp.LeaderPort = a.leader.port
				a.leaderLock.Unlock()
				enc.Encode(resp)
				return
			} else {
				a.leaderLock.Unlock()
				resp.Type = ClientConn
				resp.Request.No = 0
				a.clientLock.Lock()
				resp.Request.Id = a.clientId
				a.clients[a.clientId] = ClientInfo{id: a.clientId,
					reqno: 0, request: make(map[int]int), conn: conn}
				a.clientId++
				a.clientLock.Unlock()
				enc.Encode(resp)
			}
			go a.handleClientRequest(conn)
		}
	}(l)
	return nil
}

func (a *Agent) SendHeartbeat() {
	// if I am not a leader don't spam and send heartbeats
	a.leaderLock.Lock()
	if !a.isLeader {
		a.leaderLock.Unlock()
		return
	}
	a.leaderLock.Unlock()
	for _, p := range a.peers {
		p.Send(Msg{Type: Heartbeat, FromAddress: a.addr, FromPort: a.port}, true)
	}
}

func (a *Agent) handleMessage(m Msg, send bool) {
	p, ok := a.addrToPeer[net.JoinHostPort(m.FromAddress, m.FromPort)]
	if !ok {
		log.Print("From Address is not on Peers List: Rejecting")
		log.Print(net.JoinHostPort(m.FromAddress, m.FromPort))
		log.Print(a.addrToPeer)
		return
	}

	switch m.Type {
	case Heartbeat:
		// if we have already determined that the leader is current
		// for this round then dont do anything
		if a.leaderCurrent {
			return
		}
		// if the leader needs to be updated and this is the leader
		// we think it should be then say that this leader is alive
		if !a.leaderCurrent &&
			a.leader != nil &&
			a.leader.addr == p.addr &&
			a.leader.port == p.port {
			a.leaderCurrent = true
		}
	case Prepare:
		a.Prepare(m.Round, m.Request, p, send)
	case Promise:
		a.Promise(m.Round, m.RoundValue, m.Request, p, send, false)
	case Nack:
		a.Nack(m.Round, m.RoundValue, m.Request, p, send)
	case AcceptRequest:
		a.AcceptRequest(m.Round, m.Value, m.Request, p, send)
	case Accepted:
		a.Accepted(m.Round, m.Value, m.Request, p, send)
	default:
		log.Println("Received Message With Bad Type")
	}
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
		a.leaderTimeout = time.Tick(LeaderTimeout / 2)
		a.heartbeat = time.Tick(LeaderTimeout)
		defer conn.Close()
		for {
			conn.SetDeadline(time.Now().Add(time.Millisecond * 100))
			select {
			case <-a.done:
				log.Println("Killing Agents")
				return
			case <-a.heartbeat:
				a.SendHeartbeat()
			case <-a.leaderTimeout:
				// At the beginning of each heart beat round
				// if the leader has not been updated kill it
				// otherwise say that the leader is not current
				// and wait for the leader to verify that it is alive
				if !a.leaderCurrent {
					a.leader = nil
				}
				a.leaderCurrent = false
				log.Println("Sending heartbeat")
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
			log.Print("Received Message: ", m)
			a.Messages.Append(m)
			a.handleMessage(m, true)
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
func (a *Agent) Request(value Value, r RequestInfo) error {
	log.Printf("Requesting Entry: %v", r.Entry)
	log.Printf("Request Info: %v", r)
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}
	a.instances[r.Entry].myvalue = value

	if !a.isLeader {
		a.StartRequest(a.instances[r.Entry].round+1, value, r, true)
	} else {
		a.Promise(-1, RoundValue{}, r, Peer{a.addr, a.port, nil}, true, true)
	}
	// wait for this proposal to be accepted or a timeout to occur
	timeout := make(chan bool)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()
	log.Print("WAITING ON: ", a.instances[r.Entry].accepted)
	select {
	case <-a.instances[r.Entry].accepted:
		return nil
	case <-timeout:
		return RequestTimeout
	}
}

func (a *Agent) StartRequest(round int, value Value, r RequestInfo, send bool) {
	// make a new round for this specific entry
	a.newRound(r.Entry)
	a.instances[r.Entry].myvalue = value
	for _, p := range a.peers {
		log.Print("Sending Prepare Message with Round: ", round)
		p.Send(Msg{Type: Prepare,
			FromAddress: a.addr, FromPort: a.port,
			Request: r,
			Round:   round}, send)
	}
}

var NoValue RoundValue = RoundValue{-1, -1}

// LastAccepted is a function that returns the last RoundValue that has been
// accepted by this agent and commited to its history. If no value was
// previously committed to this agent's history it returns a NoValue.
func (a *Agent) LastAccepted(entry int) RoundValue {
	l := len(a.instances[entry].history)
	if l == 0 {
		return NoValue
	}
	return a.instances[entry].history[l-1]
}

// Prepare sends an either a Promise or a Nack to the peer that sent the
// Prepare request. If it corresponds with this round and this Agent has not
// accepted anything else, then it sends a Promise, which reserves this value
// to be the one that the Proposer requests. Otherwise the Agent sends a Nack
// response to the Proposer signifying that this slot has already been taken.
func (a *Agent) Prepare(n int, r RequestInfo, p Peer, send bool) {
	log.Print("PREPARE: ", n, r, p, send)
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}
	instance := &a.instances[r.Entry]
	if n > instance.round || (n == instance.round && instance.promisedRound == false) {
		if r.NoSet {
			log.Print("Not Allowed to Accept")
			return
		}
		instance.round = n
		instance.promisedRound = true
		instance.acceptedRound = false
		p.Send(Msg{Type: Promise,
			FromAddress: a.addr, FromPort: a.port,
			Request: r,
			Round:   n, RoundValue: a.LastAccepted(r.Entry)}, send)
	} else {
		log.Printf("NACK: %v, %v, %v", n, instance.round, instance.promisedRound)
		p.Send(Msg{Type: Nack,
			FromAddress: a.addr, FromPort: a.port,
			Request: r,
			Round:   n, RoundValue: a.LastAccepted(r.Entry)}, send)
	}
}

// The Proposer receives several promises for this round
func (a *Agent) Promise(n int, la RoundValue, r RequestInfo, p Peer, send bool, shortcut bool) {
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}

	instance := &a.instances[r.Entry]
	if !shortcut {
		log.Print("PROMISE: ", r.Entry, n, la, r, p, send)
		// only accept promises for the round we are currently on
		if n != instance.round {
			log.Printf("r != instance.round: %v != %v", r, instance.round)
			return
		}
		instance.round = n
		if la.Round < 0 {
			log.Print("My Value: ", instance.myvalue)
			instance.votes[r.Val]++
			instance.voted[p] = true
		} else {
			instance.votes[la.Val]++
			instance.voted[p] = true
		}
	}
	// If we have been promised a quorum of votes
	if shortcut || a.Quorum(len(instance.voted)) {
		log.Print("Quorum Has been Reached: ", r)
		a.leaderLock.Lock()
		a.isLeader = true
		a.leader = &a.Peer
		a.leaderLock.Unlock()
		// get the value that has the most votes
		mv := r.Val
		nv := 0
		for k, v := range instance.votes {
			if v > nv {
				mv = k
				nv = v
			}
		}
		for _, p := range a.peers {
			log.Print("Sending Accept Request Message: ", mv)
			p.Send(Msg{Type: AcceptRequest,
				FromAddress: a.addr, FromPort: a.port,
				Request: r,
				Round:   n, Value: mv}, send)
		}
		return
	}
	log.Print("Not at Quorum: ", instance.voted)
}

func (a *Agent) Nack(n int, rv RoundValue, r RequestInfo, p Peer, send bool) {
	log.Print("NACK: ", n, rv, r, p, send)
	a.leaderLock.Lock()
	a.isLeader = false
	a.leaderLock.Unlock()
	instance := &a.instances[r.Entry]
	// if we have recieved a nack for a greater round than this
	if rv.Round > instance.round {
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		a.StartRequest(rv.Round+1, rv.Val, r, send)
	}
	if n < instance.round {
		return
	}
	a.newRound(r.Entry)
	for _, p := range a.peers {
		p.Send(Msg{Type: Prepare,
			FromAddress: a.addr, FromPort: a.port,
			Request: r,
			Round:   instance.round}, send)
	}
}

func (a *Agent) AcceptRequest(n int, v Value, r RequestInfo, p Peer, send bool) {
	log.Print("ACCEPTREQUEST: ", n, v, r, p, send)
	instance := &a.instances[r.Entry]
	if n != instance.round {
		log.Print("Not for the right round")
		return
	}
	if instance.acceptedRound && instance.history[len(instance.history)-1].Val != v {
		log.Print("Already Accepted")
		return
	}
	instance.acceptedRound = true
	instance.AcceptedValue = v
	instance.history = append(instance.history, RoundValue{n, v})
	// if we are not responding to our own request
	if p.addr != a.addr || p.port != a.port {
		// when I send back a set request say that they are the leader
		a.leaderLock.Lock()
		a.leader = &p
		a.isLeader = false
		a.leaderLock.Unlock()
	}
	p.Send(Msg{Type: Accepted,
		FromAddress: a.addr, FromPort: a.port,
		Request: r,
		Round:   n, Value: v}, send)
}

func (a *Agent) Accepted(n int, v Value, r RequestInfo, p Peer, send bool) {
	log.Printf("ACCEPTED: %+v", r)
	instance := &a.instances[r.Entry]
	if n != instance.round {
		return
	}
	instance.naccepted++
	if a.Quorum(instance.naccepted) {
		instance.AcceptedValue = v
		instance.history = append(instance.history, RoundValue{n, v})
		log.Print("APPENDED to History: ", v)
		log.Printf("Adding to Values: %+v %+v", r.Entry, v)
		log.Printf("Values prior: %+v", a.Values)
		a.Values.InsertAt(r.Entry, v)
		log.Printf("Values After: %+v", a.Values)
		a.clientLock.Lock()
		a.clients[r.Id].request[r.No] = r.Entry
		log.Print("Client Request: ", a.clients[r.Id])
		a.clientLock.Unlock()
		// need to add it to the history (pass around client id and reqno?)
		instance.accepted <- true
	}
}
