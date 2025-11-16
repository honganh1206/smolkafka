package log

import "os"

type segment struct {
	store *store
	index *index
	// Calculate relative offsets for index entries
	baseOffset, nextOffset uint64
	// Used to compare the store file/index sizes to the configured limits,
	// allowing us to know whether the segment is maxed out
	config Config
}

func newSegment(dir string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
	}

	var err error
	storeFile, err := os.OpenFile()
}
