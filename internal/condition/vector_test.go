package condition

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestVectorEntryCarriesNoMessage guards the structural enforcement of the
// constitution's "typed conditions, never prose" rule.
//
// The control law is given only a Vector. If Entry ever gained a message-like
// field, a future rule could match on human-readable text and the message would
// silently become the interface. This test fails on that change rather than
// waiting for the behaviour to drift.
func TestVectorEntryCarriesNoMessage(t *testing.T) {
	entry := reflect.TypeOf(Entry{})
	for i := range entry.NumField() {
		name := strings.ToLower(entry.Field(i).Name)
		if strings.Contains(name, "message") || strings.Contains(name, "body") ||
			strings.Contains(name, "text") {
			t.Fatalf("Entry.%s exposes prose to the control law", entry.Field(i).Name)
		}
	}
}

func TestDeriveAggregatesByTypeAndIgnoresSatisfied(t *testing.T) {
	now := time.Now().UTC()
	set := NewSet("gen-1")
	mustUpsert(t, set, Condition{
		Type: "Thread", Subject: "b", Status: StatusTrue, Reason: "Actionable",
		FirstObservedAt: now.Add(-2 * time.Hour), ObservedGeneration: "gen-1",
	})
	mustUpsert(t, set, Condition{
		Type: "Thread", Subject: "a", Status: StatusTrue, Reason: "Actionable",
		FirstObservedAt: now.Add(-5 * time.Hour), ObservedGeneration: "gen-1",
	})
	mustUpsert(t, set, Condition{
		Type: "Thread", Subject: "c", Status: StatusFalse, Reason: "Resolved",
		FirstObservedAt: now.Add(-9 * time.Hour), ObservedGeneration: "gen-1",
	})

	vector := Derive(set, now)
	entry, ok := vector.Find("Thread")
	if !ok {
		t.Fatal("expected a Thread entry")
	}
	if entry.Count != 2 {
		t.Fatalf("count = %d, want 2 (the satisfied condition must not contribute)", entry.Count)
	}
	if entry.OldestAge != 5*time.Hour {
		t.Fatalf("oldest age = %s, want 5h", entry.OldestAge)
	}
	if got := strings.Join(entry.Subjects, ","); got != "a,b" {
		t.Fatalf("subjects = %q, want deterministic order %q", got, "a,b")
	}
	if vector.Total() != 2 {
		t.Fatalf("total = %d, want 2", vector.Total())
	}
}

// TestUnknownIsUnsatisfiedAndBlocking pins the fail-closed reading of missing
// evidence: Unknown is not absence.
func TestUnknownIsUnsatisfiedAndBlocking(t *testing.T) {
	now := time.Now().UTC()
	set := NewSet("gen-1")
	mustUpsert(t, set, Condition{
		Type: "Thread", Subject: "a", Status: StatusUnknown, Reason: "Unreadable",
		FirstObservedAt: now, ObservedGeneration: "gen-1",
	})
	vector := Derive(set, now)
	if !vector.Blocking() {
		t.Fatal("unknown evidence must block arena mutation")
	}
	if vector.Total() != 1 {
		t.Fatal("unknown evidence must contribute to the error term")
	}
}

func TestHashIgnoresAgeButTracksShape(t *testing.T) {
	now := time.Now().UTC()
	build := func(age time.Duration, count int) Vector {
		set := NewSet("gen-1")
		for i := range count {
			mustUpsert(t, set, Condition{
				Type: "Thread", Subject: string(rune('a' + i)), Status: StatusTrue,
				Reason: "Actionable", FirstObservedAt: now.Add(-age),
				ObservedGeneration: "gen-1",
			})
		}
		return Derive(set, now)
	}
	if build(time.Hour, 2).Hash() != build(9*time.Hour, 2).Hash() {
		t.Fatal("hash must ignore age; a plant merely getting older has not changed shape")
	}
	if build(time.Hour, 2).Hash() == build(time.Hour, 3).Hash() {
		t.Fatal("hash must change when the plant shape changes")
	}
}

func TestEmptyVector(t *testing.T) {
	if !Derive(NewSet("gen-1"), time.Now()).Empty() {
		t.Fatal("a set with no unsatisfied conditions is at setpoint")
	}
	if !Derive(nil, time.Now()).Empty() {
		t.Fatal("a nil set must not panic and must read as empty")
	}
}

func mustUpsert(t *testing.T, set *Set, c Condition) {
	t.Helper()
	if err := set.Upsert(c); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}
