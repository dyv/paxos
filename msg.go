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
	Id    int   `json:"id,omitempty"`
	No    int   `json:"no,omitempty"`
	Val   Value `json:"val,omitempty"`
	Entry int   `json:"entry,omitempty"`
	NoSet bool  `json:"noset,omitempty"` // flag to indicate that we should not set the value (we are only querying)
}

type Msg struct {
	Type          MsgType     `json:"type,omitempty"`
	FromAddress   string      `json:"fromaddress,omitempty"`
	FromPort      string      `json:"fromport,omitempty"`
	LeaderAddress string      `json:"leaderAddress,omitempty"`
	LeaderPort    string      `json:"leaderPort,omitempty"`
	Request       RequestInfo `json:"request,omitempty"`
	Entry         int         `json:"entry,omitempty"` // what entry in the log this is meant for
	Round         int         `json:"round,omitempty"`
	Value         Value       `json:"value,omitempty"`
	RoundValue    roundValue  `json:"roundvalue,omitempty"` // For Previous Value (is this necessary)
	Error         string      `json:"error,omitempty"`
}
