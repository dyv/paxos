// paxos is an implementation of the paxos algorithm for a distributed log
// system. It replicates a distributed log and supports registering
// applications to be replicated using the paxos cluster.
//
// It is implemented as multi-paxos with collapsed roles, with leader
// nodes. This means that by default all nodes are Proposers, Acceptors, and
// Learners of the log. But, with the leader optimization, if a leader is
// currently active that leader is the sole Proposer and clients will be
// redirected to it.
//
// Applications are run on the leader node. When control transfers over to a
// new leader node (which only occurs on leader timeouts), applications are
// "caught up" using the log that has been agreed on.
//
// Optimizations that are used are Nack responses (rather than no
// response), leader election, bypassing Propose/Promise steps if a leader is
// up to date, "Master Leases" (Paxos Made Live).
//
// Noticibly Absent: Dependency on Absolute Time agreement across nodes
//
// Future work includes: application snapshots to reduce "catch up" cost
//
// References:
//
// - Paxos Made Live - Chandra, Griesemer, Redstone
//
// - http://en.wikipedia.org/wiki/Paxos_%28computer_science%29
//
// - Paxos Made Simple - Lamport
//
// - The Part-Time Parliament - Lamport
package paxos
