package log

import "github.com/hashicorp/raft"

type Config struct {
	Raft struct {
		raft.Config
		// Fully qualified domain name
		// so the node can advertise itself to its cluster and its client
		BindAddr string
		StreamLayer *StreamLayer
		Bootstrap bool
	}
	Segment struct {
		MaxStoreBytes uint64
		MaxIndexBytes uint64
		InitialOffset uint64
	}
}
