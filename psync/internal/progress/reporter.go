package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Reporter tracks and displays sync progress.
type Reporter struct {
	total        int
	transferred  atomic.Int64 // bytes
	filesDone    atomic.Int64 // files completed
	filesSkipped atomic.Int64 // files skipped (identical)
	filesFailed  atomic.Int64

	startTime time.Time
	quiet     bool
	jsonMode  bool
	out       io.Writer

	mu           sync.Mutex
	workerStatus []string // current file per worker
}

// JSONEvent is emitted in --json-progress mode.
type JSONEvent struct {
	Event       string  `json:"event"`
	WorkerID    int     `json:"worker_id,omitempty"`
	File        string  `json:"file,omitempty"`
	SizeBytes   int64   `json:"size_bytes,omitempty"`
	SpeedMBs    float64 `json:"speed_mb_s,omitempty"`
	FilesDone   int64   `json:"files_done,omitempty"`
	FilesTotal  int     `json:"files_total,omitempty"`
	BytesDone   int64   `json:"bytes_done,omitempty"`
	ElapsedSec  float64 `json:"elapsed_sec,omitempty"`
	ETASec      float64 `json:"eta_sec,omitempty"`
}

// New creates a Reporter.
func New(total, threads int, quiet, jsonMode bool) *Reporter {
	return &Reporter{
		total:        total,
		startTime:    time.Now(),
		quiet:        quiet,
		jsonMode:     jsonMode,
		out:          os.Stdout,
		workerStatus: make([]string, threads),
	}
}

// StartFile records that a worker has begun transferring a file.
func (r *Reporter) StartFile(workerID int, rel string, size int64) {
	if r.jsonMode {
		r.emitJSON(JSONEvent{
			Event:    "start",
			WorkerID: workerID,
			File:     rel,
			SizeBytes: size,
		})
		return
	}
	if r.quiet {
		return
	}
	r.mu.Lock()
	if workerID < len(r.workerStatus) {
		r.workerStatus[workerID] = fmt.Sprintf("[W%d] %s (%s)", workerID, rel, humanBytes(size))
	}
	r.mu.Unlock()
}

// CompleteFile records a completed file transfer and updates stats.
func (r *Reporter) CompleteFile(workerID int, rel string, size int64, elapsed time.Duration, skipped bool) {
	if skipped {
		r.filesSkipped.Add(1)
	} else {
		r.filesDone.Add(1)
		r.transferred.Add(size)
	}

	if r.jsonMode {
		var speedMBs float64
		if elapsed.Seconds() > 0 {
			speedMBs = float64(size) / elapsed.Seconds() / 1e6
		}
		done := r.filesDone.Load() + r.filesSkipped.Load()
		r.emitJSON(JSONEvent{
			Event:      "done",
			WorkerID:   workerID,
			File:       rel,
			SizeBytes:  size,
			SpeedMBs:   speedMBs,
			FilesDone:  done,
			FilesTotal: r.total,
			BytesDone:  r.transferred.Load(),
			ElapsedSec: time.Since(r.startTime).Seconds(),
			ETASec:     r.eta(done),
		})
		return
	}
	if r.quiet {
		return
	}

	var speedMBs float64
	if elapsed.Seconds() > 0 {
		speedMBs = float64(size) / elapsed.Seconds() / 1e6
	}
	done := r.filesDone.Load() + r.filesSkipped.Load()
	tag := "XFER"
	if skipped {
		tag = "SKIP"
	}
	fmt.Fprintf(r.out, "\r%-80s\r[%s] [%d/%d] %s (%.2f MB/s)\n",
		"", tag, done, r.total, rel, speedMBs)
}

// FailFile records a failed file.
func (r *Reporter) FailFile(workerID int, rel string, err error) {
	r.filesFailed.Add(1)
	if r.jsonMode {
		r.emitJSON(JSONEvent{Event: "fail", WorkerID: workerID, File: rel})
		return
	}
	fmt.Fprintf(r.out, "\r%-80s\r[FAIL] %s: %v\n", "", rel, err)
}

// Tick prints the aggregate status line (call periodically from a goroutine).
func (r *Reporter) Tick() {
	if r.quiet || r.jsonMode {
		return
	}
	done := r.filesDone.Load() + r.filesSkipped.Load()
	elapsed := time.Since(r.startTime)
	transferred := r.transferred.Load()
	speed := float64(transferred) / elapsed.Seconds() / 1e6

	eta := r.eta(done)
	etaStr := "?"
	if eta >= 0 {
		etaStr = fmt.Sprintf("%.0fs", eta)
	}

	r.mu.Lock()
	active := make([]string, 0, len(r.workerStatus))
	for _, s := range r.workerStatus {
		if s != "" {
			active = append(active, s)
		}
	}
	r.mu.Unlock()

	line := fmt.Sprintf("[%d/%d files] [%s] [%.2f MB/s] [elapsed %s] [ETA %s]",
		done, r.total, humanBytes(transferred), speed, elapsed.Round(time.Second), etaStr)
	if len(active) > 0 {
		line += " | " + strings.Join(active, " | ")
	}
	// Truncate to avoid wrapping.
	if len(line) > 200 {
		line = line[:200]
	}
	fmt.Fprintf(r.out, "\r%-200s", line)
}

// Summary prints the final summary.
func (r *Reporter) Summary() {
	elapsed := time.Since(r.startTime)
	done := r.filesDone.Load()
	skipped := r.filesSkipped.Load()
	failed := r.filesFailed.Load()
	bytes := r.transferred.Load()

	if r.jsonMode {
		r.emitJSON(JSONEvent{
			Event:      "summary",
			FilesDone:  done,
			FilesTotal: r.total,
			BytesDone:  bytes,
			ElapsedSec: elapsed.Seconds(),
		})
		return
	}
	fmt.Fprintf(r.out, "\r%-200s\r", "") // clear tick line
	fmt.Fprintf(r.out, "\nSync complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(r.out, "  Transferred : %d files (%s)\n", done, humanBytes(bytes))
	fmt.Fprintf(r.out, "  Skipped     : %d files (identical)\n", skipped)
	fmt.Fprintf(r.out, "  Failed      : %d files\n", failed)
}

func (r *Reporter) eta(done int64) float64 {
	if done == 0 {
		return -1
	}
	elapsed := time.Since(r.startTime).Seconds()
	rate := float64(done) / elapsed
	remaining := float64(r.total) - float64(done)
	if rate == 0 {
		return -1
	}
	return remaining / rate
}

func (r *Reporter) emitJSON(e JSONEvent) {
	data, _ := json.Marshal(e)
	fmt.Fprintf(r.out, "%s\n", data)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
