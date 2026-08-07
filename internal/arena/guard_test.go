package arena

import (
	"errors"
	"testing"
)

// TestSelfModificationIsRefused pins the one refusal that cannot be reasoned
// around: a loop permitted to edit its own arena or control law could widen its
// authority by editing the code that limits it.
func TestSelfModificationIsRefused(t *testing.T) {
	guarded := [][]string{
		{"internal/arena/arena.go"},
		{"internal/policy/brakes.go"},
		{"internal/arena"},
		{"README.md", "internal/policy/law.go"},
		{"./internal/arena/guard.go"},
	}
	for _, changed := range guarded {
		refusal, refused := InspectDiff(changed)
		if !refused {
			t.Fatalf("%v must be refused", changed)
		}
		if len(refusal.Paths) == 0 || refusal.Error() == "" {
			t.Fatalf("%v must be refused with an explanation", changed)
		}
		if err := CheckDiff(changed); !errors.Is(err, ErrSelfModification) {
			t.Fatalf("err = %v, want ErrSelfModification", err)
		}
	}
}

// TestGuardMatchesWholeSegmentsOnly guards against a substring check. A
// directory named internal/policyholder is not the control law, and refusing
// it would be a false positive that erodes trust in the guard.
func TestGuardMatchesWholeSegmentsOnly(t *testing.T) {
	allowed := [][]string{
		{"internal/policyholder/thing.go"},
		{"internal/arenas/thing.go"},
		{"docs/internal/arena.md"},
		{"internal/condition/set.go"},
		{"cmd/loopc/main.go"},
	}
	for _, changed := range allowed {
		if _, refused := InspectDiff(changed); refused {
			t.Fatalf("%v must not be refused: it is not the arena or the control law", changed)
		}
		if err := CheckDiff(changed); err != nil {
			t.Fatalf("CheckDiff(%v) = %v, want nil", changed, err)
		}
	}
}

// TestTraversalCannotDisguiseGuardedPath proves a path that climbs out of the
// repository is discarded rather than silently normalised into an allow.
func TestTraversalCannotDisguiseGuardedPath(t *testing.T) {
	if _, refused := InspectDiff([]string{"docs/../internal/arena/guard.go"}); !refused {
		t.Fatal("a traversal resolving into the arena must still be refused")
	}
	if _, refused := InspectDiff([]string{"../outside/file.go", ""}); refused {
		t.Fatal("a path escaping the repository is not a guarded path")
	}
}

func TestEmptyDiffIsAllowed(t *testing.T) {
	if _, refused := InspectDiff(nil); refused {
		t.Fatal("an empty diff has nothing to refuse")
	}
	if err := CheckDiff([]string{}); err != nil {
		t.Fatalf("CheckDiff on empty = %v, want nil", err)
	}
}

func TestRefusalPathsAreDeduped(t *testing.T) {
	refusal, refused := InspectDiff([]string{
		"internal/arena/arena.go", "internal/arena/arena.go", "internal/policy/law.go",
	})
	if !refused {
		t.Fatal("expected refusal")
	}
	if len(refusal.Paths) != 2 {
		t.Fatalf("paths = %v, want duplicates collapsed", refusal.Paths)
	}
}
