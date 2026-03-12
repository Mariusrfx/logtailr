package discovery

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type JournalctlScanner struct{}

func NewJournalctlScanner() *JournalctlScanner {
	return &JournalctlScanner{}
}

func (s *JournalctlScanner) Name() string {
	return "journalctl"
}

func (s *JournalctlScanner) Scan() ScanResult {
	var result ScanResult

	if _, err := exec.LookPath("systemctl"); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("journalctl scanner: systemctl not found in PATH"))
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "systemctl", "list-units",
		"--type=service", "--state=running", "--no-pager", "--plain").Output()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("journalctl scanner: systemctl failed: %w", err))
		return result
	}

	units := parseSystemctlOutput(string(out))

	for _, unit := range units {
		if !isSafeName(unit) {
			result.Errors = append(result.Errors, fmt.Errorf("journalctl scanner: skipping unit %q (invalid name)", unit))
			continue
		}

		result.Sources = append(result.Sources, DiscoveredSource{
			Name: "journalctl:" + unit,
			Type: "journalctl",
			Unit: unit,
		})
	}

	return result
}

func parseSystemctlOutput(output string) []string {
	var units []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header/footer lines
		if strings.HasPrefix(line, "UNIT") || strings.HasPrefix(line, "LOAD") {
			continue
		}
		// Footer line typically starts with a digit or contains "loaded units listed"
		if strings.Contains(line, "loaded units listed") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		unit := fields[0]
		if !strings.HasSuffix(unit, ".service") {
			continue
		}

		units = append(units, unit)
	}
	return units
}
