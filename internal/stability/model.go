package stability

import (
	"context"
	"errors"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/agent"
	"github.com/jamesonstone/loopc/internal/arena"
	"github.com/jamesonstone/loopc/internal/condition"
)

// ConditionType is the single type this harness models.
const ConditionType condition.Type = "Thread"

// Model is a deterministic stand-in for the language model.
//
// It plays all three adapter roles, but only the roles: it senses from the
// observation it is handed rather than reading the plant directly, and it
// chooses only from the classes the control law admitted. An agent that
// reached around either boundary would make the suite prove nothing.
type Model struct {
	plant *Plant

	// Classified, Proposed, and Acted count the calls to each role.
	Classified int
	Proposed   int
	Acted      int
	// Handed records the admissible set from the most recent proposal.
	Handed []action.Class
}

// NewModel returns a model that acts on the given plant.
func NewModel(plant *Plant) *Model { return &Model{plant: plant} }

// ErrNoObservation is returned when the payload is not a subject list.
var ErrNoObservation = errors.New("observation payload is not a subject list")

// Classify converts the observation into typed conditions.
//
// It reads the payload rather than the plant so the sensor cannot see state the
// controller was never given.
func (m *Model) Classify(_ context.Context, observation agent.Observation) ([]condition.Condition, error) {
	m.Classified++
	subjects, ok := observation.Payload.([]string)
	if !ok {
		return nil, ErrNoObservation
	}
	conditions := make([]condition.Condition, 0, len(subjects))
	for _, subject := range subjects {
		conditions = append(conditions, condition.Condition{
			Type: ConditionType, Subject: subject, Status: condition.StatusTrue,
			Reason: "Actionable",
			// Prose the control law must never read.
			Message: "reviewer asked for a change on " + subject,
		})
	}
	return conditions, nil
}

// Propose selects the first admissible arena-mutating class and the
// lowest-ordered open subject.
//
// Both choices are deterministic so that a failing run reproduces exactly, and
// so that any oscillation observed is the loop's rather than the agent's.
func (m *Model) Propose(_ context.Context, vector condition.Vector, admissible []action.Class) (agent.Proposal, error) {
	m.Proposed++
	m.Handed = append([]action.Class(nil), admissible...)

	target := ""
	if entry, ok := vector.Find(ConditionType); ok && len(entry.Subjects) > 0 {
		target = entry.Subjects[0]
	}
	for _, class := range admissible {
		if !class.MutatesArena() {
			continue
		}
		return agent.Proposal{
			Class: class, Target: target,
			Intent:           "clear " + target,
			PredictedCleared: []condition.Key{{Type: ConditionType, Subject: target}},
		}, nil
	}
	return agent.Proposal{
		Class: action.RequestHuman, Intent: "no arena-mutating class was admitted",
	}, nil
}

// Act applies the declaration to the plant.
//
// The reported path is an ordinary source file inside the arena, so the
// runtime's post-action checks exercise their success path here rather than
// only their refusal path.
func (m *Model) Act(_ context.Context, declaration action.Declaration, _ arena.Arena) (agent.Result, error) {
	m.Acted++
	changed := m.plant.Resolve(declaration.Target)
	result := agent.Result{Changed: []string{"internal/subject/" + declaration.Target + ".go"}}
	if !changed {
		result.Note = "plant did not change"
	}
	return result, nil
}
