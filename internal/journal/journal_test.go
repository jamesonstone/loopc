package journal

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	_ "modernc.org/sqlite"
)

// TestAppendOnlyIsEnforcedByTheEngine is the load-bearing test of the ledger.
//
// Append-only must not depend on this package promising to only ever INSERT.
// The triggers are asserted through a direct connection that bypasses the
// package entirely, which is the case the guarantee exists for.
func TestAppendOnlyIsEnforcedByTheEngine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.db")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := ledger.Append(Record{Kind: KindCycleStarted, CycleID: "c1"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("direct open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`UPDATE records SET kind = 'tampered'`); err == nil {
		t.Fatal("UPDATE must abort: the ledger is append-only")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE err = %v, want the append-only abort", err)
	}

	if _, err := db.Exec(`DELETE FROM records`); err == nil {
		t.Fatal("DELETE must abort: the ledger is append-only")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE err = %v, want the append-only abort", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want the original record intact", count)
	}
}

// TestSequencesNeverRestartAcrossReopen proves a reopened ledger continues its
// history. Renumbering would make recorded actions indistinguishable.
func TestSequencesNeverRestartAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "journal.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for range 3 {
		if _, err := first.Append(Record{Kind: KindCycleStarted, CycleID: "c1"}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	first.Close()

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

	records, err := second.All()
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("records = %d, want 4: nothing may be overwritten", len(records))
	}
	for i, got := range records {
		if got.Sequence != int64(i+1) {
			t.Fatalf("record %d has sequence %d, want ordered sequences", i, got.Sequence)
		}
	}
}

func TestAppendStampsTimeAndCommitsBeforeReturning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.db")
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

	// A separate reader must see the record without any further flush.
	records, err := Read(path)
	if err != nil || len(records) != 1 {
		t.Fatalf("read = %d records, err = %v, want 1 committed record", len(records), err)
	}
}

func TestFilePermissionsArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.db")
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
	records, err := Read(filepath.Join(t.TempDir(), "absent.db"))
	if err != nil {
		t.Fatalf("err = %v, want nil: a controller that never ran has nothing to recover", err)
	}
	if len(records) != 0 {
		t.Fatal("a missing ledger is empty")
	}
}

func TestUseAfterCloseIsRefused(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := ledger.Append(Record{Kind: KindMeasured}); err != ErrClosed {
		t.Fatalf("append err = %v, want ErrClosed", err)
	}
	if _, err := ledger.All(); err != ErrClosed {
		t.Fatalf("all err = %v, want ErrClosed", err)
	}
	if _, err := ledger.Sequence(); err != ErrClosed {
		t.Fatalf("sequence err = %v, want ErrClosed", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("second close = %v, want nil", err)
	}
}

func TestSequenceReportsHighestWritten(t *testing.T) {
	ledger, _ := Open(filepath.Join(t.TempDir(), "journal.db"))
	defer ledger.Close()

	got, err := ledger.Sequence()
	if err != nil || got != 0 {
		t.Fatalf("sequence = %d, err = %v, want 0 on an empty ledger", got, err)
	}
	ledger.Append(Record{Kind: KindMeasured})
	ledger.Append(Record{Kind: KindMeasured})
	if got, _ := ledger.Sequence(); got != 2 {
		t.Fatalf("sequence = %d, want 2", got)
	}
}

func TestRoundTripPreservesTypedPayloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.db")
	ledger, _ := Open(path)
	declaration := action.Declaration{
		ID: "d1", Class: action.PatchCode, Target: "t", Intent: "i",
		Generation: "gen-1", DeclaredAt: time.Now().UTC().Truncate(time.Second),
	}
	if _, err := ledger.Append(Record{
		Kind: KindDeclared, Declaration: &declaration, Generation: "gen-1",
	}); err != nil {
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
	if !records[0].Declaration.DeclaredAt.Equal(declaration.DeclaredAt) {
		t.Fatal("timestamps must survive the round trip")
	}
}
