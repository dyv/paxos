package paxos

import (
	"encoding/json"
	"errors"
	"log"
	"net"
)

// Peer represents a connection.
type Peer struct {
	addr   string
	port   string
	client net.Conn
}

// NewPeer establishes a new peer connection with the address and port
// supplied.
func NewPeer(address, port string) (*Peer, error) {
	client, err := net.Dial("udp", address+":"+port)
	if err != nil {
		log.Println("ERROR DIALING:", err)
		return nil, err
	}
	return &Peer{address, port, client}, nil
}

// Send sends a message to this peer (depending if send is true).
func (p Peer) Send(m Msg, send bool) error {
	if !send {
		return nil
	}
	//log.Print("Destination: ", p.addr, ":", p.port)
	//log.Print("Msg: ", m)
	client, err := net.Dial("udp", p.addr+":"+p.port)
	if err != nil {
		log.Println("Error Dialing Peer:", err)
		return err
	}
	enc := json.NewEncoder(client)
	if enc == nil {
		log.Print("Encoder is nil")
		return errors.New("Encoder is nil")
	}
	return enc.Encode(m)
}
