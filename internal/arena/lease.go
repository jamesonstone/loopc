package arena

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrHeld is returned when an action is already in flight.
var ErrHeld = errors.New("another action is already in flight")

// Lease serialises action execution globally.
//
// One action runs at a time and the next is selected only after remeasurement.
// Without this, two concurrent actions would make the outcome triple
// unattributable: the recorded error delta could not be assigned to either
// class, and the outer loop would learn from noise.
type Lease struct {
	mu       sync.Mutex
	holder   string
	acquired time.Time
}

// NewLease returns an unheld lease.
func NewLease() *Lease { return &Lease{} }

// Acquire takes the lease for a declaration, refusing if it is already held.
func (l *Lease) Acquire(declarationID string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.holder != "" {
		return fmt.Errorf("%w: held by %s since %s", ErrHeld, l.holder,
			l.acquired.UTC().Format(time.RFC3339))
	}
	l.holder = declarationID
	l.acquired = now
	return nil
}

// Release frees the lease. Releasing an unheld lease is a no-op so that
// recovery paths can always run without needing to know the prior state.
func (l *Lease) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.holder = ""
	l.acquired = time.Time{}
}

// Holder returns the declaration currently in flight, if any.
func (l *Lease) Holder() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.holder, l.holder != ""
}
