package bookmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

const (
	dirName  = ".logtailr"
	fileName = "bookmarks.json"
	dirPerm  = 0700
	filePerm = 0600
)

var (
	ErrBookmarkNotFound = errors.New("bookmark not found")
	namePattern         = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:@-]*$`)
)

type Bookmark struct {
	File    string    `json:"file"`
	Offset  int64     `json:"offset"`
	Inode   uint64    `json:"inode"`
	SavedAt time.Time `json:"saved_at"`
}

type Manager struct {
	path string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("bookmark: cannot determine home directory: %w", err)
	}
	return NewManagerWithDir(filepath.Join(home, dirName))
}

func NewManagerWithDir(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("bookmark: cannot create directory %q: %w", dir, err)
	}
	return &Manager{path: filepath.Join(dir, fileName)}, nil
}

func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("bookmark name cannot be empty")
	}
	if len(name) > 256 {
		return fmt.Errorf("bookmark name too long (max 256 chars)")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("bookmark name %q contains invalid characters", name)
	}
	return nil
}

func (m *Manager) Load(name string) (*Bookmark, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	all, err := m.readAll()
	if err != nil {
		return nil, err
	}

	bm, ok := all[name]
	if !ok {
		return nil, ErrBookmarkNotFound
	}
	return bm, nil
}

func (m *Manager) Save(name string, bm *Bookmark) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	all, err := m.readAll()
	if err != nil {
		return err
	}

	all[name] = bm
	return m.writeAll(all)
}

func (m *Manager) List() (map[string]*Bookmark, error) {
	return m.readAll()
}

func (m *Manager) Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	all, err := m.readAll()
	if err != nil {
		return err
	}

	delete(all, name)
	return m.writeAll(all)
}

func (m *Manager) readAll() (map[string]*Bookmark, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]*Bookmark), nil
		}
		return nil, fmt.Errorf("bookmark: read %q: %w", m.path, err)
	}

	var all map[string]*Bookmark
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("bookmark: parse %q: %w", m.path, err)
	}
	if all == nil {
		all = make(map[string]*Bookmark)
	}
	return all, nil
}

func (m *Manager) writeAll(all map[string]*Bookmark) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("bookmark: marshal: %w", err)
	}

	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return fmt.Errorf("bookmark: write %q: %w", tmp, err)
	}

	if err := os.Rename(tmp, m.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("bookmark: rename %q: %w", m.path, err)
	}
	return nil
}

func GetInode(path string) (uint64, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0, fmt.Errorf("bookmark: stat %q: %w", path, err)
	}
	return stat.Ino, nil
}
