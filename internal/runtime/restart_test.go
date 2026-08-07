package runtime

import (
	"context"
	"testing"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/agent"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/policy"
)

// TestBrakesSurviveRestart is the regression guard for a runaway.
//
// The brakes read the e(t) series. If that series lived only in memory, a
// restarted controller would forget it had been stalling and would resume
// acting on a plant it had already failed to fix — the exact runaway the
// derivative brake exists to stop. The samples are therefore journalled and
// replayed on construction.
func TestBrakesSurviveRestart(t *testing.T) {
	model := &fakeAgent{
		conditions: []condition.Condition{threadCondition("a", condition.StatusTrue)},
		proposal: agent.Proposal{
			Class: action.PatchCode, Target: "a", Intent: "attempt a fix",
		},
	}
	engine, path := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))

	// The agent acts but never reduces error: the plant keeps reporting the
	// same unresolved condition.
	for range 3 {
		if _, err := engine.RunOnce(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	if engine.Series().Len() == 0 {
		t.Fatal("expected samples to accumulate")
	}
	stalledBefore := engine.Series().NoReductionRun()
	if stalledBefore == 0 {
		t.Fatal("expected the loop to register unproductive cycles")
	}

	// Restart: a fresh engine reading the same ledger must resume with the
	// history intact rather than a clean slate.
	//
	// The baseline is appended because only an audit run writes one to the
	// ledger; in real operation the audit cycle that unlocked reconcile left
	// its record in this same file.
	records := append(readRecords(t, path), baselineFor(t, ModeReconcile)...)
	if len(journal.Samples(records)) == 0 {
		t.Fatal("samples must be journalled, not held only in memory")
	}

	resumed, _ := newEngine(t, ModeReconcile, &fakePlant{}, model, records)
	if resumed.Series().Len() == 0 {
		t.Fatal("a restarted controller must replay its e(t) history")
	}
	if got := resumed.Series().NoReductionRun(); got != stalledBefore {
		t.Fatalf("no-reduction run after restart = %d, want %d preserved", got, stalledBefore)
	}
}

// TestDerivativeBrakeStopsARestartedRunaway proves the replayed history
// actually withdraws authority rather than merely being present.
func TestDerivativeBrakeStopsARestartedRunaway(t *testing.T) {
	model := &fakeAgent{
		conditions: []condition.Condition{threadCondition("a", condition.StatusTrue)},
		proposal: agent.Proposal{
			Class: action.PatchCode, Target: "a", Intent: "attempt a fix",
		},
	}
	engine, path := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))
	for range 3 {
		if _, err := engine.RunOnce(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	history := append(readRecords(t, path), baselineFor(t, ModeReconcile)...)
	resumed, _ := newEngine(t, ModeReconcile, &fakePlant{}, model, history)
	// Arm the derivative brake on the resumed engine.
	resumed.config.Brakes = policy.Brakes{DerivativeWindow: 2}

	before := model.acted
	cycle, err := resumed.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if model.acted != before {
		t.Fatal("a restarted controller with a stalled history must not act")
	}
	if cycle.Selected != action.RequestHuman {
		t.Fatalf("selected = %s, want request_human after the brake withdraws authority",
			cycle.Selected)
	}
}
