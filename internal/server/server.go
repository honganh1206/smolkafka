package server

import (
	"context"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	api "github.com/honganh1206/smolkafka/api/v1"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Config struct {
	CommitLog
	Authorizer
}

const (
	objectWildcard = "*"
	produceAction  = "produce"
	consumeAction  = "consume"
)

var _ api.LogServer = (*grpcServer)(nil)

type grpcServer struct {
	// Must be embedded to have forward compatible implementation??
	api.UnimplementedLogServer
	*Config
}

// We are not tying ourself with a specific log implementation
// because we might need to pass in a different log implementation based on our needs at the time.
type CommitLog interface {
	Append(*api.Record) (uint64, error)
	Read(uint64) (*api.Record, error)
}

// Again we are not binding ourselves with a specific auth implementation
type Authorizer interface {
	Authorize(subject, object, action string) error
}

// Create a gRPC server and register our service with it
func NewGRPCServer(config *Config, grpcOpts ...grpc.ServerOption) (*grpc.Server, error) {
	logger := zap.L().Named("server")
	zapOpts := []grpc_zap.Option{
		grpc_zap.WithDurationField(
			func(duration time.Duration) zapcore.Field {
				return zap.Int64(
					"grpc.time_ns",
					duration.Nanoseconds(),
				)
			},
		),
	}

	trace.ApplyConfig(trace.Config{
		// Always record traces in detail
		// Also not to be used in prod due to performance impact
		DefaultSampler: trace.AlwaysSample(),
	})
	// What views we have here?
	// Bytes received/sent per RPC + Latency + Completed RPCs
	err := view.Register(ocgrpc.DefaultClientViews...)
	if err != nil {
		return nil, err
	}

	grpcOpts = append(grpcOpts,
		// 1st elem - Per-request (per-stream invocation) interceptor
		grpc.StreamInterceptor(
			// One interceptor out of chain of interceptors
			grpc_middleware.ChainStreamServer(
				grpc_ctxtags.StreamServerInterceptor(),
				grpc_auth.StreamServerInterceptor(authenticate),
				grpc_zap.StreamServerInterceptor(logger, zapOpts...),
			)),
		// 2nd elem - Run for every RPC invocation
		// Unary interceptor/middleware (single request - single response) performing per-request auth
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			// Side note: The gRPC integration libs for different tools are super useful :)
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_zap.UnaryServerInterceptor(logger, zapOpts...),
			grpc_auth.UnaryServerInterceptor(authenticate))))

	gsrv := grpc.NewServer(grpcOpts...)
	srv, err := newgrpcServer(config)
	if err != nil {
		return nil, err
	}

	api.RegisterLogServer(gsrv, srv)
	return gsrv, nil
}

func newgrpcServer(config *Config) (srv *grpcServer, err error) {
	srv = &grpcServer{
		Config: config,
	}
	return srv, nil
}

func (s *grpcServer) Produce(ctx context.Context, req *api.ProduceRequest) (*api.ProduceResponse, error) {
	if err := s.Authorizer.Authorize(
		// Extract subject from context?
		subject(ctx),
		objectWildcard,
		produceAction,
	); err != nil {
		return nil, err
	}

	offset, err := s.CommitLog.Append(req.Record)
	if err != nil {
		return nil, err
	}
	return &api.ProduceResponse{Offset: offset}, nil
}

func (s *grpcServer) Consume(ctx context.Context, req *api.ConsumeRequest) (*api.ConsumeResponse, error) {
	if err := s.Authorizer.Authorize(
		subject(ctx),
		objectWildcard,
		produceAction,
	); err != nil {
		return nil, err
	}
	record, err := s.CommitLog.Read(req.Offset)
	if err != nil {
		return nil, err
	}
	return &api.ConsumeResponse{Record: record}, nil
}

// Implement a bidirectional streaming RPC.
// The client can stream data to the server's log
// and the server can tell the client whether each request succeeded
func (s *grpcServer) ProduceStream(stream api.Log_ProduceStreamServer) error {
	for {
		// 1st stream as the produce stream
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		res, err := s.Produce(stream.Context(), req)
		if err != nil {
			return err
		}
		// Send the response to the client
		if err = stream.Send(res); err != nil {
			return err
		}
	}
}

// Implement a server-side streaming RPC.
// The client can tell the server where in the log to read records
// and the server will stream every record that follows (even those not in the log)
func (s *grpcServer) ConsumeStream(req *api.ConsumeRequest, stream api.Log_ConsumeStreamServer) error {
	for {
		select {
		case <-stream.Context().Done():
			// Context of stream done
			return nil
		default:
			// Tell the server which log to read records
			res, err := s.Consume(stream.Context(), req)
			switch err.(type) {
			case nil:
			case api.ErrOffsetOutOfRange:
				continue
			default:
				return err
			}
			// What if the server reaches the end of the log?
			// Then the server waits until someone appends a record to the log
			// the continue streaming records to the client
			if err = stream.Send(res); err != nil {
				return err
			}
			req.Offset++
		}
	}
}

// Interceptor that reads the subject out of the client's cert (work kinda like a middleware)
// and write it to the RPC's context
func authenticate(ctx context.Context) (context.Context, error) {
	// Extract peer information (who is on the other side of the RPC)
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return ctx, status.New(
			codes.Unknown,
			"couldn't find peer info",
		).Err()
	}
	if peer.AuthInfo == nil {
		// Nothing then return placeholder?
		return context.WithValue(ctx, subjectContextKey{}, ""), nil
	}
	// Type-assertion
	tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
	if !ok {
		// This might be extra?
		return context.WithValue(ctx, subjectContextKey{}, ""), nil
	}
	// Why at index 00? Does that mean that is where the only peer auth info is?
	subject := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName
	ctx = context.WithValue(ctx, subjectContextKey{}, subject)
	return ctx, nil
}

// Concrete type to extract client's cert's subject from the context
type subjectContextKey struct{}

// Return the client's cert's subject
func subject(ctx context.Context) string {
	// Cast it to a specific type instead of any here
	return ctx.Value(subjectContextKey{}).(string)
}
