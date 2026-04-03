# psync

A parallel file synchronisation tool inspired by rsync, with native Go parallelism and checkpoint/resume capability.

## Features

- Parallel transfers — each worker goroutine is pinned to a dedicated OS thread via `runtime.LockOSThread()`
- SHA-256 checksum comparison to skip identical files
- SSH/SFTP remote transfers (one connection per worker, no shelling out)
- Checkpoint/resume — atomic JSON state file, survives crashes
- Glob include/exclude filtering, size filtering, time filtering
- Real-time progress (per-file speed, ETA, aggregate throughput) or `--json-progress` for programmatic use
- `--dry-run` and `--delete` support
- Single static binary, no CGo

## Install

```sh
# From source (requires Go 1.22+)
git clone https://github.com/psync/psync
cd psync
make install          # installs to $GOPATH/bin/psync
```

## Usage

```
psync [flags] <source> <destination>
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--threads N` | `4` | Number of parallel worker goroutines |
| `--delete` | `false` | Remove destination files not present at source |
| `--dry-run` | `false` | Simulate without writing |
| `--quiet` | `false` | Suppress per-file output |
| `--json-progress` | `false` | Emit newline-delimited JSON progress events |
| `--include "pattern"` | | Include files matching glob (repeatable) |
| `--exclude "pattern"` | | Exclude files matching glob (repeatable) |
| `--min-size N` | `0` | Only sync files ≥ N bytes |
| `--max-size N` | `0` | Only sync files ≤ N bytes |
| `--newer-than <RFC3339>` | | Only sync files modified after timestamp |
| `--resume <file.json>` | | Resume from checkpoint file |
| `--checkpoint-dir <dir>` | `.` | Directory to write checkpoint files |
| `--ssh-key <path>` | | Path to SSH private key |
| `--ssh-port N` | `22` | SSH port |

### Examples

```sh
# Basic local sync, 8 threads
psync --threads 8 /data/source /data/destination

# Remote destination via SSH
psync --threads 4 --ssh-key ~/.ssh/id_rsa /local/data user@host:/remote/path

# Dry run with exclusions
psync --dry-run --exclude "*.tmp" --exclude ".git" /src /dst

# Resume an interrupted sync
psync --resume ./psync_checkpoint_20240315_143022.json

# Sync only files larger than 1 MB, modified in the last 24 h
psync --min-size 1048576 --newer-than 2024-03-14T00:00:00Z /src /dst

# Delete files at destination not in source, quiet mode
psync --delete --quiet /src /dst

# Emit JSON progress for pipeline integration
psync --json-progress /src /dst | jq .
```

## Checkpoint file format

```json
{
  "session_id": "<uuid>",
  "started_at": "<RFC3339>",
  "source": "/path/to/source",
  "destination": "/path/to/destination",
  "total_files": 1000,
  "completed": ["rel/path/file1.txt"],
  "failed": [{"path": "rel/path/bad.bin", "error": "permission denied"}],
  "pending": ["rel/path/file2.txt"]
}
```

Checkpoints are written atomically: the tool writes to `<name>.tmp` then renames it, so a crash during flush cannot corrupt the previous state.

## Development

```sh
make build   # build binary
make test    # run unit tests
make lint    # run golangci-lint
make clean   # remove binary
```

## Architecture

```
cmd/psync/main.go          – cobra CLI, orchestration, signal handling
internal/config/           – Config struct
internal/discovery/walk.go – recursive walker with glob/size/time filters
internal/checkpoint/       – JSON checkpoint read/write/resume
internal/transfer/
  file.go                  – local copy, SHA-256 checksum
  ssh.go                   – SSH dial, SFTP helpers
  worker.go                – worker pool (N goroutines, each LockOSThread'd)
  delta.go                 – placeholder for future block-level delta sync
internal/progress/         – terminal progress reporter + JSON emitter
```
