package policy

import (
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/trend"
)

// TestBrakesNeverWidenTheAdmissibleSet is the load-bearing test of the
// constitution's asymmetry. Whatever the scalar history says, applying the
// brakes may only ever produce a subset of what typed conditions admitted.
func TestBrakesNeverWidenTheAdmissibleSet(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread", Admits: []action.Class{action.PatchCode}}}}
	histories := map[string]*trend.Series{
		"empty":   trend.NewSeries(16),
		"stalled": stalledSeries(),
		"looping": loopingSeries(t),
	}
	brakes := Brakes{IntegralBudget: time.Hour, DerivativeWindow: 2, OscillationThreshold: 2}

	for name, series := range histories {
		t.Run(name, func(t *testing.T) {
			vector := threadVector(t, 1, 10*time.Minute)
			before := law.Admit(vector).Classes()
			admissible := law.Admit(vector)
			brakes.Apply(admissible, vector, series)
			after := admissible.Classes()

			if len(after) > len(before) {
				t.Fatalf("brakes widened the set: before %v, after %v", before, after)
			}
			for _, class := range after {
				if !contains(before, class) {
					t.Fatalf("brakes admitted %s, which typed conditions had not", class)
				}
			}
		})
	}
}

// TestIntegralBrakeEscalatesRatherThanRepeating pins the constitutional
// requirement that a persistent condition terminates in request_human.
func TestIntegralBrakeEscalatesRatherThanRepeating(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread", Admits: []action.Class{action.PatchCode}}}}
	brakes := Brakes{IntegralBudget: time.Hour}
	vector := threadVector(t, 1, 3*time.Hour)

	admissible := law.Admit(vector)
	brakes.Apply(admissible, vector, trend.NewSeries(16))

	if admissible.AnyArena() {
		t.Fatal("a condition past its budget must withdraw the arena-mutating classes")
	}
	selected, ok := law.Select(vector, admissible)
	if !ok || selected != action.RequestHuman {
		t.Fatalf("selected = %s (ok=%t), want request_human", selected, ok)
	}
}

// TestDerivativeBrakeStopsUnproductiveActing is the guard against burning
// effort forever, expressed as a control-theoretic stop rather than a budget.
func TestDerivativeBrakeStopsUnproductiveActing(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread", Admits: []action.Class{action.PatchCode}}}}
	brakes := Brakes{DerivativeWindow: 2}
	vector := threadVector(t, 3, time.Minute)

	admissible := law.Admit(vector)
	withdrawals := brakes.Apply(admissible, vector, stalledSeries())

	if admissible.AnyArena() {
		t.Fatal("a stalled loop must stop acting")
	}
	if len(withdrawals) == 0 {
		t.Fatal("the stop must be explained")
	}
	if !admissible.Allows(action.RequestHuman) {
		t.Fatal("escalation must survive the derivative brake")
	}
}

// TestOscillationBrakeIsSurgical proves a limit cycle withdraws only the
// looping class. One class being wrong for a plant shape is not evidence that
// every class is.
func TestOscillationBrakeIsSurgical(t *testing.T) {
	law := Law{Rules: []Rule{{Type: "Thread",
		Admits: []action.Class{action.PatchCode, action.PatchDocs}}}}
	brakes := Brakes{OscillationThreshold: 2}
	vector := threadVector(t, 1, time.Minute)

	series := trend.NewSeries(16)
	for range 2 {
		series.Append(trend.Sample{Scalar: 1, VectorHash: vector.Hash(), Class: action.PatchCode})
	}

	admissible := law.Admit(vector)
	brakes.Apply(admissible, vector, series)

	if admissible.Allows(action.PatchCode) {
		t.Fatal("the looping class must be withdrawn")
	}
	if !admissible.Allows(action.PatchDocs) {
		t.Fatal("a different class must remain available after a limit cycle")
	}
}

func TestBrakesTolerateNilInputs(t *testing.T) {
	brakes := DefaultBrakes()
	if got := brakes.Apply(nil, condition.Vector{}, nil); got != nil {
		t.Fatal("a nil admissible set must be tolerated")
	}
	admissible := NewAdmissible()
	brakes.Apply(admissible, condition.Vector{}, nil)
}

func threadVector(t *testing.T, count int, age time.Duration) condition.Vector {
	t.Helper()
	now := time.Now().UTC()
	set := condition.NewSet("gen-1")
	for i := range count {
		err := set.Upsert(condition.Condition{
			Type: "Thread", Subject: string(rune('a' + i)), Status: condition.StatusTrue,
			Reason: "Actionable", FirstObservedAt: now.Add(-age), ObservedGeneration: "gen-1",
		})
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	return condition.Derive(set, now)
}

func stalledSeries() *trend.Series {
	series := trend.NewSeries(16)
	for range 4 {
		series.Append(trend.Sample{Scalar: 3, VectorHash: "shape", Class: action.PatchCode})
	}
	return series
}

func loopingSeries(t *testing.T) *trend.Series {
	t.Helper()
	series := trend.NewSeries(16)
	hash := threadVector(t, 1, 10*time.Minute).Hash()
	for range 3 {
		series.Append(trend.Sample{Scalar: 1, VectorHash: hash, Class: action.PatchCode})
	}
	return series
}

func contains(classes []action.Class, want action.Class) bool {
	for _, class := range classes {
		if class == want {
			return true
		}
	}
	return false
}
