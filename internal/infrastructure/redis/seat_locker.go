package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redisclient "github.com/redis/go-redis/v9"
)

// Every key for one showtime uses the same Redis Cluster hash tag. This keeps
// the multi-key Lua script valid when Redis is later deployed as a cluster.
// PSETEX: P (Milliseconds) + SET + EX (Expire)
const acquireSeatsLua = `
for i, key in ipairs(KEYS) do
  local current = redis.call('GET', key)
  if current and current ~= ARGV[1] then
    return {0, key}
  end
end
for i, key in ipairs(KEYS) do
  redis.call('PSETEX', key, ARGV[2], ARGV[1])
end
return {1, ''}
`

const releaseSeatsLua = `
local released = 0
for i, key in ipairs(KEYS) do
  if redis.call('GET', key) == ARGV[1] then
    released = released + redis.call('DEL', key)
  end
end
return released
`

type SeatLocker struct {
	client  *redisclient.Client
	acquire *redisclient.Script
	release *redisclient.Script
}

func NewSeatLocker(client *redisclient.Client) *SeatLocker {
	return &SeatLocker{
		client:  client,
		acquire: redisclient.NewScript(acquireSeatsLua),
		release: redisclient.NewScript(releaseSeatsLua),
	}
}

func NewClient(redisURL string) (*redisclient.Client, error) {
	options, err := redisclient.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	return redisclient.NewClient(options), nil
}

func (l *SeatLocker) Acquire(
	ctx context.Context,
	showtimeID int64,
	seatIDs []int64,
	owner string,
	ttl time.Duration,
) (bool, error) {
	keys := seatKeys(showtimeID, seatIDs)
	result, err := l.acquire.Run(
		ctx,
		l.client,
		keys,
		owner,
		strconv.FormatInt(ttl.Milliseconds(), 10),
	).Slice()
	if err != nil {
		return false, fmt.Errorf("acquire Redis seat locks: %w", err)
	}
	if len(result) == 0 {
		return false, fmt.Errorf("acquire Redis seat locks: empty script response")
	}
	acquired, ok := result[0].(int64)
	if !ok {
		return false, fmt.Errorf("acquire Redis seat locks: unexpected script response %T", result[0])
	}
	return acquired == 1, nil
}

func (l *SeatLocker) Release(
	ctx context.Context,
	showtimeID int64,
	seatIDs []int64,
	owner string,
) error {
	if len(seatIDs) == 0 {
		return nil
	}
	if _, err := l.release.Run(ctx, l.client, seatKeys(showtimeID, seatIDs), owner).Result(); err != nil {
		return fmt.Errorf("release Redis seat locks: %w", err)
	}
	return nil
}

func seatKeys(showtimeID int64, seatIDs []int64) []string {
	keys := make([]string, 0, len(seatIDs))
	showtime := strconv.FormatInt(showtimeID, 10)
	for _, seatID := range seatIDs {
		keys = append(
			keys,
			"seat-hold:{showtime:"+showtime+"}:seat:"+strconv.FormatInt(seatID, 10),
		)
	}
	return keys
}
