package journal

import (
	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/trend"
)

// Samples returns the recorded e(t) history, oldest first.
//
// The brakes read this history, so it has to survive a restart. A controller
// that resumed with an empty series would forget that it had already been
// stalling or looping and would resume acting — the runaway the brakes exist
// to prevent.
func Samples(records []Record) []trend.Sample {
	out := make([]trend.Sample, 0)
	for _, record := range records {
		if record.Sample != nil {
			out = append(out, *record.Sample)
		}
	}
	return out
}

// HasAuditBaseline reports whether a successful non-mutating observation was
// recorded for exactly this policy.
//
// Reconcile cannot start without one. The hash must match exactly: a baseline
// recorded under a different policy proves nothing about the one about to run,
// so a near match is treated as no match.
func HasAuditBaseline(records []Record, policyHash string) bool {
	if policyHash == "" {
		return false
	}
	for _, record := range records {
		if record.Kind == KindAuditBaseline && record.PolicyHash == policyHash {
			return true
		}
	}
	return false
}

// Pending returns a declaration that reached prepared or started but never
// reached a terminal state.
//
// Recovery reads external state before retrying, so this reports what needs
// reconciling rather than what to redo. Returning it does not authorise a
// repeat of the action.
func Pending(records []Record) (action.Declaration, bool) {
	open := make(map[string]action.Declaration)
	order := make([]string, 0)
	for _, record := range records {
		if record.Declaration == nil {
			continue
		}
		id := record.Declaration.ID
		switch record.Kind {
		case KindDeclared, KindStarted:
			if _, seen := open[id]; !seen {
				order = append(order, id)
			}
			open[id] = *record.Declaration
		case KindTerminal:
			delete(open, id)
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		if declaration, ok := open[order[i]]; ok {
			return declaration, true
		}
	}
	return action.Declaration{}, false
}

// Outcomes returns every recorded outcome triple, oldest first. This is the
// outer loop's training set.
func Outcomes(records []Record) []action.Outcome {
	out := make([]action.Outcome, 0)
	for _, record := range records {
		if record.Kind == KindOutcome && record.Outcome != nil {
			out = append(out, *record.Outcome)
		}
	}
	return out
}

// CycleIDs returns every cycle that was opened, oldest first.
func CycleIDs(records []Record) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, record := range records {
		if record.Kind != KindCycleStarted || record.CycleID == "" {
			continue
		}
		if !seen[record.CycleID] {
			seen[record.CycleID] = true
			out = append(out, record.CycleID)
		}
	}
	return out
}
