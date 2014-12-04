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
	"runtime"
	"sync"
	"time"
)

// initialize the random number generator and log format.
func init() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Lshortfile)
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// RoundValue is a pair used by Agents to store the value that they have
// accepted for a given round.
type roundValue struct {
	Round int   `json:"round,omitempty"`
	Val   Value `json:"value,omitempty"`
}

// ClientInfo stores information for a Paxos connection with a client.
type ClientInfo struct {
	id      int         // the unique identifier of a client
	reqno   int         // the highest request number seen from this client
	request map[int]int // map from the request number to the entry number (in the log)
	conn    net.Conn    // the tcp connection to this client
}

// Paxos(.*)Flag are the flags that the paxos cluster expects the client
// application to accept.
// They will be filled with the address and port respectively that the
// paxos leader is running at.
var (
	PaxosAddressFlag = "paxos_addr"
	PaxosPortFlag    = "paxos_port"
)

// LeaderTimeout specifies the amount of time after which, if no heartbeat
// message was recieved from the leader, the leader is invalidated.
var LeaderTimeout time.Duration = 1 * time.Second

// Agent collapses the multiple roles in Paxos into a single role.
// It plays the role of the Proposer, Acceptor, and Learner.
// The agent is responsible for handling the entire Replicated Log, this way
// it does not incur the overhead of an instance of paxos per log entry.
// The leader is maintained between log entries.
type Agent struct {
	// Agent and client information
	Peer                            // the peer (port + address) that represents this node
	udpaddr        *net.UDPAddr     // the udpaddress this node is listening to
	addrToPeer     map[string]Peer  // a map from peer address to Peer structure
	instances      []*PaxosInstance // the paxos instances that correspond to log entries
	instanceLock   *sync.Mutex
	servingClients bool               // indicates if agent is serving clients
	servingAgents  bool               // indicates if agent is serving other agents
	clientLock     *sync.Mutex        // lock to restrict concurrent access to client info
	clientId       int                // the next id to assign
	clients        map[int]ClientInfo // map client id to client info
	// leader information
	leaderLock    *sync.Mutex      // restricts concurrent access to leaders
	isLeader      bool             // is this current node the leader
	leader        *Peer            // who is the current leader
	leaderCurrent bool             // is the leader current (check every LeaderTimeout)
	leaderTimeout <-chan time.Time // channel to start check if leader is current
	heartbeat     <-chan time.Time // channel to initiate leader heartbeart messages
	done          chan bool        // channel to indicate that the agent should stop
	// Logs
	Messages *MsgLog   // the log of messages sent
	Values   *ValueLog // the "replicated" log of values committed
	entry    int       // the highest entry committed
	// Application
	cmdLock *sync.Mutex
	Cmd     *exec.Cmd // the command that runs the client application
	Killed  chan bool
}

// ipv4Reg matches ipv4 addresses.
var ipv4Reg = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)

// host is the cached host address used in GetAddress.
var host = "NONE"

// getAddress gets the localhosts IPv4 address.
func GetAddress() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		log.Print("Error Resolving Hostname:", err)
		return "", err
	}
	if host == "NONE" {
		as, err := net.LookupHost(name)
		if err != nil {
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
			err = errors.New("No IPv4 Address for Hostname")
		}
		return addr, err
	}
	return host, nil
}

