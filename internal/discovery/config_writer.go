package discovery

import (
	"fmt"
	"os"
	"strings"
)

func ToYAML(sources []DiscoveredSource) string {
	var b strings.Builder

	b.WriteString("global:\n")
	b.WriteString("  level: \"info\"\n")
	b.WriteString("  output: \"console\"\n")
	b.WriteString("\n")
	b.WriteString("sources:\n")

	for _, src := range sources {
		b.WriteString(fmt.Sprintf("  - name: %q\n", src.Name))
		b.WriteString(fmt.Sprintf("    type: %q\n", src.Type))

		switch src.Type {
		case "file":
			b.WriteString(fmt.Sprintf("    path: %q\n", src.Path))
		case "docker":
			b.WriteString(fmt.Sprintf("    container: %q\n", src.Container))
		case "journalctl":
			b.WriteString(fmt.Sprintf("    unit: %q\n", src.Unit))
		}

		b.WriteString("    follow: true\n")
		b.WriteString("\n")
	}

	return b.String()
}

func SaveConfig(path string, sources []DiscoveredSource) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file %q already exists, will not overwrite", path)
	}

	content := ToYAML(sources)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
