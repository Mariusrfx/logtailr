package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SourceRow represents a row in the sources table.
type SourceRow struct {
	ID            pgtype.UUID
	Name          string
	Type          string
	Path          string
	Container     string
	Unit          string
	Priority      string
	OutputFormat  string
	Namespace     string
	Pod           string
	LabelSelector string
	Kubeconfig    string
	Follow        bool
	Parser        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s *Store) ListSources(ctx context.Context) ([]SourceRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, type, path, container, unit, priority, output_format,
		        namespace, pod, label_selector, kubeconfig, follow, parser,
		        created_at, updated_at
		 FROM sources ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list sources: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[SourceRow])
}

func (s *Store) GetSourceByID(ctx context.Context, id pgtype.UUID) (*SourceRow, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, name, type, path, container, unit, priority, output_format,
		        namespace, pod, label_selector, kubeconfig, follow, parser,
		        created_at, updated_at
		 FROM sources WHERE id = $1`, id)

	var src SourceRow
	if err := scanSource(row, &src); err != nil {
		return nil, fmt.Errorf("store: get source by id: %w", err)
	}
	return &src, nil
}

func (s *Store) GetSourceByName(ctx context.Context, name string) (*SourceRow, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, name, type, path, container, unit, priority, output_format,
		        namespace, pod, label_selector, kubeconfig, follow, parser,
		        created_at, updated_at
		 FROM sources WHERE name = $1`, name)

	var src SourceRow
	if err := scanSource(row, &src); err != nil {
		return nil, fmt.Errorf("store: get source by name: %w", err)
	}
	return &src, nil
}

func (s *Store) CreateSource(ctx context.Context, src *SourceRow) error {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO sources (name, type, path, container, unit, priority, output_format,
		                      namespace, pod, label_selector, kubeconfig, follow, parser)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id, created_at, updated_at`,
		src.Name, src.Type, src.Path, src.Container, src.Unit, src.Priority, src.OutputFormat,
		src.Namespace, src.Pod, src.LabelSelector, src.Kubeconfig, src.Follow, src.Parser,
	).Scan(&src.ID, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create source: %w", err)
	}
	return nil
}

func (s *Store) UpdateSource(ctx context.Context, src *SourceRow) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE sources SET name=$2, type=$3, path=$4, container=$5, unit=$6, priority=$7,
		        output_format=$8, namespace=$9, pod=$10, label_selector=$11, kubeconfig=$12,
		        follow=$13, parser=$14
		 WHERE id = $1`,
		src.ID, src.Name, src.Type, src.Path, src.Container, src.Unit, src.Priority,
		src.OutputFormat, src.Namespace, src.Pod, src.LabelSelector, src.Kubeconfig,
		src.Follow, src.Parser)
	if err != nil {
		return fmt.Errorf("store: update source: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: update source: not found")
	}
	return nil
}

func (s *Store) DeleteSource(ctx context.Context, id pgtype.UUID) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete source: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: delete source: not found")
	}
	return nil
}

func scanSource(row pgx.Row, src *SourceRow) error {
	return row.Scan(
		&src.ID, &src.Name, &src.Type, &src.Path, &src.Container, &src.Unit,
		&src.Priority, &src.OutputFormat, &src.Namespace, &src.Pod, &src.LabelSelector,
		&src.Kubeconfig, &src.Follow, &src.Parser, &src.CreatedAt, &src.UpdatedAt,
	)
}
