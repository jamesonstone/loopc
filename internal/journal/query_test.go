package journal

import (
	"testing"

	"github.com/jamesonstone/loopc/internal/action"
)

// TestAuditBaselineRequiresExactPolicy pins why the baseline is bound to a
// hash: a baseline recorded under a different policy proves nothing about the
// one about to act, so a near match must be treated as no match.
func TestAuditBaselineRequiresExactPolicy(t *testing.T) {
	records := []Record{{Kind: KindAuditBaseline, PolicyHash: "abc123"}}

	if !HasAuditBaseline(records, "abc123") {
		t.Fatal("an exact policy match must satisfy the baseline")
	}
	if HasAuditBaseline(records, "abc124") {
		t.Fatal("a different policy must not satisfy the baseline")
	}
	if HasAuditBaseline(records, "") {
		t.Fatal("an empty policy hash must never satisfy the baseline")
	}
	if HasAuditBaseline(nil, "abc123") {
		t.Fatal("no history means no baseline")
	}
}

// TestBaselineIgnoresOtherKinds proves an ordinary cycle record cannot be
// mistaken for a baseline.
func TestBaselineIgnoresOtherKinds(t *testing.T) {
	records := []Record{{Kind: KindMeasured, PolicyHash: "abc123"}}
	if HasAuditBaseline(records, "abc123") {
		t.Fatal("only an audit baseline record unlocks reconcile")
	}
}

// TestPendingFindsUnterminatedAction covers crash recovery: an action that
// reached prepared or started but never terminal needs reconciling.
func TestPendingFindsUnterminatedAction(t *testing.T) {
	first := action.Declaration{ID: "d1", Class: action.PatchCode}
	second := action.Declaration{ID: "d2", Class: action.PatchDocs}
	records := []Record{
		{Kind: KindDeclared, Declaration: &first},
		{Kind: KindStarted, Declaration: &first},
		{Kind: KindTerminal, Declaration: &first},
		{Kind: KindDeclared, Declaration: &second},
		{Kind: KindStarted, Declaration: &second},
	}

	pending, found := Pending(records)
	if !found || pending.ID != "d2" {
		t.Fatalf("pending = %+v found=%t, want the unterminated d2", pending, found)
	}
}

func TestPendingIsEmptyWhenAllTerminal(t *testing.T) {
	declaration := action.Declaration{ID: "d1", Class: action.PatchCode}
	records := []Record{
		{Kind: KindDeclared, Declaration: &declaration},
		{Kind: KindStarted, Declaration: &declaration},
		{Kind: KindTerminal, Declaration: &declaration},
	}
	if _, found := Pending(records); found {
		t.Fatal("a terminated action is not pending")
	}
	if _, found := Pending(nil); found {
		t.Fatal("no history means nothing pending")
	}
}

func TestOutcomesAreTheTrainingSet(t *testing.T) {
	first := action.Outcome{DeclarationID: "d1", Class: action.PatchCode, ErrorDelta: -1}
	second := action.Outcome{DeclarationID: "d2", Class: action.PatchDocs, ErrorDelta: 0}
	records := []Record{
		{Kind: KindOutcome, Outcome: &first},
		{Kind: KindMeasured},
		{Kind: KindOutcome, Outcome: &second},
	}

	outcomes := Outcomes(records)
	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %d, want 2", len(outcomes))
	}
	if outcomes[0].DeclarationID != "d1" || outcomes[1].DeclarationID != "d2" {
		t.Fatal("outcomes must be returned oldest first")
	}
	if !outcomes[0].Reduced() || outcomes[1].Reduced() {
		t.Fatal("reduction must be derived from the recorded delta")
	}
}

func TestCycleIDsAreUniqueAndOrdered(t *testing.T) {
	records := []Record{
		{Kind: KindCycleStarted, CycleID: "c1"},
		{Kind: KindMeasured, CycleID: "c1"},
		{Kind: KindCycleStarted, CycleID: "c2"},
		{Kind: KindCycleStarted, CycleID: "c1"},
	}
	ids := CycleIDs(records)
	if len(ids) != 2 || ids[0] != "c1" || ids[1] != "c2" {
		t.Fatalf("ids = %v, want [c1 c2]", ids)
	}
}
