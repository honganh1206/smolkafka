package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	api "github.com/honganh1206/smolkafka/api/v1"
	"github.com/honganh1206/smolkafka/internal/auth"
	"github.com/honganh1206/smolkafka/internal/config"
	internalLog "github.com/honganh1206/smolkafka/internal/log"
	"github.com/honganh1206/smolkafka/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// Ensure config files exist (usually in ~/.proglog via 'make gencert')
	// If you encounter errors about missing files, run 'make init' and 'make gencert' in the project root.

	// 1. Setup Server
	// Listen on a random available port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer l.Close()

	serverAddr := l.Addr().String()
	fmt.Printf("Server listening on %s\n", serverAddr)

	// Setup TLS Config for Server
	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerKeyFile,
		CAFile:        config.CAFile,
		ServerAddress: serverAddr,
		Server:        true,
	})
	if err != nil {
		log.Fatalf("Failed to setup server TLS. Make sure certificates are generated (run 'make gencert'): %v", err)
	}
	serverCreds := credentials.NewTLS(serverTLSConfig)

	// Create a temporary directory for the commit log
	dir, err := os.MkdirTemp("", "smolkafka-example")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Initialize the Commit Log
	clog, err := internalLog.NewLog(dir, internalLog.Config{})
	if err != nil {
		log.Fatalf("Failed to create commit log: %v", err)
	}
	defer clog.Remove()

	// Initialize the Authorizer
	authorizer := auth.New(config.ACLModelFile, config.ACLPolicyFile)

	// Create and Start the gRPC Server
	cfg := &server.Config{
		CommitLog:  clog,
		Authorizer: authorizer,
	}
	srv, err := server.NewGRPCServer(cfg, grpc.Creds(serverCreds))
	if err != nil {
		log.Fatalf("Failed to create gRPC server: %v", err)
	}

	// Run server in a goroutine
	go func() {
		if err := srv.Serve(l); err != nil {
			// server.Stop() will close the listener and Serve returns nil or error.
			// If it returns error after Stop, it might be relevant, but usually we ignore it on shutdown.
			log.Printf("Server stopped: %v", err)
		}
	}()
	defer srv.Stop()

	// 2. Setup Client (Root Client for full access)
	// We use the RootClient certs which should have permissions in the ACL policy
	clientTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile: config.RootClientCertFile,
		KeyFile:  config.RootClientKeyFile,
		CAFile:   config.CAFile,
		Server:   false,
	})
	if err != nil {
		log.Fatalf("Failed to setup client TLS: %v", err)
	}
	clientCreds := credentials.NewTLS(clientTLSConfig)

	// Dial the server
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(clientCreds))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	client := api.NewLogClient(conn)

	// 3. Demonstrate Usage
	ctx := context.Background()

	// A. Produce a single record
	fmt.Println("\n--- Single Produce/Consume ---")
	msg := "Hello smolkafka!"
	fmt.Printf("Producing message: '%s'\n", msg)
	produceRes, err := client.Produce(ctx, &api.ProduceRequest{
		Record: &api.Record{
			Value: []byte(msg),
		},
	})
	if err != nil {
		log.Fatalf("Failed to produce: %v", err)
	}
	fmt.Printf("Produced message at offset: %d\n", produceRes.Offset)

	// B. Consume that record
	fmt.Println("Consuming message...")
	consumeRes, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produceRes.Offset,
	})
	if err != nil {
		log.Fatalf("Failed to consume: %v", err)
	}
	fmt.Printf("Consumed message: '%s' at offset: %d\n", consumeRes.Record.Value, consumeRes.Record.Offset)

	// C. Produce Stream
	fmt.Println("\n--- Stream Produce ---")
	stream, err := client.ProduceStream(ctx)
	if err != nil {
		log.Fatalf("Failed to create produce stream: %v", err)
	}

	streamMsgs := []string{"Stream 1", "Stream 2", "Stream 3"}
	for _, m := range streamMsgs {
		err := stream.Send(&api.ProduceRequest{
			Record: &api.Record{Value: []byte(m)},
		})
		if err != nil {
			log.Fatalf("Failed to send to stream: %v", err)
		}
		res, err := stream.Recv()
		if err != nil {
			log.Fatalf("Failed to receive from stream: %v", err)
		}
		fmt.Printf("Stream sent: '%s', confirmed at offset: %d\n", m, res.Offset)
	}

	fmt.Println("\nExample finished successfully.")
}
