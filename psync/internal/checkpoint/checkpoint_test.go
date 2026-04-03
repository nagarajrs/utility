package checkpoint_test

import (
	"os"
	"testing"

	"github.com/psync/psync/internal/checkpoint"
)

func TestNewAndLoad(t *testing.T) {
	dir := t.TempDir()

	s, err := checkpoint.New(dir, "/src", "/dst")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if s.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	files := []string{"a/b.txt", "c.txt", "d/e/f.txt"}
	s.SetPending(files)

	if err := s.MarkCompleted("a/b.txt"); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if err := s.MarkFailed("c.txt", "permission denied"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	// Checkpoint file must exist.
	if _, err := os.Stat(s.FilePath()); err != nil {
		t.Fatalf("checkpoint file missing: %v", err)
	}

	// Load the checkpoint back.
	loaded, err := checkpoint.Load(s.FilePath())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.SessionID != s.SessionID {
		t.Errorf("session ID mismatch: %s vs %s", loaded.SessionID, s.SessionID)
	}
	if loaded.TotalFiles != 3 {
		t.Errorf("expected TotalFiles=3, got %d", loaded.TotalFiles)
	}
	if len(loaded.Completed) != 1 || loaded.Completed[0] != "a/b.txt" {
		t.Errorf("unexpected Completed: %v", loaded.Completed)
	}
	if len(loaded.Failed) != 1 || loaded.Failed[0].Path != "c.txt" {
		t.Errorf("unexpected Failed: %v", loaded.Failed)
	}
	if len(loaded.Pending) != 1 || loaded.Pending[0] != "d/e/f.txt" {
		t.Errorf("unexpected Pending: %v", loaded.Pending)
	}
}

func TestCompletedSet(t *testing.T) {
	dir := t.TempDir()
	s, _ := checkpoint.New(dir, "/src", "/dst")
	s.SetPending([]string{"x.txt", "y.txt"})
	_ = s.MarkCompleted("x.txt")

	set := s.CompletedSet()
	if !set["x.txt"] {
		t.Error("x.txt should be in completed set")
	}
	if set["y.txt"] {
		t.Error("y.txt should not be in completed set")
	}
}
