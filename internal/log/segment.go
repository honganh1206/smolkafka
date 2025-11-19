package log

import (
	"fmt"
	"os"
	"path"

	api "github.com/honganh1206/smolkafka/api/v1"
	"google.golang.org/protobuf/proto"
)

// A fixed-sized chunk that make up a log.
// The basic storage unit of an append-only log file.
type segment struct {
	store *store
	index *index
	// Calculate relative offsets for index entries
	baseOffset, nextOffset uint64
	// Used to compare the store file/index sizes to the configured limits,
	// allowing us to know whether the segment is maxed out
	config Config
}

// Can be invoked when the current segment hits its maximum size
func newSegment(dir string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
	}

	var err error
	storeFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0o644, // User read and write
	)
	if err != nil {
		return nil, err
	}
	if s.store, err = newStore(storeFile); err != nil {
		return nil, err
	}
	indexFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".index")),
		os.O_RDWR|os.O_CREATE,
		0o644,
	)
	if err != nil {
		return nil, err
	}
	if s.index, err = newIndex(indexFile, c); err != nil {
		return nil, err
	}

	// Set the segment's next offset to prepare for the next record.
	// We first check the relative offset of the last record in the segment.
	if off, _, err := s.index.Read(-1); err != nil {
		// Empty index, so the next record would be the first record
		// and the record's offset would be the segment's base offset
		s.nextOffset = baseOffset
	} else {
		// Index has at least one entry,
		// so the offset of the next record should take the offset at the end of the segment.
		// Without the 1, re-opening an existing segment would set nextOffset equal to the last committed offset, thus duplicating offsets and file corruption
		s.nextOffset = baseOffset + uint64(off) + 1
	}
	return s, nil
}

func (s *segment) Append(record *api.Record) (offset uint64, err error) {
	cur := s.nextOffset
	record.Offset = cur
	p, err := proto.Marshal(record)
	if err != nil {
		return 0, err
	}

	_, pos, err := s.store.Append(p)
	if err != nil {
		return 0, err
	}
	if err = s.index.Write(
		// index offset relative to base offset
		uint32(s.nextOffset-uint64(s.baseOffset)),
		pos,
	); err != nil {
		return 0, err
	}
	s.nextOffset++
	return cur, nil
}

func (s *segment) Read(off uint64) (*api.Record, error) {
	// Read the position from the index (logical offset)
	_, pos, err := s.index.Read(int64(off - s.baseOffset))
	if err != nil {
		return nil, err
	}
	// Read the position from the store (physical offset)
	p, err := s.store.Read(pos)
	if err != nil {
		return nil, err
	}

	record := &api.Record{}
	err = proto.Unmarshal(p, record)
	return record, err
}

func (s *segment) IsMaxed() bool {
	// Either above logical capacity or physical capacity
	return s.store.size >= s.config.Segment.MaxStoreBytes || s.index.size >= s.config.Segment.MaxIndexBytes
}

func (s *segment) Remove() error {
	if err := s.Close(); err != nil {
		return err
	}
	if err := os.Remove(s.index.Name()); err != nil {
		return err
	}
	if err := os.Remove(s.store.Name()); err != nil {
		return err
	}
	return nil
}

func (s *segment) Close() error {
	if err := s.index.Close(); err != nil {
		return err
	}
	if err := s.store.Close(); err != nil {
		return err
	}
	return nil
}

// Return the nearest and lesser multiple of k in j to stay under the user's disk capacity.
// nearestMultiple(9, 4) == 8
func nearestMultiple(j, k uint64) uint64 {
	if j >= 0 {
		return (j / k) * k
	}
	return ((j - k + 1) / k) * k
}
