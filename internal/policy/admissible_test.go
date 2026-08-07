package policy

import (
	"testing"

	"github.com/jamesonstone/loopc/internal/action"
)

// TestNonArenaClassesAreAlwaysAdmissible pins the constitutional guarantee that
// makes failing closed always possible: halting and escalating mutate nothing,
// so no brake can take them away.
func TestNonArenaClassesAreAlwaysAdmissible(t *testing.T) {
	admissible := NewAdmissible()
	admissible.WithdrawArena("everything withdrawn")

	for _, class := range action.NonArena() {
		if !admissible.Allows(class) {
			t.Fatalf("%s must remain admissible after every withdrawal", class)
		}
	}
	if admissible.AnyArena() {
		t.Fatal("no arena-mutating class should survive WithdrawArena")
	}
}

// TestWithdrawIsIgnoredForNonArenaClasses proves a brake cannot remove the
// escape hatch even by asking directly.
func TestWithdrawIsIgnoredForNonArenaClasses(t *testing.T) {
	admissible := NewAdmissible()
	admissible.Withdraw(action.RequestHuman, "attempted")
	admissible.Withdraw(action.Defer, "attempted")

	if !admissible.Allows(action.RequestHuman) || !admissible.Allows(action.Defer) {
		t.Fatal("non-arena classes must not be withdrawable")
	}
	if len(admissible.Withdrawals()) != 0 {
		t.Fatal("withdrawing a non-arena class must not record a withdrawal")
	}
}

// TestFreshAdmissibleGrantsNoAuthority pins the floor: a controller that has
// admitted nothing can still escalate, but can mutate nothing.
func TestFreshAdmissibleGrantsNoAuthority(t *testing.T) {
	admissible := NewAdmissible()
	for _, class := range action.ArenaMutating() {
		if admissible.Allows(class) {
			t.Fatalf("%s must not be admissible before typed conditions admit it", class)
		}
	}
	got := admissible.Classes()
	if len(got) != len(action.NonArena()) {
		t.Fatalf("classes = %v, want only the non-arena classes", got)
	}
}

// TestWithdrawalIsPermanentForTheCycle proves a later admission cannot
// resurrect a class a brake already took away. Without this, rule evaluation
// order could silently defeat a brake.
func TestWithdrawalIsPermanentForTheCycle(t *testing.T) {
	admissible := NewAdmissible()
	admissible.admit(action.PatchCode)
	if !admissible.Allows(action.PatchCode) {
		t.Fatal("expected patch_code to be admitted")
	}

	admissible.Withdraw(action.PatchCode, "derivative brake")
	admissible.admit(action.PatchCode)

	if admissible.Allows(action.PatchCode) {
		t.Fatal("a withdrawn class must not be re-admissible within the cycle")
	}
	withdrawals := admissible.Withdrawals()
	if len(withdrawals) != 1 || withdrawals[0].Reason != "derivative brake" {
		t.Fatalf("withdrawals = %+v, want the original reason preserved", withdrawals)
	}
}

func TestWithdrawalsAreDeterministic(t *testing.T) {
	for range 5 {
		admissible := NewAdmissible()
		admissible.WithdrawArena("stalled")
		got := admissible.Withdrawals()
		if len(got) != len(action.ArenaMutating()) {
			t.Fatalf("withdrawals = %d, want %d", len(got), len(action.ArenaMutating()))
		}
		for i := 1; i < len(got); i++ {
			if got[i-1].Class > got[i].Class {
				t.Fatal("withdrawals must be in deterministic class order")
			}
		}
	}
}

func TestAdmitIgnoresInvalidAndNonArenaClasses(t *testing.T) {
	admissible := NewAdmissible()
	admissible.admit(action.Class("not_a_class"))
	admissible.admit(action.RequestHuman)

	if admissible.AnyArena() {
		t.Fatal("admit must ignore unknown and non-arena classes")
	}
}
