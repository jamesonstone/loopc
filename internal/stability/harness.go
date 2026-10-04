package stability

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/arena"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/policy"
	"github.com/jamesonstone/loopc/internal/runtime"
)

// Law is the control law under test: an actionable thread admits a code patch.
func Law() policy.Law {
	return policy.Law{Rules: []policy.Rule{
		{Type: ConditionType, Admits: []action.Class{action.PatchCode}},
	}}
}

// Config assembles a reconcile-mode configuration with the given brakes.
func Config(brakes policy.Brakes) runtime.Config {
	return runtime.Config{Mode: runtime.ModeReconcile, Law: Law(), Brakes: brakes}
}

// NoBrakes disables all three brakes, for tests measuring the loop's own
// convergence rather than a brake's intervention.
func NoBrakes() policy.Brakes { return policy.Brakes{} }

// Baseline returns the audit observation reconcile requires, bound to the
// exact policy under test.
func Baseline(config runtime.Config) []journal.Record {
	return []journal.Record{{
		Kind: journal.KindAuditBaseline, PolicyHash: config.PolicyHash(),
	}}
}

// Harness is one assembled controller plus the plant it regulates.
type Harness struct {
	Engine *runtime.Engine
	Plant  *Plant
	Model  *Model
	Ledger *journal.Journal
}

// Build assembles a deterministic controller over the plant.
//
// The clock advances by a fixed step and identifiers are sequential, so a run
// is reproducible and the recorded ages the integral brake reads are exact
// rather than dependent on how fast the test machine happens to be.
func Build(directory string, plant *Plant, brakes policy.Brakes) (*Harness, error) {
	ledger, err := journal.Open(filepath.Join(directory, "journal.db"))
	if err != nil {
		return nil, err
	}
	config := Config(brakes)
	model := NewModel(plant)

	clock := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	counter := 0

	engine, err := runtime.New(runtime.Options{
		Config: config, Plant: plant, Agent: model,
		Journal: ledger, History: Baseline(config),
		Arena: arena.Arena{
			Root: filepath.Join(directory, "lane"), Owner: "o", Repository: "r",
			Lane: "GH-1", Branch: "GH-1", DefaultBranch: "main",
		},
		Now: func() time.Time { clock = clock.Add(time.Minute); return clock },
		NewID: func() string {
			counter++
			return fmt.Sprintf("id-%03d", counter)
		},
	})
	if err != nil {
		ledger.Close()
		return nil, err
	}
	return &Harness{Engine: engine, Plant: plant, Model: model, Ledger: ledger}, nil
}

// Close releases the ledger.
func (h *Harness) Close() error { return h.Ledger.Close() }

// Run advances the loop until the plant converges or the cycle budget is spent.
//
// The budget is what makes non-convergence a test failure rather than a hang: a
// controller that never settles must be reported, not waited on.
func (h *Harness) Run(ctx context.Context, budget int) (Trace, error) {
	trace := Trace{}
	for range budget {
		if h.Plant.Converged() {
			return trace, nil
		}
		cycle, err := h.Engine.RunOnce(ctx)
		if err != nil {
			return trace, err
		}
		trace.Cycles = append(trace.Cycles, cycle)
		if cycle.Selected == action.RequestHuman {
			trace.Escalated = true
			return trace, nil
		}
	}
	return trace, nil
}

// Trace is what a run did, in the terms the stability criteria are stated in.
type Trace struct {
	Cycles    []runtime.Cycle
	Escalated bool
}

// Len returns the number of cycles run.
func (t Trace) Len() int { return len(t.Cycles) }

// Acted returns how many cycles performed an arena mutation.
func (t Trace) Acted() int {
	count := 0
	for _, cycle := range t.Cycles {
		if cycle.Acted {
			count++
		}
	}
	return count
}

// Scalars returns the measured error at the start of each cycle.
func (t Trace) Scalars() []float64 {
	out := make([]float64, 0, len(t.Cycles))
	for _, cycle := range t.Cycles {
		out = append(out, cycle.Scalar)
	}
	return out
}

// Overshoot counts cycles where error rose rather than fell.
func (t Trace) Overshoot() int {
	count := 0
	scalars := t.Scalars()
	for i := 1; i < len(scalars); i++ {
		if scalars[i] > scalars[i-1] {
			count++
		}
	}
	return count
}

// Vector returns the error term measured at the start of a cycle.
func (t Trace) Vector(index int) condition.Vector { return t.Cycles[index].Vector }
