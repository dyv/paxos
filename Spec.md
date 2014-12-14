# Specification for Paxos Protocol
## Goal
Replicate an application robustly across multiple nodes using Paxos. Present a clean API to clients and client applications.
## High Level Description
Paxos is a protocol for reaching consensus on a single value. This Paxos application is used for reaching consensus on multiple values, a replicated-log. This Paxos implementation uses leader election as an optimization to increase performance and reduce network latency effects. This Paxos implementation also collapses the roles of Proposer, Acceptor, and Learner, into a single role. It is commonly accepted that the distinction between these roles only becomes important when there are many instances of paxos running. Most applications of Paxos do not need this distinction, therefore, this Paxos implementation does not incur the cost of this abstraction.
This Paxos protocol specifically implements multi-paxos with collapsed-roles and leader-election. The high level description of such a protocol can be found on the Paxos wikipedia page. The terms that will be used here are the standard and can also be found on that site.
## APIs:
There are three different APIs that must be implemented to conform with this Paxos specification. First there is the Paxos API which is used internally, within the Paxos Cluster itself in order to communicate with each other, clients, and applications. Then there is the Client API which is used by clients to connect and make requests to the Paxos Cluster. Finally, there is the Application API. The application API is responsible for brokering communication between the registered client application that is replicated across the Paxos Cluster, and the Paxos Cluster itself.
### Background: Messages
There is a single, consistent message structure that is used throughout all APIs. The messages encoded using JSON and unused and zero-valued entries are omitted from the JSON structures to reduce packet size. Recognized zero-values are 0, "", and false.
Msg is the message type. The message structure contains all of the fields that could be possibly used by any communication. It always contains the type of the message, which is also described below. Messages always record the address and port from which it came, and sometimes, record that of the current leader, if there is one. Messages used also can contain a request field, which holds the information for the current client request. This allows the Paxos cluster to keep track of specific requests as they travel throughout the cluster. The request field should be considered read only and should not be altered after creation by the Proposer. Entry is used to keep track of what entry in the replicated log this message is attempting to fill. Round is used to keep track of what Paxos round we are on for this Entry. Each Paxos round starts off at 0 for each unique log entry. This allows us to think of each log entry as an instance of Paxos, while not incurring the overhead of actually creating multiple instances of Paxos. Value is used to carry the value that should be set for this round. Round value is used in Nack responses to send back the value that has already been accepted for this log entry, and the round that it was accepted for. Finally error allows the Paxos Agents to send error information to other Paxos Agents and clients.
#### Message:
```
Msg{
    Number: type,           // type is a member of MsgType
   	string: formaddress,    // ipv4 address of the sender
    string: fromport        // port of the node that sent it
    string: leaderaddress   // ipv4 address of the leader
    string: leaderport      // port of the leader
    Object: request         // request is a RequestInfo for
                            // this client transaction
    Number: entry           // log entry for this request
    Number: round           // paxos round for this entry 
    string: value           // value to assign to this entry
    Object: roundvalue      // roundvalue is a “roundValue”
                            // previous round & value accepted
    string: error           // error description
}
```
#### RequestInfo:
RequestInfo is the associated information for this client transaction. Since each Paxos transaction is initiated by a client, it has associated information regarding the transaction. RequestInfo logs the clients unique identification number which is given to it by the Paxos node is has connected to. It also logs the request number for this client. This way the Paxos node it has connected to can keep track of requests on a per client basis, while giving clients the ability to request to see their old requests (by giving an old request number).

The RequestInfo structure also contains the value that the client is trying to set. After the client has connected with the Paxos cluster, the entry number is assigned to be the next uncommitted log entry. The leader is then in charge of incrementing this entry number, and retrying the commit process until it does get committed to the replicated log.

Noset is a flag that tells the Paxos cluster that this is merely a query for a certain entry and that no value should be committed to the log. This is useful during the “catch up” phase of Paxos nodes as they must request old values, and is also useful during client application recovery. If they were in the middle of a “transaction” with the Paxos cluster, they will be able to see which entries were in fact committed to the replicated log.
```
RequestInfo{
    Number: id    // uid of the client for the request
    Number: no    // request number for this client
    String: val   // value the client is attempting to log
    Number: entry // entry in the log for this request
    Bool: noset   // true iff only response should be Nack
}   
```
#### Message Type
Message Types are used to specify the type of the Msg. Though they have strings associated with them, they are represented as integers in an enum. They are zero-indexed and are in the order as follows.

0. Empty: An empty message is an invalid message and should be ignored.
Required Fields: None
1. Prepare: Prepare is the initial Prepare request sent by the Proposer.
Required Fields: FromAddress, FromPort, Request, Round
2. Promise: Promise is one of the possible responses an Acceptor can send to the Proposer after receiving a Prepare request. If a proposer has not promised the specified entry, with a round greater than or equal to the round specified in the Prepare request to anyone else and there is no current leader, then the Acceptor should send back a Promise to the Proposer. After sending this Promise, this Acceptor should never send a Promise for this entry with a round less than or equal to the round it promised for.
Required Fields: FromAddress, FromPort, Request, Round, RoundValue.
3. Nack: The response by an Acceptor saying that it has already promised a higher round number to a different Proposer.
4. AcceptRequest: The message sent out by a Proposer, after receiving a quorum of Promises, for those nodes to accept the associated Msg.value.
5. Accepted
6. ClientRequest
7. ClientResponse
8. ClientRedirect
9. ClientConn
10. ClientConnectRequest
11. LogRequest
12. LogResponse
13. ClientApp
14. AppResponse
15. Heartbeat
16. Done
17. Error
#### Paxos API
Paxos Agents communicate with other Agents through UDP. They communicate with clients and applications through TCP.
