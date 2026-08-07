package policy

import (
	"sort"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
)

// Rule maps a typed condition to the arena-mutating classes it admits.
//
// Matching is on Type and, optionally, exact Reason values. Nothing here reads
// prose: [condition.Vector] does not carry a message, so a rule cannot be
// written against one even by mistake.
type Rule struct {
	Type    condition.Type
	Reasons []string
	Admits  []action.Class
}

// matches reports whether the rule applies to an aggregated entry.
// An empty Reasons list matches the type regardless of reason.
func (r Rule) matches(e condition.Entry) bool {
	if r.Type != e.Type {
		return false
	}
	if len(r.Reasons) == 0 {
		return true
	}
	for _, want := range r.Reasons {
		for _, got := range e.Reasons {
			if want == got {
				return true
			}
		}
	}
	return false
}

// Law is the deterministic control law: an ordered rule table plus the
// priority used to break ties between admitted classes.
type Law struct {
	Rules    []Rule
	Priority []action.Class
}

// Admit builds the admissible set from typed conditions alone.
//
// This is the only function that can add an arena-mutating class, and it takes
// no scalar, no series, and no brake. Blocking evidence admits nothing at all:
// the constitution requires unestablished evidence to fail closed, so an
// Unknown condition leaves only the classes that mutate nothing.
func (l Law) Admit(v condition.Vector) *Admissible {
	admissible := NewAdmissible()
	if v.Blocking() {
		admissible.WithdrawArena("unestablished evidence blocks arena mutation")
		return admissible
	}
	for _, entry := range v.Entries() {
		for _, rule := range l.Rules {
			if !rule.matches(entry) {
				continue
			}
			for _, class := range rule.Admits {
				admissible.admit(class)
			}
		}
	}
	return admissible
}

// Select returns the highest-priority admissible class.
//
// Selection is total: the non-arena classes are always admissible, so there is
// always something to choose. When no arena-mutating class survives, the
// controller escalates rather than idling silently — except when the plant is
// already at its setpoint, where waiting is correct.
func (l Law) Select(v condition.Vector, a *Admissible) (action.Class, bool) {
	if a == nil {
		return action.Defer, false
	}
	if v.Empty() {
		return action.Defer, true
	}
	for _, class := range l.priority() {
		if class.MutatesArena() && a.Allows(class) {
			return class, true
		}
	}
	if a.AnyArena() {
		return action.Defer, false
	}
	return action.RequestHuman, true
}

// priority returns the tie-break order, falling back to enum order so that
// selection never depends on map iteration or rule declaration order.
func (l Law) priority() []action.Class {
	if len(l.Priority) > 0 {
		return l.Priority
	}
	out := make([]action.Class, len(action.Classes))
	copy(out, action.Classes)
	return out
}

// Targets returns the subjects a class may act on for the given vector, in
// deterministic order, so that discovery order never changes the result.
func Targets(v condition.Vector) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, entry := range v.Entries() {
		for _, subject := range entry.Subjects {
			if !seen[subject] {
				seen[subject] = true
				out = append(out, subject)
			}
		}
	}
	sort.Strings(out)
	return out
}
