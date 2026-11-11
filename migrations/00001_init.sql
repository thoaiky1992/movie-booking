-- +goose Up
-- Cinema booking schema - PostgreSQL 16+
-- Run against an empty database (or a schema without conflicting object names).
-- All timestamps are TIMESTAMPTZ; store price snapshots in booking_items.


CREATE EXTENSION IF NOT EXISTS btree_gist;

-- 1. Cinema locations
CREATE TABLE cinemas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Screening rooms
CREATE TABLE rooms (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cinema_id BIGINT NOT NULL REFERENCES cinemas(id),
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_rooms_cinema_name UNIQUE (cinema_id, name)
);
CREATE INDEX idx_rooms_cinema ON rooms(cinema_id);

-- 3. Physical seats (availability is per showtime, not per seat)
CREATE TABLE seats (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    room_id BIGINT NOT NULL REFERENCES rooms(id),
    row_label VARCHAR(10) NOT NULL,
    seat_number INT NOT NULL,
    CONSTRAINT ck_seats_number_positive CHECK (seat_number > 0),
    CONSTRAINT uq_seats_position UNIQUE (room_id, row_label, seat_number),
    CONSTRAINT uq_seats_id_room UNIQUE (id, room_id)
);
CREATE INDEX idx_seats_room ON seats(room_id);

-- 4. Movies
CREATE TABLE movies (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    duration_minutes INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_movies_duration_positive CHECK (duration_minutes > 0)
);

-- 5. Showtimes
CREATE TABLE showtimes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    movie_id BIGINT NOT NULL REFERENCES movies(id),
    room_id BIGINT NOT NULL REFERENCES rooms(id),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_showtimes_time CHECK (ends_at > starts_at),
    CONSTRAINT uq_showtimes_id_room UNIQUE (id, room_id),
    CONSTRAINT ex_showtimes_room_overlap EXCLUDE USING GIST (
        room_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    )
);
CREATE INDEX idx_showtimes_movie_start ON showtimes(movie_id, starts_at);
CREATE INDEX idx_showtimes_room_start ON showtimes(room_id, starts_at);

-- 6. Customers
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    full_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. Bookings
CREATE TABLE bookings (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    showtime_id BIGINT NOT NULL REFERENCES showtimes(id),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    total_amount NUMERIC(12,2) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,
    CONSTRAINT ck_bookings_status CHECK (
        status IN ('PENDING', 'CONFIRMED', 'CANCELLED', 'EXPIRED')
    ),
    CONSTRAINT ck_bookings_amount CHECK (total_amount >= 0),
    CONSTRAINT uq_bookings_id_showtime UNIQUE (id, showtime_id)
);
CREATE INDEX idx_bookings_user_created ON bookings(user_id, created_at DESC);
CREATE INDEX idx_bookings_pending_expiry ON bookings(expires_at)
    WHERE status = 'PENDING';
CREATE INDEX idx_bookings_showtime ON bookings(showtime_id);

-- 8. Itemized booking history and price snapshot
CREATE TABLE booking_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    booking_id BIGINT NOT NULL,
    showtime_id BIGINT NOT NULL,
    seat_id BIGINT NOT NULL REFERENCES seats(id),
    price NUMERIC(12,2) NOT NULL,
    CONSTRAINT fk_items_booking_showtime FOREIGN KEY (booking_id, showtime_id)
        REFERENCES bookings(id, showtime_id),
    CONSTRAINT uq_items_booking_seat UNIQUE (booking_id, seat_id),
    CONSTRAINT ck_items_price CHECK (price >= 0)
);
CREATE INDEX idx_booking_items_booking ON booking_items(booking_id);

-- 9. Current seat ownership / temporary holds
-- Only one row per (showtime, seat). Expired holds can be reclaimed atomically.
CREATE TABLE seat_claims (
    showtime_id BIGINT NOT NULL,
    seat_id BIGINT NOT NULL,
    room_id BIGINT NOT NULL,
    booking_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_seat_claims PRIMARY KEY (showtime_id, seat_id),
    CONSTRAINT fk_claims_showtime_room FOREIGN KEY (showtime_id, room_id)
        REFERENCES showtimes(id, room_id),
    CONSTRAINT fk_claims_seat_room FOREIGN KEY (seat_id, room_id)
        REFERENCES seats(id, room_id),
    CONSTRAINT fk_claims_booking_showtime FOREIGN KEY (booking_id, showtime_id)
        REFERENCES bookings(id, showtime_id),
    CONSTRAINT ck_claims_status CHECK (status IN ('HELD', 'BOOKED')),
    CONSTRAINT ck_claims_expiry CHECK (
        (status = 'HELD' AND expires_at IS NOT NULL)
        OR (status = 'BOOKED' AND expires_at IS NULL)
    )
);
CREATE INDEX idx_seat_claims_booking ON seat_claims(booking_id);
CREATE INDEX idx_seat_claims_expiration ON seat_claims(expires_at)
    WHERE status = 'HELD';

-- 10. Payment attempts / transactions
CREATE TABLE payments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    booking_id BIGINT NOT NULL REFERENCES bookings(id),
    provider VARCHAR(50) NOT NULL,
    provider_transaction_id VARCHAR(255),
    amount NUMERIC(12,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ,
    CONSTRAINT ck_payments_amount CHECK (amount >= 0),
    CONSTRAINT ck_payments_status CHECK (
        status IN ('PENDING', 'SUCCESS', 'FAILED', 'REFUNDED')
    )
);
CREATE INDEX idx_payments_booking ON payments(booking_id);
CREATE UNIQUE INDEX uq_payments_provider_transaction
    ON payments(provider, provider_transaction_id)
    WHERE provider_transaction_id IS NOT NULL;


-- +goose Down
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS seat_claims;
DROP TABLE IF EXISTS booking_items;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS showtimes;
DROP TABLE IF EXISTS movies;
DROP TABLE IF EXISTS seats;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS cinemas;

-- Keep btree_gist because it may have existed before this migration.
