package paxos

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"time"
)

type Server struct {
	addr string
	port string
}

var startTimeout time.Duration = LeaderTimeout
var multTimeout time.Duration = 2
var endTimeout time.Duration = 2 * time.Minute

// Client is the user-side struct that connects with
// the distributed Paxos store
type Client struct {
	// server is the node that it connects to
	s          net.Conn
	leaderAddr string
	leaderPort string
	// servers are "address:port" strings
	Servers []Server
	Retries int
	// ID
	id    int // id for the current client connection
	reqno int // request number

}

func NewClient() *Client {
	c := new(Client)
	c.Servers = make([]Server, 0)
	c.Retries = 10
	return c
}

func (c *Client) AddServer(addr, port string) {
	for _, s := range c.Servers {
		if s.addr == addr && s.port == port {
			return
		}
	}
	c.Servers = append(c.Servers, Server{addr, port})
}

var RedirectError error = errors.New("Redirect")

func (c *Client) Connect(s Server) error {
	var err error
	// exponential backoff
	tried := startTimeout
	for i := 0; i < c.Retries; i++ {
		c.s, err = net.Dial("tcp", net.JoinHostPort(s.addr, s.port))
		if err == nil {
			log.Print("Successfully Connected")
			break
		}

		time.Sleep(tried)
		tried *= multTimeout
		if tried > endTimeout {
			tried = endTimeout
		}
	}
	dec := json.NewDecoder(c.s)
	var resp Msg
	err = dec.Decode(&resp)
	if err != nil {
		log.Print("Error Decoding Server Response:", err)
		return err
	}
	if resp.Type == ClientRedirect {
		return c.Redirect(resp.LeaderAddress, resp.LeaderPort)
	}
	if resp.Type == ClientConn {
		log.Print("Recieived Connection Information: ")
		c.id = resp.Request.Id
		c.reqno = resp.Request.No
		log.Print(c.id, c.reqno)
	}
	c.leaderAddr = s.addr
	c.leaderPort = s.port
	return err
}

func (c *Client) ConnectAddr(addr, port string) error {
	return c.Connect(Server{addr, port})
}

func (c *Client) Redirect(addr, port string) error {
	var err error
	for _, s := range c.Servers {
		// if we succssfully connect
		if s.addr == addr && s.port == port {
			err = c.Connect(s)
			return err
		}
	}
	c.AddServer(addr, port)
	c.Connect(c.Servers[len(c.Servers)-1])
	return err
}

// this way they can also query for old values that they entered
func (c *Client) RequestId(id int, val Value) (Value, error) {
	log.Printf("CLIENT REQUEST: %v %v", id, val)
request:
	enc := json.NewEncoder(c.s)
	var m Msg = Msg{Type: ClientRequest,
		Request: RequestInfo{c.id, id, val, 0, false}}
	log.Print("Client Sending Request: ", m)
	err := enc.Encode(m)
	if err != nil {
		log.Print("Error encoding client request:", err)
		return nil, err
	}
	dec := json.NewDecoder(c.s)
	var resp Msg
	err = dec.Decode(&resp)
	if err != nil {
		log.Print("Error Decoding Server Response:", err)
		return nil, err
	}
	if resp.Type == ClientRedirect {
		c.Redirect(resp.LeaderAddress, resp.LeaderPort)
		goto request
	}
	if resp.Error != "" {
		log.Print("Return Type Error")
		return nil, errors.New(resp.Error)
	}
	log.Print("Client Received Response: ", resp)
	return resp.Value, err
}

func (c *Client) NewRequest(val Value) (Value, error) {
	log.Printf("Sending New Request with ID: %v %v", c.reqno, val)
	v, err := c.RequestId(c.reqno, val)
	c.reqno++
	return v, err
}

func (c *Client) Request(val Value) (Value, error) {
	return c.RequestId(c.reqno, val)
}

func (c *Client) GetLog() (<-chan Value, error) {
	ch := make(chan Value)
	enc := json.NewEncoder(c.s)
	var m Msg = Msg{Type: LogRequest}
	log.Print("Client Sending Log Request: ", m)
	err := enc.Encode(m)
	if err != nil {
		log.Print("Error Encoding Log Request:", err)
		return nil, err
	}
	go func() {
		for {
			dec := json.NewDecoder(c.s)
			var resp Msg
			err = dec.Decode(&resp)
			if err != nil {
				log.Print("Error Decoding Server Response:", err)
				break
			}
			if resp.Type == Done {
				break
			}
			if resp.Type != LogResponse {
				log.Print("Received Invalid Response for GetLog")
				continue
			}
			ch <- resp.Value
		}
		close(ch)
	}()
	return ch, nil
}
