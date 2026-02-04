package loadbalance

import (
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

// Pick a server from the servers discovered by the resolver
// to handle each RPC

var _ base.PickerBuilder = (*Picker)(nil)

type Picker struct {
	mu sync.RWMutex
	leader balancer.SubConn
	followers []balancer.SubConn
	current uint64
}

// Build() sets up the picker with a map of subconnections.
func (p *Picker) Build(buildInfo base.PickerBuildInfo) balancer.Picker {
	p.mu.Lock()
	defer p.mu.Unlock()
	var followers []balancer.SubConn
	for sc, scInfo := range buildInfo.ReadySCs {
		isLeader := scInfo.Address.Attributes.Value("is_leader").(bool)
		if isLeader {
			p.leader = sc
			continue
		}
		followers = append(followers, sc)
	}
	p.followers = followers
	return p
}

var _ balancer.Picker = (*Picker)(nil)

func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error){
	p.mu.Lock()
	defer p.mu.Unlock()
	var result balancer.PickResult
	if strings.Contains(info.FullMethodName, "Produce") || len(p.followers) == 0 {
		result.SubConn = p.leader
	} else if strings.Contains(info.FullMethodName, "Consume") {
		// TODO: Short-circuit to else?
		result.SubConn = p.nextFollower()
	}

	if result.SubConn == nil {
		return result, balancer.ErrNoSubConnAvailable
	}
	return result, nil
}

// Balance the consume calls across the followers
// with the round-robin algorithm
func (p *Picker) nextFollower() balancer.SubConn {
	cur := atomic.AddUint64(&p.current, uint64(1))
	len := uint64(len(p.followers))
	idx := int(cur % len)
	return p.followers[idx]
}

func init() {
	// gRPC provides a base balancer,
	// and this base balancer takes input from gRPC, manages subconnections 
	// and collects and aggregates connectivity states?
	balancer.Register(base.NewBalancerBuilder(Name, &Picker{}, base.Config{}))
}
