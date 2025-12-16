# Server-to-server Service Discovery

The coolest thing :) Machines discovering other machines!

A service discovery solution maintains an up-to-date list of services, their locations and their health.

Downstream services use this list/registry to discover locations of upstream services.

In the past, we configured static address as applications run on static hardware. However, now that nodes change frequently, we use service discovery between servers.

Without using service discovery for clusters of servers, we have to manage a lot of load balancers and DNS records (might be hundreds)

## What does service discovery do?

- Manage a registry of services containing info such as IPs and ports for services to use.
- Health-check service instances.
- De-register services when they are offline.

Some stand-alone services for service-discovery are Consul, Zookeeper and Etcd.

When we use standalone service-discovery service, we don't have to build our own service-discoery service (well obviously!). However, we do have to learn how to operate it.

Why should we use it for our distributed log service? Other services might use our service to write logs, so it's better if our service knows which service ours is interacting with.

## Serf - Embed Service Discovery

We will use Serf, a library that provides decentralized cluster membership (way for nodes to monitor each other without a central coordinator), failure detection and orchestration.

Plus, we can embed it into our distributed services.

> So when to use standalone service-discovery service? Example is when we need to integrate our service discovery with many platforms, which takes lots of time.
>
> In that case, we could just use a stand-alone discovery service.

Serf is quicker for us to build our service against, and moving to a stand-alone service-discovery service from Serf is much easier as well.

### Configure Serf

1. Create a Serf node on each server.
2. Configure each Serf node with an address to listen on and accept connections from other Serf nodes.
3. Configure each Serf node with addresses of other Serf nodes.
4. Handle Serf's cluster discovery events.

Each member has a status, which is either Alive - Leaving - Left - Failed (unexpectedly leave the cluster).
