// Package trend records the scalar error series e(t).
//
// Everything here is observability and braking. The scalar may narrow the
// admissible action set but never widen it, so no function in this package
// returns an action, a class, or an authorisation — only measurements and the
// evidence a brake needs to withdraw something.
package trend

import (
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/condition"
)

// Sample is one cycle's measurement of the plant.
type Sample struct {
	CycleID    string        `json:"cycle_id"`
	At         time.Time     `json:"at"`
	Scalar     float64       `json:"scalar"`
	Total      int           `json:"total"`
	VectorHash string        `json:"vector_hash"`
	Class      action.Class  `json:"class,omitempty"`
	Blocking   bool          `json:"blocking"`
	OldestAge  time.Duration `json:"oldest_age"`
}

// Scalar reduces the typed error vector to a single number for trend, plots,
// and brakes.
//
// The reduction is a plain count of unsatisfied condition instances. It is
// deliberately not weighted or tuned: any weighting would be a readiness score
// in disguise, and the constitution forbids a number that admits an action.
// This value is only ever compared against its own history.
func Scalar(v condition.Vector) float64 { return float64(v.Total()) }

// Series is a bounded history of samples, newest last.
type Series struct {
	samples []Sample
	limit   int
}

// DefaultLimit bounds the retained history. Every buffer in the runtime is
// bounded by construction.
const DefaultLimit = 512

// NewSeries returns an empty series retaining at most limit samples.
func NewSeries(limit int) *Series {
	if limit <= 0 {
		limit = DefaultLimit
	}
	return &Series{limit: limit}
}

// Append records a sample, discarding the oldest once the limit is reached.
func (s *Series) Append(sample Sample) {
	s.samples = append(s.samples, sample)
	if len(s.samples) > s.limit {
		s.samples = s.samples[len(s.samples)-s.limit:]
	}
}

// Len returns the number of retained samples.
func (s *Series) Len() int { return len(s.samples) }

// Samples returns the retained history, oldest first.
func (s *Series) Samples() []Sample {
	out := make([]Sample, len(s.samples))
	copy(out, s.samples)
	return out
}

// Latest returns the most recent sample.
func (s *Series) Latest() (Sample, bool) {
	if len(s.samples) == 0 {
		return Sample{}, false
	}
	return s.samples[len(s.samples)-1], true
}

// Delta returns the change in scalar error across the two most recent samples.
// A negative delta means the plant moved toward its setpoint.
func (s *Series) Delta() (float64, bool) {
	if len(s.samples) < 2 {
		return 0, false
	}
	last := s.samples[len(s.samples)-1]
	previous := s.samples[len(s.samples)-2]
	return last.Scalar - previous.Scalar, true
}

// Converged reports whether the plant is at its setpoint.
func (s *Series) Converged() bool {
	latest, ok := s.Latest()
	return ok && latest.Scalar == 0
}
