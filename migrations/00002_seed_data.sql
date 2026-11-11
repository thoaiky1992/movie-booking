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

INSERT INTO movie_details (
    movie_id,
    synopsis,
    director,
    cast_members,
    genres,
    poster_url,
    trailer_url,
    release_date,
    age_rating,
    original_language
)
SELECT
    movies.id,
    details.synopsis,
    details.director,
    details.cast_members,
    details.genres,
    details.poster_url,
    details.trailer_url,
    details.release_date,
    details.age_rating,
    details.original_language
FROM (
    VALUES
        ('Dune: Part Two', 'Paul Atreides unites with Chani and the Fremen while seeking revenge against the conspirators who destroyed his family.', 'Denis Villeneuve', 'Timothée Chalamet, Zendaya, Rebecca Ferguson', 'Science Fiction, Adventure, Drama', 'https://example.com/posters/dune-part-two.jpg', 'https://example.com/trailers/dune-part-two', DATE '2024-03-01', 'PG-13', 'English'),
        ('Inside Out 2', 'Riley enters her teenage years as new emotions arrive at Headquarters and challenge the original team.', 'Kelsey Mann', 'Amy Poehler, Maya Hawke, Kensington Tallman', 'Animation, Family, Comedy', 'https://example.com/posters/inside-out-2.jpg', 'https://example.com/trailers/inside-out-2', DATE '2024-06-14', 'PG', 'English'),
        ('The Wild Robot', 'A shipwrecked robot adapts to life on an island and becomes the guardian of an orphaned gosling.', 'Chris Sanders', 'Lupita Nyong''o, Pedro Pascal, Kit Connor', 'Animation, Adventure, Family', 'https://example.com/posters/the-wild-robot.jpg', 'https://example.com/trailers/the-wild-robot', DATE '2024-09-27', 'PG', 'English'),
        ('Mai', 'A woman carrying the weight of her past finds hope and love after meeting her new neighbor.', 'Trấn Thành', 'Phương Anh Đào, Tuấn Trần, Trấn Thành', 'Drama, Romance', 'https://example.com/posters/mai.jpg', 'https://example.com/trailers/mai', DATE '2024-02-10', 'T18', 'Vietnamese'),
        ('Đào, Phở và Piano', 'A young couple attempts to reunite amid the final days of the 1946 battle for Hanoi.', 'Phi Tiến Sơn', 'Doãn Quốc Đam, Cao Thị Thùy Linh, Trần Lực', 'History, Drama, War', 'https://example.com/posters/dao-pho-va-piano.jpg', 'https://example.com/trailers/dao-pho-va-piano', DATE '2024-02-10', 'T13', 'Vietnamese'),
        ('Godzilla x Kong: The New Empire', 'Godzilla and Kong confront a colossal threat hidden within the Hollow Earth.', 'Adam Wingard', 'Rebecca Hall, Brian Tyree Henry, Dan Stevens', 'Action, Science Fiction, Adventure', 'https://example.com/posters/godzilla-x-kong.jpg', 'https://example.com/trailers/godzilla-x-kong', DATE '2024-03-29', 'PG-13', 'English')
) AS details(
    movie_title,
    synopsis,
    director,
    cast_members,
    genres,
    poster_url,
    trailer_url,
    release_date,
    age_rating,
    original_language
)
JOIN movies ON movies.title = details.movie_title;

INSERT INTO movie_external_ratings (
    movie_id,
    source,
    rating_score,
    max_score,
    vote_count,
    external_url
)
SELECT
    movies.id,
    ratings.source,
    ratings.rating_score,
    ratings.max_score,
    ratings.vote_count,
    ratings.external_url
FROM (
    VALUES
        ('Dune: Part Two', 'IMDb', 8.50, 10.00, 650000, 'https://www.imdb.com/find/?q=Dune%3A%20Part%20Two'),
        ('Dune: Part Two', 'Rotten Tomatoes', 92.00, 100.00, 500, 'https://www.rottentomatoes.com/search?search=Dune%20Part%20Two'),
        ('Inside Out 2', 'IMDb', 7.50, 10.00, 180000, 'https://www.imdb.com/find/?q=Inside%20Out%202'),
        ('Inside Out 2', 'Rotten Tomatoes', 91.00, 100.00, 320, 'https://www.rottentomatoes.com/search?search=Inside%20Out%202'),
        ('The Wild Robot', 'IMDb', 8.20, 10.00, 120000, 'https://www.imdb.com/find/?q=The%20Wild%20Robot'),
        ('The Wild Robot', 'Rotten Tomatoes', 96.00, 100.00, 240, 'https://www.rottentomatoes.com/search?search=The%20Wild%20Robot'),
        ('Mai', 'IMDb', 6.80, 10.00, 3500, 'https://www.imdb.com/find/?q=Mai%202024'),
        ('Mai', 'Google Users', 89.00, 100.00, NULL, 'https://www.google.com/search?q=Mai+2024+movie'),
        ('Đào, Phở và Piano', 'IMDb', 6.10, 10.00, 900, 'https://www.imdb.com/find/?q=Dao%20Pho%20va%20Piano'),
        ('Đào, Phở và Piano', 'Google Users', 86.00, 100.00, NULL, 'https://www.google.com/search?q=Dao+Pho+va+Piano'),
        ('Godzilla x Kong: The New Empire', 'IMDb', 6.10, 10.00, 115000, 'https://www.imdb.com/find/?q=Godzilla%20x%20Kong%20The%20New%20Empire'),
        ('Godzilla x Kong: The New Empire', 'Rotten Tomatoes', 54.00, 100.00, 310, 'https://www.rottentomatoes.com/search?search=Godzilla%20x%20Kong')
) AS ratings(movie_title, source, rating_score, max_score, vote_count, external_url)
JOIN movies ON movies.title = ratings.movie_title;

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
DELETE FROM movie_external_ratings
WHERE movie_id IN (
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

DELETE FROM movie_details
WHERE movie_id IN (
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
