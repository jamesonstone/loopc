// Package journal is the append-only ledger.
//
// It is stored as newline-delimited JSON opened with O_APPEND, so append-only
// is a property of the file descriptor rather than a convention the code is
// trusted to honour: the kernel positions every write at the end, and nothing
// in this package seeks, truncates, or rewrites.
//
// The ledger is also the training set. Outcome records are written from the
// first cycle even though nothing learns from them yet, because the schema
// cannot be retrofitted onto cycles that have already run while a learner can
// be added at any time. A flat, greppable stream suits that second purpose
// better than a database would at this scale.
package journal

import (
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/policy"
	"github.com/jamesonstone/loopc/internal/trend"
)

// Kind names the shape of a record.
type Kind string

const (
	// KindCycleStarted opens a cycle.
	KindCycleStarted Kind = "cycle_started"
	// KindMeasured records the observed conditions and the error term.
	KindMeasured Kind = "measured"
	// KindAdmitted records the admissible set and any withdrawals.
	KindAdmitted Kind = "admitted"
	// KindDeclared is the prepared state: a committed declaration.
	KindDeclared Kind = "action_declared"
	// KindStarted is the started state: execution has begun.
	KindStarted Kind = "action_started"
	// KindObserved is the observed state: external effect has been read back.
	KindObserved Kind = "action_observed"
	// KindTerminal is the terminal state: succeeded, failed, or refused.
	KindTerminal Kind = "action_terminal"
	// KindOutcome records the triple the outer loop will learn from.
	KindOutcome Kind = "outcome_recorded"
	// KindAuditBaseline records a successful non-mutating observation.
	KindAuditBaseline Kind = "audit_baseline"
	// KindCycleFinished closes a cycle.
	KindCycleFinished Kind = "cycle_finished"
)

// Terminal outcomes for an action.
const (
	TerminalSucceeded = "succeeded"
	TerminalFailed    = "failed"
	TerminalRefused   = "refused"
)

// Record is one line of the ledger.
//
// Every field is optional except Sequence, Kind, and At, so that a reader can
// decode the complete stream without knowing which kinds a given version wrote.
// Adding a field is always safe; removing one is not.
type Record struct {
	Sequence    int64                 `json:"sequence"`
	Kind        Kind                  `json:"kind"`
	At          time.Time             `json:"at"`
	CycleID     string                `json:"cycle_id,omitempty"`
	Generation  string                `json:"generation,omitempty"`
	Mode        string                `json:"mode,omitempty"`
	PolicyHash  string                `json:"policy_hash,omitempty"`
	Conditions  []condition.Condition `json:"conditions,omitempty"`
	Sample      *trend.Sample         `json:"sample,omitempty"`
	Admissible  []action.Class        `json:"admissible,omitempty"`
	Withdrawals []policy.Withdrawal   `json:"withdrawals,omitempty"`
	Declaration *action.Declaration   `json:"declaration,omitempty"`
	Outcome     *action.Outcome       `json:"outcome,omitempty"`
	Terminal    string                `json:"terminal,omitempty"`
	Note        string                `json:"note,omitempty"`
	Detail      map[string]any        `json:"detail,omitempty"`
}
