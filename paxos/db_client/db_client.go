package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/dyv/distfs/paxos"
)

type OpType int

const (
	GET OpType = iota
	PUT
)

type Op struct {
	T OpType
	K string
	V string
}

// an in memory Key Value Store: ie. a map
type DB struct {
	pax  *paxos.Client
	Data map[string]string
}

func NewDB() *DB {
	return &DB{paxos.NewClient(), make(map[string]string)}
}

func (db *DB) Put(op Op, rep bool) string {
	if rep {
		db.pax.NewRequest(op)
	}
	db.Data[op.K] = op.V
	return op.V
}

func (db *DB) Get(op Op, rep bool) string {
	if rep {
		db.pax.NewRequest(op)
	}
	return db.Data[op.K]
}

// perform performs the op but discards the type
func (db *DB) Perform(op Op, rep bool) string {
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

func handleConnection(conn net.Conn, db *DB) {
	dec := json.NewDecoder(conn)
	for {
		var op Op
		err := dec.Decode(&op)
		if err != nil {
			fmt.Println("Done handling connection: ", err, op)
			break
		}
		db.Perform(op, false)
	}
	return
}

var paxos_port string
var paxos_addr string

func init() {
	const (
		defaultPort = "1234"
		defaultAddr = "127.0.0.1"
	)
	flag.StringVar(&paxos_port, "paxos_port", "1234", "the port paxos is running on")
	flag.StringVar(&paxos_addr, "paxos_addr", "127.0.0.1", "the address paxos is running on")
}

func main() {
	// Connect With Paxos Agent
	db := NewDB()
	db.pax.AddServer(paxos_addr, paxos_port)
	db.pax.ConnectAddr(paxos_addr, paxos_port)
	old_vals, err := db.pax.GetLog()
	if err != nil {
		fmt.Println("Error Requesting Log from Server")
		return
	}
	for v := range old_vals {
		db.Perform(v.(Op), false)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		t := scanner.Text()
		args := strings.Fields(t)
		if len(args) < 2 {
			continue
		}
		switch args[0] {
		case "GET":
			fmt.Println(db.Get(Op{GET, args[1], ""}, true))
		case "PUT":
			if len(args) > 3 {
				fmt.Println("Invalid Number of Arguments")
				continue
			}
			fmt.Println(db.Get(Op{PUT, args[1], args[2]}, true))
		default:
			fmt.Println("Invalid Argument: %v", args[0])
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
