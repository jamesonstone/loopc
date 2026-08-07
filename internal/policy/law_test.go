package policy

import (
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
)

// TestBlockingEvidenceAdmitsNothing pins fail-closed: unestablished evidence
// leaves only the classes that mutate nothing, no matter what rules match.
func TestBlockingEvidenceAdmitsNothing(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread", Admits: []action.Class{action.PatchCode}}}}
	now := time.Now().UTC()
	set := condition.NewSet("gen-1")
	mustUpsert(t, set, condition.Condition{
		Type: "Thread", Subject: "a", Status: condition.StatusUnknown,
		Reason: "Unreadable", FirstObservedAt: now, ObservedGeneration: "gen-1",
	})

	admissible := law.Admit(condition.Derive(set, now))

	if admissible.AnyArena() {
		t.Fatal("unknown evidence must admit no arena-mutating class")
	}
	if !admissible.Allows(action.RequestHuman) {
		t.Fatal("escalation must survive blocking evidence")
	}
}

func TestRuleMatchesOnTypedReasonsOnly(t *testing.T) {
	law := Law{Rules: []Rule{
		{Type: "Thread", Reasons: []string{"Actionable"}, Admits: []action.Class{action.PatchCode}},
		{Type: "Thread", Reasons: []string{"Nitpick"}, Admits: []action.Class{action.PatchDocs}},
	}}
	admissible := law.Admit(threadVector(t, 1, time.Minute))

	if !admissible.Allows(action.PatchCode) {
		t.Fatal("the matching reason must admit its class")
	}
	if admissible.Allows(action.PatchDocs) {
		t.Fatal("a non-matching reason must admit nothing")
	}
}

func TestEmptyReasonsMatchWholeType(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread", Admits: []action.Class{action.PatchCode}}}}
	if !law.Admit(threadVector(t, 1, time.Minute)).Allows(action.PatchCode) {
		t.Fatal("a rule with no reasons must match the type")
	}
}

func TestUnmatchedTypeAdmitsNothing(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Other", Admits: []action.Class{action.PatchCode}}}}
	if law.Admit(threadVector(t, 1, time.Minute)).AnyArena() {
		t.Fatal("a rule for another type must admit nothing")
	}
}

// TestSelectDefersAtSetpoint proves the controller waits rather than escalating
// when there is nothing wrong.
func TestSelectDefersAtSetpoint(t *testing.T) {
	law := Law{}
	selected, ok := law.Select(condition.Vector{}, NewAdmissible())
	if !ok || selected != action.Defer {
		t.Fatalf("selected = %s (ok=%t), want defer at setpoint", selected, ok)
	}
}

// TestSelectEscalatesWhenNothingCanAct pins the difference between "nothing to
// do" and "something to do but no way to do it".
func TestSelectEscalatesWhenNothingCanAct(t *testing.T) {
	law := Law{}
	selected, ok := law.Select(threadVector(t, 1, time.Minute), NewAdmissible())
	if !ok || selected != action.RequestHuman {
		t.Fatalf("selected = %s (ok=%t), want request_human", selected, ok)
	}
}

func TestSelectHonoursPriority(t *testing.T) {
	law := Law{
		Rules: []Rule{{Type: "Thread",
			Admits: []action.Class{action.PatchCode, action.PatchDocs}}},
		Priority: []action.Class{action.PatchDocs, action.PatchCode},
	}
	vector := threadVector(t, 1, time.Minute)
	selected, ok := law.Select(vector, law.Admit(vector))
	if !ok || selected != action.PatchDocs {
		t.Fatalf("selected = %s, want the configured priority winner patch_docs", selected)
	}
}

func TestSelectIsStableWithoutExplicitPriority(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread",
		Admits: []action.Class{action.PatchDocs, action.PatchCode}}}}
	vector := threadVector(t, 1, time.Minute)
	for range 10 {
		selected, _ := law.Select(vector, law.Admit(vector))
		if selected != action.PatchCode {
			t.Fatalf("selected = %s, want stable enum-order fallback patch_code", selected)
		}
	}
}

func TestTargetsAreDeterministic(t *testing.T) {
	vector := threadVector(t, 3, time.Minute)
	for range 5 {
		got := Targets(vector)
		if len(got) != 3 || got[0] != "a" || got[2] != "c" {
			t.Fatalf("targets = %v, want deterministic [a b c]", got)
		}
	}
}

func TestSelectHandlesNilAdmissible(t *testing.T) {
	if _, ok := (Law{}).Select(condition.Vector{}, nil); ok {
		t.Fatal("a nil admissible set must not yield a selection")
	}
}

func mustUpsert(t *testing.T, set *condition.Set, c condition.Condition) {
	t.Helper()
	if err := set.Upsert(c); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}
