package server

import (
	"context"

	api "github.com/honganh1206/smolkafka/api/v1"
)

type Config struct {
	CommitLog CommitLog
}

var _ api.LogServer = (*grpcServer)(nil)

type grpcServer struct {
	// Must be embedded to have forward compatible implementation??
	api.UnimplementedLogServer
	*Config
}

func newgrpcServer(config *Config) (srv *grpcServer, err error) {
	srv = &grpcServer{
		Config: config,
	}
	return srv, nil
}

func (s *grpcServer) Produce(ctx context.Context, req *api.ProduceRequest) (*api.ProduceResponse, error) {
	offset, err := s.CommitLog.Append(req.Record)
	if err != nil {
		return nil, err
	}
	return &api.ProduceResponse{Offset: offset}, nil
}

func (s *grpcServer) Consume(ctx context.Context, req *api.ConsumeRequest) (*api.ConsumeResponse, error) {
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
			case api.ErrOutOfRange:
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
