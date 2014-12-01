package paxos

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"time"
)

// Client is the user-side struct that connects with
// the distributed Paxos store
type Client struct {
	// server is the node that it connects to
	s          net.Conn
	server     Server
	enc        *json.Encoder
	dec        *json.Decoder
	LeaderAddr string
	LeaderPort string
	// servers are "address:port" strings
	Servers map[Server]bool
	Retries int
	// ID
	id    int // id for the current client connection
	reqno int // request number
	// example value for
	example_value EncoderDecoder
}

func NewClient() *Client {
	c := new(Client)
	c.Servers = make(map[Server]bool, 0)
	c.Retries = 10
	c.example_value = NewStrReq("")
	return c
}

func (c *Client) RegisterExampleValue(ex EncoderDecoder) {
	c.example_value = ex
}

func (c *Client) AddServer(addr, port string) {
	c.Servers[Server{addr, port}] = true
}

func (c *Client) Connect(s Server) error {
	c.server = s
	c.Servers[s] = true
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
	c.enc = json.NewEncoder(c.s)
	c.dec = json.NewDecoder(c.s)
	var resp Msg
	err = c.dec.Decode(&resp)
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
	c.LeaderAddr = s.addr
	c.LeaderPort = s.port
	return err
}

func (c *Client) ConnectFirst() error {
	for s := range c.Servers {
		return c.Connect(s)
	}
	return errors.New("No Servers")
}

func (c *Client) ConnectNew() error {
	for s := range c.Servers {
		if c.server != s {
			return c.Connect(s)
		}
	}
	return errors.New("No New Server Found")
}

func (c *Client) ConnectAddr(addr, port string) error {
	return c.Connect(Server{addr, port})
}

func (c *Client) Redirect(addr, port string) error {
	var err error
	c.AddServer(addr, port)
	c.Connect(Server{addr, port})
	return err
}

// this way they can also query for old values that they entered
func (c *Client) RequestId(id int, val EncoderDecoder) (interface{}, error) {
	log.Printf("CLIENT REQUEST: %v %v", id, val)
request:
	v, err := val.Encode()
	if err != nil {
		log.Print("Error Encoding Client Request")
		return nil, err
	}
	var m Msg = Msg{Type: ClientRequest,
		Request: RequestInfo{c.id, id, Value(v), 0, false}}
	log.Print("Client Sending Request: ", m)

	err = c.enc.Encode(m)
	if err != nil {
		log.Print("Error encoding client request:", err)
		return nil, err
	}
	var resp Msg
	err = c.dec.Decode(&resp)
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
	nv := c.example_value
	nv.Decode([]byte(resp.Value))
	return nv, err
}

func (c *Client) NewRequest(val EncoderDecoder) (interface{}, error) {
	log.Printf("Sending New Request with ID: %v %v", c.reqno, val)
	v, err := c.RequestId(c.reqno, val)
	c.reqno++
	return v, err
}

func (c *Client) Request(val EncoderDecoder) (interface{}, error) {
	return c.RequestId(c.reqno, val)
}
