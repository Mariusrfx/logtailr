package cmd

import (
	"context"
	"fmt"
	"time"

	"logtailr/internal/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Apply or rollback PostgreSQL schema migrations for logtailr.`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE:  runMigrateUp,
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback all migrations",
	RunE:  runMigrateDown,
}

var migrateVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current migration version",
	RunE:  runMigrateVersion,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateVersionCmd)
}

func resolveDBURL() (string, error) {
	url := viper.GetString("database.url")
	if url == "" {
		return "", fmt.Errorf("database URL is required: use --db-url flag, LOGTAILR_DB_URL env var, or database.url in config")
	}
	return url, nil
}

func runMigrateUp(_ *cobra.Command, _ []string) error {
	dbURL, err := resolveDBURL()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.RunMigrations(dbURL); err != nil {
		return err
	}

	fmt.Println("Migrations applied successfully.")
	return nil
}

func runMigrateDown(_ *cobra.Command, _ []string) error {
	dbURL, err := resolveDBURL()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.MigrateDown(dbURL); err != nil {
		return err
	}

	fmt.Println("Migrations rolled back successfully.")
	return nil
}

func runMigrateVersion(_ *cobra.Command, _ []string) error {
	dbURL, err := resolveDBURL()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer s.Close()

	version, dirty, err := s.MigrateVersion(dbURL)
	if err != nil {
		return err
	}

	status := "clean"
	if dirty {
		status = "dirty"
	}
	fmt.Printf("Version: %d (%s)\n", version, status)
	return nil
}
