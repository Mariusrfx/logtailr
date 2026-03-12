package discovery

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const scanTimeout = 10 * time.Second

type DockerScanner struct{}

func NewDockerScanner() *DockerScanner {
	return &DockerScanner{}
}

func (s *DockerScanner) Name() string {
	return "docker"
}

func (s *DockerScanner) Scan() ScanResult {
	var result ScanResult

	if _, err := exec.LookPath("docker"); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("docker scanner: docker not found in PATH"))
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Names}}").Output()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("docker scanner: docker ps failed: %w", err))
		return result
	}

	names := parseDockerPsOutput(string(out))

	for _, name := range names {
		if !isSafeName(name) {
			result.Errors = append(result.Errors, fmt.Errorf("docker scanner: skipping container %q (invalid name)", name))
			continue
		}

		result.Sources = append(result.Sources, DiscoveredSource{
			Name:      "docker:" + name,
			Type:      "docker",
			Container: name,
		})
	}

	return result
}

func parseDockerPsOutput(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
