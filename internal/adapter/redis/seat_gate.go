package redisadapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"movie_booking/internal/domain"
	"movie_booking/internal/platform/resilience"
	"movie_booking/internal/usecase"
)

// Key layout: seat:hold:{<showtimeID>}:<seatID> = <bookingID>
//
// The {showtimeID} hash tag pins every key of one showtime to one Redis Cluster
// slot, so a multi-seat Lua script never spans slots.

// acquireScript is all-or-nothing. It checks every seat first, then writes
// them all. Redis runs scripts atomically, so no other client can interleave.
//
//	KEYS: seat keys        ARGV[1]: bookingID   ARGV[2]: ttl in ms
//	returns 0 on success, or the 1-based index of the first seat already locked.
var acquireScript = redis.NewScript(`
for i, key in ipairs(KEYS) do
  if redis.call('EXISTS', key) == 1 then
    return i
  end
end
for _, key in ipairs(KEYS) do
  redis.call('SET', key, ARGV[1], 'PX', ARGV[2])
end
return 0
`)

// releaseScript deletes only keys still owned by the caller. This is
// compare-and-delete, so a late release can never free a seat another customer
// has since locked.
//
//	KEYS: seat keys   ARGV[1]: bookingID   returns: number of keys deleted
var releaseScript = redis.NewScript(`
local released = 0
for _, key in ipairs(KEYS) do
  if redis.call('GET', key) == ARGV[1] then
    redis.call('DEL', key)
    released = released + 1
  end
end
return released
`)

// confirmScript replaces the hold TTL with an absolute expiry, for paid seats.
//
//	KEYS: seat keys   ARGV[1]: bookingID   ARGV[2]: unix ms expiry
var confirmScript = redis.NewScript(`
local n = 0
for _, key in ipairs(KEYS) do
  if redis.call('GET', key) == ARGV[1] then
    redis.call('PEXPIREAT', key, ARGV[2])
    n = n + 1
  end
end
return n
`)

type SeatGate struct{ client *redis.Client }

func NewSeatGate(c *redis.Client) *SeatGate { return &SeatGate{client: c} }

var _ usecase.SeatGate = (*SeatGate)(nil)

func seatKeys(showtimeID uuid.UUID, seatIDs []uuid.UUID) []string {
	keys := make([]string, len(seatIDs))
	for i, s := range seatIDs {
		keys[i] = fmt.Sprintf("seat:hold:{%s}:%s", showtimeID, s)
	}
	return keys
}

func (g *SeatGate) Acquire(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID, ttl time.Duration) error {
	res, err := acquireScript.Run(ctx, g.client, seatKeys(showtimeID, seatIDs),
		bookingID.String(), ttl.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("acquire seat locks: %w", err)
	}
	if res != 0 {
		return domain.ErrSeatUnavailable
	}
	return nil
}

func (g *SeatGate) Release(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID) error {
	if len(seatIDs) == 0 {
		return nil
	}
	if err := releaseScript.Run(ctx, g.client, seatKeys(showtimeID, seatIDs), bookingID.String()).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("release seat locks: %w", err)
	}
	return nil
}

func (g *SeatGate) Confirm(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID, until time.Time) error {
	if len(seatIDs) == 0 {
		return nil
	}
	err := confirmScript.Run(ctx, g.client, seatKeys(showtimeID, seatIDs),
		bookingID.String(), until.UnixMilli()).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("confirm seat locks: %w", err)
	}
	return nil
}

// BreakerGate wraps the seat gate in a circuit breaker. When Redis is failing,
// holds skip the gate and go straight to the database guard, instead of paying
// a dial timeout on every request. Seat conflicts are healthy answers and never
// count as failures.
type BreakerGate struct {
	inner   usecase.SeatGate
	breaker *resilience.Breaker
}

func NewBreakerGate(inner usecase.SeatGate, threshold int, cooldown time.Duration) *BreakerGate {
	return &BreakerGate{inner: inner, breaker: resilience.NewBreaker(threshold, cooldown)}
}

var _ usecase.SeatGate = (*BreakerGate)(nil)

func (g *BreakerGate) Acquire(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID, ttl time.Duration) error {
	var conflict error
	err := g.breaker.Execute(func() error {
		e := g.inner.Acquire(ctx, showtimeID, seatIDs, bookingID, ttl)
		if errors.Is(e, domain.ErrSeatUnavailable) {
			conflict = e
			return nil
		}
		return e
	})
	if conflict != nil {
		return conflict
	}
	return err // resilience.ErrOpen or a Redis error: caller degrades to the DB guard
}

// Release and Confirm are best effort. If Redis is unavailable they are skipped:
// the database is authoritative and Redis keys expire on their own TTL.
func (g *BreakerGate) Release(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID) error {
	err := g.breaker.Execute(func() error { return g.inner.Release(ctx, showtimeID, seatIDs, bookingID) })
	if errors.Is(err, resilience.ErrOpen) {
		return nil
	}
	return err
}

func (g *BreakerGate) Confirm(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID, until time.Time) error {
	err := g.breaker.Execute(func() error { return g.inner.Confirm(ctx, showtimeID, seatIDs, bookingID, until) })
	if errors.Is(err, resilience.ErrOpen) {
		return nil
	}
	return err
}
