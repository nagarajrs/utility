package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FailedFile records a file that failed to transfer.
type FailedFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// State represents the full checkpoint state for a sync session.
type State struct {
	SessionID   string       `json:"session_id"`
	StartedAt   time.Time    `json:"started_at"`
	Source      string       `json:"source"`
	Destination string       `json:"destination"`
	TotalFiles  int          `json:"total_files"`
	Completed   []string     `json:"completed"`
	Failed      []FailedFile `json:"failed"`
	Pending     []string     `json:"pending"`

	mu       sync.Mutex
	filePath string
}

// New creates a new checkpoint State and writes the initial file.
func New(dir, source, destination string) (*State, error) {
	sessionID := uuid.New().String()
	ts := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("psync_checkpoint_%s.json", ts)
	filePath := filepath.Join(dir, filename)

	s := &State{
		SessionID:   sessionID,
		StartedAt:   time.Now().UTC(),
		Source:      source,
		Destination: destination,
		Completed:   []string{},
		Failed:      []FailedFile{},
		Pending:     []string{},
		filePath:    filePath,
	}
	if err := s.flush(); err != nil {
		return nil, fmt.Errorf("create checkpoint: %w", err)
	}
	return s, nil
}

// Load reads an existing checkpoint file.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read checkpoint %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse checkpoint %s: %w", path, err)
	}
	s.filePath = path
	return &s, nil
}

// FilePath returns the path of the checkpoint file.
func (s *State) FilePath() string {
	return s.filePath
}

// SetPending initialises the pending list and total count.
func (s *State) SetPending(files []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Pending = files
	s.TotalFiles = len(files)
}

// MarkCompleted moves a file from pending to completed and flushes.
func (s *State) MarkCompleted(rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Completed = append(s.Completed, rel)
	s.removePending(rel)
	return s.flush()
}

// MarkFailed records a file failure and flushes.
func (s *State) MarkFailed(rel, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Failed = append(s.Failed, FailedFile{Path: rel, Error: errMsg})
	s.removePending(rel)
	return s.flush()
}

// CompletedSet returns a set of completed file paths for fast lookup.
func (s *State) CompletedSet() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := make(map[string]bool, len(s.Completed))
	for _, p := range s.Completed {
		m[p] = true
	}
	return m
}

// removePending removes rel from the pending list (caller must hold lock).
func (s *State) removePending(rel string) {
	for i, p := range s.Pending {
		if p == rel {
			s.Pending = append(s.Pending[:i], s.Pending[i+1:]...)
			return
		}
	}
}

// flush writes state atomically (caller must hold lock).
func (s *State) flush() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

// Flush is a public, thread-safe flush (e.g. on SIGINT).
func (s *State) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}
