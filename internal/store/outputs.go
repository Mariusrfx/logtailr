package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// OutputRow represents a row in the outputs table.
type OutputRow struct {
	ID        pgtype.UUID
	Name      string
	Type      string
	Config    []byte // JSONB
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Store) ListOutputs(ctx context.Context) ([]OutputRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, type, config, enabled, created_at, updated_at
		 FROM outputs ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list outputs: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[OutputRow])
}

func (s *Store) GetOutputByID(ctx context.Context, id pgtype.UUID) (*OutputRow, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, name, type, config, enabled, created_at, updated_at
		 FROM outputs WHERE id = $1`, id)

	var out OutputRow
	if err := scanOutput(row, &out); err != nil {
		return nil, fmt.Errorf("store: get output by id: %w", err)
	}
	return &out, nil
}

func (s *Store) CreateOutput(ctx context.Context, out *OutputRow) error {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO outputs (name, type, config, enabled)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at, updated_at`,
		out.Name, out.Type, out.Config, out.Enabled,
	).Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create output: %w", err)
	}
	return nil
}

func (s *Store) UpdateOutput(ctx context.Context, out *OutputRow) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE outputs SET name=$2, type=$3, config=$4, enabled=$5
		 WHERE id = $1`,
		out.ID, out.Name, out.Type, out.Config, out.Enabled)
	if err != nil {
		return fmt.Errorf("store: update output: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: update output: not found")
	}
	return nil
}

func (s *Store) DeleteOutput(ctx context.Context, id pgtype.UUID) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM outputs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete output: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: delete output: not found")
	}
	return nil
}

func scanOutput(row pgx.Row, out *OutputRow) error {
	return row.Scan(
		&out.ID, &out.Name, &out.Type, &out.Config, &out.Enabled,
		&out.CreatedAt, &out.UpdatedAt,
	)
}
