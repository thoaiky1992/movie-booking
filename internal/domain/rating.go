package domain

import (
	"time"

	"github.com/google/uuid"
)

// RatingValue is one score from one source, normalised to its own scale
// (e.g. IMDb 8.8/10, Rotten Tomatoes 92/100).
type RatingValue struct {
	Source    string    `json:"source"`
	Value     float64   `json:"value"`
	Scale     float64   `json:"scale"`
	Votes     int64     `json:"votes,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

type RatingSnapshot struct {
	MovieID  uuid.UUID
	Ratings  []RatingValue
	SyncedAt time.Time
}

// MergeRatings overlays fresh values onto previously known ones, matched by
// Source. A partner that failed this round keeps its last good value instead
// of disappearing from the page.
func MergeRatings(previous, fresh []RatingValue) []RatingValue {
	out := make([]RatingValue, 0, len(previous)+len(fresh))
	index := make(map[string]int, len(previous)+len(fresh))
	for _, v := range previous {
		index[v.Source] = len(out)
		out = append(out, v)
	}
	for _, v := range fresh {
		if i, ok := index[v.Source]; ok {
			out[i] = v
			continue
		}
		index[v.Source] = len(out)
		out = append(out, v)
	}
	return out
}
