package discovery

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	maxFileSizeBytes = 1 << 30 // 1GB
)

type FileScanner struct {
	roots []string
}

func NewFileScanner(roots ...string) *FileScanner {
	if len(roots) == 0 {
		roots = []string{"/var/log"}
	}
	return &FileScanner{roots: roots}
}

func (s *FileScanner) Name() string {
	return "file"
}

func (s *FileScanner) Scan() ScanResult {
	var result ScanResult
	seen := make(map[string]bool)

	for _, root := range s.roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("file scanner: invalid path %q: %w", root, err))
			continue
		}

		err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("file scanner: %w", err))
				return nil
			}

			if d.IsDir() {
				return nil
			}

			if !d.Type().IsRegular() {
				return nil
			}

			if !strings.HasSuffix(d.Name(), ".log") {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("file scanner: stat %q: %w", path, err))
				return nil
			}

			if info.Size() == 0 {
				return nil
			}

			if info.Size() > maxFileSizeBytes {
				return nil
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil
			}

			if seen[absPath] {
				return nil
			}
			seen[absPath] = true

			name := deriveFileName(absPath)

			result.Sources = append(result.Sources, DiscoveredSource{
				Name: name,
				Type: "file",
				Path: absPath,
			})

			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("file scanner: walk %q: %w", absRoot, err))
		}
	}

	return result
}

func deriveFileName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		name = base
	}

	dir := filepath.Dir(path)
	parent := filepath.Base(dir)
	if parent != "." && parent != "/" && parent != "log" {
		name = parent + "-" + name
	}

	return sanitizeDiscoveredName(name)
}

func sanitizeDiscoveredName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	result := b.String()
	if result == "" {
		return "unknown"
	}
	return result
}
