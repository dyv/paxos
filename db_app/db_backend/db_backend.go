package db_backend

import (
	"encoding/json"
	"fmt"

	"github.com/dyv/paxos"
)

type OpType int

const (
	GET OpType = iota
	PUT
)

func (o OpType) String() string {
	switch o {
	case GET:
		return "GET"
	case PUT:
		return "PUT"
	default:
		return "ERROR"
	}
}

type Op struct {
	T OpType
	K string
	V string
}

func (o *Op) Encode() ([]byte, error) {
	return json.Marshal(*o)
}

func (o *Op) Decode(b []byte) error {
	return json.Unmarshal(b, o)
}

// an in memory Key Value Store: ie. a map
type DB struct {
	App  *paxos.App
	Data map[string]string
}

func NewDB() *DB {
	db := &DB{paxos.NewApp(), make(map[string]string)}
	db.App.RegisterExampleValue(GetOp("ex_key_app"))
	return db
}

func (db *DB) Put(op *Op, rep bool) string {
	// make a request for this operation to be done. But only once it is, and
	// we have received the log for it then we are good
	db.Data[op.K] = op.V
	return op.V
}

func PutOp(k string, v string) *Op {
	return &Op{PUT, k, v}
}

func (db *DB) Get(op *Op, rep bool) string {
	return db.Data[op.K]
}

func GetOp(k string) *Op {
	return &Op{GET, k, ""}
}

// perform performs the op but discards the type
// rep indicates whether we want this to be replicated on the paxos cluster
func (db *DB) Perform(op *Op, rep bool) string {
	switch op.T {
	case PUT:
		return db.Put(op, rep)
	case GET:
		return db.Get(op, rep)
	default:
		fmt.Printf("INVALID OPERATION: %+v\n", op.T)
	}
	return ""
}
