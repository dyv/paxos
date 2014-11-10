package paxos

import "net/rpc"

type Server struct {
	addr string
	port string
}

// Client is the user-side struct that connects with
// the distributed Paxos store
type Client struct {
	// server is the node that it connects to
	s *rpc.Client
	// servers are "address:port" strings
	servers []Server
}

func (c *Client) AddServer(addr, port string) {
	c.servers = append(c.servers, Server{addr, port})
}

func (c *Client) Connect(s Server) error {
	var err error
	c.s, err = rpc.DialHTTP("tcp", s.addr+":"+s.port)
	return err
}

func (c *Client) Reconnect() error {
	var err error
	for _, s := range c.servers {
		// if we succssfully connect
		err = c.Connect(s)
		if err == nil {
			return nil
		}
	}
	return err
}

func (c *Client) Request(v string) (interface{}, error) {
	err := rpc.ErrShutdown
	var reply interface{}
	for err != rpc.ErrShutdown {
		err = c.s.Call("Agent.Request", v, &reply)
		if err == rpc.ErrShutdown {
			err = c.Reconnect()
		}
	}
	return v, err
}
