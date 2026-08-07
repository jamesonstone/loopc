package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamesonstone/loopc/internal/action"
	"github.com/jamesonstone/loopc/internal/agent"
	"github.com/jamesonstone/loopc/internal/arena"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/policy"
)

// fakePlant is a deterministic stand-in for a real system under control.
type fakePlant struct {
	generation string
	err        error
}

func (p *fakePlant) Observe(context.Context) (agent.Observation, error) {
	if p.err != nil {
		return agent.Observation{}, p.err
	}
	generation := p.generation
	if generation == "" {
		generation = "gen-1"
	}
	return agent.Observation{Generation: generation}, nil
}

// fakeAgent lets each test script the sensor, the proposer, and the actor
// independently, so a failure is attributable to one role.
type fakeAgent struct {
	conditions  []condition.Condition
	classifyErr error
	proposal    agent.Proposal
	proposeErr  error
	result      agent.Result
	actErr      error

	classified int
	proposed   int
	acted      int
	handed     []action.Class
}

func (a *fakeAgent) Classify(context.Context, agent.Observation) ([]condition.Condition, error) {
	a.classified++
	return a.conditions, a.classifyErr
}

func (a *fakeAgent) Propose(_ context.Context, _ condition.Vector, admissible []action.Class) (agent.Proposal, error) {
	a.proposed++
	a.handed = append([]action.Class(nil), admissible...)
	return a.proposal, a.proposeErr
}

func (a *fakeAgent) Act(context.Context, action.Declaration, arena.Arena) (agent.Result, error) {
	a.acted++
	return a.result, a.actErr
}

func threadCondition(subject string, status condition.Status) condition.Condition {
	return condition.Condition{
		Type: "Thread", Subject: subject, Status: status,
		Reason: "Actionable", Message: "human prose that the law must never read",
	}
}

func testLaw() policy.Law {
	return policy.Law{Rules: []policy.Rule{
		{Type: "Thread", Admits: []action.Class{action.PatchCode}},
	}}
}

// newEngine builds an engine with a deterministic clock and identifiers.
func newEngine(t *testing.T, mode Mode, plant Plant, model agent.Agent, history []journal.Record) (*Engine, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	ledger, err := journal.Open(path)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	t.Cleanup(func() { ledger.Close() })

	clock := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	counter := 0

	engine, err := New(Options{
		Config: Config{
			Mode: mode, Law: testLaw(),
			Brakes: policy.Brakes{DerivativeWindow: 0, OscillationThreshold: 0},
		},
		Plant: plant, Agent: model, Journal: ledger, History: history,
		Arena: arena.Arena{
			Root: "/tmp/loopc-lane", Owner: "o", Repository: "r",
			Lane: "GH-1", Branch: "GH-1", DefaultBranch: "main",
		},
		Now: func() time.Time { clock = clock.Add(time.Second); return clock },
		NewID: func() string {
			counter++
			return fmt.Sprintf("id-%d", counter)
		},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine, path
}

// baselineFor returns history containing an audit baseline for the same policy
// the test engine will run under.
func baselineFor(t *testing.T, mode Mode) []journal.Record {
	t.Helper()
	config := Config{
		Mode: mode, Law: testLaw(),
		Brakes: policy.Brakes{DerivativeWindow: 0, OscillationThreshold: 0},
	}
	return []journal.Record{{
		Kind: journal.KindAuditBaseline, PolicyHash: config.PolicyHash(),
	}}
}

func readRecords(t *testing.T, path string) []journal.Record {
	t.Helper()
	records, err := journal.Read(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	return records
}

func kinds(records []journal.Record) []journal.Kind {
	out := make([]journal.Kind, 0, len(records))
	for _, record := range records {
		out = append(out, record.Kind)
	}
	return out
}

func hasKind(records []journal.Record, want journal.Kind) bool {
	for _, record := range records {
		if record.Kind == want {
			return true
		}
	}
	return false
}
