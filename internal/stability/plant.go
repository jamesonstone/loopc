// Package stability provides a deterministic plant and agent for proving the
// control loop converges.
//
// It exists so the first evidence for or against the thesis costs no tokens,
// runs against a system we fully control, and keeps any instability
// attributable to the controller rather than to a model. Nothing here touches
// the network, the clock, or a random source: a failing run reproduces exactly.
package stability

import (
	"context"
	"sort"

	"github.com/jamesonstone/loopc/internal/agent"
)

// Plant is a system under control whose state evolves in response to actions.
//
// The static fakes in the runtime tests are enough to prove authority gates and
// useless for proving stability, because a plant that never changes can neither
// converge nor oscillate. This one can do both.
type Plant struct {
	generation string
	open       map[string]bool
	spawn      map[string]string
	unfixable  map[string]bool

	// Observations counts sensor reads, including the remeasurement that
	// closes an acting cycle.
	Observations int
	// Attempts counts actions aimed at this plant, successful or not.
	Attempts int
}

// NewPlant returns an empty plant bound to a generation.
func NewPlant(generation string) *Plant {
	return &Plant{
		generation: generation,
		open:       make(map[string]bool),
		spawn:      make(map[string]string),
		unfixable:  make(map[string]bool),
	}
}

// Seed opens conditions on the plant.
func (p *Plant) Seed(subjects ...string) {
	for _, subject := range subjects {
		p.open[subject] = true
	}
}

// SpawnOnResolve makes resolving one subject open another.
//
// This is how the rung 1 limit cycle is reproduced on demand: an agent fixes A
// and thereby creates B, a reviewer flags B, fixing B recreates A, and the
// controller can loop forever while the error count never falls.
func (p *Plant) SpawnOnResolve(subject, spawns string) {
	p.spawn[subject] = spawns
}

// MarkUnfixable makes subjects that no action will ever clear, modelling
// feedback no patch can satisfy.
func (p *Plant) MarkUnfixable(subjects ...string) {
	for _, subject := range subjects {
		p.unfixable[subject] = true
	}
}

// Inject opens a condition mid-run, modelling a reviewer commenting during
// convergence.
func (p *Plant) Inject(subjects ...string) { p.Seed(subjects...) }

// Open returns the currently unresolved subjects in deterministic order.
func (p *Plant) Open() []string {
	out := make([]string, 0, len(p.open))
	for subject := range p.open {
		out = append(out, subject)
	}
	sort.Strings(out)
	return out
}

// Count returns how many conditions are unresolved.
func (p *Plant) Count() int { return len(p.open) }

// Converged reports whether the plant has reached its setpoint.
func (p *Plant) Converged() bool { return len(p.open) == 0 }

// Resolve attempts to clear one subject and reports whether the plant changed.
//
// An unfixable subject stays open, so the error term does not fall and the
// controller has to notice that acting is not working rather than being told.
func (p *Plant) Resolve(subject string) bool {
	p.Attempts++
	if !p.open[subject] || p.unfixable[subject] {
		return false
	}
	delete(p.open, subject)
	if spawned, ok := p.spawn[subject]; ok {
		p.open[spawned] = true
	}
	return true
}

// Observe implements the runtime's plant interface.
func (p *Plant) Observe(context.Context) (agent.Observation, error) {
	p.Observations++
	return agent.Observation{Generation: p.generation, Payload: p.Open()}, nil
}
