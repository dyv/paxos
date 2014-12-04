package main

import (
	"log"
	"os"

	"github.com/dyv/paxos"
)

func main() {
	args := os.Args
	if len(args) != 4 {
		log.Fatalln("ERROR INVALID ARGUMENTS")
	}
	my_port := args[1]
	agent, err := paxos.NewAgent(my_port, false)
	agent.RegisterCmd("db_app")
	addr, err := paxos.GetAddress()
	if err != nil {
		log.Fatalln("Error Getting Address: ", err)
	}
	for i := 2; i < len(args); i++ {
		agent.Connect(addr, args[i])
	}
	agent.Run()
	agent.Wait()
}
