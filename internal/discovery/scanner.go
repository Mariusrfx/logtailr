package discovery

import "regexp"

// safeNamePattern matches names safe to pass to external commands.
// Reuses the same pattern as tailer.ValidateExternalName.
var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:@-]*$`)

func isSafeName(name string) bool {
	return name != "" && len(name) <= 256 && safeNamePattern.MatchString(name)
}

type DiscoveredSource struct {
	Name      string
	Type      string // "file", "docker", "journalctl"
	Path      string
	Container string
	Unit      string
}

type ScanResult struct {
	Sources []DiscoveredSource
	Errors  []error
}

type Scanner interface {
	Name() string
	Scan() ScanResult
}
