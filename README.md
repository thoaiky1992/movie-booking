# Movie Booking System

Backend đặt vé dùng Go/Gin, PostgreSQL, Redis và Goose. PostgreSQL là nguồn sự
thật cho booking/seat ownership; Redis giảm tranh chấp ở đường ghi nóng và cung
cấp TTL nhanh cho seat hold.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## MakeFile

Run build make command with tests
```bash
make all
```

Build the application
```bash
make build
```

Run the application
```bash
make run
```
Create DB container
```bash
make docker-run
```

Shutdown DB Container
```bash
make docker-down
```

DB Integrations Test:
```bash
make itest
```

## Database migrations

The project uses [Goose](https://github.com/pressly/goose) for PostgreSQL
migrations. Migration files live in `migrations/`. The Makefile reads the
existing `BLUEPRINT_DB_*` values from `.env`, so start PostgreSQL before
applying migrations:

```bash
make docker-run
make migration-up
```

Create a migration:

```bash
make migration-create name=create_movies
```

Check migration status or roll back the latest migration:

```bash
make migration-status
make migration-down
```

Goose is pinned as a Go tool in `go.mod`; Go downloads it automatically on the
first use. To use an installed binary instead, run a command with `GOOSE=goose`,
for example:

```bash
make migration-status GOOSE=goose
```

## Catalog APIs

The read-only catalog endpoints are served under `/api/v1`:

```text
GET /api/v1/cinemas
GET /api/v1/cinemas/:cinemaID/movies
GET /api/v1/movies/:movieID
GET /api/v1/cinemas/:cinemaID/movies/:movieID/showtimes
GET /api/v1/showtimes/:showtimeID/seats
POST /api/v1/showtimes/:showtimeID/holds
POST /api/v1/bookings/:bookingID/confirm
POST /api/v1/bookings/:bookingID/cancel
```

The seat-map endpoint returns seats grouped by row. Seat status is one of
`AVAILABLE`, `HELD`, or `BOOKED`; an expired hold is treated as available.
The movie-detail endpoint includes the `external_ratings` array.

Create an all-or-nothing 15-minute seat hold:

```bash
curl -X POST http://localhost:8080/api/v1/showtimes/12/holds \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"seat_ids":[1,2]}'
```

Cancel a pending booking owned by the authenticated user and release its seats:

```bash
curl -X POST http://localhost:8080/api/v1/bookings/42/cancel \
  -H 'X-User-ID: 1'
```

Confirm a pending booking and enqueue its confirmation email:

```bash
curl -X POST http://localhost:8080/api/v1/bookings/42/confirm \
  -H 'X-User-ID: 1'
```

Confirmation atomically changes the booking from `PENDING` to `CONFIRMED` and
its seat claims from `HELD` to `BOOKED`. The endpoint is idempotent. It publishes
one `email:booking_confirmation` task per booking to the Asynq `email` queue in
Redis. The background worker consumes the task and the current `MockMailer`
waits for a random 1–2 seconds, then prints a
`mock_booking_confirmation_email_sent` success log instead of contacting an
external mail provider.

Only `PENDING` bookings can be cancelled. A booking belonging to another user is
reported as not found; an already confirmed, cancelled, or expired booking
returns `409 BOOKING_NOT_CANCELLABLE`.

`X-User-ID` is resolved by HTTP middleware, verified against PostgreSQL, and
stored in the request context before the booking handler runs. In production,
this header must be overwritten by a trusted authentication gateway; do not
trust a value supplied directly by a public client.

```json
{
  "success": true,
  "data": {
    "booking_id": 42,
    "showtime_id": 1,
    "seat_ids": [1, 2],
    "hold_token": "0fcb4f93-b17f-4668-a310-b28e66d9e6d2",
    "status": "PENDING",
    "total_amount": "200000.00",
    "expires_at": "2026-11-11T10:15:00Z"
  }
}
```

## Seat-hold concurrency design

The Redis Lua script checks every seat key first and writes every key with
`PSETEX` only if the full set is free. Keys use a showtime hash tag, so the
script also works on Redis Cluster. This is a fast contention gate, not the
source of truth.

The PostgreSQL transaction creates a pending booking and inserts every
`seat_claims` row. Its primary key `(showtime_id, seat_id)` serializes races.
`ON CONFLICT ... DO UPDATE ... WHERE expires_at <= NOW()` permits only expired
holds to be reclaimed. If fewer than all requested seats are returned, the
transaction rolls back, including the booking and booking items. Consequently,
a Redis restart or temporary outage cannot cause a double sale.

The expiry worker selects pending bookings in configurable batches with
`FOR UPDATE OF b SKIP LOCKED`, deletes their `HELD` claims, and marks them
`EXPIRED` in one transaction. Multiple API replicas can run this worker safely.
Redis cleanup is compare-and-delete by the opaque hold token; TTL remains the
self-healing fallback.

Relevant configuration:

```text
REDIS_URL=redis://localhost:6379/0
DB_MAX_OPEN_CONNS=20
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME_MINUTES=30
DB_CONN_MAX_IDLE_TIME_MINUTES=5
SEAT_HOLD_TTL_SECONDS=900
HOLD_EXPIRY_INTERVAL_SECONDS=5
HOLD_EXPIRY_BATCH_SIZE=100
RATING_UPDATE_INTERVAL_SECONDS=30
ASYNQ_CONCURRENCY=5
```

The seat-hold expiry worker, Asynq email worker, and external-rating updater are
separate processes. Run them locally in separate terminals with:

```bash
make run-hold-expiry-worker
make run-email-worker
make run-rating-updater-worker
```

The rating updater runs once immediately and then every
`RATING_UPDATE_INTERVAL_SECONDS` (default `30`). Each cycle selects one row from
`movie_external_ratings`, assigns a random score from `0` through its
`max_score`, and refreshes `last_synced_at`.

Docker Compose starts `api` and all three workers after their dependencies are
ready:

```bash
docker compose up --build
```

The database query uses `SKIP LOCKED`, so the worker implementation supports
multiple replicas. Remove the fixed Compose `container_name` before using
`docker compose up --scale worker=N`.

Each worker polling cycle generates a `trace_id` and propagates it through the
PostgreSQL expiry call, Redis cleanup, and all structured logs. Idle cycle logs
are emitted at `DEBUG`; cycles that release seats are emitted at `INFO` with
`released_seats`, `database_calls`, and `latency_ms`.

All successful API responses use the same envelope:

```json
{
  "success": true,
  "data": {},
  "trace_id": "8bc0d82e5c33435eb1d9f4ecbd1f8682"
}
```

Errors use a separate, consistent envelope:

```json
{
  "success": false,
  "error": {
    "code": "SHOWTIME_NOT_FOUND",
    "message": "showtime not found"
  },
  "trace_id": "8bc0d82e5c33435eb1d9f4ecbd1f8682"
}
```

The API accepts an optional `X-Trace-ID` request header and always returns it
in both the response header and body. Access, error, and lifecycle logs are
written as JSON and include `trace_id` for request-scoped entries. Set
`LOG_LEVEL` to `debug`, `info` (default), `warn`, or `error`.

Live reload the application:
```bash
make watch
```

Run the test suite:
```bash
make test
```

Run the hot-seat contention load test after choosing 1 to 10 seats. The same command
can be repeated while any seat is still held or booked; later runs then report
zero successful booking requests instead of aborting. By default, the configured
VUs continuously send requests for one minute:

```bash
make loadtest-hot-seat SHOWTIME_ID=1 SEAT_IDS=193,194,195
```

The scenario and configurable performance thresholds are documented in
[`loadtests/k6/README.md`](loadtests/k6/README.md).

Clean up binary from the last build:
```bash
make clean
```
