package paxos

// the client essentially tries to replicate the log locally
import (
	"container/heap"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// RequestResponse is used to manage the client Application. It is used to
// indicate when the response is done, what value is associated with it, and if
// there is an error.
type RequestResponse struct {
	done chan bool
	V    EncoderDecoder
	E    error
}

type EncoderDecoder interface {
	Encode() ([]byte, error)
	Decode([]byte) error
}

type StringRequest struct {
	s *string
}

func NewStrReq(s string) *StringRequest {
	str := StringRequest{&s}
	return &str
}

func (s *StringRequest) Encode() ([]byte, error) {
	return []byte(*s.s), nil
}

func (s *StringRequest) Decode(bs []byte) error {
	log.Printf("Decoding from: %+v\n", string(bs))
	str := string(bs)
	*s.s = str
	return nil
}

func (s *StringRequest) String() string {
	return *s.s
}

type AppLogEntry struct {
	Value interface{}
	Entry int
}

// App is the handler for the application that is being replicated by Paxos.
type App struct {
	// server is the node that it connects to
	s    net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	msgs chan Msg
	cv   *sync.Cond
	mq   *MsgQueue
	Log  chan AppLogEntry
	// requests ensures that for each request with reqno we first process the
	// log, then tell the request that it has completed
	requestsLock *sync.Mutex
	requests     map[int]*RequestResponse
	// servers are the paxos servers we are trying to connect with
	Leader Server
	// Retries is how many times we are willing to retry connection
	Retries int
	//
	id            int
	reqno         int
	example_value EncoderDecoder
}

func NewApp() *App {
	c := new(App)
	c.Retries = 1
	c.msgs = make(chan Msg)
	c.Log = make(chan AppLogEntry)
	c.requestsLock = &sync.Mutex{}
	c.cv = sync.NewCond(&sync.Mutex{})
	c.requests = make(map[int]*RequestResponse)
	q := make(MsgQueue, 0, 100)
	c.mq = &q
	heap.Init(c.mq)
	c.example_value = NewStrReq("")
	return c
}

func (c *App) RegisterExampleValue(ex EncoderDecoder) {
	c.example_value = ex
}

func (c *App) Connect(s Server) error {
	c.Leader = s
	var err error
	// exponential backoff
	tried := startTimeout
	for i := 0; i < c.Retries; i++ {
		log.Print("Dialing: ", net.JoinHostPort(s.addr, s.port))
		c.s, err = net.Dial("tcp", net.JoinHostPort(s.addr, s.port))
		if err == nil {
			log.Print("Successfully Connected")
			break
		}
		log.Print("Unable To Connect")
		time.Sleep(tried)
		tried *= multTimeout
		if tried > endTimeout {
			tried = endTimeout
		}
	}
	if err != nil {
		log.Print("Unable to Connect To Server:", err)
	}
	c.dec = json.NewDecoder(c.s)
	c.enc = json.NewEncoder(c.s)
	c.enc.Encode(Msg{Type: ClientConnectRequest})
	var resp Msg
	err = c.dec.Decode(&resp)
	if err != nil {
		log.Print("Error Decoding Server Response:", err)
		return err
	}
	if resp.Type == ClientRedirect {
		log.Fatal("Redirect: Terminating Client Application")
	}
	if resp.Type == ClientConn {
		log.Print("Recieived Connection Information: ")
		c.id = resp.Request.Id
		c.reqno = resp.Request.No
		log.Print(c.id, c.reqno)
	}
	go c.handleResponses()
	return err
}

func (c *App) ConnectAddr(addr, port string) error {
	return c.Connect(Server{addr, port})
}

func (c *App) RunApplication(server, port string) {
	log.Print("Connecting With Application")
	c.ConnectAddr(server, port)
	c.enc = json.NewEncoder(c.s)
	// ClientApp message starts the Log Streaming
	err := c.enc.Encode(Msg{Type: ClientApp})
	if err != nil {
		log.Println("Error Encoding Client Application Request")
	}
}

// Invariant: Log entries always come in order from the Paxos Cluster starting
// with 0
func (c *App) handleResponses() {
	for {
		var resp Msg
		log.Printf("Decoding Message: \n")
		err := c.dec.Decode(&resp)
		if err != nil {
			log.Print("Error Decoding Server Response:", err)
			// if the server dies exit
			if err == io.EOF {
				os.Exit(0)
			}
			continue
		}
		log.Printf("Decoded Message: %+v\n", resp)
		switch resp.Type {
		case LogResponse:
			c.requests[resp.Request.Entry] = &RequestResponse{make(chan bool), nil, nil}

			// decode the transmitted value and store that instead
			nv := c.example_value
			log.Printf("Pre Decode: %+v, %+v\n", nv, resp.Value)
			err := nv.Decode([]byte(resp.Value))
			log.Printf("Post Decode: %+v\n", nv)
			if err != nil {
				log.Print("Error Decoding Value")
				continue
			}
			var ape AppLogEntry
			ape.Value = nv
			ape.Entry = resp.Request.Entry
			c.Log <- ape
			c.requestsLock.Lock()
			<-c.requests[resp.Request.Entry].done
			// send the server the response
			bs, err := c.requests[resp.Request.Entry].V.Encode()
			if err != nil {
				log.Print("Error Encoding Value")
				continue
			}
			log.Printf("Sending Up Value: %+v\n", string(bs))
			c.enc.Encode(Msg{Type: AppResponse,
				Value:   Value(bs),
				Entry:   resp.Request.Entry,
				Request: resp.Request})
			delete(c.requests, resp.Request.Entry)
			c.requestsLock.Unlock()
		case ClientRedirect:
			// redirects are fatal, it means the leader is not who we think
			log.Fatal("Redirect Application Fatal: ", err)
		default:
			log.Printf("Bad Message: ERR: %+v, MSG: %+v", err, resp)
		}
	}
}

// only once a log has been committed (for a given entry) will the request
// allowed to proceed
func (c *App) Commit(entry int, res EncoderDecoder) {
	c.requests[entry].V = res
	c.requests[entry].done <- true
}
