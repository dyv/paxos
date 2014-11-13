package paxos

import (
	"encoding/json"
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
}

func NewClient() *Client {
	c := new(Client)
	c.Servers = make([]Server, 0)
	c.Retries = 10
	return c
}

func (c *Client) AddServer(addr, port string) {
	c.Servers = append(c.Servers, Server{addr, port})
}

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
	c.leaderAddr = s.addr
	c.leaderPort = s.port
	return err
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

func (c *Client) Request(val string) (string, error) {
request:
	enc := json.NewEncoder(c.s)
	args := []interface{}{Value(val)}
	var m Msg = Msg{ClientRequest, "0", "0", args}
	log.Print("Client Sending Request: ", m)
	err := enc.Encode(m)
	if err != nil {
		log.Print("Error encoding client request:", err)
		return "", err
	}
	dec := json.NewDecoder(c.s)
	var resp Msg
	err = dec.Decode(&resp)
	if err != nil {
		log.Print("Error Decoding Server Response:", err)
		return "", err
	}
	if resp.Type == ClientRedirect {
		c.Redirect(resp.Args[0].(string), resp.Args[1].(string))
		goto request
	}
	log.Print("Client Received Response: ", resp)
	return resp.Args[0].(string), err
}
