# Movie Booking System

Backend for a cinema booking platform: catalogue and showtime search, seat holds
with TTL, payments, cancellations with refund policy, asynchronous ticket emails,
and third-party movie ratings that never slow down the detail page.

Stack: Go 1.27 · Gin · PostgreSQL 18 · Redis 8 · Kafka (KRaft) · Goose migrations · Clean Architecture.

---

## Quick start

```bash
docker compose up -d --build        # postgres, redis, kafka, migrate, api, worker, swagger, redisinsight
make seed                           # demo cinema, 1 hall (60 seats), 3 movies, 9 showtimes
open http://localhost:8081          # Swagger UI (port 8081)
curl localhost:8080/health
```

Port conflicts: every host port is overridable (`API_HOST_PORT`, `POSTGRES_HOST_PORT`,
`REDIS_HOST_PORT`, `KAFKA_HOST_PORT`, `SWAGGER_HOST_PORT`, `REDISINSIGHT_HOST_PORT`).
Kafka's advertised port follows `KAFKA_HOST_PORT`, so override both together.

Run binaries on the host instead (infrastructure in Docker):

```bash
docker compose up -d postgres redis kafka
make migrate-up && make seed
make run              # API on :8080       (reads .env)
make run-worker       # outbox relay, sweepers, rating sync, email consumer
```

Customer endpoints identify the caller with `X-User-ID: <uuid>`. Create one with
`POST /api/v1/users`. This is a placeholder for a verified JWT at the gateway, and
the header must not be exposed to the internet as-is.

---

## Architecture

```
cmd/            api · worker · migrate · seed         (composition only)
internal/
  domain/       entities, invariants, state rules       (no I/O, no imports from the project)
  usecase/      application services + port interfaces  (imports domain only)
  adapter/
    http/       Gin handlers, DTOs, error mapping
    postgres/   pgx repositories, transactions, outbox relay
    redis/      seat gate (Lua), rating cache, dedupe
    kafka/      publisher, consumer with retries + DLQ
    payment/    PSP port implementation (mock: charges and refunds succeed)
    rating/     rating-partner mock (deterministic IMDb / Rotten Tomatoes / TMDB values)
    mail/       ticket composer (QR) and log mailer (emails are logged, not sent)
  app/          composition root shared by api and worker
  worker/       periodic job scheduler
  platform/     circuit breaker
db/migrations/  goose SQL migrations (embedded in the binary)
```

Dependency rule, enforced by the import graph: adapters depend on use cases, use
cases depend on domain, nothing depends on adapters except `app` and `cmd`.

### ERD

```mermaid
erDiagram
    users ||--o{ bookings : places
    movies ||--o{ showtimes : screened_as
    cinemas ||--o{ halls : contains
    halls ||--o{ seats : layout
    halls ||--o{ showtimes : hosts
    showtimes ||--o{ bookings : sold_for
    bookings ||--o{ booking_seats : reserves
    seats ||--o{ booking_seats : "held as"
    bookings ||--o{ payments : paid_by
    movies ||--o| movie_ratings : rated_by
    bookings ||--o{ outbox_events : "emits (same tx)"
```

Full DDL: [db/migrations/00001_init_schema.sql](db/migrations/00001_init_schema.sql).

---

## Anti double-booking

Requirement: at most one live customer may hold or buy a given seat for a given showtime.

Two layers, and the database is the one that decides:

| Layer | Mechanism | Role |
|---|---|---|
| **Redis gate** | Lua script, all-or-nothing `EXISTS` check then `SET … PX`, keys `seat:hold:{showtime}:{seat}` | Rejects hot-seat contention in memory, before PostgreSQL. |
| **PostgreSQL** | Partial unique index `uq_booking_seats_active ON booking_seats(showtime_id, seat_id) WHERE status IN ('HELD','CONFIRMED')` | **Final arbiter.** Holds even if Redis is down, flushed, or wrong. |

Hold sequence ([internal/usecase/booking.go](internal/usecase/booking.go) `Hold`):

