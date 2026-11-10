-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ---------------------------------------------------------------------------
-- Identity
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,              -- stored lower-cased by the application
    full_name  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_users_email ON users (email);

-- ---------------------------------------------------------------------------
-- Catalog
-- ---------------------------------------------------------------------------
CREATE TABLE movies (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    duration_minutes INT  NOT NULL CHECK (duration_minutes > 0),
    genres           TEXT[] NOT NULL DEFAULT '{}',
    certificate      TEXT NOT NULL DEFAULT '',
    release_date     DATE,
    poster_url       TEXT NOT NULL DEFAULT '',
    imdb_id          TEXT UNIQUE,         -- e.g. tt0111161, used by OMDb
    tmdb_id          BIGINT UNIQUE,       -- used by TMDB
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Trigram index makes ILIKE '%term%' search indexable.
CREATE INDEX idx_movies_title_trgm ON movies USING gin (title gin_trgm_ops);
CREATE INDEX idx_movies_genres     ON movies USING gin (genres);
CREATE INDEX idx_movies_release    ON movies (release_date DESC NULLS LAST);

CREATE TABLE cinemas (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    address    TEXT NOT NULL,
    city       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cinemas_city_name ON cinemas (city, name);

CREATE TABLE halls (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cinema_id  UUID NOT NULL REFERENCES cinemas (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_halls_cinema_name UNIQUE (cinema_id, name)
);

CREATE TABLE seats (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hall_id              UUID NOT NULL REFERENCES halls (id) ON DELETE CASCADE,
    row_label            TEXT NOT NULL,
    seat_number          INT  NOT NULL CHECK (seat_number > 0),
    seat_type            TEXT NOT NULL DEFAULT 'STANDARD' CHECK (seat_type IN ('STANDARD', 'VIP', 'COUPLE')),
    price_modifier_cents BIGINT NOT NULL DEFAULT 0 CHECK (price_modifier_cents >= 0),
    CONSTRAINT uq_seats_position UNIQUE (hall_id, row_label, seat_number)
);

CREATE TABLE showtimes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id         UUID NOT NULL REFERENCES movies (id),
    hall_id          UUID NOT NULL REFERENCES halls (id),
    starts_at        TIMESTAMPTZ NOT NULL,
    ends_at          TIMESTAMPTZ NOT NULL,
    base_price_cents BIGINT NOT NULL CHECK (base_price_cents >= 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    -- A hall can never run two overlapping showtimes.
    CONSTRAINT ex_showtimes_hall_overlap EXCLUDE USING gist (
        hall_id WITH =, tstzrange(starts_at, ends_at) WITH &&
    )
);
CREATE INDEX idx_showtimes_movie_start ON showtimes (movie_id, starts_at);
CREATE INDEX idx_showtimes_hall_start  ON showtimes (hall_id, starts_at);
CREATE INDEX idx_showtimes_start       ON showtimes (starts_at);

-- ---------------------------------------------------------------------------
-- Booking
-- ---------------------------------------------------------------------------
CREATE TABLE bookings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users (id),
    showtime_id  UUID NOT NULL REFERENCES showtimes (id),
    status       TEXT NOT NULL CHECK (status IN ('HELD', 'CONFIRMED', 'CANCELLED', 'EXPIRED')),
    total_cents  BIGINT NOT NULL CHECK (total_cents >= 0),
    ticket_code  TEXT NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ,            -- hold deadline; NULL once confirmed/closed
    confirmed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status <> 'HELD' OR expires_at IS NOT NULL)
);
CREATE INDEX idx_bookings_user_created ON bookings (user_id, created_at DESC);
CREATE INDEX idx_bookings_showtime     ON bookings (showtime_id);
-- Partial index: the hold sweeper only ever scans live holds.
CREATE INDEX idx_bookings_hold_expiry  ON bookings (expires_at) WHERE status = 'HELD';

-- One row per (booking, seat). Status mirrors the booking lifecycle per seat.
CREATE TABLE booking_seats (
    booking_id  UUID NOT NULL REFERENCES bookings (id) ON DELETE CASCADE,
    showtime_id UUID NOT NULL REFERENCES showtimes (id),
    seat_id     UUID NOT NULL REFERENCES seats (id),
    status      TEXT NOT NULL CHECK (status IN ('HELD', 'CONFIRMED', 'RELEASED')),
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    PRIMARY KEY (booking_id, seat_id)
);
-- THE anti double-booking guarantee: at most one live (HELD or CONFIRMED)
-- row per seat per showtime. Enforced by PostgreSQL regardless of Redis.
CREATE UNIQUE INDEX uq_booking_seats_active
    ON booking_seats (showtime_id, seat_id)
    WHERE status IN ('HELD', 'CONFIRMED');

CREATE TABLE payments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id   UUID NOT NULL REFERENCES bookings (id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    status       TEXT NOT NULL CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED', 'REFUND_PENDING', 'REFUNDED', 'NO_REFUND')),
    provider_ref TEXT NOT NULL DEFAULT '',
    refund_cents BIGINT NOT NULL DEFAULT 0 CHECK (refund_cents >= 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- At most one in-flight or successful payment per booking (blocks double charge).
CREATE UNIQUE INDEX uq_payments_open_per_booking
    ON payments (booking_id)
    WHERE status IN ('PENDING', 'SUCCEEDED');
CREATE INDEX idx_payments_refund_pending ON payments (updated_at) WHERE status = 'REFUND_PENDING';

-- ---------------------------------------------------------------------------
-- Ratings from third parties (durable fallback behind Redis)
-- ---------------------------------------------------------------------------
CREATE TABLE movie_ratings (
    movie_id     UUID PRIMARY KEY REFERENCES movies (id) ON DELETE CASCADE,
    ratings      JSONB NOT NULL DEFAULT '[]',
    synced_at    TIMESTAMPTZ,            -- last successful sync (NULL = never)
    attempted_at TIMESTAMPTZ             -- last sync attempt, successful or not
);
CREATE INDEX idx_movie_ratings_attempted ON movie_ratings (attempted_at);

-- ---------------------------------------------------------------------------
-- Transactional outbox: events committed atomically with business data.
-- ---------------------------------------------------------------------------
CREATE TABLE outbox_events (
    id           UUID PRIMARY KEY,
    topic        TEXT NOT NULL,
    message_key  TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS movie_ratings;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS booking_seats;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS showtimes;
DROP TABLE IF EXISTS seats;
DROP TABLE IF EXISTS halls;
DROP TABLE IF EXISTS cinemas;
DROP TABLE IF EXISTS movies;
DROP TABLE IF EXISTS users;
