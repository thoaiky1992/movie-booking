# Hot-seat contention load test

This k6 test sends many concurrent requests that try to hold the same group of
seats for the same showtime. It validates both correctness and endpoint latency:

- an available seat group can have one request return `201` with a `PENDING` booking;
- a group containing a `HELD` or `BOOKED` seat can have zero winners;
- no run may have more than one winner;
- every non-winning request must return `409 SEATS_UNAVAILABLE`;
- VUs continuously send requests for the configured duration (one minute by default);
- no unexpected response such as `401`, `429`, or `500` is allowed;
- hold-request p95 and p99 latency must stay below the configured limits.

The script first calls the seat-map API to validate that all target seats belong
to the showtime. It logs each initial seat status but also runs when any seat is
already `HELD` or `BOOKED`, so the same command can be repeated safely.

## Run

Start the application and seat-expiry worker, then choose an upcoming showtime
and 1 to 10 available seats from `GET /api/v1/showtimes/:showtimeID/seats`:

```bash
SHOWTIME_ID=12 \
SEAT_IDS=1,2,3 \
USER_IDS=1,2,3,4,5 \
VUS=200 \
DURATION=10s \
k6 run loadtests/k6/hot_seat_contention.js
```

Supported settings:

```text
BASE_URL=http://localhost:8080
SHOWTIME_ID=<required>
SEAT_IDS=<required: 1 to 10 unique, comma-separated IDs>
USER_IDS=1,2,3,4,5
VUS=200
DURATIO=10s
REQUEST_TIMEOUT=10s
P95_MS=1000
P99_MS=2000
```

The winning booking ID is printed as `WINNER booking_id=...`. The summary metric
`hot_seat_booking_succeeded` shows the number of requests that obtained the
whole seat group. On a repeated run containing a still-unavailable seat it reports `0.00%`
with `0` successful requests, while the run still passes. The test leaves a
winning booking in `PENDING`, so either cancel it manually or wait for the
normal hold TTL and expiry worker before expecting the seats to become available
again.

Unexpected responses are printed with their HTTP status and a truncated body.
This makes infrastructure limits such as an exhausted PostgreSQL connection
pool visible instead of incorrectly treating them as ordinary seat conflicts.

To validate that PostgreSQL still prevents a double sale without the Redis fast
path, run the API, stop Redis, and repeat the test with another available seat group.
The expected correctness threshold remains at most one winner; latency will be
higher because each request waits briefly for the Redis fallback.
