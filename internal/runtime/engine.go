// Package runtime is the sampled control loop.
//
// One cycle is: observe, type conditions, compute e(t), admit from typed
// conditions, let the brakes withdraw, select, act inside the arena, remeasure.
// Remeasurement happens within the cycle that acted, so the outcome triple is
// complete and attributable even if the controller stops immediately after.
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jamesonstone/loopc/internal/agent"
	"github.com/jamesonstone/loopc/internal/arena"
	"github.com/jamesonstone/loopc/internal/condition"
	"github.com/jamesonstone/loopc/internal/journal"
	"github.com/jamesonstone/loopc/internal/trend"
)

// Plant is the system under control. The runtime knows nothing about what it
// is; a controller supplies one and the domain stays out of this package.
type Plant interface {
	Observe(ctx context.Context) (agent.Observation, error)
}

// Engine runs the loop.
type Engine struct {
	config   Config
	plant    Plant
	agent    agent.Agent
	lane     arena.Arena
	lease    *arena.Lease
	ledger   *journal.Journal
	series   *trend.Series
	previous *condition.Set
	now      func() time.Time
	newID    func() string
}

// Options configures a new engine. Clock and IDs are injectable so the
// stability suite can drive the loop deterministically.
type Options struct {
	Config  Config
	Plant   Plant
	Agent   agent.Agent
	Arena   arena.Arena
	Journal *journal.Journal
	History []journal.Record
	Now     func() time.Time
	NewID   func() string
}

var (
	// ErrNoPlant is returned when no plant was supplied.
	ErrNoPlant = errors.New("engine requires a plant")
	// ErrNoJournal is returned when no ledger was supplied.
	ErrNoJournal = errors.New("engine requires a journal")
	// ErrNoAgent is returned when reconcile has no agent to act with.
	ErrNoAgent = errors.New("engine requires an agent")
)

// New validates authority and returns an engine.
//
// Reconcile is refused here rather than at the moment of action, so a
// misconfigured controller fails at startup instead of after observing.
func New(options Options) (*Engine, error) {
	if options.Plant == nil {
		return nil, ErrNoPlant
	}
	if options.Journal == nil {
		return nil, ErrNoJournal
	}
	if options.Agent == nil {
		return nil, ErrNoAgent
	}
	if err := authorise(options.Config, options.History); err != nil {
		return nil, err
	}
	if options.Config.Mode == ModeReconcile {
		if err := options.Arena.Validate(); err != nil {
			return nil, err
		}
	}
	engine := &Engine{
		config: options.Config,
		plant:  options.Plant,
		agent:  options.Agent,
		lane:   options.Arena,
		lease:  arena.NewLease(),
		ledger: options.Journal,
		series: trend.NewSeries(trend.DefaultLimit),
		now:    options.Now,
		newID:  options.NewID,
	}
	if engine.now == nil {
		engine.now = func() time.Time { return time.Now().UTC() }
	}
	if engine.newID == nil {
		engine.newID = randomID
	}
	// Rebuild e(t) from the ledger so a restart does not release the brakes.
	for _, sample := range journal.Samples(options.History) {
		engine.series.Append(sample)
	}
	return engine, nil
}

// Series exposes the recorded e(t) history.
func (e *Engine) Series() *trend.Series { return e.series }

// Run samples until the context is cancelled, polling faster while the plant
// is away from its setpoint.
func (e *Engine) Run(ctx context.Context, publish func(Cycle, error)) error {
	if publish == nil {
		publish = func(Cycle, error) {}
	}
	var delay time.Duration
	for {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		cycle, err := e.RunOnce(ctx)
		publish(cycle, err)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delay = e.nextDelay(cycle, err)
	}
}

// nextDelay backs off to the idle interval once the plant is at its setpoint
// and nothing is being attempted.
func (e *Engine) nextDelay(cycle Cycle, err error) time.Duration {
	active := e.config.ActivePoll
	if active <= 0 {
		active = time.Minute
	}
	idle := e.config.IdlePoll
	if idle <= 0 {
		idle = 15 * time.Minute
	}
	if err != nil || !cycle.Vector.Empty() || cycle.Acted {
		return active
	}
	return idle
}

// randomID returns an opaque identifier. Uniqueness matters for attributing
// outcomes; readability does not.
func randomID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buffer)
}