// NewAgent creates a new agent that is located at the given port on this
// machine. try_recover specifies whether the agent should recover its state
// from the saved message logs.
func NewAgent(port string, try_recover bool) (*Agent, error) {
	var err error
	a := &Agent{}
	addr, err := GetAddress()
	if err != nil {
		log.Println("Error Getting Address:", err)
		return nil, errors.New("Cannot Resolve Local IP Address")
	}
	a.leaderLock = &sync.Mutex{}
	a.clientLock = &sync.Mutex{}
	log.Println("NEW AGENT: @", addr)
	a.addr = addr
	a.port = port
	a.udpaddr, err = net.ResolveUDPAddr("udp", net.JoinHostPort(a.addr, port))
	if err != nil {
		log.Print("Error resolving UDP address")
		return nil, err
	}
	a.addrToPeer = make(map[string]Peer)
	// vote metadata
	a.done = make(chan bool)
	a.instances = make([]*PaxosInstance, 0)
	a.instanceLock = &sync.Mutex{}
	a.Connect(a.addr, a.port)
	a.Messages, err = NewMsgLog(1000, path.Join("logs", net.JoinHostPort(a.addr, port)), a, try_recover)
	if err != nil {
		log.Print("Error Initializing Message Log: ", err)
		return nil, err
	}
	a.Values = NewValueLog(1000)
	a.clients = make(map[int]ClientInfo)
	a.cmdLock = &sync.Mutex{}
	a.Killed = make(chan bool, 2)
	return a, nil
}

// RegisterExecCmd registers the given command as the "application" command. It
// runs this command when this agent becomes leader.
func (a *Agent) RegisterExecCmd(cmd *exec.Cmd) {
	a.Cmd = cmd
}

// RegisterCmd is a thin wrapper around RegisterExecCmd. In addition to
// registering the command name with the given arguments it appends the flags
// that indicate the address and port of the paxos node. It also redirects the
// commands stdin, stdout, and stderr to point to the same location as the
// agent.
func (a *Agent) RegisterCmd(name string, args ...string) {
	args = append(args, "-"+PaxosAddressFlag+"="+a.addr)
	args = append(args, "-"+PaxosPortFlag+"="+a.port)
	a.RegisterExecCmd(exec.Command(name, args...))
	a.Cmd.Stdin = os.Stdin
	a.Cmd.Stdout = os.Stdout
	a.Cmd.Stderr = os.Stderr
}

// setupLeader starts the leader processes after this agent becomes leader. It
// runs the client program and fills in missing paxos entries.
func (a *Agent) setupLeader() {
	log.Print("SETTING UP LEADER")
	if a.Cmd != nil {
		go func() {
			log.Printf("Starting cmd: %+v\n", a.Cmd)
			a.cmdLock.Lock()
			err := a.Cmd.Start()
			a.cmdLock.Unlock()
			if err != nil {
				log.Printf("Error Running Command: %+v", err)
			}
		}()
	}
	// Rework catchup mechnism (what is the upperbound for catching up?)
	/*a.Values.Lock()
	for entry, v := range a.Values.Log {
		// If I have not received this message then get this message
		if !v.Committed {
			a.Request(v.Val, RequestInfo{Entry: entry, NoSet: true})
			//time.Sleep(10 * time.Millisecond)
		}
	}
	fmt.Println("LOG COMMITTED")
	fmt.Println(a.Values)
	a.Values.Unlock()*/
	fmt.Println("LEADER LOG:", a.Values)

}

// Close shuts down the agent. It stops serving agents and clients. It also
// shuts down the client application if one is running.
func (a *Agent) Close() {
	if a.servingAgents {
		log.Println("Closing Agent-to-Agent Connections")
		a.done <- true
		a.servingAgents = false
		log.Println("Closed")
	}
	log.Println("closing clients")
	if a.servingClients {
		log.Println("Closing Client Connections")
		a.done <- true
		a.servingClients = false
		log.Println("Closed")
	}
	log.Println("cmd shutdown")
	a.cmdLock.Lock()
	log.Println("unlocked cmd")
	if a.Cmd != nil && a.Cmd.Process != nil {
		err := a.Cmd.Process.Kill()
		if err != nil {
			log.Println("Error Killing Application Process")
		}
	}
	log.Println("cmd done")
	a.cmdLock.Unlock()
	a.leaderLock.Lock()
	a.isLeader = false
	log.Println("changed leader status")
	a.leaderLock.Unlock()
	log.Println("Shutdown Agent")
	a.Killed <- true
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
	return nil
}

