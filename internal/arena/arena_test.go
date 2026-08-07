package arena

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// TestDefaultBranchIsRefused pins the guard that keeps the controller off the
// one branch a human would have to force-push to recover.
func TestDefaultBranchIsRefused(t *testing.T) {
	lane := Arena{Root: "/tmp/lane", Branch: "main", DefaultBranch: "main"}
	if err := lane.Validate(); !errors.Is(err, ErrDefaultBranch) {
		t.Fatalf("err = %v, want ErrDefaultBranch", err)
	}
}

func TestValidateRequiresCompleteArena(t *testing.T) {
	cases := map[string]Arena{
		"no root":    {Branch: "GH-1", DefaultBranch: "main"},
		"no branch":  {Root: "/tmp/lane", DefaultBranch: "main"},
		"no default": {Root: "/tmp/lane", Branch: "GH-1"},
	}
	for name, lane := range cases {
		t.Run(name, func(t *testing.T) {
			if err := lane.Validate(); !errors.Is(err, ErrIncomplete) {
				t.Fatalf("err = %v, want ErrIncomplete", err)
			}
		})
	}
	relative := Arena{Root: "lane", Branch: "GH-1", DefaultBranch: "main"}
	if err := relative.Validate(); !errors.Is(err, ErrNotCanonical) {
		t.Fatalf("err = %v, want ErrNotCanonical for a relative root", err)
	}
}

// TestContainsRejectsTraversal proves a path cannot escape the arena by
// walking through a parent.
func TestContainsRejectsTraversal(t *testing.T) {
	lane := Arena{Root: "/home/u/worktrees/o/r/GH-1", Branch: "GH-1", DefaultBranch: "main"}
	inside := []string{
		"/home/u/worktrees/o/r/GH-1",
		"/home/u/worktrees/o/r/GH-1/internal/thing.go",
		"internal/thing.go",
	}
	for _, path := range inside {
		if !lane.Contains(path) {
			t.Fatalf("%q should be inside the arena", path)
		}
	}
	outside := []string{
		"/home/u/worktrees/o/r/GH-1/../GH-2/file.go",
		"/home/u/worktrees/o/r/GH-2",
		"/etc/passwd",
		"../../escape",
		"/home/u/worktrees/o/r/GH-10/file.go",
	}
	for _, path := range outside {
		if lane.Contains(path) {
			t.Fatalf("%q should be outside the arena", path)
		}
	}
	if err := lane.CheckPath("/etc/passwd"); !errors.Is(err, ErrOutsideArena) {
		t.Fatalf("err = %v, want ErrOutsideArena", err)
	}
}

// TestSiblingLanePrefixIsNotContained guards against a plain string-prefix
// check, which would treat GH-10 as living inside GH-1.
func TestSiblingLanePrefixIsNotContained(t *testing.T) {
	lane := Arena{Root: "/w/GH-1"}
	if lane.Contains("/w/GH-10/file.go") {
		t.Fatal("GH-10 is a sibling of GH-1, not a child")
	}
}

func TestCanonicalRoot(t *testing.T) {
	want := filepath.Join("/home/u", "worktrees", "owner", "repo", "GH-7")
	if got := CanonicalRoot("/home/u", "owner", "repo", "GH-7"); got != want {
		t.Fatalf("root = %q, want %q", got, want)
	}
	lane := Arena{Root: want, Owner: "owner", Repository: "repo", Lane: "GH-7"}
	if !lane.IsCanonical("/home/u") {
		t.Fatal("a lane at its canonical path must report canonical")
	}
	relocated := Arena{Root: "/somewhere/else", Owner: "owner", Repository: "repo", Lane: "GH-7"}
	if relocated.IsCanonical("/home/u") {
		t.Fatal("a relocated lane must not report canonical")
	}
}

// TestLeaseSerialisesActions pins why the lease exists: two concurrent actions
// would make the recorded error delta unattributable to either class.
func TestLeaseSerialisesActions(t *testing.T) {
	lease := NewLease()
	now := time.Now()
	if err := lease.Acquire("d1", now); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := lease.Acquire("d2", now); !errors.Is(err, ErrHeld) {
		t.Fatalf("err = %v, want ErrHeld", err)
	}
	holder, held := lease.Holder()
	if !held || holder != "d1" {
		t.Fatalf("holder = %q held=%t, want d1", holder, held)
	}
	lease.Release()
	if _, held := lease.Holder(); held {
		t.Fatal("lease must be free after release")
	}
	if err := lease.Acquire("d2", now); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	lease.Release()
	lease.Release()
}
