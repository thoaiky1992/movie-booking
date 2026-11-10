// Package resilience contains small, dependency-free fault-tolerance primitives.
package resilience

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned without calling the protected function while the circuit is open.
var ErrOpen = errors.New("circuit open")

type state int

const (
	closed state = iota
	open
	halfOpen
)

// Breaker is a consecutive-failure circuit breaker.
//
//	closed    → calls flow; `threshold` consecutive failures open the circuit
//	open      → calls fail fast for `cooldown`
//	half-open → one probe call; success closes, failure re-opens
type Breaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	now       func() time.Time

	state    state
	failures int
	openedAt time.Time
	probing  bool
}

func NewBreaker(threshold int, cooldown time.Duration) *Breaker {
	return &Breaker{threshold: threshold, cooldown: cooldown, now: time.Now}
}

// Execute runs fn if the circuit allows it and records the outcome.
func (b *Breaker) Execute(fn func() error) error {
	if err := b.allow(); err != nil {
		return err
	}
	err := fn()
	b.record(err)
	return err
}

func (b *Breaker) allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case open:
		if b.now().Sub(b.openedAt) < b.cooldown {
			return ErrOpen
		}
		b.state = halfOpen
		b.probing = true
		return nil
	case halfOpen:
		if b.probing {
			return ErrOpen
		}
		b.probing = true
	}
	return nil
}

func (b *Breaker) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.probing = false
	if err == nil {
		b.failures = 0
		b.state = closed
		return
	}
	b.failures++
	if b.state == halfOpen || b.failures >= b.threshold {
		b.state = open
		b.openedAt = b.now()
	}
}
