package core

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

const (
	WorkerStatusQueued    = "queued"
	WorkerStatusCancelled = "cancelled"
)

var workerIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type WorkerJob struct {
	OK           bool   `json:"ok"`
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	Payload      string `json:"payload,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	WorkerDir    string `json:"worker_dir"`
	NoShell      bool   `json:"no_shell"`
	SafetyNotice string `json:"safety_notice"`
}

type WorkerListResult struct {
	OK        bool        `json:"ok"`
	WorkerDir string      `json:"worker_dir"`
	Jobs      []WorkerJob `json:"jobs"`
}

func EnqueueWorkerJob(kind, payload string) (WorkerJob, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return WorkerJob{OK: false}, fmt.Errorf("worker job kind is required")
	}
	if strings.ContainsAny(kind, `/\`) || len(kind) > 80 {
		return WorkerJob{OK: false}, fmt.Errorf("invalid worker job kind")
	}
	dir, err := workerDir()
	if err != nil {
		return WorkerJob{OK: false}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return WorkerJob{OK: false}, err
	}
	now := time.Now().UTC()
	id := makeWorkerJobID(kind, payload, now)
	job := WorkerJob{
		OK:           true,
		ID:           id,
		Kind:         kind,
		Status:       WorkerStatusQueued,
		Payload:      redactFreeform(payload),
		CreatedAt:    now.Format(time.RFC3339Nano),
		UpdatedAt:    now.Format(time.RFC3339Nano),
		WorkerDir:    dir,
		NoShell:      true,
		SafetyNotice: "worker MVP records lifecycle state only; it never executes shell commands",
	}
	return job, writeWorkerJob(job)
}

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

func CancelWorkerJob(id string) (WorkerJob, error) {
	job, err := ReadWorkerJob(id)
	if err != nil {
		return job, err
	}
	if job.Status == WorkerStatusCancelled {
		return job, nil
	}
	if job.Status != WorkerStatusQueued {
		return job, fmt.Errorf("worker job %s cannot be cancelled from status %s", id, job.Status)
	}
	job.Status = WorkerStatusCancelled
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	job.OK = true
	return job, writeWorkerJob(job)
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

func makeWorkerJobID(kind, payload string, t time.Time) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + payload + "\x00" + t.Format(time.RFC3339Nano)))
	return "job-" + t.Format("20060102T150405Z") + "-" + hex.EncodeToString(sum[:])[:12]
}