1. Load the showtime and seats. Verify that the seats belong to the showtime's hall.
2. `domain.NewHold` checks the rules: seat count, no duplicates, showtime in the future. It computes the total and `expires_at = now + SEAT_HOLD_TTL`.
3. Redis `acquireScript` claims every seat atomically or none. A conflict returns 409 immediately.
4. A PostgreSQL transaction ([booking_repo.go](internal/adapter/postgres/booking_repo.go) `CreateHold`):
   - Lazily reclaims seats whose hold has already expired, so a customer never waits for the sweeper.
   - Inserts the booking and its seats with one `INSERT … SELECT unnest(…)`.
   - A unique violation maps to `ErrSeatUnavailable`, and the Redis keys are released (compare-and-delete).
5. If Redis itself fails, the request continues with the DB guard alone. A circuit breaker stops repeated attempts to a dead Redis.

TTL release, three independent mechanisms:

- **Redis TTL** frees the lock automatically.
- **Hold sweeper** (worker, every 10s): `UPDATE … FROM (… FOR UPDATE SKIP LOCKED)` expires overdue bookings in batches and releases their seats.
- **Lazy reclaim** inside `CreateHold` (described above).

Payment does the same under lock: `Pay` takes `SELECT … FOR UPDATE` on the booking and rejects it if the hold has expired. The charge runs outside any transaction, so no row lock is held across a network call. If the hold expires or the booking is cancelled during the charge, the money is refunded automatically.

Cancellation refund policy ([domain/booking.go](internal/domain/booking.go)): 100% at 24h or more before showtime, 50% between 2h and 24h, 0% under 2h. A held booking is released with no refund.

---

## Notifications: transactional outbox

`Pay` and `Cancel` write the event to `outbox_events` **in the same transaction** that changes the booking. The worker's relay publishes committed rows to Kafka (`booking.events`, key = booking ID), so a crash after commit cannot lose a notification. Publishing is at-least-once, and every event carries an `event_id` that the consumer uses for deduplication.

Consumer ([adapter/kafka/kafka.go](internal/adapter/kafka/kafka.go)):

- Consumer group `notification-service`. Offsets commit only after success or DLQ publish.
- Transient failures retry 5 times with exponential backoff and full jitter.
- After exhausting retries, the message goes to `notification.dlq`. If the DLQ publish itself fails, the consumer blocks and retries, because skipping an offset would lose the message.
- Idempotency: `SET NX notif:sent:<event_id>` claims the send. The claim is released on send failure so the retry can actually send.

The email carries the ticket code and a QR code (inline `multipart/related` image, `Content-ID`). The QR encodes only the ticket code, with no personal data.

---

## Ratings: never on the request path

`GET /api/v1/movies/:id` makes no partner call. It reads:

1. Redis `rating:movie:<id>`, with a budget of half of `RATING_READ_BUDGET` (default 80ms).
2. On a miss, PostgreSQL `movie_ratings`, the last known good snapshot. It warms Redis asynchronously (cache-aside).
3. Otherwise `status: "unavailable"` and the page still renders.

The `ratings.status` field reports `cache`, `database` or `unavailable`.

The worker's `rating-sync` job (every `RATING_SYNC_INTERVAL`, default 1h) calls the rating partner (mock) concurrently:

- Each call has a `PARTNER_TIMEOUT` of 500ms.
- Each partner is wrapped in a circuit breaker (opens after 5 consecutive failures, cooldown 30s).
- Results are merged per source. A partner that fails this round keeps its previous value.
- The snapshot goes to PostgreSQL and to Redis with `RATING_CACHE_TTL` (12h).

Partner: a mock (`internal/adapter/rating/mock.go`) that returns deterministic IMDb, Rotten Tomatoes and TMDB values for each movie and logs every call. Real partner clients can be added behind the same `RatingProvider` port.

---

## API

