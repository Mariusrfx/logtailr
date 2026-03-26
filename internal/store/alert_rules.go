package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// AlertRuleRow represents a row in the alert_rules table.
type AlertRuleRow struct {
	ID        pgtype.UUID
	Name      string
	Type      string
	Severity  string
	Pattern   string
	Level     string
	Source    string
	Threshold int
	Window    string
	Cooldown  string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Store) ListAlertRules(ctx context.Context) ([]AlertRuleRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, type, severity, pattern, level, source, threshold,
		        window, cooldown, enabled, created_at, updated_at
		 FROM alert_rules ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list alert rules: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[AlertRuleRow])
}

func (s *Store) GetAlertRuleByID(ctx context.Context, id pgtype.UUID) (*AlertRuleRow, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, name, type, severity, pattern, level, source, threshold,
		        window, cooldown, enabled, created_at, updated_at
		 FROM alert_rules WHERE id = $1`, id)

	var r AlertRuleRow
	if err := scanAlertRule(row, &r); err != nil {
		return nil, fmt.Errorf("store: get alert rule by id: %w", err)
	}
	return &r, nil
}

func (s *Store) CreateAlertRule(ctx context.Context, r *AlertRuleRow) error {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO alert_rules (name, type, severity, pattern, level, source, threshold,
		                          window, cooldown, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, created_at, updated_at`,
		r.Name, r.Type, r.Severity, r.Pattern, r.Level, r.Source, r.Threshold,
		r.Window, r.Cooldown, r.Enabled,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create alert rule: %w", err)
	}
	return nil
}

func (s *Store) UpdateAlertRule(ctx context.Context, r *AlertRuleRow) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE alert_rules SET name=$2, type=$3, severity=$4, pattern=$5, level=$6,
		        source=$7, threshold=$8, window=$9, cooldown=$10, enabled=$11
		 WHERE id = $1`,
		r.ID, r.Name, r.Type, r.Severity, r.Pattern, r.Level,
		r.Source, r.Threshold, r.Window, r.Cooldown, r.Enabled)
	if err != nil {
		return fmt.Errorf("store: update alert rule: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: update alert rule: not found")
	}
	return nil
}

func (s *Store) DeleteAlertRule(ctx context.Context, id pgtype.UUID) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete alert rule: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: delete alert rule: not found")
	}
	return nil
}

func scanAlertRule(row pgx.Row, r *AlertRuleRow) error {
	return row.Scan(
		&r.ID, &r.Name, &r.Type, &r.Severity, &r.Pattern, &r.Level,
		&r.Source, &r.Threshold, &r.Window, &r.Cooldown, &r.Enabled,
		&r.CreatedAt, &r.UpdatedAt,
	)
}
