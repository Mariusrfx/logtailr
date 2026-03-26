package store

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store holds the database connection pool and provides access to all sub-stores.
type Store struct {
	Pool *pgxpool.Pool
}

// New creates a new Store with a connection pool to the given PostgreSQL URL.
// The URL should be in the format: postgres://user:pass@host:port/dbname?sslmode=disable
func New(ctx context.Context, dbURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("store: invalid database URL: %w", err)
	}

	cfg.MinConns = 2
	cfg.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: cannot create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: cannot connect to database: %w", err)
	}

	return &Store{Pool: pool}, nil
}

// Close releases all database connections.
func (s *Store) Close() {
	s.Pool.Close()
}

// RunMigrations applies all pending up migrations.
func (s *Store) RunMigrations(dbURL string) error {
	m, err := newMigrate(dbURL)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("store: migration up failed: %w", err)
	}
	return nil
}

// MigrateDown rolls back all migrations.
func (s *Store) MigrateDown(dbURL string) error {
	m, err := newMigrate(dbURL)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("store: migration down failed: %w", err)
	}
	return nil
}

// MigrateVersion returns the current migration version and dirty state.
func (s *Store) MigrateVersion(dbURL string) (uint, bool, error) {
	m, err := newMigrate(dbURL)
	if err != nil {
		return 0, false, err
	}
	defer closeMigrate(m)

	version, dirty, err := m.Version()
	if err != nil {
		return 0, false, fmt.Errorf("store: cannot get migration version: %w", err)
	}
	return version, dirty, nil
}

func closeMigrate(m *migrate.Migrate) {
	srcErr, dbErr := m.Close()
	_ = srcErr
	_ = dbErr
}

// newMigrate creates a golang-migrate instance with embedded SQL files.
// The dbURL is converted to use the pgx5 driver scheme.
func newMigrate(dbURL string) (*migrate.Migrate, error) {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("store: cannot open embedded migrations: %w", err)
	}

	// golang-migrate expects "pgx5://" scheme for pgx/v5 driver
	migrateURL := convertToPgx5Scheme(dbURL)

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, migrateURL)
	if err != nil {
		return nil, fmt.Errorf("store: cannot init migrator: %w", err)
	}
	return m, nil
}

// convertToPgx5Scheme replaces "postgres://" or "postgresql://" with "pgx5://"
// as required by the golang-migrate pgx/v5 database driver.
func convertToPgx5Scheme(dbURL string) string {
	if len(dbURL) >= 13 && dbURL[:13] == "postgresql://" {
		return "pgx5://" + dbURL[13:]
	}
	if len(dbURL) >= 11 && dbURL[:11] == "postgres://" {
		return "pgx5://" + dbURL[11:]
	}
	return dbURL
}
