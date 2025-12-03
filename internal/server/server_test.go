package server

import (
	"context"
	"net"
	"os"
	"testing"

	api "github.com/honganh1206/smolkafka/api/v1"
	"github.com/honganh1206/smolkafka/internal/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestServer(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T,
		client api.LogClient,
		config *Config,
	){
		// This map of function testing name with the function is cool
		"produce/consume a message to/from the log succeeds": testProduceConsume,
		"produce/consume stream succeeds":                    testProduceConsumeStream,
		"consume past log boundary fails":                    testConsumePastBoundary,
	} {
		t.Run(scenario, func(t *testing.T) {
			client, config, teardown := setupTest(t, nil)
			// I like how we use defer for teardown() here.
			// In C# we have to put it in a separate method
			// then remember to call it after every test.
			defer teardown()
			fn(t, client, config)
		})
	}
}

func setupTest(t *testing.T, fn func(*Config)) (
	client api.LogClient, cfg *Config, teardown func(),
) {
	t.Helper()

	// Create a listener - server-side component that waits for incoming connection attempts.
	// This listener uses the Transmission Control Protocol - Data arrives sequentially and error-free.
	// Small tip? Using port 0 means the OS assigns a free port for us.
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	// This DialOption disables transport security for this specific ClientConn
	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Create a listener on the local address our server will run on.
	// This listener listens to our gRPC server for data packets.
	cc, err := grpc.NewClient(l.Addr().String(), clientOptions...)
	require.NoError(t, err)

	dir, err := os.MkdirTemp("", "server-test")
	require.NoError(t, err)

	clog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	cfg = &Config{
		CommitLog: clog,
	}
	if fn != nil {
		// Each function will interact with the commit log
		fn(cfg)
	}
	server, err := NewGrpcServer(cfg)
	require.NoError(t, err)

	go func() {
		// Go really abstracts away all the mechanism of a listener here,
		// including the ability to generate a socket (I guess?) when there is a connection between the client and the server.
		server.Serve(l)
	}()

	client = api.NewLogClient(cc)

	return client, cfg, func() {
		// Tear-down sequence
		server.Stop()
		cc.Close()
		l.Close()
		clog.Remove()
	}
}

func testProduceConsume(t *testing.T, client api.LogClient, config *Config) {
	ctx := context.Background()
	want := &api.Record{
		Value: []byte("Hello world"),
	}

	produce, err := client.Produce(
		ctx,
		&api.ProduceRequest{
			Record: want,
		},
	)
	require.NoError(t, err)

	// Here the client produces the request as well as consume the same request back
	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset,
	})
	require.NoError(t, err)
	require.Equal(t, want.Value, consume.Record.Value)
	require.Equal(t, want.Offset, consume.Record.Offset)
}

func testConsumePastBoundary(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: &api.Record{
			Value: []byte("Hello world"),
		},
	})
	require.NoError(t, err)

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		// Past the offset boundary
		Offset: produce.Offset + 1,
	})
	if consume != nil {
		t.Fatal("consume not nil")
	}
	got := status.Code(err)
	want := status.Code(api.ErrOffsetOutOfRange{}.GRPCStatus().Err())

	if got != want {
		t.Fatalf("got err: %v, want: %v", got, want)
	}
}

func testProduceConsumeStream(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	records := []*api.Record{
		{
			Value:  []byte("1st message"),
			Offset: 0,
		},
		{
			Value:  []byte("2nd message"),
			Offset: 1,
		},
	}

	// Test produce request and consume it itself
	{
		// Standaline lexical scope. Keep certain variables within that block only.
		stream, err := client.ProduceStream(ctx)
		require.NoError(t, err)

		for offset, record := range records {
			// Each stream is responsible for a request?
			err = stream.Send(&api.ProduceRequest{
				Record: record,
			})
			require.NoError(t, err)
			// Stream produces the request and consumes the request by itself,
			// since with ProduceStream we have two streams operating indepedently?
			res, err := stream.Recv()
			require.NoError(t, err)
			if res.Offset != uint64(offset) {
				t.Fatalf(
					"got offset: %d, want: %d",
					res.Offset,
					offset,
				)
			}
		}
	}

	// Test consume only
	{
		stream, err := client.ConsumeStream(
			ctx,
			&api.ConsumeRequest{Offset: 0})
		require.NoError(t, err)

		for i, record := range records {
			// Only receive, no send since consume stream
			res, err := stream.Recv()
			require.NoError(t, err)
			require.Equal(t, res.Record, &api.Record{
				Value:  record.Value,
				Offset: uint64(i),
			})
		}
	}
}
