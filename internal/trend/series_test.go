package trend

import (
	"testing"

	"github.com/jamesonstone/loopc/internal/action"
)

// TestNoReductionRunSkipsIdleCycles pins the rule that an idle cycle is not
// evidence that acting is failing. Counting idle cycles would trip the
// derivative brake on a plant that is simply quiet.
func TestNoReductionRunSkipsIdleCycles(t *testing.T) {
	series := NewSeries(16)
	series.Append(Sample{Scalar: 3, Class: action.PatchCode})
	series.Append(Sample{Scalar: 3})
	series.Append(Sample{Scalar: 3, Class: action.PatchCode})
	series.Append(Sample{Scalar: 3})

	if got := series.NoReductionRun(); got != 2 {
		t.Fatalf("run = %d, want 2 acting cycles counted and idle ones skipped", got)
	}
}

// TestNoReductionRunEndsOnImprovement proves progress resets the brake.
//
// A cycle's effect appears in the following sample, so the cycle at index 0
// reduced error (5 to 2) and ends the run. The two cycles after it each left
// the scalar at 2 and therefore count.
func TestNoReductionRunEndsOnImprovement(t *testing.T) {
	series := NewSeries(16)
	series.Append(Sample{Scalar: 5, Class: action.PatchCode})
	series.Append(Sample{Scalar: 2, Class: action.PatchCode})
	series.Append(Sample{Scalar: 2, Class: action.PatchCode})
	series.Append(Sample{Scalar: 2})

	if got := series.NoReductionRun(); got != 2 {
		t.Fatalf("run = %d, want 2: the reduction at index 0 ends the run", got)
	}
}

func TestStalledRequiresTheFullWindow(t *testing.T) {
	series := NewSeries(16)
	series.Append(Sample{Scalar: 4, Class: action.PatchCode})
	series.Append(Sample{Scalar: 4, Class: action.PatchCode})

	if series.Stalled(3) {
		t.Fatal("one unproductive cycle must not trip a window of three")
	}
	series.Append(Sample{Scalar: 4, Class: action.PatchCode})
	series.Append(Sample{Scalar: 4})
	if !series.Stalled(3) {
		t.Fatal("three unproductive acting cycles must trip a window of three")
	}
}

func TestStalledIgnoresNonPositiveWindow(t *testing.T) {
	series := NewSeries(4)
	series.Append(Sample{Scalar: 1, Class: action.PatchCode})
	series.Append(Sample{Scalar: 1, Class: action.PatchCode})
	if series.Stalled(0) || series.Stalled(-1) {
		t.Fatal("a non-positive window disables the brake")
	}
}

// TestRepeatDetectsLimitCycles covers the expected first failure of rung 1:
// the same class applied to the same plant shape, over and over.
func TestRepeatDetectsLimitCycles(t *testing.T) {
	series := NewSeries(16)
	for range 3 {
		series.Append(Sample{Scalar: 1, VectorHash: "shape-a", Class: action.PatchCode})
	}
	series.Append(Sample{Scalar: 1, VectorHash: "shape-b", Class: action.PatchCode})

	if got := series.Repeat("shape-a", action.PatchCode); got != 3 {
		t.Fatalf("repeat = %d, want 3", got)
	}
	if !series.Oscillating("shape-a", action.PatchCode, 3) {
		t.Fatal("three repeats must read as oscillation at threshold three")
	}
	if series.Oscillating("shape-b", action.PatchCode, 3) {
		t.Fatal("a different shape must not count toward the same limit cycle")
	}
	if series.Oscillating("shape-a", action.PatchCode, 0) {
		t.Fatal("a non-positive threshold disables detection")
	}
}

func TestSeriesIsBounded(t *testing.T) {
	series := NewSeries(3)
	for i := range 10 {
		series.Append(Sample{Scalar: float64(i)})
	}
	if series.Len() != 3 {
		t.Fatalf("len = %d, want the series bounded at 3", series.Len())
	}
	latest, ok := series.Latest()
	if !ok || latest.Scalar != 9 {
		t.Fatalf("latest = %+v, want the newest sample retained", latest)
	}
}

func TestDeltaAndConverged(t *testing.T) {
	series := NewSeries(8)
	if _, ok := series.Delta(); ok {
		t.Fatal("a single-sample series has no delta")
	}
	series.Append(Sample{Scalar: 4})
	series.Append(Sample{Scalar: 1})
	delta, ok := series.Delta()
	if !ok || delta != -3 {
		t.Fatalf("delta = %v (ok=%t), want -3", delta, ok)
	}
	if series.Converged() {
		t.Fatal("scalar 1 is not the setpoint")
	}
	series.Append(Sample{Scalar: 0})
	if !series.Converged() {
		t.Fatal("scalar 0 is the setpoint")
	}
}

func TestDefaultLimitAppliedForNonPositive(t *testing.T) {
	if NewSeries(0).limit != DefaultLimit || NewSeries(-5).limit != DefaultLimit {
		t.Fatal("a non-positive limit must fall back to the default bound")
	}
}