// Run starts this Paxos node serving clients and agents. It runs in the
// background.
func (a *Agent) Run() error {
	err := a.ServeClients()
	if err != nil {
		log.Print("Failed to Serve Clients")
		return err
	}
	err = a.ServeAgents()
	if err != nil {
		log.Print("Failed to Serve Agents:", err)
		return err
	}
	log.Print("Server is Running at: ", net.JoinHostPort(a.addr, a.port))
	return nil
}

func (a *Agent) Wait() {
	<-a.Killed
}

// StreamLogs streams the "replicated" logs to the given connection, in order.
func (a *Agent) StreamLogs(enc *json.Encoder) {
	log.Println("Streaming Logs: ", a.Values)
	// make a.Values iterable
	stream := a.Values.Stream()
	for entry := range stream {
		log.Printf("Streaming Value: LogResponse: %+v\n", entry.Val)
		m := Msg{Type: LogResponse, Request: entry.Request, Value: entry.Val}
		log.Printf("Streaming Log Value: %+v\n", m)
		enc.Encode(m)
	}
}

// handleClientRequests handles client requests. It handles Client Apps, App
// Responses, Connection Requests, and Client Requests. It loops until the
// connection is closed.
func (a *Agent) handleClientRequest(conn net.Conn) {
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var msg Msg
		err := dec.Decode(&msg)
		if err != nil {
			log.Print("Error Decoding Request: ", err, conn.RemoteAddr(), conn.LocalAddr())
			enc.Encode("Error Decoding from Connection: " + err.Error())
			conn.Close()
			return
		}
		log.Println("Decoded Client Request:", msg)
		var resp Msg
		switch msg.Type {
		case Done:
			log.Println("Termination Request Received")
			a.Close()
			os.Exit(0)
		case ClientApp:
			log.Println("Client Application Requesting To Join Paxos Cluster")
			go a.StreamLogs(enc)
			// Start Streaming Here for this connection
		case AppResponse:
			log.Printf("Application Response Received: %#v, #%d\n", msg.Value, msg.Entry)
			a.instanceLock.Lock()
			instance := a.instances[msg.Entry]
			a.instanceLock.Unlock()
			instance.Lock()
			if instance.waiting {
				log.Println("Sending Response back to Waiting Client:", msg.Entry)
				instance.Result = msg.Value
				instance.Unlock()
				instance.resultset <- true
				continue
			}
			instance.Unlock()
		case ClientConnectRequest:
			log.Println("Received Connect Request")
			// if there is a leader then redirect to that leader
			a.leaderLock.Lock()
			if !a.isLeader && a.leader != nil {
				resp.Type = ClientRedirect
				resp.LeaderAddress = a.leader.addr
				resp.LeaderPort = a.leader.port
				a.leaderLock.Unlock()
				enc.Encode(resp)
				err := conn.Close()
				if err != nil {
					log.Println("ERROR: Paxos Closed Client Connection With Error: %v")
				}
				log.Println("Redirected Client and Closed Connection")
				return
			} else {
				log.Println("Accepted Client")
				a.leaderLock.Unlock()
				resp.Type = ClientConn
				resp.Request.No = 0
				a.clientLock.Lock()
				resp.Request.Id = a.clientId
				a.clients[a.clientId] = ClientInfo{id: a.clientId,
					reqno: 0, request: make(map[int]int), conn: conn}
				a.clientId++
				a.clientLock.Unlock()
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
				err := conn.Close()
				if err != nil {
					log.Println("ERROR: handleClientRequest: Error Closing Client Connection:", err)
				}
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
			//log.Printf("HISTORY: %+v, client no: %+v", ci, cno)
			if vi, ok := ci.request[cno]; ok {
				log.Print("Passing Back Previous Response: ", ci.request[cno])
				a.clientLock.Unlock()
				// if we have handled this request before
				resp.Type = ClientResponse
				// pass back the result that was decided on by the client
				// application
				resp.Value = a.instances[vi].Result
				break
			}
			a.clientLock.Unlock()
			a.Messages.Append(msg)
			msg.Request.Entry = a.entry
			a.entry++
			err = a.Request(msg.Value, msg.Request)
			log.Println("Returned From Request")
			if err != nil {
				log.Println("ERROR in Request:", err)
				resp.Type = Error
				resp.Error = err.Error()
			} else {
				vi := ci.request[cno]
				log.Println("Generated Entry")
				resp.Type = ClientResponse
				resp.Value = a.instances[vi].Result
				break
			}
		default:
			err = fmt.Errorf("Invalid Message Type: %v", msg.Type)
			resp.Type = Error
			resp.Error = err.Error()
		}
		resp.Request = msg.Request
		log.Print("ENCODING RESPONSE handleClientRequest")
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
	a.servingClients = true
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

// SendHeartbeat sends a heartbeat message to this agent's peers if it is the
// leader.
func (a *Agent) SendHeartbeat() {
	// if I am not a leader don't spam and send heartbeats
	a.leaderLock.Lock()
	if !a.isLeader {
		a.leaderLock.Unlock()
		return
	}
	a.leaderLock.Unlock()
	for _, p := range a.addrToPeer {
		p.Send(Msg{Type: Heartbeat, FromAddress: a.addr, FromPort: a.port}, true)
	}
}

// handlePaxosMessage handles messages received from other paxos nodes.
func (a *Agent) handlePaxosMessage(m Msg, send bool) {
	p, ok := a.addrToPeer[net.JoinHostPort(m.FromAddress, m.FromPort)]
	if !ok {
		log.Print("From Address is not on Peers List: Rejecting")
		log.Print(net.JoinHostPort(m.FromAddress, m.FromPort))
		log.Print(a.addrToPeer)
		return
	}

	switch m.Type {
	case Heartbeat:
		// it the leader is current then don't do anything
		if a.leaderCurrent {
			return
		}
		// if the leader's currency is pending then reassert it is a leadership
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
				a.leaderLock.Lock()
				if !a.leaderCurrent {
					a.leader = nil
				}
				a.leaderCurrent = false
				a.leaderLock.Unlock()
			default:
			}
			n, _, err := conn.ReadFromUDP(by)
			if err != nil {
				// if the error is a timeout just try again
				if err.(*net.OpError).Timeout() {
					continue
				}
				if err != nil {
					log.Println("Error Reading From Socket:", err)
					return
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
			//log.Print("Received Message: ", m)
			a.Messages.Append(m)
			a.handlePaxosMessage(m, true)
		}
	}(a, conn, by)
	return nil

}

// Quorum returns whether the number given signifies exactly one Quorum for
// this Agent/Proposer.
func (a *Agent) Quorum(n int) bool {
	return n == len(a.addrToPeer)/2+1
}

// RequestTimeout is returned if the client request timedout
var RequestTimeout error = errors.New("paxos: request timed out")

// Request is an RPC call that client applications can call. When a Paxos Agent
// receives a Request RPC it takes the role of the proposer. As proposer it
// sends preparation messages to a Quorum of Acceptors.
func (a *Agent) Request(value Value, r RequestInfo) error {
	log.Printf("Requesting Entry: %v", r.Entry)
	log.Printf("Request Info: %v", r)
	a.instanceLock.Lock()
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}
	instance := a.instances[r.Entry]
	a.instanceLock.Unlock()
	instance.Lock()
	instance.myvalue = value
	next_round := instance.round
	instance.Unlock()
	if !a.isLeader {
		a.StartRequest(next_round, value, r, true)
	} else {
		a.Promise(-1, roundValue{}, r, Peer{a.addr, a.port, nil}, true, true)
	}
	// wait for this proposal to be accepted (assigned a log entry) or a timeout to occur
	timeout := make(chan bool)
	go func() {
		time.Sleep(5 * time.Second)
		timeout <- true
	}()
	log.Print("WAITING ON: ", a.instances[r.Entry].accepted)
	a.instanceLock.Lock()
	accepted := a.instances[r.Entry].accepted
	a.instanceLock.Unlock()
	select {
	case <-accepted:
		a.instanceLock.Lock()
		instance := a.instances[r.Entry]
		instance.Lock()
		log.Print("ENTRY WAS ACCEPTED: ", instance)
		instance.Unlock()
		a.instanceLock.Unlock()
	case <-timeout:
		return RequestTimeout
	}
	// now we have assigned a specific log entry to this request, we send it
	// out to the client applications that are running. After one of them has
	if a.Cmd != nil {
		a.instanceLock.Lock()
		a.instances[r.Entry].waiting = true
		resultset := a.instances[r.Entry].resultset
		a.instanceLock.Unlock()
		<-resultset
		a.instanceLock.Lock()
		instance := a.instances[r.Entry]
		a.instanceLock.Unlock()
		instance.Lock()
		instance.waiting = false
		instance.Unlock()
		log.Println("REQUEST COMPLETE: Result set")
	} else {
		// when no cmd is assigned send back the accepted value as confirmation
		log.Println("NO CMD assigned to Node")
		a.instances[r.Entry].Result = a.instances[r.Entry].AcceptedValue
	}
	return nil
}

func (a *Agent) StartRequest(round int, value Value, r RequestInfo, send bool) {
	// make a new round for this specific entry
	a.newRound(r.Entry)
	a.instances[r.Entry].myvalue = value
	log.Print("Starting Client Request: Sending Prepare Message with Round: ", round)
	for _, p := range a.addrToPeer {
		p.Send(Msg{Type: Prepare,
			FromAddress: a.addr, FromPort: a.port,
			Request: r,
			Round:   round}, send)
	}
}

// noValue is the value a roundValue takes if there is no corresponding value
// assigned
var noValue roundValue = roundValue{-1, ""}

// LastAccepted is a function that returns the last RoundValue that has been
// accepted by this agent and commited to its history. If no value was
// previously committed to this agent's history it returns a NoValue.
func (a *Agent) LastAccepted(entry int) roundValue {
	l := len(a.instances[entry].history)
	if l == 0 {
		return noValue
	}
	return a.instances[entry].history[l-1]
}

// Prepare sends an either a Promise or a Nack to the peer that sent the
// Prepare request. If it corresponds with this round and this Agent has not
// accepted anything else, then it sends a Promise, which reserves this value
// to be the one that the Proposer requests. Otherwise the Agent sends a Nack
// response to the Proposer signifying that this slot has already been taken.
func (a *Agent) Prepare(n int, r RequestInfo, p Peer, send bool) {
	//log.Print("PREPARE: ", n, r, p, send)
	a.instanceLock.Lock()
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}
	instance := a.instances[r.Entry]
	a.instanceLock.Unlock()
	instance.Lock()
	defer instance.Unlock()
	if n > instance.round || (n == instance.round && instance.promisedRound == false) {
		if r.NoSet {
			log.Print("Not Allowed to Accept")
			return
		}
		instance.round = n
		instance.promisedRound = true
		instance.acceptedRound = false
		if p.addr != a.addr || p.port != a.port {
			// when I send back a set request say that they are the leader
			a.leaderLock.Lock()
			a.leader = &p
			a.isLeader = false
			a.leaderLock.Unlock()
		}

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

// Keep track of the number of promises received for each round.
// If this agent reach a quorum of promises for this round it becomes the
// leader and sends out accept messages.
func (a *Agent) Promise(n int, la roundValue, r RequestInfo, p Peer, send bool, shortcut bool) {
	a.instanceLock.Lock()
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}

	instance := a.instances[r.Entry]
	a.instanceLock.Unlock()
	if !shortcut {
		//log.Print("PROMISE: ", r.Entry, n, la, r, p, send)
		// only accept promises for the round we are currently on
		if n != instance.round {
			log.Printf("r != instance.round: %v != %v", r, instance.round)
			return
		}
		instance.round = n
		if la.Round < 0 {
			//log.Print("My Value: ", instance.myvalue)
			//log.Printf("r.Val: %+v", r.Val)
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
		if !a.isLeader {
			// Leadership is transferred to this agent
			a.isLeader = true
			a.leader = &a.Peer
			a.leaderLock.Unlock()
			if shortcut {
				log.Println("SHORTCUT")
			}
			a.setupLeader()
		} else {
			a.leaderLock.Unlock()
		}
		// get the value that has the most votes
		mv := r.Val
		nv := 0
		for k, v := range instance.votes {
			if v > nv {
				mv = k
				nv = v
			}
		}
		for _, p := range a.addrToPeer {
			p.Send(Msg{Type: AcceptRequest,
				FromAddress: a.addr, FromPort: a.port,
				Request: r,
				Round:   n, Value: mv}, send)
		}
		return
	}
}

// If an agent receives a nack response for a round greater than this then stop
// trying to assign this value and accept the other one.
func (a *Agent) Nack(n int, rv roundValue, r RequestInfo, p Peer, send bool) {
	//log.Print("NACK: ", n, rv, r, p, send)
	a.leaderLock.Lock()
	a.isLeader = false
	a.leaderLock.Unlock()
	a.instanceLock.Lock()
	instance := a.instances[r.Entry]
	a.instanceLock.Unlock()
	// if we have recieved a nack for a greater round than this
	if rv.Round > instance.round {
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		a.StartRequest(rv.Round+1, rv.Val, r, send)
	}
	if n < instance.round {
		return
	}
	a.newRound(r.Entry)
	for _, p := range a.addrToPeer {
		p.Send(Msg{Type: Prepare,
			FromAddress: a.addr, FromPort: a.port,
			Request: r,
			Round:   instance.round}, send)
	}
}

// If an agent receives an AcceptRequest and who it sends from is the leader,
// then the agent accepts this value.
func (a *Agent) AcceptRequest(n int, v Value, r RequestInfo, p Peer, send bool) {
	log.Print("ACCEPTREQUEST: ", n, v, r, p, send)
	log.Print("ENTRY:", r.Entry)
	a.instanceLock.Lock()
	if r.Entry >= len(a.instances) {
		for i := len(a.instances); i < ((r.Entry+1)*3)/2; i++ {
			a.instances = append(a.instances, NewPaxosInstance())
		}
	}
	instance := a.instances[r.Entry]
	a.instanceLock.Unlock()
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
	instance.history = append(instance.history, roundValue{n, v})
	if a.entry < r.Entry+1 {
		a.entry = r.Entry + 1
	}
	if p.addr != a.addr || p.port != a.port {
		a.Values.InsertAt(r.Entry, v, r)
	}
	// if we are not responding to our own request
	p.Send(Msg{Type: Accepted,
		FromAddress: a.addr, FromPort: a.port,
		Request: r,
		Round:   n, Value: v}, send)
}

// If a majority of nodes Accepted this response then this response is done.
func (a *Agent) Accepted(n int, v Value, r RequestInfo, p Peer, send bool) {
	log.Printf("ACCEPTED: %+v", r)
	a.instanceLock.Lock()
	instance := a.instances[r.Entry]
	a.instanceLock.Unlock()
	if n != instance.round {
		return
	}
	instance.Lock()
	defer instance.Unlock()
	instance.naccepted++
	if a.Quorum(instance.naccepted) {
		instance.AcceptedValue = v
		instance.history = append(instance.history, roundValue{n, v})
		log.Print("APPENDED to History: ", v)
		log.Printf("Adding to Values: %+v %+v", r, v)
		log.Printf("Values prior: %+v", a.Values)
		a.Values.InsertAt(r.Entry, v, r)
		log.Printf("Values After: %+v", a.Values)
		a.clientLock.Lock()
		a.clients[r.Id].request[r.No] = r.Entry
		log.Print("Client Request: ", a.clients[r.Id])
		a.clientLock.Unlock()
		// need to add it to the history (pass around client id and reqno?)
		instance.accepted <- true
	}
}
