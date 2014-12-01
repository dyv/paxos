package paxos

type MsgType uint

const (
	Empty MsgType = iota
	Prepare
	Promise
	Nack
	AcceptRequest
	Accepted
	ClientRequest
	ClientResponse
	ClientRedirect
	ClientConn
	ClientConnectRequest
	LogRequest
	LogResponse
	ClientApp
	AppResponse
	Heartbeat
	Done
	Error
)

func (m MsgType) String() string {
	switch m {
	case Empty:
		return "Empty"
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
	case ClientRedirect:
		return "Redirect"
	case ClientConn:
		return "ClientConn"
	case ClientConnectRequest:
		return "ClientConnectRequest"
	case LogRequest:
		return "LogRequest"
	case LogResponse:
		return "LogResponse"
	case ClientApp:
		return "ClientApp"
	case AppResponse:
		return "AppResponse"
	case Heartbeat:
		return "Heartbeat"
	case Done:
		return "Done"
	case Error:
		return "Error"
	}
	return "INVALID"
}

// request info is the stuff that stays constant throughout all the messages
type RequestInfo struct {
	Id    int   `json:"id"`
	No    int   `json:"no"`
	Val   Value `json:"val"`
	Entry int   `json:"entry"`
	NoSet bool  `json:"noset"` // flag to indicate that we should not set the value (we are only querying)
}

type Msg struct {
	Type          MsgType     `json:"type"`
	FromAddress   string      `json:"fromaddress"`
	FromPort      string      `json:"fromport"`
	LeaderAddress string      `json:"leaderAddress"`
	LeaderPort    string      `json:"leaderPort"`
	Request       RequestInfo `json:"request"`
	Entry         int         `json:"entry"` // what entry in the log this is meant for
	Round         int         `json:"round"`
	Value         Value       `json:"value"`
	RoundValue    RoundValue  `json:"roundvalue"` // For Previous Value (is this necessary)
	Error         string      `json:"error"`
}
