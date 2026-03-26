package cmd

import (
	"context"
	"fmt"
	"time"

	"logtailr/internal/config"
	"logtailr/internal/store"

	"github.com/spf13/cobra"
)

var importConfigPath string

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import YAML configuration into PostgreSQL",
	Long: `Import an existing YAML configuration file into the PostgreSQL database.

Uses upsert semantics: existing entries are updated, new ones are created.
This allows safe re-imports without duplicating data.`,
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVar(&importConfigPath, "config-file", "", "Path to YAML config file to import (required)")
	_ = importCmd.MarkFlagRequired("config-file")
}

func runImport(_ *cobra.Command, _ []string) error {
	dbURL, err := resolveDBURL()
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig(importConfigPath, true)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := config.ImportToStore(ctx, st, cfg); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Printf("Imported %d source(s), ", len(cfg.Sources))
	outputCount := 0
	if cfg.Outputs.OpenSearch != nil && cfg.Outputs.OpenSearch.Enabled {
		outputCount++
	}
	if cfg.Outputs.Webhook != nil && cfg.Outputs.Webhook.Enabled {
		outputCount++
	}
	if cfg.Outputs.File != nil {
		outputCount++
	}
	fmt.Printf("%d output(s)", outputCount)
	if cfg.Alerts != nil && cfg.Alerts.Enabled {
		fmt.Printf(", %d alert rule(s)", len(cfg.Alerts.Rules))
	}
	fmt.Println(" successfully.")
	return nil
}
