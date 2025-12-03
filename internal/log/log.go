package log

import (
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	api "github.com/honganh1206/smolkafka/api/v1"
)

// NOTE: We still can have uppercase and lowercase fields mixed :)
type Log struct {
	mu            sync.RWMutex
	Dir           string
	Config        Config
	activeSegment *segment
	segments      []*segment
}

type originReader struct {
	*store
	off int64
}

// Read the physical bytes from the disk and increment the offset?
// NOTE: This satisfy the io.Reader interface
func (o *originReader) Read(p []byte) (int, error) {
	n, err := o.ReadAt(p, o.off)
	o.off += int64(n)
	return n, err
}

func NewLog(dir string, c Config) (*Log, error) {
	if c.Segment.MaxStoreBytes == 0 {
		c.Segment.MaxStoreBytes = 1024
	}
	if c.Segment.MaxIndexBytes == 0 {
		c.Segment.MaxIndexBytes = 1024
	}

	l := &Log{
		Dir:    dir,
		Config: c,
	}

	return l, l.setup()
}

func (l *Log) Append(record *api.Record) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	off, err := l.activeSegment.Append(record)
	if err != nil {
		return 0, err
	}

	if l.activeSegment.IsMaxed() {
		err = l.newSegment(off + 1)
	}

	return off, nil
}

func (l *Log) Read(off uint64) (*api.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var s *segment
	for _, segment := range l.segments {
		if segment.baseOffset <= off && off < segment.nextOffset {
			// The right segment :)
			s = segment
			break
		}
	}

	if s == nil || s.nextOffset <= off {
		return nil, api.ErrOffsetOutOfRange{Offset: off}
	}

	return s.Read(off)
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, segment := range l.segments {
		if err := segment.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (l *Log) Remove() error {
	if err := l.Close(); err != nil {
		return err
	}
	return os.RemoveAll(l.Dir)
}

func (l *Log) Reset() error {
	if err := l.Remove(); err != nil {
		return err
	}
	return l.setup()
}

func (l *Log) LowestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.segments[0].baseOffset, nil
}

func (l *Log) HighestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	off := l.segments[len(l.segments)-1].nextOffset
	if off == 0 {
		return 0, nil
	}
	return off - 1, nil
}

func (l *Log) Truncate(lowest uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var segments []*segment
	for _, s := range l.segments {
		if s.nextOffset <= lowest+1 {
			// Remove the only segment
			if err := s.Remove(); err != nil {
				return err
			}
			continue
		}
		segments = append(segments, s)
	}
	// Really nice engineering here IMO:
	// To avoid data races, we use mutex and give each thread/goroutine here local variables for mutation (if this method is invoked by different goroutines btw)
	// then join the result when we are done?
	l.segments = segments
	return nil
}

// Needed when we implement coordinate consensus (multiple systems work together to agree on a common value, state or action)
// and support snapshots + restore a log
func (l *Log) Reader() io.Reader {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Reader for each segment?
	readers := make([]io.Reader, len(l.segments))
	for i, segment := range l.segments {
		// Why origin? Do we reuse readers for different segments?
		readers[i] = &originReader{segment.store, 0}
	}
	return io.MultiReader(readers...)
}

// The log bootstrapping itself
func (l *Log) setup() error {
	files, err := os.ReadDir(l.Dir)
	if err != nil {
		return err
	}
	var baseOffsets []uint64

	for _, file := range files {
		offStr := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		// Parse string to uint of base 10 (bit size 0 why?)
		off, _ := strconv.ParseUint(offStr, 10, 0)
		baseOffsets = append(baseOffsets, off)
	}

	// Sort the slice given the less function (2nd argument)
	// We are doing functional programming here huh?
	// NOTE: I know that I should use slices.Sort
	// but there are problems related to uint64 not compatible with slices.Compare
	// so let's hold it for now
	sort.Slice(baseOffsets, func(i, j int) bool {
		return baseOffsets[i] < baseOffsets[j]
	})

	for i := 0; i < len(baseOffsets); i++ {
		// Check able to create segments from log?
		if err = l.newSegment(baseOffsets[i]); err != nil {
			return err
		}
		// baseOffset contains dup for index and store (what and how?) so we skip the dup
		// Does it mean when we check segment for index we also check for store?
		// So we don't have to do it twice here?
		i++
	}

	// Log is new, so we bootstrap the initial segment
	if l.segments == nil {
		if err = l.newSegment(
			l.Config.Segment.InitialOffset,
		); err != nil {
			return err
		}
	}

	return nil
}

func (l *Log) newSegment(off uint64) error {
	s, err := newSegment(l.Dir, off, l.Config)
	if err != nil {
		return err
	}

	l.segments = append(l.segments, s)
	l.activeSegment = s
	return nil
}
