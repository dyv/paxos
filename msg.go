package paxos

// The type for a Paxos Message
type MsgType uint

const (
	Empty                MsgType = iota
	Prepare                      // A request to prepare
	Promise                      // A Promise to the Proposer
	Nack                         // Response to indicate that another value has already been chosen
	AcceptRequest                // A Request to Accept this value
	Accepted                     // Response to indicate that the value has been accepted
	ClientRequest                // A Request made by a client
	ClientResponse               // A Response for the client
	ClientRedirect               // Redirect the client
	ClientConn                   // A Client's Connection Info
	ClientConnectRequest         // A request to Connect with the Paxos Cluster as a Client
	LogRequest                   // Request the values of the replicated log
	LogResponse                  // An Entry to the replicated log
	ClientApp                    // A Connection Attempt by a Client Application
	AppResponse                  // The result evaluated by the application for a given Log entry
	Heartbeat                    // The Leader's heartbeat
	Done                         // Done processing: Kill the server
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

// Msg Contains the information for each paxos message
type Msg struct {
	Type          MsgType     `json:"type,omitempty"`          // The Type of the Message
	FromAddress   string      `json:"fromaddress,omitempty"`   // The address of who sent it
	FromPort      string      `json:"fromport,omitempty"`      // The port of who sent it
	LeaderAddress string      `json:"leaderAddress,omitempty"` // The leader's address
	LeaderPort    string      `json:"leaderPort,omitempty"`    // The leader's port
	Request       RequestInfo `json:"request,omitempty"`       // The client request info that is associated with this msg
	Entry         int         `json:"entry,omitempty"`         // what entry in the log this is meant for
	Round         int         `json:"round,omitempty"`         // The round this msg corresponds to
	Value         Value       `json:"value,omitempty"`         // The Payload for the message
	RoundValue    roundValue  `json:"roundvalue,omitempty"`    // For Previous Value
	Error         string      `json:"error,omitempty"`         // Indication if an error has occurred
}
