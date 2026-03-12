package bookmark

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	m, err := NewManagerWithDir(dir)
	if err != nil {
		t.Fatalf("NewManagerWithDir: %v", err)
	}
	return m
}

func TestSave_And_Load(t *testing.T) {
	m := newTestManager(t)

	bm := &Bookmark{
		File:    "/var/log/app.log",
		Offset:  12345,
		Inode:   99999,
		SavedAt: time.Now().Truncate(time.Second),
	}

	if err := m.Save("myapp", bm); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.Load("myapp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.File != bm.File {
		t.Errorf("File = %q, want %q", loaded.File, bm.File)
	}
	if loaded.Offset != bm.Offset {
		t.Errorf("Offset = %d, want %d", loaded.Offset, bm.Offset)
	}
	if loaded.Inode != bm.Inode {
		t.Errorf("Inode = %d, want %d", loaded.Inode, bm.Inode)
	}
}

func TestLoad_NotFound(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Load("nonexistent")
	if !errors.Is(err, ErrBookmarkNotFound) {
		t.Errorf("expected ErrBookmarkNotFound, got %v", err)
	}
}

func TestSave_CreatesFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	m, err := NewManagerWithDir(dir)
	if err != nil {
		t.Fatalf("NewManagerWithDir: %v", err)
	}

	bm := &Bookmark{File: "/tmp/test.log", Offset: 100, Inode: 1}
	if err := m.Save("test", bm); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
		t.Errorf("bookmarks file not created: %v", err)
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	m := newTestManager(t)

	bm1 := &Bookmark{File: "/tmp/a.log", Offset: 100, Inode: 1}
	if err := m.Save("test", bm1); err != nil {
		t.Fatalf("Save 1: %v", err)
	}

	bm2 := &Bookmark{File: "/tmp/a.log", Offset: 500, Inode: 1}
	if err := m.Save("test", bm2); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	loaded, err := m.Load("test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Offset != 500 {
		t.Errorf("Offset = %d, want 500", loaded.Offset)
	}
}

func TestList(t *testing.T) {
	m := newTestManager(t)

	if err := m.Save("a", &Bookmark{File: "/a.log", Offset: 10, Inode: 1}); err != nil {
		t.Fatalf("Save a: %v", err)
	}
	if err := m.Save("b", &Bookmark{File: "/b.log", Offset: 20, Inode: 2}); err != nil {
		t.Fatalf("Save b: %v", err)
	}

	all, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len = %d, want 2", len(all))
	}
	if all["a"].Offset != 10 {
		t.Errorf("a.Offset = %d, want 10", all["a"].Offset)
	}
	if all["b"].Offset != 20 {
		t.Errorf("b.Offset = %d, want 20", all["b"].Offset)
	}
}

func TestDelete(t *testing.T) {
	m := newTestManager(t)

	if err := m.Save("test", &Bookmark{File: "/a.log", Offset: 10, Inode: 1}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := m.Delete("test"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := m.Load("test")
	if !errors.Is(err, ErrBookmarkNotFound) {
		t.Errorf("expected ErrBookmarkNotFound after delete, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	m := newTestManager(t)

	if err := m.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent should not error, got %v", err)
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"myapp", false},
		{"my-app", false},
		{"my_app.v2", false},
		{"app:prod", false},
		{"", true},
		{"-starts-with-dash", true},
		{"has spaces", true},
		{"has/slash", true},
		{"a" + string(make([]byte, 256)), true},
	}

	for _, tt := range tests {
		err := ValidateName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestGetInode(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	inode, err := GetInode(f.Name())
	if err != nil {
		t.Fatalf("GetInode: %v", err)
	}
	if inode == 0 {
		t.Error("expected non-zero inode")
	}
}
