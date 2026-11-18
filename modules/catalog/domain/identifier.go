package domain

type CinemaID struct{ value int64 }

func NewCinemaID(value int64) (CinemaID, error) {
	if value <= 0 {
		return CinemaID{}, ErrInvalidCinemaID
	}
	return CinemaID{value: value}, nil
}

func (id CinemaID) Int64() int64 { return id.value }

type MovieID struct{ value int64 }

func NewMovieID(value int64) (MovieID, error) {
	if value <= 0 {
		return MovieID{}, ErrInvalidMovieID
	}
	return MovieID{value: value}, nil
}

func (id MovieID) Int64() int64 { return id.value }

type ShowtimeID struct{ value int64 }

func NewShowtimeID(value int64) (ShowtimeID, error) {
	if value <= 0 {
		return ShowtimeID{}, ErrInvalidShowtimeID
	}
	return ShowtimeID{value: value}, nil
}

func (id ShowtimeID) Int64() int64 { return id.value }