Full contract in [openapi.yaml](openapi.yaml), served by Swagger UI.

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/health` | – | Postgres and Redis ping, 503 when degraded |
| POST | `/api/v1/users` | – | Register |
| GET | `/api/v1/movies?q=&genre=&page=&page_size=` | – | Title search (trigram index), genre filter, pagination |
| GET | `/api/v1/movies/:id` | – | Details plus ratings from cache or DB |
| GET | `/api/v1/cinemas?page=` | – | Pagination |
| GET | `/api/v1/cinemas/:id/halls` | – | |
| GET | `/api/v1/showtimes?movie_id=&cinema_id=&from=&to=&page=` | – | Filters on indexed columns |
| GET | `/api/v1/showtimes/:id/seats` | – | Seat map: `AVAILABLE` / `HELD` / `SOLD` |
| POST | `/api/v1/showtimes/:id/holds` | ✔ | `{"seat_ids": [...]}`, max 10, TTL 10 min |
| GET | `/api/v1/bookings/:id` | ✔ | Owner only (others get 404) |
| POST | `/api/v1/bookings/:id/payments` | ✔ | Charge and confirm |
| POST | `/api/v1/bookings/:id/cancel` | ✔ | Release seats and refund per policy |

Errors: `{"error": {"code": "...", "message": "..."}}`. Mapping: `seat_unavailable` 409,
`hold_expired` 410, `payment_declined` 402, `not_found` 404, `invalid_input` 400.

---

## Database

Tables: `users`, `movies`, `cinemas`, `halls`, `seats`, `showtimes`, `bookings`,
`booking_seats`, `payments`, plus `movie_ratings` and `outbox_events`.

Notable constraints and indexes:

- `uq_booking_seats_active` (partial unique): the double-booking guarantee.
- `ex_showtimes_hall_overlap` (exclusion, `btree_gist`): a hall cannot run two overlapping showtimes.
- `uq_payments_open_per_booking` (partial unique): at most one in-flight or successful payment per booking.
- `idx_bookings_hold_expiry` (partial, `status='HELD'`): the sweeper scans only live holds.
- `idx_movies_title_trgm` (GIN, `pg_trgm`): title search. `idx_movies_genres` (GIN): genre containment.
- `idx_outbox_unpublished` (partial): the relay scans only unpublished rows.

---

## Configuration

Environment variables, with defaults in [internal/config/config.go](internal/config/config.go).
Local values are in `.env` (gitignored). Production values come from the real environment.

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://movie:movie@localhost:5432/movie` | |
| `REDIS_ADDR` | `localhost:6379` | |
| `KAFKA_BROKERS` | `localhost:9092` | comma separated |
| `SEAT_HOLD_TTL` | `10m` | Hold duration |
| `MAX_SEATS_PER_BOOKING` | `10` | |
| `CURRENCY` | `VND` | |
| `MOCK_PAYMENT_DECLINE_ALL` | `false` | Simulate declines |
| `MOCK_PAYMENT_DECLINE_ALL` | `false` | Make the payment mock decline every charge |
| `PARTNER_TIMEOUT` | `500ms` | Per partner call |
| `RATING_CACHE_TTL` / `RATING_SYNC_INTERVAL` / `RATING_READ_BUDGET` | `12h` / `1h` / `80ms` | |
| `CORS_ALLOWED_ORIGINS` | empty | Comma separated |

---

## Feature flows

Each diagram shows which service talks to which, in what order, and what the outcome is.
Services: **API** (HTTP), **Worker** (background jobs and the Kafka consumer), **PostgreSQL**,
**Redis**, **Kafka**, and the **mocks** for payment, email and rating partners.

### System overview

```mermaid
flowchart LR
  Client([Customer or k6]) -->|HTTP JSON| API[API - Gin]
  API -->|reads and transactions| PG[(PostgreSQL)]
  API -->|seat locks and rating cache| RD[(Redis)]
  API -->|charge and refund| PSP[Payment mock]
  Worker[Worker] -->|sweeps, sync, retries| PG
  Worker -->|seat release, dedupe, cache| RD
  Worker -->|refund retry| PSP
  Worker -->|outbox relay and consume| KF{{Kafka booking.events}}
  Worker -->|send email| ML[Mail mock - logs]
  Worker -->|rating fetch| RP[Rating partner mock]
```

