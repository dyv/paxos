package paxos

type MsgType uint

const (
	Prepare MsgType = iota
	Promise
	Nack
	AcceptRequest
	Accepted
	ClientRequest
	ClientResponse

	Error
)

func (m MsgType) String() string {
	switch m {
	case Prepare:
		return "Prepare"
	case Promise:
		return "Promise"
	case Nack:
		return "Nack"
	case AcceptRequest:
		return "AcceptRequest"
	case Accepted:
		return "Accepted"
	case ClientRequest:
		return "ClientRequest"
	case ClientResponse:
		return "ClientResponse"
	case Error:
		return "Error"
	}
	return "INVALID"
}

type Msg struct {
	Type    MsgType       `json:"type"`
	Address string        `json:"address"`
	Port    string        `json:"port"`
	Args    []interface{} `json:"args"`
}
