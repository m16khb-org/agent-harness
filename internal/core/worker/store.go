package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var workerIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func ReadWorkerJob(id string) (WorkerJob, error) {
	if !workerIDRe.MatchString(id) || strings.Contains(id, "..") {
		return WorkerJob{OK: false, ID: id}, fmt.Errorf("invalid worker job id")
	}
	dir, err := workerDir()
	if err != nil {
		return WorkerJob{OK: false, ID: id}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	var job WorkerJob
	if err := json.Unmarshal(b, &job); err != nil {
		return WorkerJob{OK: false, ID: id, WorkerDir: dir}, err
	}
	job.WorkerDir = dir
	return job, nil
}

func ListWorkerJobs() (WorkerListResult, error) {
	dir, err := workerDir()
	if err != nil {
		return WorkerListResult{OK: false}, err
	}
	result := WorkerListResult{OK: true, WorkerDir: dir, Jobs: []WorkerJob{}}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		job, err := ReadWorkerJob(id)
		if err == nil {
			result.Jobs = append(result.Jobs, job)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt > result.Jobs[j].CreatedAt })
	return result, nil
}

func writeWorkerJob(job WorkerJob) error {
	if !workerIDRe.MatchString(job.ID) || strings.Contains(job.ID, "..") {
		return fmt.Errorf("invalid worker job id")
	}
	dir, err := workerDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	job.WorkerDir = dir
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, job.ID+".json"), append(b, '\n'), 0o600)
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
func DetectStuckWorkerJobs() (WorkerListResult, error) {
	dir, err := workerDir()
	if err != nil {
		return WorkerListResult{OK: false}, err
	}
	result := WorkerListResult{OK: true, WorkerDir: dir, Jobs: []WorkerJob{}}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		job, err := ReadWorkerJob(id)
		if err != nil {
			continue
		}
		if job.Status == WorkerStatusRunning && !isPIDAlive(job.PID) {
			// Lock per job so we don't race with another modifier.
			fixed := false
			lockErr := withWorkerJobLock(dir, id, func() error {
				current, reReadErr := ReadWorkerJob(id)
				if reReadErr != nil {
					return reReadErr
				}
				if current.Status != WorkerStatusRunning || isPIDAlive(current.PID) {
					// Status changed or PID became alive since we first
					// checked; nothing to fix.
					return nil
				}
				current.Status = WorkerStatusFailed
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

func makeWorkerJobID(kind, payload string, t time.Time) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + payload + "\x00" + t.Format(time.RFC3339Nano)))
	return "job-" + t.Format("20060102T150405Z") + "-" + hex.EncodeToString(sum[:])[:12]
}
