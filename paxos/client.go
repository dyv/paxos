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
	s            net.Conn
	server       Server
	server_index int
	enc          *json.Encoder
	dec          *json.Decoder
	LeaderAddr   string
	LeaderPort   string
	// servers are "address:port" strings
	Servers      map[Server]int // map to index
	ServersSlice []Server
	Retries      int
	// ID
	id    int // id for the current client connection
	reqno int // request number
	// example value for
	example_value EncoderDecoder
}

func NewClient() *Client {
	c := new(Client)
	c.Servers = make(map[Server]int, 0)
	c.Retries = 1
	c.example_value = NewStrReq("")
	return c
}

func (c *Client) RegisterExampleValue(ex EncoderDecoder) {
	c.example_value = ex
}

func (c *Client) AddServer(addr, port string) {
	if _, ok := c.Servers[Server{addr, port}]; !ok {
		c.ServersSlice = append(c.ServersSlice, Server{addr, port})
		c.Servers[Server{addr, port}] = len(c.ServersSlice) - 1
	}
}

func (c *Client) Connect(s Server) error {
	c.server = s
	c.AddServer(s.addr, s.port)
	var err error
	// exponential backoff
	tried := startTimeout
	for i := 0; i < c.Retries; i++ {
		log.Println("Trying to Connect: ", s, i, c.Retries)
		ns, err := net.Dial("tcp", net.JoinHostPort(s.addr, s.port))
		if err == nil {
			c.s = ns
			c.server = s
			log.Println("Successfully Connected")
			break
		}
		time.Sleep(tried)
		tried *= multTimeout
		if tried > endTimeout {
			tried = endTimeout
		}
	}
	if err != nil {
		log.Println("Err != nil: Connecting To New")
		return c.ConnectNew()
	}
	log.Println("Connected:", c.s, c.server)
	c.enc = json.NewEncoder(c.s)
	c.dec = json.NewDecoder(c.s)
	// tell server we are trying to connect
	c.enc.Encode(Msg{Type: ClientConnectRequest})
	log.Println("Decoding Response:")
	var resp Msg
	err = c.dec.Decode(&resp)
	log.Println("First Response Received")
	if err != nil {
		log.Println("Error Decoding Server Response:", err)
		c.ConnectNew()
		return err
	}
	if resp.Type == ClientRedirect {
		log.Println("Redirect Received")
		err := c.s.Close()
		if err != nil {
			log.Println("ERROR: Closed Paxos Connection with Error:", err)
		}
		//c.s = nil
		return c.Redirect(resp.LeaderAddress, resp.LeaderPort)
	}
	if resp.Type == ClientConn {
		log.Println("Recieived Connection Information: ")
		c.id = resp.Request.Id
		c.reqno = resp.Request.No
		log.Println(c.id, c.reqno)
	}
	c.LeaderAddr = s.addr
	c.LeaderPort = s.port
	log.Println("Connected With Error: ", err)
	return err
}

func (c *Client) ConnectFirst() error {
	if len(c.ServersSlice) == 0 {
		return errors.New("No Servers Availible")
	}
	return c.Connect(c.ServersSlice[0])
}

func (c *Client) ConnectNew() error {
	if c.s != nil {
		err := c.s.Close()
		if err != nil {
			log.Println("Error Closing Connection")
		}
	}
	if c.s == nil {
		log.Println("c.s == nil")
		return c.ConnectFirst()
	}
	index := c.Servers[c.server]
	s := c.ServersSlice[(index+1)%len(c.ServersSlice)]
	err := c.Connect(s)
	log.Println("Connect New:", err)
	return err
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
		log.Println("Received Redirect")
		c.s.Close()
		//c.s = nil
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
