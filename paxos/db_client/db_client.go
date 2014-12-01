// db_client is a simple example client application for paxos. It is run by the
// paxos leader node. Clients to this Database application connect with the
// Paxos cluster directly. The paxos cluster then forwards operations to our
// database after they have been committed. It is just like another client
// application that is connected to the paxos cluster.
package main

import (
	"flag"
	"log"

	"github.com/dyv/distfs/paxos"
	"github.com/dyv/distfs/paxos/db_client/db_backend"
)

var paxos_port string
var paxos_addr string

func init() {
	const (
		defaultPort = "1234"
		defaultAddr = "127.0.0.1"
	)
	flag.StringVar(&paxos_port, "paxos_port", "1234", "the port paxos is running on")
	flag.StringVar(&paxos_addr, "paxos_addr", "127.0.0.1", "the address paxos is running on")
	log.SetFlags(log.Lshortfile)
}

func handlePaxos(db *db_backend.DB) {
	// Waiting for LOG Entries to apply
	// Log entries come in order
	for m := range db.App.Log {
		// perform the received operation
		res := db.Perform(m.Value.(*db_backend.Op), false)
		// mark this entry as complete and send the result back to the paxos
		// server
		db.App.Commit(m.Entry, paxos.NewStrReq(res))
	}
}

func main() {
	// Connect With Paxos Agent
	flag.Parse()
	log.Printf("RUNNING PAXOS APP: -paxos_addr=%s -paxos_port=%s\n", paxos_addr, paxos_port)
	log.Println("Running Paxos Client Application: db_client")
	db := db_backend.NewDB()
	log.Println("Created New Database")
	db.App.RunApplication(paxos_addr, paxos_port)
	handlePaxos(db)
}
