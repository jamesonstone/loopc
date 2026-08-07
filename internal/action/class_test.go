package action

import (
	"errors"
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/condition"
)

// TestArenaSplitIsExhaustive pins exactly which classes require authority.
// A new class silently defaulting to non-arena would let the brakes be
// bypassed, so the split is asserted member by member.
func TestArenaSplitIsExhaustive(t *testing.T) {
	want := map[Class]bool{
		PatchCode: true, PatchTest: true, PatchDocs: true, ReplyNoChange: true,
		RequestHuman: false, Defer: false,
	}
	if len(want) != len(Classes) {
		t.Fatalf("enum has %d members, test covers %d", len(Classes), len(want))
	}
	for class, mutates := range want {
		if !class.Valid() {
			t.Fatalf("%s must be a valid class", class)
		}
		if class.MutatesArena() != mutates {
			t.Fatalf("%s.MutatesArena() = %t, want %t", class, class.MutatesArena(), mutates)
		}
	}
	if len(NonArena()) != 2 {
		t.Fatalf("NonArena() = %v, want exactly request_human and defer", NonArena())
	}
}

// TestReplyNoChangeIsArenaMutating records a deliberate judgement: it writes no
// code but does write to GitHub, which is inside the blast radius the arena
// exists to bound.
func TestReplyNoChangeIsArenaMutating(t *testing.T) {
	if !ReplyNoChange.MutatesArena() {
		t.Fatal("reply_no_change writes to GitHub and must require authority")
	}
}

func TestParseRejectsUnknownClasses(t *testing.T) {
	if _, err := Parse("patch_code"); err != nil {
		t.Fatalf("parse of a known class failed: %v", err)
	}
	_, err := Parse("rm_minus_rf")
	if !errors.Is(err, ErrUnknownClass) {
		t.Fatalf("err = %v, want ErrUnknownClass", err)
	}
}

func TestDeclarationValidation(t *testing.T) {
	base := Declaration{
		ID: "d1", Class: PatchCode, Target: "thread-1",
		Intent: "address feedback", Generation: "gen-1",
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid declaration rejected: %v", err)
	}

	cases := map[string]struct {
		mutate func(Declaration) Declaration
		want   error
	}{
		"no id":         {func(d Declaration) Declaration { d.ID = ""; return d }, ErrMissingID},
		"no intent":     {func(d Declaration) Declaration { d.Intent = ""; return d }, ErrMissingIntent},
		"no generation": {func(d Declaration) Declaration { d.Generation = ""; return d }, ErrMissingGeneration},
		"no target":     {func(d Declaration) Declaration { d.Target = ""; return d }, ErrMissingTarget},
		"bad class":     {func(d Declaration) Declaration { d.Class = "nope"; return d }, ErrUnknownClass},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.mutate(base).Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestNonArenaDeclarationNeedsNoTarget records that there is nothing to bound
// when nothing is written.
func TestNonArenaDeclarationNeedsNoTarget(t *testing.T) {
	declaration := Declaration{
		ID: "d1", Class: RequestHuman, Intent: "escalate", Generation: "gen-1",
	}
	if err := declaration.Validate(); err != nil {
		t.Fatalf("non-arena declaration without a target rejected: %v", err)
	}
}

// TestScoreRequiresEveryPredictionToClear pins the deliberate absence of
// partial credit: a sensor that is consistently half-right should not read as
// calibrated.
func TestScoreRequiresEveryPredictionToClear(t *testing.T) {
	keys := []condition.Key{
		{Type: "Thread", Subject: "a"},
		{Type: "Thread", Subject: "b"},
	}
	declaration := Declaration{PredictedCleared: keys}
	before := buildSet(t, condition.StatusTrue, condition.StatusTrue)
	partial := buildSet(t, condition.StatusFalse, condition.StatusTrue)

	predicted, cleared, hit := Score(declaration, before, partial)
	if predicted != 2 || cleared != 1 {
		t.Fatalf("predicted=%d cleared=%d, want 2 and 1", predicted, cleared)
	}
	if hit {
		t.Fatal("a partially correct prediction must not count as a hit")
	}

	full := buildSet(t, condition.StatusFalse, condition.StatusFalse)
	if _, _, hit := Score(declaration, before, full); !hit {
		t.Fatal("clearing every predicted condition must count as a hit")
	}
}

func TestScoreWithNoPredictionIsNotAHit(t *testing.T) {
	if _, _, hit := Score(Declaration{}, nil, nil); hit {
		t.Fatal("predicting nothing must not count as a hit")
	}
}

func TestOutcomeReduced(t *testing.T) {
	if !(Outcome{ErrorDelta: -1}).Reduced() {
		t.Fatal("a negative delta moved the plant toward its setpoint")
	}
	if (Outcome{ErrorDelta: 0}).Reduced() {
		t.Fatal("no change is not a reduction")
	}
}

func buildSet(t *testing.T, statuses ...condition.Status) *condition.Set {
	t.Helper()
	set := condition.NewSet("gen-1")
	for i, status := range statuses {
		err := set.Upsert(condition.Condition{
			Type: "Thread", Subject: string(rune('a' + i)), Status: status,
			Reason: "Actionable", FirstObservedAt: time.Now(), ObservedGeneration: "gen-1",
		})
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	return set
}
