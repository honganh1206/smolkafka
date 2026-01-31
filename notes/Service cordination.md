# Service coordination

Focus on consensus - getting distributed services to agree on shared state even in the face of failures.

Raft consensus to put servers into leader-follower relationship, where followers replicate the leader's data.

## Leader Election

The leader sends heartbeats to the followers to let them know its existence.

If followers time out heartbeat request from the leader, the followers begin an election to choose the next leader.

Candidates will continuously hold elections until there is a winner.

## Raft servers

A Raft instance consists of:

- A finite-state machine (FSM) that applies the command we give Raft
- A log store where Raft stores the commands
- A stable store where Raft stores the cluster's configs
- A snapshot store where Raft stores compact snapshots of its data
- A transport that Raft uses to connect with the server's peers

Every Raft server has a **term**: An increasing integer telling other servers how authoritative and current this server is.

Terms work as a *logical clock* to capture chronological and causal relationships in distributed systems.
The leader always has a higher term compared to its followers' to prevent vote splits.

When a candidate begins an election, it increments its term.

When there is a new leader, the followers update their terms to match each other.
The terms don't change until the next election.

Scenario: A job system with a database of jobs to run, so we initialize multiple job runners.
However, we do not want all runners to run simultaneously and duplicate the work, so we use Raft to elect the leader to run the jobs only.

In most cases, we rely on Raft for its leader selection and replication to get *consensus on state* (achieve consensus on a replicated log, then apply that log to identical state machines on each node).

We also use Raft to replicate a log of commands, then we use state machines to execute those commands.

### Log replication

The leader accepts client requests, and each request represents some command to run across the cluster.

The leader appends the command to its log, then requests its followers to append the command to their logs.

When the leader considers the command as committed (majority of followers have replicated the command), the leader executes the command with a finite-state machine(an abstract machine holding a limited number of states) and responds to the client with the result.

The leader then tracks the highest committed offset and sends this in the requests to its followers. The followers then execute all commands up to the highest committed offset with its finite-state machine.

> [!tip] Number of servers in a Raft cluster.
> Recommended number of servers in a Raft cluster is 3 (tolerate 1 server failure) and 5 (tolerate 2 server failures).
> Odd number cluster sizes are recommended because Raft will handle (N-1)/2 failures.
