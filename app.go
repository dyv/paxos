package paxos

// the client essentially tries to replicate the log locally
import (
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

// EncoderDecoder is the interface that ensures that all values given to the
// paxos cluster can be encoded as a string for internal representation.
type EncoderDecoder interface {
	Encode() ([]byte, error)
	Decode([]byte) error
}

// StringRequest is a basic value that satisfies the EncoderDecoder interface.
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

// AppLogEntry is an entry that the application log processes.
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
	// Log is the stream of log entries that are fed into the application
	Log          chan AppLogEntry
	requestsLock *sync.Mutex
	requests     map[int]*RequestResponse
	// Leader is the leader node of the paxos server that we are connected to
	Leader Server
	// Retries is how many times we are willing to retry connection
	Retries int
	id      int
	reqno   int
	// example_value is an example of the EncoderDecoder that is processed by
	// the client application.
	example_value EncoderDecoder
}

// NewApp creates a new applciation.
func NewApp() *App {
	c := new(App)
	c.Retries = 1
	c.msgs = make(chan Msg)
	c.Log = make(chan AppLogEntry)
	c.requestsLock = &sync.Mutex{}
	c.cv = sync.NewCond(&sync.Mutex{})
	c.requests = make(map[int]*RequestResponse)
	c.example_value = NewStrReq("")
	return c
}

// RegisterExampleValue registers an example value for the application's
// reference.
func (c *App) RegisterExampleValue(ex EncoderDecoder) {
	c.example_value = ex
}

// Connect connects the application with the supplied server.
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
		if tried > maxTimeout {
			tried = maxTimeout
		}
	}
	if err != nil {
		log.Print("Unable to Connect To Server:", err)
	}
	c.dec = json.NewDecoder(c.s)
	c.enc = json.NewEncoder(c.s)
	// request to connect as a client
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

// ConnectAddr connects the application to the specified address.
func (c *App) ConnectAddr(addr, port string) error {
	return c.Connect(Server{addr, port})
}

// RunApplication starts up the application by connecting with the given paxos
// server and requesting to join as a client application.
func (c *App) RunApplication(server, port string) {
	log.Print("Connecting With Application")
	c.ConnectAddr(server, port)
	// ClientApp message starts the Log Streaming
	err := c.enc.Encode(Msg{Type: ClientApp})
	if err != nil {
		log.Println("Error Encoding Client Application Request")
	}
}

// handleResponses handles responses sent from the paxos server.
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
			// LogResponses come strictly in order. Therefore we have to
			// process them in order.
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

// Commit the given value as the result from the entry'th log entry. This
// allows us to process the next log entry.
func (c *App) Commit(entry int, res EncoderDecoder) {
	c.requests[entry].V = res
	c.requests[entry].done <- true
}
