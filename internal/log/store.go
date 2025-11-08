package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

// Define the encoding that we persists record sizes and index entries
var enc = binary.BigEndian // MSB at lowest address

// Number of bytes used to store the record's length
const lenWidth = 8

// Wrapper to read from/write to a file
type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	size := uint64(fi.Size())
	return &store{
		File: f,
		size: size,
		buf:  bufio.NewWriter(f),
	}, nil
}

// Persist the given bytes to the store
// About the record: It has a fixed-length header followed by the payload.
func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine the offset to append to next
	pos = s.size
	// Write binary representation of p to buffered writer following byte order enc.
	// This prevents writing directly to the file itself and thus reducing system calls.
	// If we wrote a lot of small records, this would help,
	// since we are buffering the records in memory
	// and we can invoke Flush() the accumulated data to the underlying os.File.
	if err := binary.Write(s.buf, enc, uint64(len(p))); err != nil {
		return 0, 0, err
	}

	// Write the content of p to buffer, return number of bytes written
	w, err := s.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}

	// Increment by bytes written?
	w += lenWidth
	s.size += uint64(w)
	return uint64(w), pos, nil
}

// Return the record at the given position
func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Flush the accumulated data to the underlying *os.File on disk.
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	size := make([]byte, lenWidth)
	// Determine the record's length by reading the fixed-length header.
	// First we read len(size) bytes from *os.File to size starting from pos
	// to determine the size of the payload we are going to read
	// NOTE: This uses the ReadAt method below,
	// meaning it overrides the default ReadAt method of io.ReadAt?
	if _, err := s.File.ReadAt(size, int64(pos)); err != nil {
		return nil, err
	}

	b := make([]byte, enc.Uint64(size))
	// Read the record payload
	// Then we read len(b) bytes from *os.File (again) to b starting from pos + lenWidth
	if _, err := s.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	}

	// Fun (but not so new) fact: The compiler allocates byte slices that don't escape the functions they're declared in on the stack
	// so a value escapes when it lives beyond a lifetime of a function call - especially when we return the value.
	return b, nil
}

// Read len(p) bytes into p at the off offset.
// This implements the io.ReadAt on the store type
func (s *store) ReadAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Again flush content of buffer to the file
	if err := s.buf.Flush(); err != nil {
		return 0, err
	}
	return s.File.ReadAt(p, off)
}

// Persist buffered data before closing the file
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.buf.Flush()
	if err != nil {
		return err
	}
	return s.File.Close()
}