### 1. Register a customer

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  C->>API: POST /api/v1/users with email and full_name
  API->>API: domain.NewUser lower-cases and validates
  API->>PG: INSERT INTO users
  alt email already registered
    PG-->>API: unique violation uq_users_email
    API-->>C: 409 conflict
  else
    PG-->>API: row inserted
    API-->>C: 201 user id
  end
```

### 2. Browse and search movies and showtimes

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  C->>API: GET /api/v1/movies with q, genre, page
  API->>PG: COUNT and SELECT, title ILIKE on pg_trgm index, genres containment on GIN index
  PG-->>API: page of movies and total
  API-->>C: 200 data and pagination
  C->>API: GET /api/v1/showtimes with movie_id, cinema_id, from, to
  API->>PG: showtimes JOIN movies, halls, cinemas filtered on indexed columns
  PG-->>API: page of showtimes
  API-->>C: 200 data and pagination
```

### 3. Seat map

The seat map is read straight from PostgreSQL. Redis is not involved, so the map always matches
the booking rows. A HELD seat whose deadline has passed is shown as AVAILABLE, even before the
sweeper has run.

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  C->>API: GET /api/v1/showtimes/:id/seats
  API->>PG: load showtime
  API->>PG: hall seats LEFT JOIN booking_seats and bookings, status computed in SQL
  PG-->>API: each seat with AVAILABLE, HELD or SOLD
  API-->>C: 200 seats
```

### 4. Movie detail with ratings (read path, never calls a partner)

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  participant RD as Redis
  C->>API: GET /api/v1/movies/:id
  API->>PG: SELECT movie by id
  API->>RD: GET rating:movie:id with budget of half of RATING_READ_BUDGET
  alt cache hit
    RD-->>API: snapshot, status cache
  else cache miss
    API->>PG: SELECT movie_ratings, last known snapshot
    alt snapshot exists
      PG-->>API: snapshot, status database
      API-)RD: SET rating:movie:id with TTL, in background
    else nothing yet
      PG-->>API: none, status unavailable
    end
  end
  API-->>C: 200 movie and ratings, with the page rendered either way
```

### 5. Hold seats (anti double-booking)

This is the core flow. Redis claims the seats first, as a fast gate. PostgreSQL then records the
booking under a unique index, which is the final arbiter.

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  participant RD as Redis
  C->>API: POST /api/v1/showtimes/:id/holds with seat_ids and X-User-ID
  API->>PG: load showtime and seats of this hall
  API->>API: domain.NewHold checks count, duplicates, future start, computes total and expires_at
  API->>RD: EVAL acquire script on seat:hold keys, all or nothing, PX ttl
  alt any seat already locked in Redis
    RD-->>API: index of the conflicting seat
    API-->>C: 409 seat_unavailable
  else claimed, or Redis is down and we continue
    API->>PG: BEGIN, reclaim expired holds on these seats
    API->>PG: INSERT bookings with status HELD
    API->>PG: INSERT booking_seats with unnest, status HELD
    alt uq_booking_seats_active violated
      PG-->>API: unique violation, ROLLBACK
      API->>RD: EVAL release script, compare and delete
      API-->>C: 409 seat_unavailable
    else COMMIT
      PG-->>API: committed
      API-->>C: 201 booking with expires_at
    end
  end
```

### 6. Pay and confirm

The charge runs outside any database transaction, so no row lock is held across the network call.
The outcome is then reconciled under a lock.

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  participant PSP as Payment mock
  participant RD as Redis
  C->>API: POST /api/v1/bookings/:id/payments
  API->>PG: load booking and latest payment, owner check
  API->>API: IsPayable, status HELD and now before expires_at
  API->>PG: BEGIN, lock booking FOR UPDATE, INSERT payment PENDING
  Note over PG: uq_payments_open_per_booking blocks a second payment
  API->>PSP: Charge with idempotency key equal to payment id
  alt declined or error
    PSP-->>API: declined or error
    API->>PG: payment FAILED
    API-->>C: 402 or 500
  else charged
    PSP-->>API: provider reference
    API->>PG: BEGIN, lock booking FOR UPDATE, check still HELD and not expired
    alt hold expired or booking cancelled during the charge
      API->>PG: payment REFUND_PENDING
      API->>PSP: Refund with the same idempotency key
      API-->>C: 410 hold_expired
    else still payable
      API->>PG: payment SUCCEEDED, booking CONFIRMED, seats CONFIRMED
      API->>PG: INSERT outbox_events booking.confirmed, same transaction
      PG-->>API: COMMIT
      API->>RD: EVAL confirm script, PEXPIREAT until showtime end plus 24h
      API-->>C: 200 booking CONFIRMED
    end
  end
```

