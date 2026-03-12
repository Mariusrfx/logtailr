package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"logtailr/internal/discovery"

	"github.com/spf13/cobra"
)

var (
	discoverOutput string
	discoverSave   string
	discoverScan   string
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Scan system for log sources and generate configuration",
	Long: `Scan for log files, Docker containers, and systemd services.

Generates a YAML configuration file compatible with logtailr tail --config.`,
	RunE: runDiscover,
}

func init() {
	rootCmd.AddCommand(discoverCmd)

	discoverCmd.Flags().StringVarP(&discoverOutput, "output", "o", "table", "Output format: table, yaml")
	discoverCmd.Flags().StringVar(&discoverSave, "save", "", "Path to save generated config (e.g. ./config.yaml)")
	discoverCmd.Flags().StringVar(&discoverScan, "scan", "all", "Scanners to run: all, file, docker, journalctl (comma-separated)")
}

func runDiscover(_ *cobra.Command, _ []string) error {
	scanners, err := buildScanners(discoverScan)
	if err != nil {
		return err
	}

	result := discovery.Discover(scanners)

	for _, e := range result.Errors {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}

	if len(result.Sources) == 0 {
		fmt.Println("No log sources found.")
		return nil
	}

	switch discoverOutput {
	case "yaml":
		fmt.Print(discovery.ToYAML(result.Sources))
	case "table":
		printDiscoveryTable(result.Sources)
	default:
		return fmt.Errorf("invalid output format %q (must be table or yaml)", discoverOutput)
	}

	if discoverSave != "" {
		if err := discovery.SaveConfig(discoverSave, result.Sources); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("\nConfiguration saved to %s\n", discoverSave)
	} else if discoverOutput == "table" {
		fmt.Printf("\nUse --save config.yaml to generate configuration file.\n")
	}

	return nil
}

func buildScanners(scan string) ([]discovery.Scanner, error) {
	var scanners []discovery.Scanner

	if scan == "all" {
		return []discovery.Scanner{
			discovery.NewFileScanner(),
			discovery.NewDockerScanner(),
			discovery.NewJournalctlScanner(),
		}, nil
	}

	parts := strings.Split(scan, ",")
	for _, part := range parts {
		switch strings.TrimSpace(part) {
		case "file":
			scanners = append(scanners, discovery.NewFileScanner())
		case "docker":
			scanners = append(scanners, discovery.NewDockerScanner())
		case "journalctl":
			scanners = append(scanners, discovery.NewJournalctlScanner())
		default:
			return nil, fmt.Errorf("unknown scanner %q (must be file, docker, or journalctl)", part)
		}
	}

	if len(scanners) == 0 {
		return nil, fmt.Errorf("no scanners specified")
	}

	return scanners, nil
}

func printDiscoveryTable(sources []discovery.DiscoveredSource) {
	fmt.Printf("Found %d potential log source(s):\n\n", len(sources))

	w := tabwriter.NewWriter(os.Stdout, 2, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "  TYPE\tNAME\tDETAIL")
	for _, src := range sources {
		detail := sourceDiscoveryDetail(src)
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", src.Type, src.Name, detail)
	}
	_ = w.Flush()
}

func sourceDiscoveryDetail(src discovery.DiscoveredSource) string {
	switch src.Type {
	case "file":
		return src.Path
	case "docker":
		return "container=" + src.Container
	case "journalctl":
		return "unit=" + src.Unit
	default:
		return ""
	}
}
