package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/arena"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/policy"
	"github.com/jamesonstone/loopc/internal/trend"
)

// ErrNotAdmitted is returned when an agent proposes a class it was not handed.
var ErrNotAdmitted = errors.New("proposed class is not in the admissible set")

// act runs the full prepared, started, observed, terminal sequence.
func (e *Engine) act(ctx context.Context, cycle Cycle, before *condition.Set, admissible *policy.Admissible) (Cycle, error) {
	declaration, err := e.declare(ctx, cycle, admissible)
	if err != nil {
		return cycle, err
	}
	if err := e.lease.Acquire(declaration.ID, e.now()); err != nil {
		return cycle, err
	}
	defer e.lease.Release()

	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindStarted, CycleID: cycle.ID, At: e.now(),
		Generation: cycle.Generation, Declaration: &declaration,
	}); err != nil {
		return cycle, err
	}

	result, actErr := e.agent.Act(ctx, declaration, e.lane)
	terminal := journal.TerminalSucceeded
	note := result.Note
	if actErr != nil {
		terminal, note = journal.TerminalFailed, actErr.Error()
	} else if err := e.checkWrites(result.Changed); err != nil {
		terminal, note = journal.TerminalRefused, err.Error()
	}
	cycle.Acted = terminal == journal.TerminalSucceeded

	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindObserved, CycleID: cycle.ID, At: e.now(),
		Generation: cycle.Generation, Declaration: &declaration,
		Detail: map[string]any{"changed": result.Changed},
	}); err != nil {
		return cycle, err
	}
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindTerminal, CycleID: cycle.ID, At: e.now(),
		Generation: cycle.Generation, Declaration: &declaration,
		Terminal: terminal, Note: note,
	}); err != nil {
		return cycle, err
	}

	outcome, err := e.remeasure(ctx, cycle, declaration, before)
	if err != nil {
		return cycle, err
	}
	cycle.Outcome = outcome
	return cycle, e.record(cycle, declaration.Class)
}

// declare obtains a proposal and commits it as a declaration.
//
// The proposed class is checked against the admissible set before anything is
// written. An agent cannot widen its own authority by proposing a class that
// was never admitted, and nothing executes without a committed declaration.
func (e *Engine) declare(ctx context.Context, cycle Cycle, admissible *policy.Admissible) (action.Declaration, error) {
	proposal, err := e.agent.Propose(ctx, cycle.Vector, admissible.Classes())
	if err != nil {
		return action.Declaration{}, fmt.Errorf("propose action: %w", err)
	}
	if !proposal.Class.Valid() {
		return action.Declaration{}, fmt.Errorf("%w: %q", action.ErrUnknownClass, proposal.Class)
	}
	if !admissible.Allows(proposal.Class) {
		return action.Declaration{}, fmt.Errorf("%w: %q", ErrNotAdmitted, proposal.Class)
	}
	declaration := action.Declaration{
		ID: e.newID(), Class: proposal.Class, Target: proposal.Target,
		Intent: proposal.Intent, PredictedCleared: proposal.PredictedCleared,
		Generation: cycle.Generation, VectorHash: cycle.Vector.Hash(),
		DeclaredAt: e.now(),
	}
	if err := declaration.Validate(); err != nil {
		return action.Declaration{}, err
	}
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindDeclared, CycleID: cycle.ID, At: declaration.DeclaredAt,
		Generation: cycle.Generation, Declaration: &declaration,
	}); err != nil {
		return action.Declaration{}, err
	}
	return declaration, nil
}

// checkWrites verifies after the fact that the agent stayed inside its bounds.
//
// The guarded paths are checked here as well as before acting, because an
// agent that reached them anyway is exactly the case the refusal exists for.
func (e *Engine) checkWrites(changed []string) error {
	if len(changed) == 0 {
		return nil
	}
	if err := arena.CheckDiff(changed); err != nil {
		return err
	}
	for _, path := range changed {
		if err := e.lane.CheckPath(e.lane.Root + "/" + path); err != nil {
			return err
		}
	}
	return nil
}

// remeasure observes the plant again and records the outcome triple.
//
// This happens inside the cycle that acted so the triple is complete and
// attributable even if the controller stops immediately afterwards. Waiting
// for the next cycle would lose the outcome of the last action ever taken.
func (e *Engine) remeasure(ctx context.Context, cycle Cycle, declaration action.Declaration, before *condition.Set) (*action.Outcome, error) {
	after, err := e.measure(ctx, cycle.ID, e.now())
	if err != nil {
		return nil, err
	}
	afterVector := condition.Derive(after, e.now())
	afterScalar := trend.Scalar(afterVector)
	predicted, cleared, hit := action.Score(declaration, before, after)
	outcome := action.Outcome{
		DeclarationID: declaration.ID, Class: declaration.Class,
		VectorBefore: cycle.Vector.Hash(), VectorAfter: afterVector.Hash(),
		ErrorBefore: cycle.Scalar, ErrorAfter: afterScalar,
		ErrorDelta: afterScalar - cycle.Scalar, PredictionHit: hit,
		PredictedCount: predicted, ActuallyCleared: cleared,
		RecordedAt: e.now(),
	}
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindOutcome, CycleID: cycle.ID, At: outcome.RecordedAt,
		Generation: cycle.Generation, Outcome: &outcome,
	}); err != nil {
		return nil, err
	}
	return &outcome, nil
}
