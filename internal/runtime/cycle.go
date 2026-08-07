package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/policy"
	"github.com/jamesonstone/loopc/internal/trend"
)

// Cycle is the result of one pass through the loop.
type Cycle struct {
	ID          string
	Generation  string
	Mode        Mode
	Vector      condition.Vector
	Scalar      float64
	Admissible  []action.Class
	Withdrawals []policy.Withdrawal
	Selected    action.Class
	Acted       bool
	Outcome     *action.Outcome
}

// RunOnce performs one complete cycle.
func (e *Engine) RunOnce(ctx context.Context) (Cycle, error) {
	cycle := Cycle{ID: e.newID(), Mode: e.config.Mode}
	now := e.now()
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindCycleStarted, CycleID: cycle.ID, At: now,
		Mode: string(e.config.Mode), PolicyHash: e.config.PolicyHash(),
	}); err != nil {
		return cycle, err
	}

	set, err := e.measure(ctx, cycle.ID, now)
	if err != nil {
		return cycle, err
	}
	cycle.Generation = set.Generation()
	cycle.Vector = condition.Derive(set, now)
	cycle.Scalar = trend.Scalar(cycle.Vector)

	admissible := e.config.Law.Admit(cycle.Vector)
	cycle.Withdrawals = e.config.Brakes.Apply(admissible, cycle.Vector, e.series)
	cycle.Admissible = admissible.Classes()
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindAdmitted, CycleID: cycle.ID, At: e.now(),
		Generation: cycle.Generation, Admissible: cycle.Admissible,
		Withdrawals: cycle.Withdrawals,
	}); err != nil {
		return cycle, err
	}

	selected, ok := e.config.Law.Select(cycle.Vector, admissible)
	if ok {
		cycle.Selected = selected
	}

	if e.config.Mode == ModeAudit {
		return e.finishAudit(cycle, set, ok)
	}
	if !ok || !selected.MutatesArena() {
		return e.finishWithoutArena(cycle, set, selected, ok)
	}
	return e.act(ctx, cycle, set, admissible)
}

// measure observes the plant and types the result.
//
// Conditions returned by the sensor are untrusted hypothesis: each is
// validated, and the generation is stamped by the runtime rather than accepted
// from the agent, so a sensor cannot bind its output to a generation other
// than the one actually observed.
func (e *Engine) measure(ctx context.Context, cycleID string, now time.Time) (*condition.Set, error) {
	observation, err := e.plant.Observe(ctx)
	if err != nil {
		return nil, fmt.Errorf("observe plant: %w", err)
	}
	observed, err := e.agent.Classify(ctx, observation)
	if err != nil {
		return nil, fmt.Errorf("classify observation: %w", err)
	}
	for _, c := range observed {
		if err := c.Validate(); err != nil {
			return nil, fmt.Errorf("sensor returned an invalid condition: %w", err)
		}
	}
	set, err := condition.Observe(e.previous, observation.Generation, observed, now)
	if err != nil {
		return nil, err
	}
	e.previous = set
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindMeasured, CycleID: cycleID, At: now,
		Generation: set.Generation(), Conditions: set.All(),
	}); err != nil {
		return nil, err
	}
	return set, nil
}

// record appends a sample to the series and closes the cycle.
func (e *Engine) record(cycle Cycle, class action.Class) error {
	oldest := time.Duration(0)
	for _, entry := range cycle.Vector.Entries() {
		if entry.OldestAge > oldest {
			oldest = entry.OldestAge
		}
	}
	sample := trend.Sample{
		CycleID: cycle.ID, At: e.now(), Scalar: cycle.Scalar,
		Total: cycle.Vector.Total(), VectorHash: cycle.Vector.Hash(),
		Class: class, Blocking: cycle.Vector.Blocking(), OldestAge: oldest,
	}
	e.series.Append(sample)
	// The sample is journalled as well as held in memory so the brakes survive
	// a restart. A controller resuming with an empty series would forget it had
	// been stalling and would start acting again.
	_, err := e.ledger.Append(journal.Record{
		Kind: journal.KindCycleFinished, CycleID: cycle.ID, At: e.now(),
		Generation: cycle.Generation, Sample: &sample,
	})
	return err
}

// finishAudit closes a non-mutating cycle and records the baseline that
// reconcile will later require.
//
// The baseline is written only when observation actually succeeded. A cycle
// that failed to observe proves nothing and must not unlock reconcile.
func (e *Engine) finishAudit(cycle Cycle, set *condition.Set, selected bool) (Cycle, error) {
	if set != nil && set.Generation() != "" {
		if _, err := e.ledger.Append(journal.Record{
			Kind: journal.KindAuditBaseline, CycleID: cycle.ID, At: e.now(),
			Generation: cycle.Generation, PolicyHash: e.config.PolicyHash(),
			Mode: string(ModeAudit),
		}); err != nil {
			return cycle, err
		}
	}
	if !selected {
		cycle.Selected = ""
	}
	return cycle, e.record(cycle, "")
}

// finishWithoutArena closes a cycle whose selected class mutates nothing.
//
// request_human and defer are still recorded: escalating and waiting are real
// outcomes the outer loop needs to see, and a controller that silently idles
// is indistinguishable from one that is stuck.
func (e *Engine) finishWithoutArena(cycle Cycle, set *condition.Set, selected action.Class, ok bool) (Cycle, error) {
	note := "no arena-mutating class was admitted"
	if !ok {
		note = "no class could be selected"
	}
	if _, err := e.ledger.Append(journal.Record{
		Kind: journal.KindTerminal, CycleID: cycle.ID, At: e.now(),
		Generation: cycle.Generation, Terminal: journal.TerminalSucceeded,
		Note: note, Detail: map[string]any{"class": string(selected)},
	}); err != nil {
		return cycle, err
	}
	return cycle, e.record(cycle, "")
}
