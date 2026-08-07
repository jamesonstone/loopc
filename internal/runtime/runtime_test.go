package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/agent"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
)

// TestReconcileRefusedWithoutBaseline pins the authority ceiling: reconcile
// cannot start without a recorded audit observation for the identical policy.
// It fails at construction, not at the moment of action.
func TestReconcileRefusedWithoutBaseline(t *testing.T) {
	ledger, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer ledger.Close()

	_, err = New(Options{
		Config: Config{Mode: ModeReconcile, Law: testLaw()},
		Plant:  &fakePlant{}, Agent: &fakeAgent{}, Journal: ledger,
	})
	if !errors.Is(err, ErrNoAuditBaseline) {
		t.Fatalf("err = %v, want ErrNoAuditBaseline", err)
	}
}

// TestBaselineIsPolicyBound proves that changing the control law invalidates
// an existing baseline, forcing a fresh audit observation.
func TestBaselineIsPolicyBound(t *testing.T) {
	ledger, _ := journal.Open(t.TempDir() + "/journal.db")
	defer ledger.Close()

	stale := []journal.Record{{Kind: journal.KindAuditBaseline, PolicyHash: "some-other-policy"}}
	_, err := New(Options{
		Config: Config{Mode: ModeReconcile, Law: testLaw()},
		Plant:  &fakePlant{}, Agent: &fakeAgent{}, Journal: ledger, History: stale,
	})
	if !errors.Is(err, ErrNoAuditBaseline) {
		t.Fatalf("err = %v, want a baseline under another policy to be rejected", err)
	}
}

// TestAuditRecordsBaselineAndActsOnNothing pins the audit ceiling: it has no
// executor, so the agent is never asked to propose or act.
func TestAuditRecordsBaselineAndActsOnNothing(t *testing.T) {
	model := &fakeAgent{conditions: []condition.Condition{
		threadCondition("a", condition.StatusTrue),
	}}
	engine, path := newEngine(t, ModeAudit, &fakePlant{}, model, nil)

	cycle, err := engine.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if model.proposed != 0 || model.acted != 0 {
		t.Fatalf("audit proposed=%d acted=%d, want 0 and 0", model.proposed, model.acted)
	}
	if cycle.Scalar != 1 || cycle.Vector.Total() != 1 {
		t.Fatalf("cycle scalar=%v total=%d, want the error measured", cycle.Scalar, cycle.Vector.Total())
	}
	records := readRecords(t, path)
	if !hasKind(records, journal.KindAuditBaseline) {
		t.Fatalf("kinds = %v, want an audit baseline recorded", kinds(records))
	}
	if hasKind(records, journal.KindDeclared) {
		t.Fatal("audit must never commit a declaration")
	}
}

// TestFailedObservationRecordsNoBaseline proves a cycle that could not observe
// does not unlock reconcile.
func TestFailedObservationRecordsNoBaseline(t *testing.T) {
	plant := &fakePlant{err: errors.New("plant unreachable")}
	engine, path := newEngine(t, ModeAudit, plant, &fakeAgent{}, nil)

	if _, err := engine.RunOnce(context.Background()); err == nil {
		t.Fatal("expected the observation failure to surface")
	}
	if hasKind(readRecords(t, path), journal.KindAuditBaseline) {
		t.Fatal("a failed observation must not record a baseline")
	}
}

// TestReconcileCommitsDeclarationBeforeActing pins the prepared, started,
// observed, terminal sequence, and that nothing executes without a committed
// declaration.
func TestReconcileCommitsDeclarationBeforeActing(t *testing.T) {
	model := &fakeAgent{
		conditions: []condition.Condition{threadCondition("a", condition.StatusTrue)},
		proposal: agent.Proposal{
			Class: action.PatchCode, Target: "a", Intent: "address feedback",
			PredictedCleared: []condition.Key{{Type: "Thread", Subject: "a"}},
		},
		result: agent.Result{Changed: []string{"internal/thing.go"}},
	}
	engine, path := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))

	cycle, err := engine.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !cycle.Acted || cycle.Outcome == nil {
		t.Fatalf("cycle acted=%t outcome=%v, want an action and a recorded outcome", cycle.Acted, cycle.Outcome)
	}

	records := readRecords(t, path)
	order := []journal.Kind{
		journal.KindDeclared, journal.KindStarted,
		journal.KindObserved, journal.KindTerminal, journal.KindOutcome,
	}
	position := 0
	for _, record := range records {
		if position < len(order) && record.Kind == order[position] {
			position++
		}
	}
	if position != len(order) {
		t.Fatalf("kinds = %v, want the full prepared/started/observed/terminal/outcome sequence",
			kinds(records))
	}
}

