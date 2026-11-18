package domain

import (
	"errors"
	"testing"
)

func TestIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		create  func(int64) (int64, error)
		wantErr error
	}{
		{
			name: "cinema",
			create: func(value int64) (int64, error) {
				id, err := NewCinemaID(value)
				return id.Int64(), err
			},
			wantErr: ErrInvalidCinemaID,
		},
		{
			name: "movie",
			create: func(value int64) (int64, error) {
				id, err := NewMovieID(value)
				return id.Int64(), err
			},
			wantErr: ErrInvalidMovieID,
		},
		{
			name: "showtime",
			create: func(value int64) (int64, error) {
				id, err := NewShowtimeID(value)
				return id.Int64(), err
			},
			wantErr: ErrInvalidShowtimeID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.create(12)
			if err != nil || got != 12 {
				t.Fatalf("create(12) = (%d, %v), want (12, nil)", got, err)
			}
			if _, err := test.create(0); !errors.Is(err, test.wantErr) {
				t.Fatalf("create(0) error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
