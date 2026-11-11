-- +goose Up
-- Development data for the core cinema-booking tables.

INSERT INTO users (email, full_name)
VALUES
    ('an.nguyen@example.com', 'Nguyễn Hoàng An'),
    ('binh.tran@example.com', 'Trần Gia Bình'),
    ('chi.le@example.com', 'Lê Minh Chi'),
    ('dung.pham@example.com', 'Phạm Anh Dũng'),
    ('ha.vo@example.com', 'Võ Ngọc Hà');

INSERT INTO cinemas (name)
VALUES
    ('CGV Vincom Center Landmark 81'),
    ('Galaxy Nguyễn Du');

INSERT INTO rooms (cinema_id, name)
SELECT cinemas.id, room_data.name
FROM (
    VALUES
        ('CGV Vincom Center Landmark 81', 'Phòng 1'),
        ('CGV Vincom Center Landmark 81', 'Phòng 2'),
        ('Galaxy Nguyễn Du', 'Phòng 1'),
        ('Galaxy Nguyễn Du', 'Phòng 2'),
        ('Galaxy Nguyễn Du', 'Phòng VIP')
) AS room_data(cinema_name, name)
JOIN cinemas ON cinemas.name = room_data.cinema_name;

-- Standard rooms have 6 rows x 10 seats; smaller rooms have the dimensions
-- declared below. generate_series keeps the seed compact and easy to adjust.
INSERT INTO seats (room_id, row_label, seat_number)
SELECT
    rooms.id,
    chr(ascii('A') + row_number - 1),
    seat_number
FROM (
    VALUES
        ('CGV Vincom Center Landmark 81', 'Phòng 1', 6, 10),
        ('CGV Vincom Center Landmark 81', 'Phòng 2', 5, 8),
        ('Galaxy Nguyễn Du', 'Phòng 1', 6, 10),
        ('Galaxy Nguyễn Du', 'Phòng 2', 6, 10),
        ('Galaxy Nguyễn Du', 'Phòng VIP', 4, 8)
) AS room_layout(cinema_name, room_name, row_count, seats_per_row)
JOIN cinemas ON cinemas.name = room_layout.cinema_name
JOIN rooms
    ON rooms.cinema_id = cinemas.id
    AND rooms.name = room_layout.room_name
CROSS JOIN LATERAL generate_series(1, room_layout.row_count) AS rows(row_number)
CROSS JOIN LATERAL generate_series(1, room_layout.seats_per_row) AS seat_numbers(seat_number);

INSERT INTO movies (title, duration_minutes)
VALUES
    ('Dune: Part Two', 166),
    ('Inside Out 2', 96),
    ('The Wild Robot', 102),
    ('Mai', 131),
    ('Đào, Phở và Piano', 100),
    ('Godzilla x Kong: The New Empire', 115);

-- Showtimes are based on the day the seed is applied so that a fresh local
-- database always starts with upcoming sessions. Each room's sessions are
-- spaced far enough apart to satisfy ex_showtimes_room_overlap.
INSERT INTO showtimes (movie_id, room_id, starts_at, ends_at)
SELECT
    movies.id,
    rooms.id,
    schedule.starts_at,
    schedule.starts_at + make_interval(mins => movies.duration_minutes)
FROM (
    SELECT
        cinema_name,
        room_name,
        movie_title,
        (
            (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Ho_Chi_Minh')::date
            + day_offset * INTERVAL '1 day'
            + start_offset
        ) AT TIME ZONE 'Asia/Ho_Chi_Minh' AS starts_at
    FROM (
        VALUES
            ('CGV Vincom Center Landmark 81', 'Phòng 1', 'Dune: Part Two', 1, INTERVAL '10 hours'),
            ('CGV Vincom Center Landmark 81', 'Phòng 1', 'Inside Out 2', 1, INTERVAL '14 hours'),
            ('CGV Vincom Center Landmark 81', 'Phòng 1', 'Mai', 1, INTERVAL '17 hours'),
            ('CGV Vincom Center Landmark 81', 'Phòng 2', 'The Wild Robot', 1, INTERVAL '11 hours'),
            ('CGV Vincom Center Landmark 81', 'Phòng 2', 'Godzilla x Kong: The New Empire', 1, INTERVAL '14 hours'),
            ('CGV Vincom Center Landmark 81', 'Phòng 2', 'Đào, Phở và Piano', 1, INTERVAL '17 hours'),
            ('Galaxy Nguyễn Du', 'Phòng 1', 'Mai', 2, INTERVAL '10 hours'),
            ('Galaxy Nguyễn Du', 'Phòng 1', 'Dune: Part Two', 2, INTERVAL '14 hours'),
            ('Galaxy Nguyễn Du', 'Phòng 2', 'Inside Out 2', 2, INTERVAL '11 hours'),
            ('Galaxy Nguyễn Du', 'Phòng 2', 'The Wild Robot', 2, INTERVAL '14 hours'),
            ('Galaxy Nguyễn Du', 'Phòng VIP', 'Godzilla x Kong: The New Empire', 2, INTERVAL '18 hours'),
            ('Galaxy Nguyễn Du', 'Phòng VIP', 'Đào, Phở và Piano', 3, INTERVAL '20 hours')
    ) AS showtime_data(
        cinema_name,
        room_name,
        movie_title,
        day_offset,
        start_offset
    )
) AS schedule
JOIN cinemas ON cinemas.name = schedule.cinema_name
JOIN rooms
    ON rooms.cinema_id = cinemas.id
    AND rooms.name = schedule.room_name
JOIN movies ON movies.title = schedule.movie_title;

-- +goose Down
-- Delete children before parents to preserve referential integrity.
DELETE FROM showtimes
WHERE room_id IN (
    SELECT rooms.id
    FROM rooms
    JOIN cinemas ON cinemas.id = rooms.cinema_id
    WHERE cinemas.name IN (
        'CGV Vincom Center Landmark 81',
        'Galaxy Nguyễn Du'
    )
)
AND movie_id IN (
    SELECT id
    FROM movies
    WHERE title IN (
        'Dune: Part Two',
        'Inside Out 2',
        'The Wild Robot',
        'Mai',
        'Đào, Phở và Piano',
        'Godzilla x Kong: The New Empire'
    )
);

DELETE FROM seats
WHERE room_id IN (
    SELECT rooms.id
    FROM rooms
    JOIN cinemas ON cinemas.id = rooms.cinema_id
    WHERE cinemas.name IN (
        'CGV Vincom Center Landmark 81',
        'Galaxy Nguyễn Du'
    )
);

DELETE FROM rooms
WHERE cinema_id IN (
    SELECT id
    FROM cinemas
    WHERE name IN (
        'CGV Vincom Center Landmark 81',
        'Galaxy Nguyễn Du'
    )
);

DELETE FROM cinemas
WHERE name IN (
    'CGV Vincom Center Landmark 81',
    'Galaxy Nguyễn Du'
);

DELETE FROM movies
WHERE title IN (
    'Dune: Part Two',
    'Inside Out 2',
    'The Wild Robot',
    'Mai',
    'Đào, Phở và Piano',
    'Godzilla x Kong: The New Empire'
);

DELETE FROM users
WHERE email IN (
    'an.nguyen@example.com',
    'binh.tran@example.com',
    'chi.le@example.com',
    'dung.pham@example.com',
    'ha.vo@example.com'
);
