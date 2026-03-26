package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// BookmarkRow represents a row in the bookmarks table.
type BookmarkRow struct {
	Name    string
	File    string
	Offset  int64
	Inode   int64
	SavedAt time.Time
}

func (s *Store) LoadBookmark(ctx context.Context, name string) (*BookmarkRow, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT name, file, "offset", inode, saved_at FROM bookmarks WHERE name = $1`, name)

	var bm BookmarkRow
	if err := row.Scan(&bm.Name, &bm.File, &bm.Offset, &bm.Inode, &bm.SavedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: load bookmark %q: %w", name, err)
	}
	return &bm, nil
}

func (s *Store) SaveBookmark(ctx context.Context, bm *BookmarkRow) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO bookmarks (name, file, "offset", inode, saved_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (name) DO UPDATE SET file=EXCLUDED.file, "offset"=EXCLUDED."offset",
		        inode=EXCLUDED.inode, saved_at=EXCLUDED.saved_at`,
		bm.Name, bm.File, bm.Offset, bm.Inode, bm.SavedAt)
	if err != nil {
		return fmt.Errorf("store: save bookmark %q: %w", bm.Name, err)
	}
	return nil
}

func (s *Store) ListBookmarks(ctx context.Context) ([]BookmarkRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT name, file, "offset", inode, saved_at FROM bookmarks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list bookmarks: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[BookmarkRow])
}

func (s *Store) DeleteBookmark(ctx context.Context, name string) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM bookmarks WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("store: delete bookmark %q: %w", name, err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("store: delete bookmark %q: not found", name)
	}
	return nil
}
