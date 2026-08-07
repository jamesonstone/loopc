package stability

import (
	"context"
	"testing"

	"github.com/jamesonstone/loopc/internal/policy"
)

// Each test in this package fails on an *unstable* loop rather than an
// incorrect one. Correctness is covered by the runtime's own unit tests; what
// is being proven here is that the controller settles, does not oscillate, does
// not spin, absorbs disturbance, and stops when it is not helping.

// TestSettling proves the loop drives a seeded plant to its setpoint within a
// bounded number of cycles, without overshoot.
//
// Settling time is asserted rather than observed: a controller that eventually
// converges after an unbounded number of cycles has not demonstrated control.
func TestSettling(t *testing.T) {
	plant := NewPlant("gen-1")
	plant.Seed("a", "b", "c", "d", "e")

	harness := build(t, plant, NoBrakes())
	trace, err := harness.Run(context.Background(), 20)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !plant.Converged() {
		t.Fatalf("plant did not converge: %d conditions still open after %d cycles",
			plant.Count(), trace.Len())
	}
	if trace.Len() != 5 {
		t.Fatalf("settling time = %d cycles, want exactly 5 for 5 conditions at one action per cycle",
			trace.Len())
	}
	if trace.Overshoot() != 0 {
		t.Fatalf("overshoot = %d, want 0: error must fall monotonically here", trace.Overshoot())
	}

	want := []float64{5, 4, 3, 2, 1}
	for i, got := range trace.Scalars() {
		if got != want[i] {
			t.Fatalf("e(t) = %v, want %v", trace.Scalars(), want)
		}
	}
}

// TestWindup proves a rate-limited controller still converges when the error
// greatly exceeds what one cycle can clear.
//
// One action per cycle against twenty conditions is exactly the actuator
// saturation that causes integral windup in a classical controller. The loop
// must grind through it rather than thrash.
func TestWindup(t *testing.T) {
	plant := NewPlant("gen-1")
	for i := range 20 {
		plant.Seed(subject(i))
	}

	harness := build(t, plant, NoBrakes())
	trace, err := harness.Run(context.Background(), 40)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !plant.Converged() {
		t.Fatalf("saturated controller did not converge: %d still open", plant.Count())
	}
	if trace.Len() != 20 {
		t.Fatalf("settling time = %d cycles, want exactly 20", trace.Len())
	}
	if trace.Overshoot() != 0 {
		t.Fatalf("overshoot = %d, want 0 under saturation", trace.Overshoot())
	}
	if acted := trace.Acted(); acted != 20 {
		t.Fatalf("acted = %d, want one action per cycle", acted)
	}
}

// TestDisturbanceRejection proves a condition injected mid-convergence is
// absorbed, and measures how long recovery takes.
//
// Without this, a controller could look stable purely because nothing ever
// happened to it.
func TestDisturbanceRejection(t *testing.T) {
	plant := NewPlant("gen-1")
	plant.Seed("a", "b", "c")

	harness := build(t, plant, NoBrakes())
	ctx := context.Background()

	// Converge partway, then disturb.
	before, err := harness.Run(ctx, 2)
	if err != nil {
		t.Fatalf("initial run: %v", err)
	}
	if plant.Count() != 1 {
		t.Fatalf("expected 1 condition remaining before the disturbance, got %d", plant.Count())
	}

	plant.Inject("x", "y")
	if plant.Count() != 3 {
		t.Fatalf("expected the disturbance to raise error to 3, got %d", plant.Count())
	}

	after, err := harness.Run(ctx, 20)
	if err != nil {
		t.Fatalf("recovery run: %v", err)
	}

	if !plant.Converged() {
		t.Fatalf("controller did not reject the disturbance: %d still open", plant.Count())
	}
	if after.Len() != 3 {
		t.Fatalf("recovery took %d cycles, want 3", after.Len())
	}
	if before.Overshoot() != 0 || after.Overshoot() != 0 {
		t.Fatal("error must fall monotonically on each side of the disturbance")
	}
}

func subject(i int) string { return string(rune('a'+i/26)) + string(rune('a'+i%26)) }

// build assembles a harness rooted in the test's temporary directory.
func build(t *testing.T, plant *Plant, brakes policy.Brakes) *Harness {
	t.Helper()
	harness, err := Build(t.TempDir(), plant, brakes)
	if err != nil {
		t.Fatalf("build harness: %v", err)
	}
	t.Cleanup(func() { harness.Close() })
	return harness
}
