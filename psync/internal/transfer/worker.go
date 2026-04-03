package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"github.com/psync/psync/internal/checkpoint"
	"github.com/psync/psync/internal/config"
	"github.com/psync/psync/internal/discovery"
	"github.com/psync/psync/internal/progress"
)

// Job is a unit of work sent to a worker.
type Job struct {
	Entry discovery.FileEntry
}

// Result is what a worker sends back after processing a job.
type Result struct {
	RelPath string
	Skipped bool
	Err     error
	Elapsed time.Duration
	Size    int64
}

// Pool orchestrates N workers consuming jobs from a channel.
type Pool struct {
	cfg         *config.Config
	reporter    *progress.Reporter
	ckpt        *checkpoint.State
	sshCfg      *SSHConfig
	isRemoteDst bool
	remoteDst   string // user@host:/path
}

// NewPool creates a worker pool.
func NewPool(
	cfg *config.Config,
	reporter *progress.Reporter,
	ckpt *checkpoint.State,
) *Pool {
	p := &Pool{cfg: cfg, reporter: reporter, ckpt: ckpt}

	// Detect remote destination.
	if idx := strings.Index(cfg.Destination, "@"); idx != -1 {
		if cidx := strings.Index(cfg.Destination, ":"); cidx != -1 {
			p.isRemoteDst = true
			p.remoteDst = cfg.Destination
			parts := strings.SplitN(cfg.Destination, "@", 2)
			hostPath := strings.SplitN(parts[1], ":", 2)
			p.sshCfg = &SSHConfig{
				User:    parts[0],
				Host:    hostPath[0],
				Port:    cfg.SSHPort,
				KeyPath: cfg.SSHKey,
			}
		}
	}
	return p
}

// Run feeds jobs through the worker pool and collects results.
// It returns after all jobs have been processed.
func (p *Pool) Run(jobs []discovery.FileEntry) {
	n := p.cfg.Threads
	runtime.GOMAXPROCS(n)

	jobCh := make(chan Job, len(jobs))
	resultCh := make(chan Result, len(jobs))

	// Pre-allocate SFTP clients (one per worker) if destination is remote.
	var sftpClients []*sftp.Client
	if p.isRemoteDst {
		sftpClients = make([]*sftp.Client, n)
		var initErr error
		var once sync.Once
		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				client, err := DialSFTP(*p.sshCfg)
				if err != nil {
					once.Do(func() { initErr = err })
					return
				}
				mu.Lock()
				sftpClients[idx] = client
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		if initErr != nil {
			fmt.Fprintf(os.Stderr, "psync: SSH connection failed: %v\n", initErr)
			os.Exit(1)
		}
		defer func() {
			for _, c := range sftpClients {
				if c != nil {
					c.Close()
				}
			}
		}()
	}

	// Start workers.
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(workerID int) {
			defer wg.Done()
			runtime.LockOSThread()
			var sftp *sftp.Client
			if p.isRemoteDst {
				sftp = sftpClients[workerID]
			}
			p.work(workerID, jobCh, resultCh, sftp)
		}(i)
	}

	// Enqueue jobs.
	for _, e := range jobs {
		jobCh <- Job{Entry: e}
	}
	close(jobCh)

	// Collect results in a separate goroutine while workers drain.
	done := make(chan struct{})
	go func() {
		for r := range resultCh {
			if r.Err != nil {
				p.reporter.FailFile(0, r.RelPath, r.Err)
				if p.ckpt != nil {
					_ = p.ckpt.MarkFailed(r.RelPath, r.Err.Error())
				}
			} else {
				p.reporter.CompleteFile(0, r.RelPath, r.Size, r.Elapsed, r.Skipped)
				if p.ckpt != nil {
					_ = p.ckpt.MarkCompleted(r.RelPath)
				}
			}
		}
		close(done)
	}()

	wg.Wait()
	close(resultCh)
	<-done
}

// work is the goroutine body for a single worker.
func (p *Pool) work(id int, jobs <-chan Job, results chan<- Result, sftpClient *sftp.Client) {
	for job := range jobs {
		e := job.Entry
		start := time.Now()
		p.reporter.StartFile(id, e.RelPath, e.Size)

		var (
			skipped bool
			err     error
		)

		if p.isRemoteDst {
			err = p.transferRemote(e, sftpClient)
		} else {
			skipped, err = p.transferLocal(e)
		}

		results <- Result{
			RelPath: e.RelPath,
			Skipped: skipped,
			Err:     err,
			Elapsed: time.Since(start),
			Size:    e.Size,
		}
	}
}

// transferLocal copies one file locally, skipping identical files.
func (p *Pool) transferLocal(e discovery.FileEntry) (skipped bool, err error) {
	dst := filepath.Join(p.cfg.Destination, filepath.FromSlash(e.RelPath))

	identical, err := FilesIdentical(e.AbsPath, dst)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("checksum %s: %w", e.RelPath, err)
	}
	if identical {
		return true, nil
	}

	if err := CopyFile(e.AbsPath, dst, p.cfg.DryRun); err != nil {
		return false, fmt.Errorf("copy %s: %w", e.RelPath, err)
	}
	return false, nil
}

// transferRemote copies one file to the remote via SFTP.
func (p *Pool) transferRemote(e discovery.FileEntry, client *sftp.Client) error {
	parts := strings.SplitN(p.remoteDst, ":", 2)
	remotePath := filepath.ToSlash(filepath.Join(parts[1], e.RelPath))

	if err := CopyFileViaSFTP(client, e.AbsPath, remotePath, p.cfg.DryRun); err != nil {
		return fmt.Errorf("sftp copy %s: %w", e.RelPath, err)
	}
	return nil
}

// checksumReader is used by ssh.go so we keep it here in the transfer package.
func checksumReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
