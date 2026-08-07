// Package policy is the deterministic control law.
//
// Its central type is the admissible action set, built so that the
// constitution's asymmetry is structural. Only [Admit] can add an
// arena-mutating class, and it reads nothing but typed conditions. Everything
// driven by the scalar — both brakes — can only call [Admissible.Withdraw].
// There is no exported path from a number to an admitted class.
package policy

import (
	"sort"

	"github.com/jamesonstone/loopc/internal/action"
)

// Admissible is the set of classes available on the current cycle.
//
// The non-arena classes are members from construction and cannot be removed:
// they mutate nothing, so escalating and halting require no authority and stay
// available no matter what the brakes have withdrawn.
type Admissible struct {
	arena     map[action.Class]bool
	withdrawn map[action.Class]string
}

// NewAdmissible returns a set containing only the non-arena classes.
//
// This is the floor. A controller that has admitted nothing can still escalate
// or wait, which is what makes failing closed always possible.
func NewAdmissible() *Admissible {
	return &Admissible{
		arena:     make(map[action.Class]bool),
		withdrawn: make(map[action.Class]string),
	}
}

// admit adds an arena-mutating class. It is unexported so that only [Admit],
// which reads typed conditions, can widen the set.
func (a *Admissible) admit(c action.Class) {
	if !c.Valid() || !c.MutatesArena() {
		return
	}
	if _, blocked := a.withdrawn[c]; blocked {
		return
	}
	a.arena[c] = true
}

// Withdraw removes an arena-mutating class and records why.
//
// Withdrawal is permanent for the cycle: a later admission cannot resurrect a
// class a brake has taken away. Asking to withdraw a non-arena class is a
// no-op, because those are never authority in the first place.
func (a *Admissible) Withdraw(c action.Class, reason string) {
	if !c.MutatesArena() {
		return
	}
	delete(a.arena, c)
	if _, exists := a.withdrawn[c]; !exists {
		a.withdrawn[c] = reason
	}
}

// WithdrawArena removes every arena-mutating class, leaving only the classes
// that mutate nothing.
func (a *Admissible) WithdrawArena(reason string) {
	for _, c := range action.ArenaMutating() {
		a.Withdraw(c, reason)
	}
}

// Allows reports whether the class may be selected this cycle.
func (a *Admissible) Allows(c action.Class) bool {
	if !c.Valid() {
		return false
	}
	if !c.MutatesArena() {
		return true
	}
	return a.arena[c]
}

// Classes returns every admissible class in deterministic order.
func (a *Admissible) Classes() []action.Class {
	out := make([]action.Class, 0, len(action.Classes))
	for _, c := range action.Classes {
		if a.Allows(c) {
			out = append(out, c)
		}
	}
	return out
}

// AnyArena reports whether any arena-mutating class survives.
func (a *Admissible) AnyArena() bool { return len(a.arena) > 0 }

// Withdrawals returns the recorded reasons in deterministic order, so an
// operator can see exactly why the controller declined to act.
func (a *Admissible) Withdrawals() []Withdrawal {
	out := make([]Withdrawal, 0, len(a.withdrawn))
	for c, reason := range a.withdrawn {
		out = append(out, Withdrawal{Class: c, Reason: reason})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Class < out[j].Class })
	return out
}

// Withdrawal records one class removed from the admissible set and why.
type Withdrawal struct {
	Class  action.Class `json:"class"`
	Reason string       `json:"reason"`
}
