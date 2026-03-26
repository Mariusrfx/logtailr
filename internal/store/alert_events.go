package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// AlertEventRow represents a row in the alert_events table.
type AlertEventRow struct {
	ID             pgtype.UUID
	RuleName       string
	Severity       string
	Message        string
	Source         string
	Count          int
	FiredAt        time.Time
	AcknowledgedAt *time.Time
}

// AlertEventFilter defines filtering and pagination options for listing alert events.
type AlertEventFilter struct {
	Severity string
	RuleName string
	Source   string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

func (s *Store) ListAlertEvents(ctx context.Context, f AlertEventFilter) ([]AlertEventRow, error) {
	var conditions []string
	var args []any
	argN := 1

	if f.Severity != "" {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argN))
		args = append(args, f.Severity)
		argN++
	}
	if f.RuleName != "" {
		conditions = append(conditions, fmt.Sprintf("rule_name = $%d", argN))
		args = append(args, f.RuleName)
		argN++
	}
	if f.Source != "" {
		conditions = append(conditions, fmt.Sprintf("source = $%d", argN))
		args = append(args, f.Source)
		argN++
	}
	if f.From != nil {
		conditions = append(conditions, fmt.Sprintf("fired_at >= $%d", argN))
		args = append(args, *f.From)
		argN++
	}
	if f.To != nil {
		conditions = append(conditions, fmt.Sprintf("fired_at <= $%d", argN))
		args = append(args, *f.To)
		argN++
	}

	query := `SELECT id, rule_name, severity, message, source, count, fired_at, acknowledged_at
	          FROM alert_events`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY fired_at DESC"

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argN, argN+1)
	args = append(args, limit, f.Offset)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list alert events: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[AlertEventRow])
}

func (s *Store) CreateAlertEvent(ctx context.Context, e *AlertEventRow) error {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO alert_events (rule_name, severity, message, source, count, fired_at)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id`,
		e.RuleName, e.Severity, e.Message, e.Source, e.Count, e.FiredAt,
	).Scan(&e.ID)
	if err != nil {
		return fmt.Errorf("store: create alert event: %w", err)
	}
	return nil
}

func (s *Store) AcknowledgeAlertEvent(ctx context.Context, id pgtype.UUID) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE alert_events SET acknowledged_at = now() WHERE id = $1 AND acknowledged_at IS NULL`,
		id)
	if err != nil {
		return fmt.Errorf("store: acknowledge alert event: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: acknowledge alert event: not found or already acknowledged")
	}
	return nil
}

func (s *Store) DeleteAlertEventsOlderThan(ctx context.Context, before time.Time) (int64, error) {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM alert_events WHERE fired_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("store: delete old alert events: %w", err)
	}
	return ct.RowsAffected(), nil
}
