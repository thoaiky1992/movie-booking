package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

type ExternalRatingRepository struct {
	db *sql.DB
}

func NewExternalRatingRepository(db *sql.DB) *ExternalRatingRepository {
	return &ExternalRatingRepository{db: db}
}

// RandomizeOne updates one randomly selected external rating. Generating the
// value in PostgreSQL keeps the selection and update atomic across workers.
func (r *ExternalRatingRepository) RandomizeOne(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE movie_external_ratings
		SET
			rating_score = ROUND((random() * max_score)::numeric, 2),
			last_synced_at = NOW()
		WHERE id = (
			SELECT id
			FROM movie_external_ratings
			ORDER BY random()
			LIMIT 1
		)`)
	if err != nil {
		return 0, fmt.Errorf("randomize external rating: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get randomized external rating count: %w", err)
	}
	return rowsAffected, nil
}
