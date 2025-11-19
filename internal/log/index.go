package log

import (
	"io"
	"os"

	// Map files into the process's memory space
	"github.com/tysonmote/gommap"
)

var (
	// The record's offset width (logical index to identify the record's order in the log stream)
	offWidth uint64 = 4
	// The record's position width in the stable storage (physical byte position in the actual on-disk segment file)
	posWidth uint64 = 8
	// Fixed-size index entry width.
	// The Nth record lives at base address + N*entwidth
	entWidth = offWidth + posWidth
)

// The index stores both the logical offset (locate the right index entry) and the byte position (locate actual bytes of the record in the store)
type index struct {
	// Persisted index file
	file *os.File
	// In-memory index file
	mmap gommap.MMap
	// Tell us the size of the index + where to write the next entry appended to the index
	size uint64
}

// Create an index for the given file
func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}

	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	idx.size = uint64(fi.Size())
	// Grow the index file to the max index size
	// so the OS knows the backing file is large enough for potential entries
	// by padding with zeros (empty space).
	// However, this creates a problem: Last entry is no longer at the end of file.
	if err = os.Truncate(f.Name(), int64(c.Segment.MaxIndexBytes)); err != nil {
		return nil, err
	}

	// Read/write memory map over the argument file descriptor.
	// We let the index treat the file as a byte slice and expose it to the address space,
	// and read + write go through the map.
	if idx.mmap, err = gommap.Map(
		idx.file.Fd(), // Descriptor for the open file
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED, // The kernel eventually flushes writes back to the file on disk.
	); err != nil {
		return nil, err
	}

	return idx, nil
}

func (i *index) Close() error {
	// Flush the changes from the map to the persisted file synchronously
	if err := i.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	}
	// Flush contents of persisted file to stable storage (disk)
	if err := i.file.Sync(); err != nil {
		return err
	}
	// Re-truncate the file to remove empty pace (padded 0s)
	// and put last entry at the end of the file again
	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}
	return i.file.Close()
}

// Take in an offset and return the record's logical offset (out) and store position (pos) in the store
func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		// Read(-1) error here
		// maybe because we have yet to have data in the file?
		return 0, 0, io.EOF
	}
	if in == -1 {
		// Last entry?
		out = uint32((i.size / entWidth) - 1)
	} else {
		// Store offset as uint32 to save four more bytes for each entry
		out = uint32(in)
	}

	// The Nth record lives at base address + N*entWidth
	pos = uint64(out) * entWidth
	if i.size < pos+entWidth {
		// Bounds-checks
		return 0, 0, io.EOF
	}

	// Entry's offset
	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	// Record's byte position in the segment store
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+entWidth])
	return out, pos, nil
}

func (i *index) Write(off uint32, pos uint64) error {
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}
	// Encode logical offset and physical byte location to in-memory mapped index file
	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)
	// Increment the position for the next write
	i.size += uint64(entWidth)
	return nil
}

func (i *index) Name() string {
	return i.file.Name()
}
