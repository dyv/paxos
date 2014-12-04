package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dyv/paxos"
	"github.com/dyv/paxos/db_app/db_backend"
)

var KillOp = errors.New("KILL")

func parseLine(line string) (*db_backend.Op, error) {
	ws := strings.Fields(line)
	if len(ws) == 1 && ws[0] == "KILL" {
		return nil, KillOp
	} else if len(ws) == 2 && ws[0] == "GET" {
		return db_backend.GetOp(ws[1]), nil
	} else if len(ws) == 3 && ws[0] == "PUT" {
		return db_backend.PutOp(ws[1], ws[2]), nil
	}
	return nil, errors.New("Invalide Database Operation")
}

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatalln("ERROR: INVALID ARGUMENTS")
	}
	addr, err := paxos.GetAddress()
	if err != nil {
		log.Fatalln("ERROR: UNABLE TO GET LOCAL ADDRESS")
	}
	c := paxos.NewClient()
	c.RegisterExampleValue(paxos.NewStrReq("dylan"))
	for i := 1; i < len(args); i++ {
		port := args[i]
		c.AddServer(addr, port)
	}
	err = c.ConnectFirst()
	if err != nil {
		log.Fatalln("ERROR: UNABLE TO CONNECT TO PAXOS NODE")
	}
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(">> ")
		text, _ := r.ReadString('\n')
		op, err := parseLine(text)
		if err == KillOp {
			err = c.Kill()
			if err != nil {
				fmt.Println(err)
			}
			continue
		}
		if err != nil {
			fmt.Println(err)
			continue
		}
		val, err := c.NewRequest(op)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(val)
	}
}
