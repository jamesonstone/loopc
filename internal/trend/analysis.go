package trend

import "github.com/jamesonstone/loopc/internal/action"

// A sample records the measurement taken at the start of a cycle together with
// the class selected during it, so the effect of an action is visible in the
// following sample rather than its own.

// NoReductionRun counts the most recent acting cycles that failed to reduce
// error.
//
// Cycles that took no action are skipped rather than counted. An idle cycle is
// not evidence that acting is not working, so it neither extends the run nor
// resets it. A cycle whose successor shows lower error ends the run.
func (s *Series) NoReductionRun() int {
	run := 0
	for i := len(s.samples) - 2; i >= 0; i-- {
		current := s.samples[i]
		if current.Class == "" {
			continue
		}
		if s.samples[i+1].Scalar < current.Scalar {
			break
		}
		run++
	}
	return run
}

// Stalled reports whether the last window acting cycles produced no error
// reduction. It is the evidence the derivative brake reads.
//
// Reporting true does not select an action. The brake may only withdraw the
// arena-mutating classes; halting and escalating stay available because they
// mutate nothing.
func (s *Series) Stalled(window int) bool {
	if window <= 0 {
		return false
	}
	return s.NoReductionRun() >= window
}

// Repeat counts how often a plant shape and action class occurred together in
// the retained history.
//
// A pair recurring without error falling is a limit cycle: the controller keeps
// applying the same class to the same shape and keeps arriving back where it
// started. This is the expected first failure of rung 1, where an agent
// resolves one item and creates the condition for another.
func (s *Series) Repeat(vectorHash string, class action.Class) int {
	count := 0
	for _, sample := range s.samples {
		if sample.VectorHash == vectorHash && sample.Class == class {
			count++
		}
	}
	return count
}

// Oscillating reports whether a plant shape and action class have recurred at
// least threshold times.
func (s *Series) Oscillating(vectorHash string, class action.Class, threshold int) bool {
	if threshold <= 0 {
		return false
	}
	return s.Repeat(vectorHash, class) >= threshold
}
