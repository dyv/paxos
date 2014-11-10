Final Repository is located at "https://github.com/dyv/distfs"
This is using Go for the primary language. Because it requires a very specific
directory structure I have provided basic scripts when running from the
"peerster/finalProject" directory. To do this run:
$ ./init.sh
$ ./build.sh
$ ./test.sh

Stage 1: Basic Implementation of Paxos Following Wikipedia Description

This protocol is the most basic of the Paxos family. Each instance of the Basic
Paxos protocol decides on a single output value. The protocol proceeds over
several rounds. A successful round has two phases. A Proposer should not
initiate Paxos if it cannot communicate with at least a Quorum of Acceptors:

Phase 1a: Prepare

A Proposer (the leader) creates a proposal identified with a number N. This
number must be greater than any previous proposal number used by this Proposer.
Then, it sends a Prepare message containing this proposal to a Quorum of
Acceptors. The Proposer decides who is in the Quorum.

Phase 1b: Promise

If the proposal's number N is higher than any previous proposal number received
from any Proposer by the Acceptor, then the Acceptor must return a promise to
ignore all future proposals having a number less than N. If the Acceptor
accepted a proposal at some point in the past, it must include the previous
proposal number and previous value in its response to the Proposer.

Otherwise, the Acceptor can ignore the received proposal. It does not have to
answer in this case for Paxos to work. However, for the sake of optimization,
sending a denial (Nack) response would tell the Proposer that it can stop its
attempt to create consensus with proposal N.

Phase 2a: Accept Request

If a Proposer receives enough promises from a Quorum of Acceptors, it needs to
set a value to its proposal. If any Acceptors had previously accepted any
proposal, then they'll have sent their values to the Proposer, who now must set
the value of its proposal to the value associated with the highest proposal
number reported by the Acceptors. If none of the Acceptors had accepted a
proposal up to this point, then the Proposer may choose any value for its
proposal.

The Proposer sends an Accept Request message to a Quorum of Acceptors with the
chosen value for its proposal.

Phase 2b: Accepted

If an Acceptor receives an Accept Request message for a proposal N, it must
accept it if and only if it has not already promised to only consider proposals
having an identifier greater than N. In this case, it should register the
corresponding value v and send an Accepted message to the Proposer and every
Learner. Else, it can ignore the Accept Request.

Note that an Acceptor can accept multiple proposals. These proposals may even
have different values in the presence of certain failures. However, the Paxos
protocol will guarantee that the Acceptors will ultimately agree on a single
value.

Rounds fail when multiple Proposers send conflicting Prepare messages, or when
the Proposer does not receive a Quorum of responses (Promise or Accepted). In
these cases, another round must be started with a higher proposal number.

Notice that when Acceptors accept a request, they also acknowledge the
leadership of the Proposer. Hence, Paxos can be used to select a leader in a
cluster of nodes.

Here is a graphic representation of the Basic Paxos protocol. Note that the
values returned in the Promise message are null the first time a proposal is
made, since no Acceptor has accepted a value before in this round.
Stage 2: Change to Multi-Paxos
Stage 3: Implement a "File System" with Read, Write
