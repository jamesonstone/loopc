package stability

import (
	"context"
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/policy"
)

// TestOscillationIsDetectedAndDamped is the test the whole harness exists for.
//
// The plant is configured so that clearing "a" opens "b" and clearing "b"
// opens "a": the agent-versus-reviewer limit cycle named as the expected first
// failure of rung 1. Error never falls, the same class keeps being applied to
// a recurring plant shape, and an undamped controller would run forever.
//
// The controller must notice and stop. It is asserted twice: once that an
// unbraked loop really does cycle, so the test cannot pass vacuously, and once
// that the braked loop escalates instead.
func TestOscillationIsDetectedAndDamped(t *testing.T) {
	t.Run("undamped loop really does cycle", func(t *testing.T) {
		plant := NewPlant("gen-1")
		plant.Seed("a")
		plant.SpawnOnResolve("a", "b")
		plant.SpawnOnResolve("b", "a")

		harness := build(t, plant, NoBrakes())
		trace, err := harness.Run(context.Background(), 12)
		if err != nil {
			t.Fatalf("run: %v", err)
		}

		if plant.Converged() {
			t.Fatal("the plant must not converge; the harness is not modelling a limit cycle")
		}
		if trace.Escalated {
			t.Fatal("an unbraked loop must not escalate; the damping proof would be vacuous")
		}
		if trace.Len() != 12 {
			t.Fatalf("cycles = %d, want the loop to run the full budget", trace.Len())
		}
		for _, scalar := range trace.Scalars() {
			if scalar != 1 {
				t.Fatalf("e(t) = %v, want error pinned at 1 throughout the cycle",
					trace.Scalars())
			}
		}
	})

	t.Run("damped loop escalates", func(t *testing.T) {
		plant := NewPlant("gen-1")
		plant.Seed("a")
		plant.SpawnOnResolve("a", "b")
		plant.SpawnOnResolve("b", "a")

		harness := build(t, plant, policy.Brakes{OscillationThreshold: 2})
		trace, err := harness.Run(context.Background(), 12)
		if err != nil {
			t.Fatalf("run: %v", err)
		}

		if !trace.Escalated {
			t.Fatalf("the loop ran %d cycles without escalating; the limit cycle was not damped",
				trace.Len())
		}
		if trace.Len() >= 12 {
			t.Fatal("escalation must happen well inside the budget")
		}
		last := trace.Cycles[trace.Len()-1]
		if last.Selected != action.RequestHuman {
			t.Fatalf("selected = %s, want request_human", last.Selected)
		}
		if len(last.Withdrawals) == 0 {
			t.Fatal("the damping must be explained by a recorded withdrawal")
		}
	})
}

// TestSteadyStateErrorEscalates proves the controller stops rather than spins
// on feedback nothing will satisfy.
//
// A residual error the loop cannot remove is steady-state error. The correct
// response is to hand it to a human, which requires no authority and is
// therefore always available.
func TestSteadyStateErrorEscalates(t *testing.T) {
	plant := NewPlant("gen-1")
	plant.Seed("a", "b")
	plant.MarkUnfixable("b")

	harness := build(t, plant, policy.Brakes{DerivativeWindow: 2})
	trace, err := harness.Run(context.Background(), 20)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !trace.Escalated {
		t.Fatalf("ran %d cycles without escalating on unfixable feedback", trace.Len())
	}
	if plant.Count() != 1 {
		t.Fatalf("open = %d, want the fixable condition cleared and the unfixable one left",
			plant.Count())
	}
	if trace.Len() >= 20 {
		t.Fatal("the controller spun instead of stopping")
	}
}

// TestDerivativeStopHaltsUnproductiveActing proves the loop stops acting once
// acting demonstrably is not helping, and that it halts rather than idling.
//
// This is the control-theoretic answer to an agent burning effort forever: not
// a budget, but a measurement that progress has ceased.
func TestDerivativeStopHaltsUnproductiveActing(t *testing.T) {
	plant := NewPlant("gen-1")
	plant.Seed("a")
	plant.MarkUnfixable("a")

	harness := build(t, plant, policy.Brakes{DerivativeWindow: 3})
	trace, err := harness.Run(context.Background(), 20)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !trace.Escalated {
		t.Fatal("the derivative brake did not halt an unproductive loop")
	}
	if acted := trace.Acted(); acted > 4 {
		t.Fatalf("acted %d times, want the brake to stop it within about the window of 3", acted)
	}
	if attempts := plant.Attempts; attempts > 4 {
		t.Fatalf("plant saw %d attempts, want acting to stop once it was not helping", attempts)
	}

	last := trace.Cycles[trace.Len()-1]
	if last.Selected != action.RequestHuman {
		t.Fatalf("selected = %s, want request_human", last.Selected)
	}
}

// TestIntegralBrakeEscalatesOnPersistence proves a condition that simply will
// not go away eventually escalates on age rather than on repetition count.
func TestIntegralBrakeEscalatesOnPersistence(t *testing.T) {
	plant := NewPlant("gen-1")
	plant.Seed("a")
	plant.MarkUnfixable("a")

	// The harness clock advances a minute per call, so a short budget is
	// reached deterministically rather than by waiting.
	harness := build(t, plant, policy.Brakes{IntegralBudget: 5 * time.Minute})
	trace, err := harness.Run(context.Background(), 20)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !trace.Escalated {
		t.Fatal("a condition past its budget must escalate")
	}
	last := trace.Cycles[trace.Len()-1]
	if last.Selected != action.RequestHuman {
		t.Fatalf("selected = %s, want request_human", last.Selected)
	}
}

// TestNonArenaClassesSurviveEveryBrake proves the escape hatch holds end to end:
// whatever the brakes withdraw, the loop can still escalate.
func TestNonArenaClassesSurviveEveryBrake(t *testing.T) {
	plant := NewPlant("gen-1")
	plant.Seed("a")
	plant.MarkUnfixable("a")

	harness := build(t, plant, policy.Brakes{
		IntegralBudget: time.Minute, DerivativeWindow: 1, OscillationThreshold: 1,
	})
	trace, err := harness.Run(context.Background(), 10)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !trace.Escalated {
		t.Fatal("with every brake armed the loop must still be able to escalate")
	}
	last := trace.Cycles[trace.Len()-1]
	for _, class := range last.Admissible {
		if class.MutatesArena() {
			t.Fatalf("%s survived every brake", class)
		}
	}
	if last.Selected != action.RequestHuman {
		t.Fatalf("selected = %s, want request_human", last.Selected)
	}
}
