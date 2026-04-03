package discovery_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/psync/psync/internal/discovery"
)

func mkFile(t *testing.T, dir, rel string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkBasic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", []byte("hello"))
	mkFile(t, dir, "sub/b.log", []byte("world"))
	mkFile(t, dir, ".git/config", []byte("git"))

	entries, err := discovery.Walk(dir, discovery.Options{})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestWalkExclude(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", []byte("keep"))
	mkFile(t, dir, "a.tmp", []byte("exclude"))
	mkFile(t, dir, ".git/config", []byte("exclude"))

	entries, err := discovery.Walk(dir, discovery.Options{
		Exclude: []string{"*.tmp", ".git"},
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	for _, e := range entries {
		if e.RelPath == "a.tmp" || e.RelPath == ".git/config" {
			t.Errorf("excluded file appeared: %s", e.RelPath)
		}
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d: %v", len(entries), entries)
	}
}

func TestWalkInclude(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", []byte("include"))
	mkFile(t, dir, "b.go", []byte("include"))
	mkFile(t, dir, "c.log", []byte("exclude"))

	entries, err := discovery.Walk(dir, discovery.Options{
		Include: []string{"*.txt", "*.go"},
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestWalkMinMaxSize(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "small.txt", []byte("hi"))           // 2 bytes
	mkFile(t, dir, "medium.txt", []byte("hello world")) // 11 bytes
	mkFile(t, dir, "large.txt", make([]byte, 100))      // 100 bytes

	entries, err := discovery.Walk(dir, discovery.Options{MinSize: 5, MaxSize: 50})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(entries) != 1 || entries[0].RelPath != "medium.txt" {
		t.Errorf("expected only medium.txt, got %v", entries)
	}
}

func TestWalkNewerThan(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Back-date the old file.
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	mkFile(t, dir, "new.txt", []byte("new"))

	cutoff := time.Now().Add(-1 * time.Hour)
	entries, err := discovery.Walk(dir, discovery.Options{NewerThan: cutoff})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(entries) != 1 || entries[0].RelPath != "new.txt" {
		t.Errorf("expected only new.txt, got %v", entries)
	}
}
