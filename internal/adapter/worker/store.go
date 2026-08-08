package worker

import (
	workercontract "agent-harness/internal/contract/worker"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
)

var workerIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// workerBucket is the sqlstore bucket holding one row per worker job.
const workerBucket = "worker"

func openWorkerDB(dir string) (*sqlstore.DB, error) {
	return sqlstore.Open(dir)
}

func ReadWorkerJob(id string) (workercontract.WorkerJob, error) {
	if !workerIDRe.MatchString(id) || strings.Contains(id, "..") {
		return workercontract.WorkerJob{OK: false, ID: id}, fmt.Errorf("invalid worker job id")
	}
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id}, err
	}
	db, err := openWorkerDB(dir)
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	b, ok, err := db.Get(workerBucket, id)
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	if !ok {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, fmt.Errorf("worker job %s: %w", id, fs.ErrNotExist)
	}
	var job workercontract.WorkerJob
	if err := json.Unmarshal(b, &job); err != nil {
		return workercontract.WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	job.WorkerDir = dir
	return job, nil
}

func ListWorkerJobs() (workercontract.WorkerListResult, error) {
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerListResult{OK: false}, err
	}
	result := workercontract.WorkerListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.WorkerJob{}}
	db, err := openWorkerDB(dir)
	if err != nil {
		return result, err
	}
	ids, err := db.List(workerBucket)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		job, err := ReadWorkerJob(id)
		if err == nil {
			result.Jobs = append(result.Jobs, job)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	result.Queue = summarizeWorkerQueue(result.Jobs)
	return result, nil
}

// summarizeWorkerQueue builds the status histogram + saturation depth (A2/G6).
func summarizeWorkerQueue(jobs []workercontract.WorkerJob) *workercontract.WorkerQueueStats {
	q := &workercontract.WorkerQueueStats{Total: len(jobs)}
	for _, job := range jobs {
		switch job.Status {
		case workercontract.WorkerStatusQueued:
			q.Queued++
		case workercontract.WorkerStatusRunning:
			q.Running++
		case workercontract.WorkerStatusSucceeded:
			q.Succeeded++
		case workercontract.WorkerStatusFailed:
			q.Failed++
		case workercontract.WorkerStatusCancelled:
			q.Cancelled++
		}
	}
	q.Depth = q.Queued + q.Running
	return q
}

func writeWorkerJob(job workercontract.WorkerJob) error {
	if !workerIDRe.MatchString(job.ID) || strings.Contains(job.ID, "..") {
		return fmt.Errorf("invalid worker job id")
	}
	dir, err := workerDir()
	if err != nil {
		return err
	}
	db, err := openWorkerDB(dir)
	if err != nil {
		return err
	}
	job.WorkerDir = dir
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	// The span lock is the CALLER's responsibility (withWorkerJobLock); the row
	// upsert itself is atomic, so a crash mid-write can never leave a truncated
	// job record that ListWorkerJobs silently drops.
	return db.Put(workerBucket, job.ID, append(b, '\n'))
}

func workerDir() (string, error) {
	if dir := os.Getenv("HARNESS_WORKER_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	dir := os.Getenv("HARNESS_STATE_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("resolve home for worker dir: %w", err)
		}
		dir = filepath.Join(home, ".local", "state", "agent-harness")
	}
	return filepath.Join(dir, "worker"), nil
}

// DetectStuckWorkerJobs scans all worker jobs. For any job stuck in
// "running" status whose PID is no longer alive, it marks the job as
// "failed" with an error message. Returns the list of jobs that were
// detected and fixed.
func DetectStuckWorkerJobs() (workercontract.WorkerListResult, error) {
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerListResult{OK: false}, err
	}
	result := workercontract.WorkerListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.WorkerJob{}}
	db, err := openWorkerDB(dir)
	if err != nil {
		return result, err
	}
	ids, err := db.List(workerBucket)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		job, err := ReadWorkerJob(id)
		if err != nil {
			continue
		}
		if job.Status == workercontract.WorkerStatusRunning && !isPIDAlive(job.PID) {
			// Lock per job so we don't race with another modifier.
			fixed := false
			lockErr := withWorkerJobLock(context.Background(), dir, id, func(context.Context) error {
				current, reReadErr := ReadWorkerJob(id)
				if reReadErr != nil {
					return reReadErr
				}
				if current.Status != workercontract.WorkerStatusRunning || isPIDAlive(current.PID) {
					// Status changed or PID became alive since we first
					// checked; nothing to fix.
					return nil
				}
				current.Status = workercontract.WorkerStatusFailed
				current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				current.OK = false
				current.Result = nil // clear any stale result
				current.SafetyNotice = "worker job was stuck in running status with dead PID; auto-marked as failed"
				if err := writeWorkerJob(current); err != nil {
					return err
				}
				job = current
				fixed = true
				return nil
			})
			if lockErr != nil {
				continue
			}
			if fixed {
				result.Jobs = append(result.Jobs, job)
			}
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	return result, nil
}

const stuckScanSentinel = ".last-stuck-scan"

// MaybeDetectStuckWorkerJobs runs DetectStuckWorkerJobs at most once per
// minInterval, gated by a stat-only sentinel file's mtime (A2/W1). The detector
// is an unbounded full scan (row list + per-job read) and the worker store has
// no TTL/GC, so calling it unconditionally on every session start would grow
// the session-start hot path without bound; amortizing keeps it cheap. Returns
// ran=false when the scan was skipped this interval. Best-effort: the sentinel
// is touched even on detector error so a transient failure cannot make every
// session re-run the scan.
func MaybeDetectStuckWorkerJobs(minInterval time.Duration) (workercontract.WorkerListResult, bool, error) {
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerListResult{OK: false}, false, err
	}
	sentinel := filepath.Join(dir, stuckScanSentinel)
	if info, statErr := os.Stat(sentinel); statErr == nil && time.Since(info.ModTime()) < minInterval {
		return workercontract.WorkerListResult{OK: true, WorkerDir: dir, Jobs: []workercontract.WorkerJob{}}, false, nil
	}
	result, detErr := DetectStuckWorkerJobs()
	if mkErr := os.MkdirAll(dir, 0o700); mkErr == nil {
		if f, oErr := os.OpenFile(sentinel, os.O_CREATE|os.O_WRONLY, 0o600); oErr == nil {
			_ = f.Close()
		}
		now := time.Now()
		_ = os.Chtimes(sentinel, now, now)
	}
	return result, true, detErr
}

func makeWorkerJobID(kind, payload string, t time.Time) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + payload + "\x00" + t.Format(time.RFC3339Nano)))
	return "job-" + t.Format("20060102T150405Z") + "-" + hex.EncodeToString(sum[:])[:12]
}
