package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// AuditRow represents a row in the audit_log table.
type AuditRow struct {
	ID         int64
	Ts         time.Time
	Action     string
	Resource   string
	ResourceID string
	RequestID  string
}

func (s *Store) RecordAudit(ctx context.Context, action, resource, resourceID, requestID string) error {
	_, err := s.q().Exec(ctx,
		`INSERT INTO audit_log (action, resource, resource_id, request_id)
		 VALUES ($1, $2, $3, $4)`,
		action, resource, resourceID, requestID)
	if err != nil {
		return fmt.Errorf("store: record audit: %w", err)
	}
	return nil
}

func (s *Store) ListAudit(ctx context.Context, limit, offset int) ([]AuditRow, error) {
	rows, err := s.q().Query(ctx,
		`SELECT id, ts, action, resource, resource_id, request_id
		 FROM audit_log ORDER BY ts DESC, id DESC
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: list audit: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[AuditRow])
}
