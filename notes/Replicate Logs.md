# Replicate Logs

Store multiple copies of the log data when we have multiple servers in the cluster.

Ensure there's a copy stored in another disk when a node's disk fail.

Replication will have a defined leader-follower relationship.

Servers should replicate each other when they discover themselves.

## Pull-based replication

Consume from each discovered server by polling the data source to check for new data (Push-based means we push the data to the replica).

Produce a copy of what the data to the local server.

Pull-based systems' flexibility is great for log & message systems where _the consumers and work loads can differ_ ? (client that streams data and client that does batch processing)

Why? Each consumer can pull based on:

- When it needs to do so
- How much it wants to pull
- What it can handle

> For short-running programs, we can make a `run` package that exports the `Run()` function responsible for running the program.
>
> For long-running programs, we make an `agent` package that exports an `Agent` type managing different components and processes that make up the service.
