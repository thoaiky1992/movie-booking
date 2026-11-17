package interfaces

import (
	"context"
	"time"
)

type SeatLocker interface {
	Acquire(
		ctx context.Context,
		showtimeID int64,
		seatIDs []int64,
		owner string,
		ttl time.Duration,
	) (bool, error)
	Release(ctx context.Context, showtimeID int64, seatIDs []int64, owner string) error
}

type ExpiredSeatLockReleaser interface {
	Release(ctx context.Context, showtimeID int64, seatIDs []int64, owner string) error
}
