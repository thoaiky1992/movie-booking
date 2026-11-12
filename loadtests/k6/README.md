# Hot-seat contention load test

This k6 test sends many concurrent requests that try to hold the same seat for
the same showtime. It validates both correctness and endpoint latency:

- exactly one request must return `201` with a `PENDING` booking;
- every other request must return `409 SEATS_UNAVAILABLE`;
- all configured iterations must finish;
- no unexpected response such as `401`, `429`, or `500` is allowed;
- hold-request p95 and p99 latency must stay below the configured limits.

The script first calls the seat-map API and aborts unless the target seat is
currently `AVAILABLE`.

## Run

Start the application and seat-expiry worker, then choose an upcoming showtime
and one available seat from `GET /api/v1/showtimes/:showtimeID/seats`:

```bash
SHOWTIME_ID=1 \
SEAT_ID=1 \
USER_IDS=1,2,3,4,5 \
VUS=200 \
k6 run loadtests/k6/hot_seat_contention.js
```

Supported settings:

```text
BASE_URL=http://localhost:8080
SHOWTIME_ID=<required>
SEAT_ID=<required>
USER_IDS=1,2,3,4,5
VUS=200
ITERATIONS_PER_VU=1
MAX_DURATION=30s
REQUEST_TIMEOUT=10s
P95_MS=1000
P99_MS=2000
```

The winning booking ID is printed as `WINNER booking_id=...`. The test leaves
that booking in `PENDING`, so either cancel it manually or wait for the normal
hold TTL and expiry worker before reusing the same seat.

Unexpected responses are printed with their HTTP status and a truncated body.
This makes infrastructure limits such as an exhausted PostgreSQL connection
pool visible instead of incorrectly treating them as ordinary seat conflicts.

To validate that PostgreSQL still prevents a double sale without the Redis fast
path, run the API, stop Redis, and repeat the test with another available seat.
The expected correctness threshold remains exactly one winner; latency will be
higher because each request waits briefly for the Redis fallback.
