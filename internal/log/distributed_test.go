package log

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/honganh1206/dynaport"
	api "github.com/honganh1206/smolkafka/api/v1"
	"github.com/stretchr/testify/require"
)

func TestMultipleNodes(t *testing.T) {
	// Setup
	var logs []*DistributedLog
	nodeCount := 3
	ports := dynaport.Get(nodeCount)

	for i := range nodeCount {
		dataDir, err := os.MkdirTemp("", "distributed-log-test")
		require.NoError(t, err)
		defer func(dir string) {
			_ = os.RemoveAll(dir)
		}(dataDir)

		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", ports[i]))
		require.NoError(t, err)

		config := Config{}
		timeout := 50 * time.Millisecond
		config.Raft.StreamLayer = NewStreamLayer(ln, nil, nil)
		config.Raft.LocalID = raft.ServerID(fmt.Sprintf("%d", i))
		config.Raft.HeartbeatTimeout = timeout
		config.Raft.ElectionTimeout = timeout
		config.Raft.LeaderLeaseTimeout = timeout
		config.Raft.CommitTimeout = 5 * time.Millisecond

		if i == 0 {
			// 1st node bootstraps the cluster
			config.Raft.Bootstrap = true
		}

		l, err := NewDistributedLog(dataDir, config)
		require.NoError(t, err)
		if i != 0 {
			err = logs[0].Join(
				fmt.Sprintf("%d", i), ln.Addr().String(),
			)

			require.NoError(t, err)
		} else {
			// Other nodes wait for the leader to join the cluster
			err = l.WaitForLeader(3 * time.Second)
			require.NoError(t, err)
		}
		logs = append(logs, l)

		// Test #1: Check if Raft replicates the records to its followers
		records := []*api.Record{
			{Value: []byte("first")},
			{Value: []byte("second")},
		}

		for _, record := range records {
			off, err := logs[0].Append(record)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				for j := range nodeCount {
					got, err := logs[j].Read(off)
					if err != nil {
						return false
					}
					record.Offset = off
					if !reflect.DeepEqual(got.Value, record.Value) {
						return false
					}
				}
				return true
				// NOTE: Are we being generous with wait and tick time?
			}, 500*time.Millisecond, 50*time.Millisecond)
		}

		// Test #2: Test leader stop replicating to a server that has left the cluster
		err = logs[0].Leave("1")
		require.NoError(t, err)
		// Time for leader and folllowers to acknowledge the new leave?
		time.Sleep(50 * time.Millisecond)

		off, err := logs[0].Append(&api.Record{
			Value: []byte("third"),
		})
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		// Since node 1 leaves the cluster, no replication
		record, err := logs[1].Read(off)
		require.IsType(t, api.ErrOffsetOutOfRange{}, err)
		require.Nil(t, record)

		record, err = logs[2].Read(off)
		require.NoError(t, err)
		require.Equal(t, []byte("third"), record.Value)
		require.Equal(t, off, record.Offset)
	}
}