If writing the confirmation fails after a successful charge, the API calls `compensateCharge` and refunds the money.

### 7. Cancel and refund

```mermaid
sequenceDiagram
  participant C as Client
  participant API as API
  participant PG as PostgreSQL
  participant RD as Redis
  participant PSP as Payment mock
  C->>API: POST /api/v1/bookings/:id/cancel
  API->>PG: load booking detail, owner check
  API->>API: CancellationRefund, HELD gives 0, CONFIRMED gives 100 or 50 or 0 percent
  API->>PG: BEGIN, lock booking, reject if a payment is PENDING
  API->>PG: booking CANCELLED, seats RELEASED, payment REFUND_PENDING or NO_REFUND
  API->>PG: INSERT outbox_events booking.cancelled, COMMIT
  API->>RD: EVAL release script, compare and delete
  opt refund is greater than 0
    API->>PSP: Refund with idempotency key equal to payment id
    alt refund succeeded
      API->>PG: payment REFUNDED
    else refund failed
      Note over PG: payment stays REFUND_PENDING, worker retries
    end
  end
  API-->>C: 200 status CANCELLED, refund_cents, refund_status
```

### 8. Hold expiry (TTL)

A seat is released by three independent mechanisms. The Redis key expires on its own. The
sweeper expires the database rows. A customer who arrives early triggers a lazy reclaim.

```mermaid
sequenceDiagram
  participant W as Worker hold-sweeper
  participant PG as PostgreSQL
  participant RD as Redis
  loop every WORKER_HOLD_SWEEP_INTERVAL
    W->>PG: UPDATE bookings to EXPIRED where HELD and expires_at at or before now, FOR UPDATE SKIP LOCKED, batch of 500
    PG-->>W: expired booking ids with showtime and seat ids
    W->>RD: EVAL release script per booking, compare and delete
  end
```

### 9. Booking confirmation email (outbox, then Kafka, then consumer)

The event is written in the same transaction as the booking, so a crash after commit cannot lose it.

```mermaid
sequenceDiagram
  participant API as API
  participant PG as PostgreSQL
  participant R as Worker outbox-relay
  participant K as Kafka booking.events
  participant N as Worker notification consumer
  participant RD as Redis
  participant M as Mail mock
  API->>PG: in the same transaction as the booking, INSERT outbox_events with published_at NULL
  loop every WORKER_OUTBOX_INTERVAL
    R->>PG: BEGIN, SELECT unpublished rows FOR UPDATE SKIP LOCKED, batch of 100
    R->>K: produce with key booking_id and value event JSON, acks all
    R->>PG: UPDATE published_at, COMMIT
  end
  K->>N: deliver to group notification-service
  N->>RD: SET NX notif:sent:event_id with 7 day TTL
  alt already claimed
    N-->>N: duplicate, skip
  else first time
    N->>N: compose HTML and QR PNG, inline by content id
    N->>M: Send
    alt send failed
      N->>RD: DEL claim so the retry can send
      N-->>N: retry up to 5 times with backoff and jitter
      N->>K: after 5 failures, produce to notification.dlq
    end
  end
  N->>K: commit offset
```

### 10. Refund retry

