package condition

import (
	"errors"
	"testing"
	"time"
)

// TestObserveCarriesPersistenceForward pins the signal the integral brake
// reads. A condition that keeps being observed must keep its original
// FirstObservedAt, or nothing would ever appear to persist and the brake could
// never fire.
func TestObserveCarriesPersistenceForward(t *testing.T) {
	start := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	later := start.Add(3 * time.Hour)

	first, err := Observe(nil, "gen-1", []Condition{
		{Type: "Thread", Subject: "a", Status: StatusTrue, Reason: "Actionable"},
	}, start)
	if err != nil {
		t.Fatalf("first observe: %v", err)
	}

	second, err := Observe(first, "gen-1", []Condition{
		{Type: "Thread", Subject: "a", Status: StatusTrue, Reason: "Actionable"},
	}, later)
	if err != nil {
		t.Fatalf("second observe: %v", err)
	}

	got, ok := second.Get(Key{Type: "Thread", Subject: "a"})
	if !ok {
		t.Fatal("condition missing from second observation")
	}
	if !got.FirstObservedAt.Equal(start) {
		t.Fatalf("FirstObservedAt = %s, want %s carried forward", got.FirstObservedAt, start)
	}
	if !got.LastTransitionAt.Equal(start) {
		t.Fatalf("LastTransitionAt = %s, want %s: an unchanged status is not a transition",
			got.LastTransitionAt, start)
	}
	if got.Age(later) != 3*time.Hour {
		t.Fatalf("age = %s, want 3h", got.Age(later))
	}
}

func TestObserveMovesTransitionOnStatusChange(t *testing.T) {
	start := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	later := start.Add(time.Hour)

	first, _ := Observe(nil, "gen-1", []Condition{
		{Type: "Thread", Subject: "a", Status: StatusTrue, Reason: "Actionable"},
	}, start)
	second, err := Observe(first, "gen-1", []Condition{
		{Type: "Thread", Subject: "a", Status: StatusFalse, Reason: "Resolved"},
	}, later)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	got, _ := second.Get(Key{Type: "Thread", Subject: "a"})
	if !got.LastTransitionAt.Equal(later) {
		t.Fatalf("LastTransitionAt = %s, want %s on a real status change",
			got.LastTransitionAt, later)
	}
	if !got.FirstObservedAt.Equal(start) {
		t.Fatal("FirstObservedAt must survive a status change")
	}
}

// TestGenerationMismatchRejected pins the staleness rule: a condition bound to
// a superseded generation may not be controlled on.
func TestGenerationMismatchRejected(t *testing.T) {
	set := NewSet("gen-2")
	err := set.Upsert(Condition{
		Type: "Thread", Subject: "a", Status: StatusTrue, Reason: "Actionable",
		ObservedGeneration: "gen-1",
	})
	if !errors.Is(err, ErrGenerationMismatch) {
		t.Fatalf("err = %v, want ErrGenerationMismatch", err)
	}
	if set.Fresh("gen-1") {
		t.Fatal("a set must not report freshness for another generation")
	}
	if !set.Fresh("gen-2") {
		t.Fatal("a set must report freshness for its own generation")
	}
}

// TestObserveRestampsGeneration proves the runtime, not the sensor, decides
// which generation a condition belongs to.
func TestObserveRestampsGeneration(t *testing.T) {
	set, err := Observe(nil, "gen-9", []Condition{
		{Type: "Thread", Subject: "a", Status: StatusTrue, Reason: "Actionable",
			ObservedGeneration: "attacker-supplied"},
	}, time.Now())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	got, _ := set.Get(Key{Type: "Thread", Subject: "a"})
	if got.ObservedGeneration != "gen-9" {
		t.Fatalf("generation = %q, want the runtime's own %q", got.ObservedGeneration, "gen-9")
	}
}

func TestValidateRequiresTypeStatusAndReason(t *testing.T) {
	cases := map[string]struct {
		condition Condition
		want      error
	}{
		"missing type":   {Condition{Status: StatusTrue, Reason: "r"}, ErrMissingType},
		"bad status":     {Condition{Type: "T", Status: "Maybe", Reason: "r"}, ErrInvalidStatus},
		"missing reason": {Condition{Type: "T", Status: StatusTrue}, ErrMissingReason},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.condition.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAllIsDeterministic(t *testing.T) {
	set := NewSet("gen-1")
	for _, subject := range []string{"c", "a", "b"} {
		mustUpsert(t, set, Condition{
			Type: "Thread", Subject: subject, Status: StatusTrue,
			Reason: "Actionable", ObservedGeneration: "gen-1",
		})
	}
	for range 5 {
		all := set.All()
		if all[0].Subject != "a" || all[1].Subject != "b" || all[2].Subject != "c" {
			t.Fatal("All must not depend on map iteration order")
		}
	}
}
