# Discover servers and Load Balance from the Client

We enable our client to:
- Discover servers in the cluster
- Direct append calls to leader and consume calls to followers
- Balance consume calls across followers

## Three load-balancing strategies

- **Server proxying** (Most common): Client sends a request to a load balancer (query the service or registry or is the service registry) -> LB knows the servers -> LB tells the client which server to send the RPC 
- **External load balancing&&: Client queries an external LB service -> LB tells client which server to send RPC to.
- **Client-side balancing** (What we will do): Client queries a service registry to know the server -> Client picks one server and sends RPC directly.

### Load balance on the Client in gRPC

When we call `grpc.Dial()`, we pass the address to the resolver -> Resolver discovers the server

If the address we give to gRPC has multiple DNS records, gRPC will balance the requests across each of those records' servers.

gRPC use **round-robin** load balancing by default: First call to first server, 2nd call to 2nd server and so on until coming back to 1st server. 

Round-robin algorithm does not consider what we know about each request, client and server. 

Our resolver discovers the server, and our picker directs produce calls to the leader + balance consume calls across the followers.
