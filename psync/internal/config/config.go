package config

import (
	"time"
)

// Config holds all parsed CLI options for a psync invocation.
type Config struct {
	// Positional
	Source      string
	Destination string

	// Core
	Threads       int
	Delete        bool
	DryRun        bool
	Quiet         bool
	JSONProgress  bool

	// Filtering
	Include   []string
	Exclude   []string
	MinSize   int64
	MaxSize   int64
	NewerThan time.Time

	// Checkpoint
	Resume        string
	CheckpointDir string

	// SSH
	SSHKey  string
	SSHPort int
}
