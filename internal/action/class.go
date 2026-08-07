// Package action defines the closed actuator enum and the durable declaration
// an agent must commit before it acts.
//
// The enum is closed on purpose. The agent chooses freely within a class, but
// the class itself is fixed, because a policy cannot be learned over an untyped
// action space — and tuning the policy from recorded outcomes is the whole
// reason the journal exists.
package action

import (
	"errors"
	"fmt"
)

// Class is one member of the closed actuator enum. Adding a member is a
// constitution change, not an implementation detail.
type Class string

const (
	// PatchCode changes product source inside the arena.
	PatchCode Class = "patch_code"
	// PatchTest changes test source inside the arena.
	PatchTest Class = "patch_test"
	// PatchDocs changes documentation inside the arena.
	PatchDocs Class = "patch_docs"
	// ReplyNoChange answers feedback without changing anything.
	ReplyNoChange Class = "reply_no_change"
	// RequestHuman escalates and stops. It is terminal, never repeated.
	RequestHuman Class = "request_human"
	// Defer waits for the plant to change on its own.
	Defer Class = "defer"
)

// Classes is every member of the enum in deterministic order.
var Classes = []Class{PatchCode, PatchTest, PatchDocs, ReplyNoChange, RequestHuman, Defer}

// arena records which classes mutate inside the arena.
//
// ReplyNoChange is arena-mutating despite changing no code: it writes to
// GitHub, which is outside the worktree but inside the blast radius the arena
// exists to bound.
var arena = map[Class]bool{
	PatchCode:     true,
	PatchTest:     true,
	PatchDocs:     true,
	ReplyNoChange: true,
	RequestHuman:  false,
	Defer:         false,
}

// Valid reports whether the class is a member of the enum.
func (c Class) Valid() bool {
	_, ok := arena[c]
	return ok
}

// MutatesArena reports whether selecting this class exercises authority.
//
// RequestHuman and Defer notify or wait and never write to a worktree, a
// branch, or GitHub. Because they mutate nothing they require no authority and
// stay available even after every arena-mutating class has been withdrawn,
// which is what lets the brakes halt and escalate without granting anything.
func (c Class) MutatesArena() bool { return arena[c] }

// NonArena returns the classes that require no authority.
func NonArena() []Class {
	out := make([]Class, 0, 2)
	for _, c := range Classes {
		if !c.MutatesArena() {
			out = append(out, c)
		}
	}
	return out
}

// ArenaMutating returns the classes that require authority.
func ArenaMutating() []Class {
	out := make([]Class, 0, len(Classes))
	for _, c := range Classes {
		if c.MutatesArena() {
			out = append(out, c)
		}
	}
	return out
}

// ErrUnknownClass is returned for a class outside the closed enum.
var ErrUnknownClass = errors.New("action class is not a member of the closed enum")

// Parse converts external text into a class, rejecting anything unknown. An
// agent proposing a class the enum does not define is refused rather than
// accommodated.
func Parse(value string) (Class, error) {
	c := Class(value)
	if !c.Valid() {
		return "", fmt.Errorf("%w: %q", ErrUnknownClass, value)
	}
	return c, nil
}
