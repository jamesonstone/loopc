package journal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ErrClosed is returned when a journal is used after being closed.
var ErrClosed = errors.New("journal is closed")

// Journal is an append-only ledger backed by SQLite.
//
// Append-only is enforced by triggers in the schema rather than by convention,
// so an UPDATE or DELETE aborts at the engine even if it comes from outside
// this package.
type Journal struct {
	mu     sync.Mutex
	db     *sql.DB
	closed bool
}

// Open opens or creates a journal.
//
// The pool is capped at a single connection. Per-connection pragmas therefore
// stay in force for every statement, and writes serialise — which costs
// nothing here, because one action runs at a time by construction.
func Open(path string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply %s: %w", pragma, err)
		}
	}
	for _, statement := range schemaStatements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply schema: %w", err)
		}
	}
	// The ledger is private local data.
	if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
		db.Close()
		return nil, fmt.Errorf("restrict journal permissions: %w", err)
	}
	return &Journal{db: db}, nil
}

// Append writes one record and returns it with its assigned sequence.
//
// The sequence comes from AUTOINCREMENT, which never reuses a value, so a
// recorded action can always be told apart from a later one. The insert has
// committed by the time this returns.
func (j *Journal) Append(record Record) (Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return Record{}, ErrClosed
	}
	if record.At.IsZero() {
		record.At = time.Now().UTC()
	}
	record.Sequence = 0
	payload, err := json.Marshal(record)
	if err != nil {
		return Record{}, fmt.Errorf("encode record: %w", err)
	}
	result, err := j.db.Exec(
		`INSERT INTO records (kind, at, cycle_id, generation, mode, policy_hash, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		string(record.Kind), record.At.UTC().Format(time.RFC3339Nano), record.CycleID,
		record.Generation, record.Mode, record.PolicyHash, string(payload))
	if err != nil {
		return Record{}, fmt.Errorf("write record: %w", err)
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		return Record{}, fmt.Errorf("resolve record sequence: %w", err)
	}
	record.Sequence = sequence
	return record, nil
}

// All returns the complete ledger in sequence order.
func (j *Journal) All() ([]Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil, ErrClosed
	}
	return queryRecords(j.db)
}

// Sequence returns the highest sequence written.
func (j *Journal) Sequence() (int64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return 0, ErrClosed
	}
	var sequence sql.NullInt64
	if err := j.db.QueryRow(`SELECT MAX(sequence) FROM records`).Scan(&sequence); err != nil {
		return 0, fmt.Errorf("read sequence: %w", err)
	}
	return sequence.Int64, nil
}

// Close releases the database.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	return j.db.Close()
}

// Read decodes a ledger from disk without holding it open.
//
// A missing file is an empty ledger, not an error: a controller that has never
// run has nothing to recover.
func Read(path string) ([]Record, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	ledger, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer ledger.Close()
	return ledger.All()
}

// queryRecords loads and decodes every row.
//
// The stored payload is the authority for a record's contents; the indexed
// columns exist only so the engine can find rows without decoding them. The
// sequence is taken from its column because it is assigned at insert.
func queryRecords(db *sql.DB) ([]Record, error) {
	rows, err := db.Query(`SELECT sequence, payload FROM records ORDER BY sequence`)
	if err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var sequence int64
		var payload string
		if err := rows.Scan(&sequence, &payload); err != nil {
			return nil, fmt.Errorf("scan journal row: %w", err)
		}
		var record Record
		if err := json.Unmarshal([]byte(payload), &record); err != nil {
			return nil, fmt.Errorf("decode journal record %d: %w", sequence, err)
		}
		record.Sequence = sequence
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	return records, nil
}
