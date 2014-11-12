package paxos

import (
	"encoding/json"
	"errors"
	"log"
	"net"
)

type Peer struct {
	addr   string
	port   string
	client net.Conn
}

func NewPeer(address, port string) (*Peer, error) {
	client, err := net.Dial("udp", address+":"+port)
	if err != nil {
		log.Println("ERROR DIALING:", err)
		return nil, err
	}
	return &Peer{address, port, client}, nil
}

func (p Peer) Send(m Msg) error {
	log.Print("Sending to: ", p.addr, ":", p.port)
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
