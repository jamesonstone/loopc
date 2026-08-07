package condition

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Set is the complete condition set observed for one generation.
//
// A generation binds the set to the exact configuration and plant head it was
// observed against. Conditions from a superseded generation are stale and may
// not be controlled on.
type Set struct {
	generation string
	items      map[Key]Condition
}

// ErrGenerationMismatch is returned when a condition is bound to a generation
// other than the set's own.
var ErrGenerationMismatch = errors.New("condition belongs to a different generation")

// NewSet returns an empty set bound to the given generation.
func NewSet(generation string) *Set {
	return &Set{generation: generation, items: make(map[Key]Condition)}
}

// Generation returns the configuration and head this set was observed against.
func (s *Set) Generation() string { return s.generation }

// Len returns the number of conditions in the set.
func (s *Set) Len() int { return len(s.items) }

// Get returns the condition stored under the key.
func (s *Set) Get(k Key) (Condition, bool) {
	c, ok := s.items[k]
	return c, ok
}

// All returns every condition in deterministic key order, so that callers,
// hashes, and journal records never depend on map iteration order.
func (s *Set) All() []Condition {
	out := make([]Condition, 0, len(s.items))
	for _, c := range s.items {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key().String() < out[j].Key().String()
	})
	return out
}

// Unsatisfied returns the conditions contributing to the error term.
func (s *Set) Unsatisfied() []Condition {
	out := make([]Condition, 0, len(s.items))
	for _, c := range s.All() {
		if c.Unsatisfied() {
			out = append(out, c)
		}
	}
	return out
}

// Blocking reports whether any condition forbids arena mutation.
func (s *Set) Blocking() bool {
	for _, c := range s.items {
		if c.Blocks() {
			return true
		}
	}
	return false
}

// Upsert stores a condition, rejecting one bound to another generation.
func (s *Set) Upsert(c Condition) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.ObservedGeneration != s.generation {
		return fmt.Errorf("%w: set %q, condition %q", ErrGenerationMismatch,
			s.generation, c.ObservedGeneration)
	}
	s.items[c.Key()] = c
	return nil
}

// Fresh reports whether the set was observed against the wanted generation. A
// stale set blocks rather than being controlled on.
func (s *Set) Fresh(want string) bool { return s.generation == want }

// Observe builds the current set from freshly observed conditions, carrying
// timestamps forward from previous where a condition persists.
//
// FirstObservedAt survives as long as the condition keeps appearing, which is
// what gives the integral brake its persistence signal. LastTransitionAt moves
// only when the status actually changes, so a condition that is re-observed
// unchanged does not look new.
func Observe(previous *Set, generation string, observed []Condition, now time.Time) (*Set, error) {
	next := NewSet(generation)
	for _, c := range observed {
		c.ObservedGeneration = generation
		c.FirstObservedAt = now
		c.LastTransitionAt = now
		if previous != nil {
			if prior, ok := previous.Get(c.Key()); ok {
				if !prior.FirstObservedAt.IsZero() {
					c.FirstObservedAt = prior.FirstObservedAt
				}
				if prior.Status == c.Status && !prior.LastTransitionAt.IsZero() {
					c.LastTransitionAt = prior.LastTransitionAt
				}
			}
		}
		if err := next.Upsert(c); err != nil {
			return nil, err
		}
	}
	return next, nil
}
