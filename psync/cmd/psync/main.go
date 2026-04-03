package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/psync/psync/internal/checkpoint"
	"github.com/psync/psync/internal/config"
	"github.com/psync/psync/internal/discovery"
	"github.com/psync/psync/internal/progress"
	"github.com/psync/psync/internal/transfer"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var cfg config.Config
	var newerThanStr string

	cmd := &cobra.Command{
		Use:   "psync [flags] <source> <destination>",
		Short: "psync – parallel file synchronisation tool",
		Long: `psync synchronises files from source to destination using parallel
worker goroutines, each pinned to a dedicated OS thread.

Supports local→local, local→remote (SSH/SFTP), and checkpoint/resume.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cfg.Resume != "" {
				// Resume mode: no positional args required.
				return nil
			}
			if len(args) != 2 {
				return fmt.Errorf("requires exactly 2 arguments: <source> <destination>")
			}
			cfg.Source = args[0]
			cfg.Destination = args[1]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if newerThanStr != "" {
				t, err := time.Parse(time.RFC3339, newerThanStr)
				if err != nil {
					return fmt.Errorf("--newer-than: %w", err)
				}
				cfg.NewerThan = t
			}
			return run(&cfg)
		},
	}

	f := cmd.Flags()

	// Core flags.
	f.IntVar(&cfg.Threads, "threads", 4, "number of parallel worker goroutines")
	f.BoolVar(&cfg.Delete, "delete", false, "delete files at destination not present at source")
	f.BoolVar(&cfg.DryRun, "dry-run", false, "simulate without writing anything")
	f.BoolVar(&cfg.Quiet, "quiet", false, "suppress per-file output")
	f.BoolVar(&cfg.JSONProgress, "json-progress", false, "emit progress as newline-delimited JSON")

	// Filtering.
	f.StringArrayVar(&cfg.Include, "include", nil, "include files matching glob pattern (repeatable)")
	f.StringArrayVar(&cfg.Exclude, "exclude", nil, "exclude files matching glob pattern (repeatable)")
	f.Int64Var(&cfg.MinSize, "min-size", 0, "only sync files >= N bytes")
	f.Int64Var(&cfg.MaxSize, "max-size", 0, "only sync files <= N bytes")
	f.StringVar(&newerThanStr, "newer-than", "", "only sync files modified after RFC3339 timestamp")

	// Checkpoint.
	f.StringVar(&cfg.Resume, "resume", "", "resume from checkpoint file")
	f.StringVar(&cfg.CheckpointDir, "checkpoint-dir", ".", "directory to write checkpoint files")

	// SSH.
	f.StringVar(&cfg.SSHKey, "ssh-key", "", "path to SSH private key")
	f.IntVar(&cfg.SSHPort, "ssh-port", 22, "SSH port")

	return cmd
}

func run(cfg *config.Config) error {
	var ckpt *checkpoint.State
	var files []discovery.FileEntry

	if cfg.Resume != "" {
		// --- Resume mode ---
		loaded, err := checkpoint.Load(cfg.Resume)
		if err != nil {
			return fmt.Errorf("load checkpoint: %w", err)
		}
		ckpt = loaded
		cfg.Source = ckpt.Source
		cfg.Destination = ckpt.Destination

		completedSet := ckpt.CompletedSet()

		// Re-walk source to rebuild pending list (filters may differ, but keep consistent).
		all, err := walkSource(cfg)
		if err != nil {
			return err
		}
		for _, e := range all {
			if !completedSet[e.RelPath] {
				files = append(files, e)
			}
		}
		fmt.Fprintf(os.Stdout, "Resuming session %s — %d files remaining\n",
			ckpt.SessionID, len(files))
	} else {
		// --- Fresh sync ---
		var err error
		files, err = walkSource(cfg)
		if err != nil {
			return err
		}

		ckpt, err = checkpoint.New(cfg.CheckpointDir, cfg.Source, cfg.Destination)
		if err != nil {
			return fmt.Errorf("create checkpoint: %w", err)
		}

		relPaths := make([]string, len(files))
		for i, e := range files {
			relPaths[i] = e.RelPath
		}
		ckpt.SetPending(relPaths)
		fmt.Fprintf(os.Stdout, "Session %s — checkpoint: %s\n",
			ckpt.SessionID, ckpt.FilePath())
		fmt.Fprintf(os.Stdout, "Syncing %d files from %s → %s\n",
			len(files), cfg.Source, cfg.Destination)
	}

	if cfg.DryRun {
		fmt.Fprintln(os.Stdout, "[DRY RUN] no files will be written")
	}

	// Set up graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\nReceived %s — flushing checkpoint and exiting...\n", sig)
		if ckpt != nil {
			_ = ckpt.Flush()
		}
		os.Exit(1)
	}()

	// Handle --delete: collect destination files not in source.
	if cfg.Delete && !cfg.DryRun && !isRemote(cfg.Destination) {
		if err := deleteOrphans(cfg, files); err != nil {
			fmt.Fprintf(os.Stderr, "psync: delete orphans: %v\n", err)
		}
	}

	// Build reporter and start tick goroutine.
	rep := progress.New(len(files), cfg.Threads, cfg.Quiet, cfg.JSONProgress)

	tickDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rep.Tick()
			case <-tickDone:
				return
			}
		}
	}()

	pool := transfer.NewPool(cfg, rep, ckpt)
	pool.Run(files)

	close(tickDone)
	rep.Summary()
	return nil
}

// walkSource walks the source directory applying filters from cfg.
func walkSource(cfg *config.Config) ([]discovery.FileEntry, error) {
	opts := discovery.Options{
		Include:   cfg.Include,
		Exclude:   cfg.Exclude,
		MinSize:   cfg.MinSize,
		MaxSize:   cfg.MaxSize,
		NewerThan: cfg.NewerThan,
	}
	entries, err := discovery.Walk(cfg.Source, opts)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.Source, err)
	}
	return entries, nil
}

// deleteOrphans removes files in destination that are not in source.
func deleteOrphans(cfg *config.Config, sourceFiles []discovery.FileEntry) error {
	srcSet := make(map[string]bool, len(sourceFiles))
	for _, e := range sourceFiles {
		srcSet[e.RelPath] = true
	}

	return filepath.WalkDir(cfg.Destination, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(cfg.Destination, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !srcSet[rel] {
			fmt.Fprintf(os.Stdout, "[DELETE] %s\n", rel)
			return os.Remove(path)
		}
		return nil
	})
}

func isRemote(path string) bool {
	return strings.Contains(path, "@") && strings.Contains(path, ":")
}
