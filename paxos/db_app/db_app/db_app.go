// db_app is a simple example client application for paxos. It is run by the
// paxos leader node.
// Clients to the application connect directly to the paxos cluster,
// which then forwards accepted messages to this application. This application
// then sends its results back to the paxos leader node which forwards that to
// the clients.
package main

import (
	"flag"
	"log"

	"github.com/dyv/distfs/paxos"
	"github.com/dyv/distfs/paxos/db_app/db_backend"
)

// variables for the paxos flag arguments
var paxos_port string
var paxos_addr string

// Paxos expects to be able to give two flags to the main function, one to
// specify the port and one to specify the address that the paxos leader is
// running at.
func init() {
	const (
		defaultPort = "1234"
		defaultAddr = "127.0.0.1"
	)
	flag.StringVar(&paxos_port, paxos.PaxosPortFlag, "1234", "the port paxos is running on")
	flag.StringVar(&paxos_addr, paxos.PaxosAddressFlag, "127.0.0.1", "the address paxos is running on")
	log.SetFlags(log.Lshortfile)
	//log.SetOutput(ioutil.Discard)
}

// Example function for handling paxos log
func handlePaxos(db *db_backend.DB) {
	// Loop through the log entries that have been accepted by the paxos leader
	// Log entries come in order
	for m := range db.App.Log {
		// perform the received operation
		log.Println("Performing Operation:", m.Value)
		res := db.Perform(m.Value.(*db_backend.Op), false)
		// mark this entry as complete and send the result back to Paxos
		db.App.Commit(m.Entry, paxos.NewStrReq(res))
	}
}

func main() {
	log.Println("STARTING CLIENT APPLICATION")
	// Connect With Paxos Agent
	flag.Parse()
	// initialize an example in memory database
	db := db_backend.NewDB()
	// connect to the paxos cluster as an application
	db.App.RunApplication(paxos_addr, paxos_port)
	// handle the log entries that have been accepted
	handlePaxos(db)
}
