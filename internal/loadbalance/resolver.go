package loadbalance

import (
	"context"
	"fmt"
	"sync"

	api "github.com/honganh1206/smolkafka/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// Resolver implements gRPC's resolver.Builder and resolver.Resolver interfaces
// to provide service discovery for the smolkafka cluster.
//
// How it works:
//  1. When a client dials "smolkafka://address", gRPC looks up a resolver by scheme
//  2. gRPC calls Build() which creates a connection to discover cluster members
//  3. The resolver calls GetServers() API to fetch all servers in the cluster
//  4. It passes server addresses (with leader/follower metadata) to gRPC
//  5. gRPC's load balancer (Picker) uses these addresses to route RPCs
type Resolver struct {
	mu sync.Mutex
	// clientConn is gRPC's handle to update the client with discovered servers.
	// We call clientConn.UpdateState() to tell gRPC about available servers.
	clientConn resolver.ClientConn
	// resolverConn is our own connection to a known server in the cluster,
	// used to call GetServers() and discover all cluster members.
	resolverConn *grpc.ClientConn
	// serviceConfig tells gRPC which load balancer to use (our custom "smolkafka" picker).
	serviceConfig *serviceconfig.ParseResult
	logger        *zap.Logger
}

var _ resolver.Builder = (*Resolver)(nil)

// Name is the scheme used in gRPC target URLs: "smolkafka://address"
const Name = "smolkafka"

// Build is called by gRPC when a client dials a "smolkafka://" target.
// It creates a connection to the target server and immediately resolves
// (discovers) all servers in the cluster by calling GetServers().
//
// Parameters:
//   - target: The dial target (e.g., "smolkafka://localhost:8400")
//   - cc: gRPC's client connection handle - we update this with discovered servers
//   - opts: Build options containing credentials for the resolver's own connection
func (r *Resolver) Build(
	target resolver.Target,
	cc resolver.ClientConn,
	opts resolver.BuildOptions,
) (resolver.Resolver, error) {
	r.logger = zap.L().Named("resolver")
	r.clientConn = cc
	var dialOpts []grpc.DialOption
	if opts.DialCreds != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(opts.DialCreds))
	}

	r.serviceConfig = r.clientConn.ParseServiceConfig(
		fmt.Sprintf(`loadBalancingConfig:[{"%s":{}}]`, Name),
	)

	var err error
	// Default to DNS resolver
	r.resolverConn, err = grpc.NewClient(target.URL.Host, dialOpts...)
	if err != nil {
		return nil, err
	}

	r.ResolveNow(resolver.ResolveNowOptions{})
	return r, nil
}

// Scheme returns the URI scheme that this resolver handles.
// Clients use "smolkafka://address" to trigger this resolver.
func (r *Resolver) Scheme() string {
	return Name
}

// init registers the resolver with gRPC's global resolver registry.
// This allows gRPC to find our resolver when clients dial "smolkafka://..." targets.
func init() {
	resolver.Register(&Resolver{})
}

var _ resolver.Resolver = (*Resolver)(nil)

// ResolveNow is called by gRPC to re-resolve (rediscover) servers.
// This happens on connection failures or when gRPC wants fresh server list.
//
// It calls GetServers() API and updates gRPC with the current cluster state,
// including which server is the leader (stored in address attributes).
func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()
	client := api.NewLogClient(r.resolverConn)
	ctx := context.Background()

	res, err := client.GetServers(ctx, &api.GetServersRequest{})
	if err != nil {
		r.logger.Error("failed to resolveserver", zap.Error(err))
		return
	}

	var addrs []resolver.Address
	for _, server := range res.Servers {
		addrs = append(addrs, resolver.Address{
			Addr: server.RpcAddr,
			Attributes: attributes.New("is_leader", server.IsLeader),
		})
	}

	r.clientConn.UpdateState(resolver.State{
		Addresses: addrs,
		ServiceConfig: r.serviceConfig,
	})
}

// Close cleans up the resolver's connection to the cluster.
// Called by gRPC when the client connection is closed.
func (r *Resolver) Close() {
	if err := r.resolverConn.Close(); err != nil {
		r.logger.Error("failed to close conn", zap.Error(err))
	}
}
