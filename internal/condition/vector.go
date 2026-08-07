package condition

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Vector is the typed error term: the only view of the plant the control law
// is given.
//
// It deliberately carries no Message. The constitution forbids a control law
// that matches on human-readable text, and omitting the field makes that
// structural rather than conventional — a rule enforced by the type system
// cannot be violated by a later edit that reaches for a convenient string.
type Vector struct {
	entries []Entry
}

// Entry aggregates every unsatisfied condition sharing one type.
type Entry struct {
	Type      Type          `json:"type"`
	Count     int           `json:"count"`
	OldestAge time.Duration `json:"oldest_age"`
	Blocking  bool          `json:"blocking"`
	Reasons   []string      `json:"reasons"`
	Subjects  []string      `json:"subjects"`
}

// Derive reduces a condition set to the typed error term. Only unsatisfied
// conditions contribute: a problem that is absent is not error.
func Derive(s *Set, now time.Time) Vector {
	if s == nil {
		return Vector{}
	}
	grouped := make(map[Type]*Entry)
	for _, c := range s.Unsatisfied() {
		e, ok := grouped[c.Type]
		if !ok {
			e = &Entry{Type: c.Type}
			grouped[c.Type] = e
		}
		e.Count++
		e.Blocking = e.Blocking || c.Blocks()
		if age := c.Age(now); age > e.OldestAge {
			e.OldestAge = age
		}
		e.Reasons = appendUnique(e.Reasons, c.Reason)
		if c.Subject != "" {
			e.Subjects = appendUnique(e.Subjects, c.Subject)
		}
	}
	out := make([]Entry, 0, len(grouped))
	for _, e := range grouped {
		sort.Strings(e.Reasons)
		sort.Strings(e.Subjects)
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return Vector{entries: out}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// Entries returns the aggregated types in deterministic order.
func (v Vector) Entries() []Entry { return v.entries }

// Empty reports whether the plant is at its setpoint.
func (v Vector) Empty() bool { return len(v.entries) == 0 }

// Total returns the number of unsatisfied condition instances.
func (v Vector) Total() int {
	total := 0
	for _, e := range v.entries {
		total += e.Count
	}
	return total
}

// Blocking reports whether any unestablished evidence forbids arena mutation.
func (v Vector) Blocking() bool {
	for _, e := range v.entries {
		if e.Blocking {
			return true
		}
	}
	return false
}

// Find returns the aggregated entry for a type.
func (v Vector) Find(t Type) (Entry, bool) {
	for _, e := range v.entries {
		if e.Type == t {
			return e, true
		}
	}
	return Entry{}, false
}

// Has reports whether the type contributes to the error term.
func (v Vector) Has(t Type) bool {
	_, ok := v.Find(t)
	return ok
}

// Hash is a deterministic fingerprint of the error term's shape, excluding
// ages so that a plant which is merely getting older does not read as changed.
// Repeating a (hash, action class) pair is how the loop recognises a limit
// cycle.
func (v Vector) Hash() string {
	var b strings.Builder
	for _, e := range v.entries {
		fmt.Fprintf(&b, "%s|%d|%t|%s|%s\n", e.Type, e.Count, e.Blocking,
			strings.Join(e.Reasons, ","), strings.Join(e.Subjects, ","))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
