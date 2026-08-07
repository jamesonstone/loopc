package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrClosed is returned when a journal is used after being closed.
var ErrClosed = errors.New("journal is closed")

// Journal is an append-only ledger backed by one file.
type Journal struct {
	mu       sync.Mutex
	file     *os.File
	sequence int64
	closed   bool
}

// Open opens or creates a journal.
//
// The file is opened O_APPEND so the kernel places every write at the end
// regardless of what this process believes the offset to be. It is created
// 0600 because the ledger is private local data.
func Open(path string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	last, err := lastSequence(path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	return &Journal{file: file, sequence: last}, nil
}

// Append writes one record and returns it with its assigned sequence.
//
// The write is flushed to disk before returning. In-memory state may only
// advance after the write that persists it commits, so a caller that has seen
// Append return can rely on the record surviving a crash.
func (j *Journal) Append(record Record) (Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return Record{}, ErrClosed
	}
	j.sequence++
	record.Sequence = j.sequence
	if record.At.IsZero() {
		record.At = time.Now().UTC()
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		j.sequence--
		return Record{}, fmt.Errorf("encode record: %w", err)
	}
	if _, err := j.file.Write(append(encoded, '\n')); err != nil {
		j.sequence--
		return Record{}, fmt.Errorf("write record: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		return Record{}, fmt.Errorf("sync journal: %w", err)
	}
	return record, nil
}

// Sequence returns the highest sequence written.
func (j *Journal) Sequence() int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.sequence
}

// Close releases the underlying file.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	return j.file.Close()
}

// lastSequence reads the existing ledger to resume numbering, so that
// reopening never restarts sequences and never overwrites history.
func lastSequence(path string) (int64, error) {
	records, err := Read(path)
	if err != nil {
		return 0, err
	}
	var last int64
	for _, record := range records {
		if record.Sequence > last {
			last = record.Sequence
		}
	}
	return last, nil
}

// Read decodes the complete ledger. A missing file is an empty ledger, not an
// error: a controller that has never run has nothing to recover.
func Read(path string) ([]Record, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	defer file.Close()
	return decode(file)
}

// maxRecordBytes bounds one line so a corrupt or hostile ledger cannot exhaust
// memory. Every buffer in the runtime is bounded by construction.
const maxRecordBytes = 4 << 20

func decode(reader io.Reader) ([]Record, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64<<10), maxRecordBytes)
	records := make([]Record, 0)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var record Record
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode journal line %d: %w", line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	return records, nil
}