```mermaid
sequenceDiagram
  participant W as Worker refund-retry
  participant PG as PostgreSQL
  participant PSP as Payment mock
  loop every WORKER_REFUND_RETRY_INTERVAL
    W->>PG: SELECT payments with status REFUND_PENDING
    loop each payment
      W->>PSP: Refund with idempotency key equal to payment id
      W->>PG: payment REFUNDED
    end
  end
```

Idempotency keys make retries safe. If the first refund actually succeeded, the second call is a no-op.

### 11. Rating sync (background, the only place partners are called)

```mermaid
sequenceDiagram
  participant W as Worker rating-sync
  participant PG as PostgreSQL
  participant P as Rating partner mock
  participant RD as Redis
  loop every RATING_SYNC_INTERVAL
    W->>PG: SELECT movies with an IMDb or TMDB id, not attempted in the last interval, oldest first
    loop each movie
      par each provider, with PARTNER_TIMEOUT
        W->>P: Fetch movie
        P-->>W: values, or error
      end
      W->>PG: read previous snapshot
      alt no fresh values
        W->>PG: mark attempt
        W->>RD: refresh previous snapshot with TTL
      else fresh values
        W->>W: merge per source, a failed partner keeps its last value
        W->>PG: UPSERT snapshot
        W->>RD: SET rating:movie:id with RATING_CACHE_TTL
      end
    end
  end
```

## Load test with k6: many customers, one seat

`loadtest/seat_contention.js` makes N customers try to hold the same seat within one second.

- `setup` registers N customers, finds a future showtime with a free seat, and records how many seats are already occupied.
- Each VU is one customer and sends one hold for that seat. All VUs start together.
- The thresholds require exactly one `201`, N-1 `409`s, and no other status codes.
- `teardown` reads the seat map and requires the seat to be HELD, with exactly one more occupied seat than before the race.

```bash
make loadtest                       # 200 customers against http://localhost:8080
make loadtest USERS=500             # more customers
k6 run -e USERS=500 loadtest/seat_contention.js
```

Each run takes the next free seat, because the previous winner keeps the seat for the hold TTL.

Measured on a laptop against the Docker stack, with the output checked against the database:

| Customers | Winners (201) | Conflicts (409) | Other | p95 hold latency | Max active rows for any seat |
|---|---|---|---|---|---|
| 200 | 1 | 199 | 0 | about 15 to 26 ms | 1 |
| 500 | 1 | 499 | 0 | about 37 ms | 1 |

The database check was a query across the whole `booking_seats` table, which showed no seat with more than one `HELD` or `CONFIRMED` row. These figures come from one machine, so treat them as correctness evidence, not capacity numbers.

## Operational notes

- **Migrations** run once in the `migrate` service before `api` and `worker` start. Goose is embedded in the binary, so a container needs no extra files.
- **Seeding** is idempotent: deterministic UUIDv5 IDs, and existing showtimes are skipped.
- **Health**: `/health` reports `degraded` (503) when a dependency is down. Redis being down does not stop bookings. It only slows detail-page fallback and skips the accelerator.
- **Graceful shutdown**: API drains in-flight requests for 10s. Worker jobs stop on context cancel.

## Known limitations and next steps

- **Auth** is a placeholder (`X-User-ID`). Replace with JWT verification at the gateway.
- **Mocks by design for now**: payment (`MockGateway`), email (`LogMailer` logs instead of sending) and rating partners (deterministic mock). Real adapters plug into the existing ports.
- **PSP**: production needs a real adapter. Charge timeouts with unknown outcome currently mark the payment `FAILED`. A reconciliation job should query the PSP by idempotency key and fix any mismatch.
- **Refund retries** run every `WORKER_REFUND_RETRY_INTERVAL`. There is no alerting on stuck `REFUND_PENDING` rows yet.
- **Rating read path** is cache-aside only. A background warmer for the most-viewed movies would reduce DB fallbacks.
- **Outbox** rows are never deleted. Add a retention job for published rows.
- **Observability**: logs are structured JSON, but there are no metrics or traces. Add Prometheus counters for hold conflicts, sweeper expiries, relay lag, and the DLQ depth.
- **Load testing** has not been done. The latency numbers in the verification log are from a single developer machine with sequential requests.
