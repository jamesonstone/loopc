package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/policy"
)

// Mode is the authority ceiling.
type Mode string

const (
	// ModeAudit observes and proposes but has no executor.
	ModeAudit Mode = "audit"
	// ModeReconcile may execute admitted actions inside the arena.
	ModeReconcile Mode = "reconcile"
)

// Valid reports whether the mode is one of the two defined ceilings.
func (m Mode) Valid() bool { return m == ModeAudit || m == ModeReconcile }

var (
	// ErrInvalidMode is returned for a mode outside the two defined ceilings.
	ErrInvalidMode = errors.New("mode must be audit or reconcile")
	// ErrNoAuditBaseline is returned when reconcile has no matching baseline.
	ErrNoAuditBaseline = errors.New(
		"reconcile requires a recorded audit observation for this exact policy")
	// ErrNoExecutor is returned when an admitted action has nothing to run it.
	ErrNoExecutor = errors.New("admitted action has no executor")
)

// Config is everything that binds a cycle's authority.
type Config struct {
	Mode       Mode
	Law        policy.Law
	Brakes     policy.Brakes
	IdlePoll   time.Duration
	ActivePoll time.Duration
	Oldest     time.Duration
}

// PolicyHash fingerprints every input that defines what the controller may do.
//
// An audit baseline is only accepted for an identical hash. Changing a rule, a
// brake, or the mode ceiling therefore invalidates the baseline and forces a
// fresh audit observation before reconcile can start again — which is the point
// of requiring one at all.
func (c Config) PolicyHash() string {
	var b strings.Builder
	for _, rule := range c.Law.Rules {
		fmt.Fprintf(&b, "rule:%s|%s|%v\n", rule.Type,
			strings.Join(rule.Reasons, ","), rule.Admits)
	}
	fmt.Fprintf(&b, "priority:%v\n", c.Law.Priority)
	fmt.Fprintf(&b, "brakes:%s|%d|%d\n", c.Brakes.IntegralBudget,
		c.Brakes.DerivativeWindow, c.Brakes.OscillationThreshold)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// Validate reports whether the configuration may run at all.
func (c Config) Validate() error {
	if !c.Mode.Valid() {
		return fmt.Errorf("%w: got %q", ErrInvalidMode, c.Mode)
	}
	return nil
}

// authorise checks the mode ceiling against recorded history.
//
// Audit always runs: it mutates nothing. Reconcile must find a baseline whose
// policy hash matches exactly, because a baseline recorded under a different
// policy proves nothing about the one about to act.
func authorise(config Config, records []journal.Record) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if config.Mode == ModeAudit {
		return nil
	}
	if !journal.HasAuditBaseline(records, config.PolicyHash()) {
		return fmt.Errorf("%w: run once in audit mode first", ErrNoAuditBaseline)
	}
	return nil
}
