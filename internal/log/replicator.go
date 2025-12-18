package log

import (
	"context"
	"sync"

	api "github.com/honganh1206/smolkafka/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Replicator struct {
	// Connect to other servers with gRPC client
	DialOptions []grpc.DialOption
	LocalServer api.LogClient
	logger      *zap.Logger
	// Guard shared fields of the replicator
	// so Join(), Leave() and Close() can be called concurrently from different goroutines.
	mu sync.Mutex
	// Track peers that currently have a replication goroutine running
	servers map[string]chan struct{}
	closed  bool
	close   chan struct{}
}

// Add the given server address to list of servers for replication
func (r *Replicator) Join(name, addr string) error {
	// Protect the shared servers map
	r.mu.Lock()
	defer r.mu.Unlock()
	r.init()

	if r.closed {
		return nil
	}

	if _, ok := r.servers[name]; ok {
		// At this point the server name has already been inserted to servers map,
		// and the replicator for it has been initialized,
		// so in subsequent calls to Join(), we skip this name
		return nil
	}
	r.servers[name] = make(chan struct{})

	// Replicator shut itself down?
	// when Leave() closes the stop channel of the peer
	go r.replicate(addr, r.servers[name])

	return nil
}

func (r *Replicator) replicate(addr string, leave chan struct{}) {
	cc, err := grpc.NewClient(addr, r.DialOptions...)
	if err != nil {
		r.logError(err, "failed to dial", addr)
		return
	}
	defer cc.Close()

	// gRPC client connecting to server
	client := api.NewLogClient(cc)

	ctx := context.Background()
	// Request to consume log record at offset 0?
	// Is it just to initialize the stream here?
	stream, err := client.ConsumeStream(
		ctx,
		&api.ConsumeRequest{
			Offset: 0,
		},
	)
	if err != nil {
		r.logError(err, "failed to consume", addr)
		return
	}

	records := make(chan *api.Record)
	go func() {
		for {
			// Repeatedly consume the logs from the discovered server
			// until reaching EOF
			recv, err := stream.Recv()
			if err != nil {
				r.logError(err, "failed to receive", addr)
				return
			}
			records <- recv.Record
		}
	}()

	for {
		select {
		case <-r.close:
			return
		case <-leave:
			return
		case record := <-records:
			// Save the copy by producing to the local server
			_, err = r.LocalServer.Produce(
				ctx,
				&api.ProduceRequest{
					Record: record,
				},
			)
			if err != nil {
				r.logError(err, "failed to produce", addr)
				return
			}
		}
	}
}

func (r *Replicator) Leave(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.init()
	// Check if server exists
	if _, ok := r.servers[name]; !ok {
		return nil
	}

	close(r.servers[name])
	delete(r.servers, name)

	return nil
}

func (r *Replicator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.init()

	if r.closed {
		return nil
	}
	r.closed = true
	close(r.close)
	return nil
}

// Lazily initialize the replicator
func (r *Replicator) init() {
	if r.logger == nil {
		r.logger = zap.L().Named("replicator")
	}
	if r.servers == nil {
		r.servers = make(map[string]chan struct{})
	}
	if r.close == nil {
		r.close = make(chan struct{})
	}
}

func (r *Replicator) logError(err error, msg, addr string) {
	// Basically log the errors
	// TODO: Export the error channel and send the error to it
	// so the user can receive and handle the error
	r.logger.Error(
		msg,
		zap.String("addr", addr),
		zap.Error(err),
	)
}
