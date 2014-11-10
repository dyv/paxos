package paxos

type MsgType uint

const (
	Prepare MsgType = iota
	Promise
	Nack
	AcceptRequest
	Accepted
)

type Msg struct {
	Type MsgType
	Args []interface{}
}
