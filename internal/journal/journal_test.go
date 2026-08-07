package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
)

// TestAppendOnlyAcrossReopen proves history survives a restart and sequences
// never restart. A reopened ledger that renumbered would make outcome records
// unattributable.
func TestAppendOnlyAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "journal.jsonl")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for range 3 {
		if _, err := first.Append(Record{Kind: KindCycleStarted, CycleID: "c1"}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	record, err := second.Append(Record{Kind: KindCycleFinished, CycleID: "c1"})
	if err != nil {
		t.Fatalf("append after reopen: %v", err)
	}
	if record.Sequence != 4 {
		t.Fatalf("sequence = %d, want 4 continuing the prior history", record.Sequence)
	}

	records, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("records = %d, want 4: nothing may be overwritten", len(records))
	}
}

func TestAppendStampsTimeAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer ledger.Close()

	record, err := ledger.Append(Record{Kind: KindMeasured})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if record.At.IsZero() {
		t.Fatal("append must stamp a time")
	}

	// The record must be durable before Append returns, so a read from a
	// separate handle sees it without any further flush.
	records, err := Read(path)
	if err != nil || len(records) != 1 {
		t.Fatalf("read = %d records, err = %v, want 1 durable record", len(records), err)
	}
}

func TestFilePermissionsArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer ledger.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permissions = %o, want 0600: the ledger is private local data", perm)
	}
}

func TestReadMissingFileIsEmptyNotAnError(t *testing.T) {
	records, err := Read(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("err = %v, want nil: a controller that never ran has nothing to recover", err)
	}
	if len(records) != 0 {
		t.Fatal("a missing ledger is empty")
	}
}

func TestReadRejectsCorruptLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	if err := os.WriteFile(path, []byte("{\"kind\":\"measured\"}\nnot-json\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v, want an error naming the corrupt line", err)
	}
}

func TestAppendAfterCloseIsRefused(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "journal.jsonl"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := ledger.Append(Record{Kind: KindMeasured}); err != ErrClosed {
		t.Fatalf("err = %v, want ErrClosed", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("second close = %v, want nil", err)
	}
}

func TestRoundTripPreservesTypedPayloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	ledger, _ := Open(path)
	declaration := action.Declaration{
		ID: "d1", Class: action.PatchCode, Target: "t", Intent: "i",
		Generation: "gen-1", DeclaredAt: time.Now().UTC().Truncate(time.Second),
	}
	if _, err := ledger.Append(Record{Kind: KindDeclared, Declaration: &declaration}); err != nil {
		t.Fatalf("append: %v", err)
	}
	ledger.Close()

	records, err := Read(path)
	if err != nil || len(records) != 1 || records[0].Declaration == nil {
		t.Fatalf("read = %+v, err = %v", records, err)
	}
	if records[0].Declaration.Class != action.PatchCode {
		t.Fatal("typed payload must survive the round trip")
	}
}
