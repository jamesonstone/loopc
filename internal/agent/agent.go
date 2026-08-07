// Package agent defines the adapter boundary between the deterministic
// controller and the language model that senses and acts.
//
// The split into three methods is the architecture in miniature. Classify is
// the sensor, turning raw observation into typed conditions. Propose is the
// actuator-planner, choosing from a set it is handed and cannot widen. Act
// executes inside the arena. The controller keeps error typing, the control
// law, the authority ceiling, and the arena to itself.
package agent

import (
	"context"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/arena"
	"github.com/jamesonstone/loopc/internal/condition"
)

// Observation is whatever the controller collected about its plant. Its shape
// is the domain's business; the runtime passes it through untouched.
type Observation struct {
	Generation string
	Subject    string
	Payload    any
}

// Proposal is what the agent offers before acting.
//
// It is not yet a declaration: the runtime validates the class against the
// admissible set and stamps identity and generation, so an agent cannot widen
// its own authority by proposing something that was never admitted.
type Proposal struct {
	Class            action.Class
	Target           string
	Intent           string
	PredictedCleared []condition.Key
}

// Result reports what executing a declaration actually did.
//
// Changed lists repository-relative paths the agent wrote, so the runtime can
// check them against the arena's guarded paths after the fact as well as
// before.
type Result struct {
	Terminal string
	Changed  []string
	Note     string
	Detail   map[string]any
}

// Sensor converts raw observation into typed conditions.
//
// Returned conditions are untrusted hypothesis. The runtime validates every
// one and stamps the generation itself; an agent cannot backdate a condition
// or bind it to a generation other than the one being observed.
type Sensor interface {
	Classify(ctx context.Context, observation Observation) ([]condition.Condition, error)
}

// Proposer chooses an action from the classes it is handed.
//
// It receives the typed error vector, which carries no human-readable message,
// and the admissible set, which it cannot widen.
type Proposer interface {
	Propose(ctx context.Context, vector condition.Vector, admissible []action.Class) (Proposal, error)
}

// Actor executes a committed declaration inside the arena.
type Actor interface {
	Act(ctx context.Context, declaration action.Declaration, lane arena.Arena) (Result, error)
}

// Agent is the full adapter. Implementations are substitutable, which matters
// because model choice is itself a policy the outer loop could later tune.
type Agent interface {
	Sensor
	Proposer
	Actor
}
