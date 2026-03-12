package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileScanner_FindsLogFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "app.log"), "some log content")
	writeFile(t, filepath.Join(dir, "error.log"), "error log content")
	writeFile(t, filepath.Join(dir, "readme.txt"), "not a log")

	scanner := NewFileScanner(dir)
	result := scanner.Scan()

	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(result.Sources))
	}

	names := make(map[string]bool)
	for _, src := range result.Sources {
		names[src.Path] = true
		if src.Type != "file" {
			t.Errorf("expected type 'file', got %q", src.Type)
		}
	}

	if !names[filepath.Join(dir, "app.log")] {
		t.Error("expected to find app.log")
	}
	if !names[filepath.Join(dir, "error.log")] {
		t.Error("expected to find error.log")
	}
}

func TestFileScanner_SkipsEmptyFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "empty.log"), "")
	writeFile(t, filepath.Join(dir, "nonempty.log"), "content")

	scanner := NewFileScanner(dir)
	result := scanner.Scan()

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 source (skipping empty), got %d", len(result.Sources))
	}
	if !strings.HasSuffix(result.Sources[0].Path, "nonempty.log") {
		t.Errorf("expected nonempty.log, got %q", result.Sources[0].Path)
	}
}

func TestFileScanner_SubdirectoryNaming(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "nginx")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(subDir, "access.log"), "log content")

	scanner := NewFileScanner(dir)
	result := scanner.Scan()

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(result.Sources))
	}

	if result.Sources[0].Name != "nginx-access" {
		t.Errorf("expected name 'nginx-access', got %q", result.Sources[0].Name)
	}
}

func TestFileScanner_NonExistentRoot(t *testing.T) {
	scanner := NewFileScanner("/nonexistent/path/12345")
	result := scanner.Scan()

	if len(result.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(result.Sources))
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors for non-existent path")
	}
}

func TestParseDockerPsOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "multiple containers",
			input:  "nginx\npostgres\nredis\n",
			expect: []string{"nginx", "postgres", "redis"},
		},
		{
			name:   "empty output",
			input:  "",
			expect: nil,
		},
		{
			name:   "trailing newlines",
			input:  "myapp\n\n",
			expect: []string{"myapp"},
		},
		{
			name:   "whitespace trimming",
			input:  "  myapp  \n  redis  \n",
			expect: []string{"myapp", "redis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDockerPsOutput(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("expected %d names, got %d: %v", len(tt.expect), len(got), got)
			}
			for i, name := range got {
				if name != tt.expect[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expect[i], name)
				}
			}
		})
	}
}

func TestParseSystemctlOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name: "typical output",
			input: `UNIT                    LOAD   ACTIVE SUB     DESCRIPTION
ssh.service             loaded active running OpenBSD Secure Shell server
nginx.service           loaded active running A high performance web server
cron.service            loaded active running Regular background program processing daemon

3 loaded units listed.`,
			expect: []string{"ssh.service", "nginx.service", "cron.service"},
		},
		{
			name:   "empty output",
			input:  "",
			expect: nil,
		},
		{
			name: "mixed unit types filtered",
			input: `ssh.service             loaded active running OpenBSD Secure Shell server
ssh.socket              loaded active listening SSH socket`,
			expect: []string{"ssh.service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSystemctlOutput(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("expected %d units, got %d: %v", len(tt.expect), len(got), got)
			}
			for i, unit := range got {
				if unit != tt.expect[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expect[i], unit)
				}
			}
		})
	}
}

func TestDiscover_MergeAndDedup(t *testing.T) {
	s1 := &mockScanner{sources: []DiscoveredSource{
		{Name: "app", Type: "file", Path: "/var/log/app.log"},
		{Name: "sys", Type: "file", Path: "/var/log/syslog"},
	}}
	s2 := &mockScanner{sources: []DiscoveredSource{
		{Name: "app", Type: "file", Path: "/var/log/app.log"}, // duplicate
		{Name: "nginx", Type: "docker", Container: "nginx"},
	}}

	result := Discover([]Scanner{s1, s2})

	if len(result.Sources) != 3 {
		t.Fatalf("expected 3 deduplicated sources, got %d", len(result.Sources))
	}
}

func TestDiscover_CollectsErrors(t *testing.T) {
	s := &mockScanner{
		sources: []DiscoveredSource{{Name: "a", Type: "file"}},
		errors:  []error{os.ErrPermission},
	}

	result := Discover([]Scanner{s})

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(result.Sources))
	}
}

func TestToYAML_GeneratesValidConfig(t *testing.T) {
	sources := []DiscoveredSource{
		{Name: "syslog", Type: "file", Path: "/var/log/syslog"},
		{Name: "docker:nginx", Type: "docker", Container: "nginx"},
		{Name: "journalctl:ssh.service", Type: "journalctl", Unit: "ssh.service"},
	}

	yaml := ToYAML(sources)

	if !strings.Contains(yaml, "global:") {
		t.Error("expected global section")
	}
	if !strings.Contains(yaml, "sources:") {
		t.Error("expected sources section")
	}
	if !strings.Contains(yaml, `type: "file"`) {
		t.Error("expected file type")
	}
	if !strings.Contains(yaml, `container: "nginx"`) {
		t.Error("expected container field")
	}
	if !strings.Contains(yaml, `unit: "ssh.service"`) {
		t.Error("expected unit field")
	}
	if !strings.Contains(yaml, "follow: true") {
		t.Error("expected follow: true")
	}
}

func TestSaveConfig_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	writeFile(t, path, "existing content")

	err := SaveConfig(path, []DiscoveredSource{{Name: "test", Type: "file"}})
	if err == nil {
		t.Fatal("expected error when file exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestSaveConfig_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	err := SaveConfig(path, []DiscoveredSource{
		{Name: "app", Type: "file", Path: "/var/log/app.log"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	if !strings.Contains(string(content), "sources:") {
		t.Error("saved config missing sources section")
	}
}

func TestIsSafeName(t *testing.T) {
	tests := []struct {
		name string
		safe bool
	}{
		{"nginx", true},
		{"my-app", true},
		{"ssh.service", true},
		{"app_v2", true},
		{"", false},
		{"../etc/passwd", false},
		{"name with spaces", false},
		{"a;rm -rf /", false},
	}

	for _, tt := range tests {
		if got := isSafeName(tt.name); got != tt.safe {
			t.Errorf("isSafeName(%q) = %v, want %v", tt.name, got, tt.safe)
		}
	}
}

func TestSanitizeDiscoveredName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"app", "app"},
		{"my app", "my-app"},
		{"file/path", "file-path"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		got := sanitizeDiscoveredName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeDiscoveredName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- helpers ---

type mockScanner struct {
	sources []DiscoveredSource
	errors  []error
}

func (m *mockScanner) Name() string { return "mock" }

func (m *mockScanner) Scan() ScanResult {
	return ScanResult{Sources: m.sources, Errors: m.errors}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}
