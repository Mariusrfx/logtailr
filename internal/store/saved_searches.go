package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SavedSearchRow represents a row in the saved_searches table.
type SavedSearchRow struct {
	ID        pgtype.UUID
	Name      string
	Filters   []byte // JSONB
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Store) ListSavedSearches(ctx context.Context) ([]SavedSearchRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, filters, created_at, updated_at
		 FROM saved_searches ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list saved searches: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[SavedSearchRow])
}

func (s *Store) GetSavedSearchByID(ctx context.Context, id pgtype.UUID) (*SavedSearchRow, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, name, filters, created_at, updated_at
		 FROM saved_searches WHERE id = $1`, id)

	var ss SavedSearchRow
	if err := scanSavedSearch(row, &ss); err != nil {
		return nil, fmt.Errorf("store: get saved search by id: %w", err)
	}
	return &ss, nil
}

func (s *Store) CreateSavedSearch(ctx context.Context, ss *SavedSearchRow) error {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO saved_searches (name, filters)
		 VALUES ($1, $2)
		 RETURNING id, created_at, updated_at`,
		ss.Name, ss.Filters,
	).Scan(&ss.ID, &ss.CreatedAt, &ss.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create saved search: %w", err)
	}
	return nil
}

func (s *Store) UpdateSavedSearch(ctx context.Context, ss *SavedSearchRow) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE saved_searches SET name=$2, filters=$3
		 WHERE id = $1`,
		ss.ID, ss.Name, ss.Filters)
	if err != nil {
		return fmt.Errorf("store: update saved search: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: update saved search: not found")
	}
	return nil
}

func (s *Store) DeleteSavedSearch(ctx context.Context, id pgtype.UUID) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM saved_searches WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete saved search: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: delete saved search: not found")
	}
	return nil
}

func scanSavedSearch(row pgx.Row, ss *SavedSearchRow) error {
	return row.Scan(&ss.ID, &ss.Name, &ss.Filters, &ss.CreatedAt, &ss.UpdatedAt)
}
