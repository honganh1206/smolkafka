package discovery

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/honganh1206/dynaport"
	"github.com/stretchr/testify/require"
)

// Mock to track times Membership (node?) calls the handler's Join() and Leave()
type handler struct {
	// TODO: What does this map mean?
	joins  chan map[string]string
	leaves chan string
}

func (h *handler) Join(id, addr string) error {
	if h.joins != nil {
		h.joins <- map[string]string{
			"id":   id,
			"addr": addr,
		}
	}
	return nil
}

func (h *handler) Leave(id string) error {
	if h.leaves != nil {
		h.leaves <- id
	}
	return nil
}

func TestMembership(t *testing.T) {
	// Cluster with multiple servers
	// all join a Membership
	m, handler := setupMember(t, nil)
	m, _ = setupMember(t, m)
	m, _ = setupMember(t, m)

	// Assert function passed as arg will be met in given time,
	// periodically check each tick.
	// So the idea might be first setting up, then invoke either join or leave,
	// wait for a while and recheck with eventually
	require.Eventually(t, func() bool {
		// What is this scenario?
		// Seem like at the start there is 1 membership
		// then 2 join and none leaves
		return 2 == len(handler.joins) &&
			3 == len(m[0].Members()) &&
			0 == len(handler.leaves)
	}, 3*time.Second, 250*time.Millisecond)

	require.NoError(t, m[2].Leave())

	require.Eventually(t, func() bool {
		return 2 == len(handler.joins) &&
			3 == len(m[0].Members()) &&
			// A membership has a registry storing other memberships' info. One node has info of other nodes
			serf.StatusLeft == m[0].Members()[2].Status &&
			1 == len(handler.leaves)
	}, 3*time.Second, 250*time.Millisecond)

	// Confirm Leave() has been invoked twice
	require.Equal(t, fmt.Sprintf("%d", 2), <-handler.leaves)
}

func setupMember(t *testing.T, members []*Membership) ([]*Membership, *handler) {
	// Latest index for new member
	id := len(members)
	ports := dynaport.Get(1)
	addr := fmt.Sprintf("%s:%d", "127.0.0.1", ports[0])
	tags := map[string]string{
		"rpc_addr": addr,
	}
	c := Config{
		NodeName: fmt.Sprintf("%d", id),
		BindAddr: addr,
		Tags:     tags,
	}

	h := &handler{}
	if len(members) == 0 {
		// First member of the cluster
		// Simulate production environment
		// where we specify at least 3 addresses
		// to make the cluster resilient to one or two node failures
		h.joins = make(chan map[string]string, 3)
		h.leaves = make(chan string, 3)
	} else {
		// We have a cluster to join
		c.StartJoinAddrs = []string{
			members[0].BindAddr,
		}
	}
	m, err := New(h, c)
	require.NoError(t, err)
	members = append(members, m)
	return members, h
}
