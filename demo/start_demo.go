package main

func main() {
	// start up 3 paxos agents
	// start_paxos 12345 12346 12347
	// start_paxos 12346 12345 12347
	// start_paxos 12347 12345 12346
	// start_client 12345
	// >> PUT first, dylan
	// >> GET first
	// >> PUT last, dylan
	// >> GET last
	// >> KILL
	// >> GET first
	// >> GET last
}
