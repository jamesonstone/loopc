// Package condition defines the typed observations the control law reads.
//
// A condition names a problem with the plant. Status reports whether that
// problem is currently present:
//
//	True     the problem is present and unsatisfied
//	False    the problem is absent
//	Unknown  the problem could not be established
//
// Unknown is not absence. It contributes to error and blocks arena mutation,
// because the constitution requires missing or ambiguous evidence to fail
// closed rather than be read as success.
package condition

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status reports whether a named problem is currently present.
type Status string

const (
	StatusTrue    Status = "True"
	StatusFalse   Status = "False"
	StatusUnknown Status = "Unknown"
)

// Valid reports whether the status is one of the three defined values.
func (s Status) Valid() bool {
	return s == StatusTrue || s == StatusFalse || s == StatusUnknown
}

// Type names a class of problem. It is machine-readable and closed by the
// controller that defines it, never free text derived from a plant.
type Type string

// Key identifies one condition instance: a type plus the exact subject it is
// about. A plant-wide condition carries an empty subject.
type Key struct {
	Type    Type
	Subject string
}

// String renders the key for logs and deterministic ordering.
func (k Key) String() string {
	if k.Subject == "" {
		return string(k.Type)
	}
	return string(k.Type) + "/" + k.Subject
}

// Condition is one typed, evidence-carrying observation about the plant.
//
// Message is for humans and is never read by the control law. That rule is
// enforced structurally rather than by convention: [Vector], the only view the
// control law is given, does not carry the field at all.
type Condition struct {
	Type               Type      `json:"type"`
	Subject            string    `json:"subject,omitempty"`
	Status             Status    `json:"status"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message,omitempty"`
	FirstObservedAt    time.Time `json:"first_observed_at"`
	LastTransitionAt   time.Time `json:"last_transition_at"`
	ObservedGeneration string    `json:"observed_generation"`
}

// Key returns the identity of this condition instance.
func (c Condition) Key() Key {
	return Key{Type: c.Type, Subject: c.Subject}
}

// Unsatisfied reports whether the condition contributes to the error term.
// Both a present problem and an unestablished one are unsatisfied.
func (c Condition) Unsatisfied() bool {
	return c.Status == StatusTrue || c.Status == StatusUnknown
}

// Blocks reports whether this condition forbids arena mutation. Only
// unestablished evidence blocks; a known problem is what the loop exists to
// act on.
func (c Condition) Blocks() bool {
	return c.Status == StatusUnknown
}

// Age reports how long the condition has been continuously observed. It is the
// persistence signal the integral brake reads.
func (c Condition) Age(now time.Time) time.Duration {
	if c.FirstObservedAt.IsZero() || now.Before(c.FirstObservedAt) {
		return 0
	}
	return now.Sub(c.FirstObservedAt)
}

var (
	// ErrMissingType is returned when a condition carries no type.
	ErrMissingType = errors.New("condition requires a type")
	// ErrInvalidStatus is returned for a status outside the defined three.
	ErrInvalidStatus = errors.New("condition status must be True, False, or Unknown")
	// ErrMissingReason is returned when a condition carries no reason.
	ErrMissingReason = errors.New("condition requires a machine-readable reason")
)

// Validate reports whether the condition is well formed enough to control on.
// A reason is mandatory: the control law matches on it, so an absent reason
// would silently widen every rule that reads it.
func (c Condition) Validate() error {
	if strings.TrimSpace(string(c.Type)) == "" {
		return ErrMissingType
	}
	if !c.Status.Valid() {
		return fmt.Errorf("%w: got %q", ErrInvalidStatus, c.Status)
	}
	if strings.TrimSpace(c.Reason) == "" {
		return fmt.Errorf("%w: type %s", ErrMissingReason, c.Type)
	}
	return nil
}
