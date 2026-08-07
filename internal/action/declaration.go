package action

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jamesonstone/loopc/internal/condition"
)

// Declaration is the durable record an agent commits before acting.
//
// It exists so that every action is attributable to a typed class and a stated
// expectation. PredictedCleared costs almost nothing to record and yields a
// calibration signal: a sensor that is confidently wrong is a sharper warning
// than a rising error.
type Declaration struct {
	ID               string          `json:"id"`
	Class            Class           `json:"class"`
	Target           string          `json:"target"`
	Intent           string          `json:"intent"`
	PredictedCleared []condition.Key `json:"predicted_cleared"`
	Generation       string          `json:"generation"`
	VectorHash       string          `json:"vector_hash"`
	DeclaredAt       time.Time       `json:"declared_at"`
}

var (
	// ErrMissingID is returned when a declaration carries no identifier.
	ErrMissingID = errors.New("declaration requires an identifier")
	// ErrMissingIntent is returned when a declaration states no intent.
	ErrMissingIntent = errors.New("declaration requires a stated intent")
	// ErrMissingGeneration is returned when a declaration is unbound.
	ErrMissingGeneration = errors.New("declaration requires a generation")
	// ErrMissingTarget is returned when an arena-mutating class names no target.
	ErrMissingTarget = errors.New("arena-mutating declaration requires a target")
)

// Validate reports whether the declaration may be committed.
//
// An arena-mutating class must name its target, because the arena checks the
// target before permitting the write. The non-arena classes may omit it: there
// is nothing to bound when nothing is written.
func (d Declaration) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return ErrMissingID
	}
	if !d.Class.Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownClass, d.Class)
	}
	if strings.TrimSpace(d.Intent) == "" {
		return ErrMissingIntent
	}
	if strings.TrimSpace(d.Generation) == "" {
		return ErrMissingGeneration
	}
	if d.Class.MutatesArena() && strings.TrimSpace(d.Target) == "" {
		return fmt.Errorf("%w: class %s", ErrMissingTarget, d.Class)
	}
	return nil
}

// Outcome is the triple the outer loop will learn from.
//
// It is recorded from the first cycle even though nothing learns from it yet:
// the schema cannot be retrofitted onto cycles already run, whereas a learner
// can be added at any time.
type Outcome struct {
	DeclarationID   string    `json:"declaration_id"`
	Class           Class     `json:"class"`
	VectorBefore    string    `json:"vector_before"`
	VectorAfter     string    `json:"vector_after"`
	ErrorBefore     float64   `json:"error_before"`
	ErrorAfter      float64   `json:"error_after"`
	ErrorDelta      float64   `json:"error_delta"`
	CyclesToClear   int       `json:"cycles_to_clear"`
	PredictionHit   bool      `json:"prediction_hit"`
	PredictedCount  int       `json:"predicted_count"`
	ActuallyCleared int       `json:"actually_cleared"`
	RecordedAt      time.Time `json:"recorded_at"`
}

// Reduced reports whether the action moved the plant toward its setpoint. It
// is the per-action input to the derivative brake.
func (o Outcome) Reduced() bool { return o.ErrorDelta < 0 }

// Score compares what the agent predicted against what actually cleared.
//
// A prediction counts as hit only when every predicted condition cleared.
// Partial credit would let a sensor stay confident while being consistently
// wrong, which is exactly the failure this signal exists to surface.
func Score(d Declaration, before, after *condition.Set) (predicted, cleared int, hit bool) {
	predicted = len(d.PredictedCleared)
	for _, key := range d.PredictedCleared {
		wasUnsatisfied := false
		if before != nil {
			if c, ok := before.Get(key); ok {
				wasUnsatisfied = c.Unsatisfied()
			}
		}
		nowSatisfied := true
		if after != nil {
			if c, ok := after.Get(key); ok {
				nowSatisfied = !c.Unsatisfied()
			}
		}
		if wasUnsatisfied && nowSatisfied {
			cleared++
		}
	}
	return predicted, cleared, predicted > 0 && cleared == predicted
}