// TestProposingAnUnadmittedClassIsRefused proves an agent cannot widen its own
// authority by naming a class the control law never admitted.
func TestProposingAnUnadmittedClassIsRefused(t *testing.T) {
	model := &fakeAgent{
		conditions: []condition.Condition{threadCondition("a", condition.StatusTrue)},
		proposal: agent.Proposal{
			Class: action.PatchTest, Target: "a", Intent: "sneak past the law",
		},
	}
	engine, path := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))

	_, err := engine.RunOnce(context.Background())
	if !errors.Is(err, ErrNotAdmitted) {
		t.Fatalf("err = %v, want ErrNotAdmitted", err)
	}
	if model.acted != 0 {
		t.Fatal("a refused proposal must never reach the actor")
	}
	if hasKind(readRecords(t, path), journal.KindDeclared) {
		t.Fatal("a refused proposal must not be committed as a declaration")
	}
}

// TestAgentIsHandedOnlyAdmissibleClasses proves the proposer chooses from a set
// it cannot widen.
func TestAgentIsHandedOnlyAdmissibleClasses(t *testing.T) {
	model := &fakeAgent{
		conditions: []condition.Condition{threadCondition("a", condition.StatusTrue)},
		proposal:   agent.Proposal{Class: action.PatchCode, Target: "a", Intent: "fix"},
	}
	engine, _ := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))
	if _, err := engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, class := range model.handed {
		if class == action.PatchTest || class == action.PatchDocs {
			t.Fatalf("agent was handed %s, which the law never admitted", class)
		}
	}
	if len(model.handed) == 0 {
		t.Fatal("the agent must be handed the admissible set")
	}
}

// TestGuardedPathWriteIsRefusedAfterTheFact proves the self-modification guard
// catches an agent that reached the control law anyway.
func TestGuardedPathWriteIsRefusedAfterTheFact(t *testing.T) {
	model := &fakeAgent{
		conditions: []condition.Condition{threadCondition("a", condition.StatusTrue)},
		proposal:   agent.Proposal{Class: action.PatchCode, Target: "a", Intent: "fix"},
		result:     agent.Result{Changed: []string{"internal/policy/brakes.go"}},
	}
	engine, path := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))

	cycle, err := engine.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if cycle.Acted {
		t.Fatal("a write touching the control law must not count as a successful action")
	}

	var terminal string
	for _, record := range readRecords(t, path) {
		if record.Kind == journal.KindTerminal && record.Terminal != "" {
			terminal = record.Terminal
		}
	}
	if terminal != journal.TerminalRefused {
		t.Fatalf("terminal = %q, want %q", terminal, journal.TerminalRefused)
	}
}

// TestSensorConditionsAreValidated proves untrusted agent output is checked
// rather than accepted.
func TestSensorConditionsAreValidated(t *testing.T) {
	model := &fakeAgent{conditions: []condition.Condition{
		{Type: "Thread", Subject: "a", Status: condition.StatusTrue},
	}}
	engine, _ := newEngine(t, ModeAudit, &fakePlant{}, model, nil)

	_, err := engine.RunOnce(context.Background())
	if !errors.Is(err, condition.ErrMissingReason) {
		t.Fatalf("err = %v, want the invalid condition rejected", err)
	}
}

// TestSetpointDefersRatherThanEscalating proves a healthy plant is left alone.
func TestSetpointDefersRatherThanEscalating(t *testing.T) {
	model := &fakeAgent{conditions: []condition.Condition{
		threadCondition("a", condition.StatusFalse),
	}}
	engine, _ := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))

	cycle, err := engine.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if cycle.Selected != action.Defer {
		t.Fatalf("selected = %s, want defer at setpoint", cycle.Selected)
	}
	if model.acted != 0 {
		t.Fatal("a plant at its setpoint must not be acted on")
	}
}

// TestBlockingEvidenceEscalatesWithoutActing proves unknown evidence fails
// closed all the way through the loop.
func TestBlockingEvidenceEscalatesWithoutActing(t *testing.T) {
	model := &fakeAgent{conditions: []condition.Condition{
		threadCondition("a", condition.StatusUnknown),
	}}
	engine, _ := newEngine(t, ModeReconcile, &fakePlant{}, model, baselineFor(t, ModeReconcile))

	cycle, err := engine.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if cycle.Selected != action.RequestHuman {
		t.Fatalf("selected = %s, want request_human on blocking evidence", cycle.Selected)
	}
	if model.acted != 0 || model.proposed != 0 {
		t.Fatal("blocking evidence must not reach the proposer or the actor")
	}
}
