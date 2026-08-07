package policy

import (
	"fmt"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/trend"
)

// Brakes bound the loop. Both borrow from PID, and both only narrow.
//
// Neither takes an [Admissible] it can widen: every method here calls only
// Withdraw. No threshold in this file causes an arena mutation that typed
// conditions had not already admitted.
type Brakes struct {
	// IntegralBudget is how long one condition may persist before the
	// arena-mutating classes are withdrawn from it and the controller
	// escalates instead of trying again.
	IntegralBudget time.Duration
	// DerivativeWindow is how many acting cycles may pass without reducing
	// error before the controller stops acting.
	DerivativeWindow int
	// OscillationThreshold is how often one plant shape and class may recur
	// before the pair is treated as a limit cycle.
	OscillationThreshold int
}

// DefaultBrakes are conservative starting values. They are configuration, not
// tuning: no value here can admit an action, so a wrong setting can only make
// the controller stop earlier than necessary.
func DefaultBrakes() Brakes {
	return Brakes{
		IntegralBudget:       6 * time.Hour,
		DerivativeWindow:     3,
		OscillationThreshold: 3,
	}
}

// Apply withdraws classes from the admissible set according to the recorded
// history. It returns the reasons applied, for the journal and for `explain`.
func (b Brakes) Apply(a *Admissible, v condition.Vector, s *trend.Series) []Withdrawal {
	if a == nil {
		return nil
	}
	b.applyIntegral(a, v)
	b.applyDerivative(a, s)
	b.applyOscillation(a, v, s)
	return a.Withdrawals()
}

// applyIntegral withdraws the arena-mutating classes once a condition has
// persisted past its budget.
//
// Persistence is the signal that repetition is not working. Withdrawing leaves
// request_human as the terminal outcome, which is the point: the constitution
// requires escalation rather than another attempt.
func (b Brakes) applyIntegral(a *Admissible, v condition.Vector) {
	if b.IntegralBudget <= 0 {
		return
	}
	for _, entry := range v.Entries() {
		if entry.OldestAge < b.IntegralBudget {
			continue
		}
		a.WithdrawArena(fmt.Sprintf(
			"integral brake: %s has persisted %s, past the %s budget",
			entry.Type, entry.OldestAge.Round(time.Second), b.IntegralBudget))
		return
	}
}

// applyDerivative withdraws the arena-mutating classes when recent acting
// cycles have stopped reducing error.
//
// This is the guard against an agent burning effort forever: if acting is not
// working, the controller stops acting. Halting needs no authority, so the
// brake can always take effect.
func (b Brakes) applyDerivative(a *Admissible, s *trend.Series) {
	if s == nil || b.DerivativeWindow <= 0 {
		return
	}
	if !s.Stalled(b.DerivativeWindow) {
		return
	}
	a.WithdrawArena(fmt.Sprintf(
		"derivative brake: %d acting cycles produced no error reduction",
		s.NoReductionRun()))
}

// applyOscillation withdraws a single class that keeps recurring against an
// unchanged plant shape.
//
// Unlike the other two brakes this is surgical: only the looping class is
// withdrawn, so the controller may still try a different one. A limit cycle is
// evidence that one class is wrong for this shape, not that every class is.
func (b Brakes) applyOscillation(a *Admissible, v condition.Vector, s *trend.Series) {
	if s == nil || b.OscillationThreshold <= 0 {
		return
	}
	hash := v.Hash()
	for _, class := range action.ArenaMutating() {
		if !s.Oscillating(hash, class, b.OscillationThreshold) {
			continue
		}
		a.Withdraw(class, fmt.Sprintf(
			"oscillation: %s has been applied %d times to an unchanged plant shape",
			class, s.Repeat(hash, class)))
	}
}
