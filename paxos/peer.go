package paxos

import (
	"encoding/gob"
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
		log.Println("dialing:", err)
		return nil, err
	}
	return &Peer{address, port, client}, nil
}

func (p Peer) Send(m Msg) error {
	enc := gob.NewEncoder(p.client)
	return enc.Encode(m)
}
