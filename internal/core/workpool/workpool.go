package workpool

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/delegation"
)

const (
	defaultPoolSize        = 4
	maxPoolSize            = 16
	defaultLeaseTTL        = "15m"
	defaultMaxAttempts     = 2
	maxTasksPerPool        = 4096
	defaultPoolStatus      = "active"
	defaultWorkTaskStatus  = "pending"
	parentWorkPoolTaskSlug = "task-fan-out-coordination"
)

func CreatePool(req CreatePoolRequest) (WorkPool, error) {
	repo, err := normalizeRepo(req.Repo)
	if err != nil {
		return WorkPool{OK: false}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return WorkPool{OK: false}, fmt.Errorf("name is required")
	}
	size, err := normalizePoolSize(req.Size)
	if err != nil {
		return WorkPool{OK: false}, err
	}
	leaseTTL, err := normalizeLeaseTTL(req.LeaseTTL)
	if err != nil {
		return WorkPool{OK: false}, err
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	if maxAttempts < 0 {
		return WorkPool{OK: false}, fmt.Errorf("max_attempts_invalid")
	}
	parentCycleID := strings.TrimSpace(req.ParentCycleID)
	if parentCycleID != "" {
		if err := validateParentCycle(parentCycleID, name); err != nil {
			return WorkPool{OK: false}, err
		}
	}

	poolID := newPoolID(repo, name)
	var pool WorkPool
	err = withPoolLock(poolID, func() error {
		now := timestampNow()
		pool = WorkPool{
			OK:            true,
			SchemaVersion: WorkPoolCurrentSchemaVersion,
			ID:            poolID,
			Repo:          repo,
			Name:          name,
			ParentCycleID: parentCycleID,
			PilotRequired: req.PilotRequired,
			Size:          size,
			LeaseTTL:      leaseTTL,
			MaxAttempts:   maxAttempts,
			Status:        defaultPoolStatus,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		var writeErr error
		pool, writeErr = writePool(pool)
		return writeErr
	})
	return pool, err
}

func AddTask(poolID string, req AddTaskRequest) (WorkTask, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return WorkTask{OK: false}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return WorkTask{OK: false, PoolID: poolID}, fmt.Errorf("title is required")
	}
	var task WorkTask
	err = withPoolLock(poolID, func() error {
		pool, err := ReadPool(poolID)
		if err != nil {
			return err
		}
		if pool.Status == "closed" {
			return fmt.Errorf("pool_closed")
		}
		if req.Pilot {
			if !pool.PilotRequired {
				return fmt.Errorf("pool_pilot_not_required")
			}
			if pool.PilotTaskID != "" {
				return fmt.Errorf("pool_pilot_already_set")
			}
		}
		taskIDs, err := taskFileIDs(poolID)
		if err != nil {
			return err
		}
		if len(taskIDs) >= maxTasksPerPool {
			return fmt.Errorf("pool_task_cap")
		}
		now := timestampNow()
		task = WorkTask{
			OK:                 true,
			SchemaVersion:      WorkPoolCurrentSchemaVersion,
			ID:                 newTaskID(len(taskIDs)+1, title),
			PoolID:             poolID,
			Title:              title,
			Instructions:       redactInstructions(req.Instructions),
			Scope:              cleanStrings(req.Scope),
			AcceptanceCriteria: cleanStrings(req.AcceptanceCriteria),
			Status:             defaultWorkTaskStatus,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		var writeErr error
		task, writeErr = writeTask(task)
		if writeErr != nil {
			return writeErr
		}
		if req.Pilot {
			pool.PilotTaskID = task.ID
			pool.UpdatedAt = now
			_, writeErr = writePool(pool)
		}
		return writeErr
	})
	return task, err
}

// withPoolLock serializes a pool-level read-modify-write span via the workpool
// state root's sqlstore span lock. Spans must not nest (see sqlstore.WithSpan).
func withPoolLock(poolID string, fn func() error) error {
	if _, err := normalizePoolID(poolID); err != nil {
		return err
	}
	db, err := openStore()
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}

// withTaskLock serializes a task-level read-modify-write span. It shares the
// same per-state-root span as withPoolLock, so it must never be entered from
// inside a pool span (the codebase keeps pool and task spans sequential).
func withTaskLock(poolID, taskID string, fn func() error) error {
	if _, err := normalizePoolID(poolID); err != nil {
		return err
	}
	if _, err := normalizeTaskID(taskID); err != nil {
		return err
	}
	db, err := openStore()
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}

func validateParentCycle(parentCycleID, poolName string) error {
	parent, err := issueops.ReadIssueOps(issueops.IssueOpsStateRoot(), parentCycleID)
	if err != nil {
		return err
	}
	req := issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             strings.TrimSpace(parent.Branch) + "-workpool",
		Title:              "workpool " + strings.TrimSpace(poolName),
		TaskScope:          parentWorkPoolTaskSlug,
		AcceptanceCriteria: []string{"pool tasks complete"},
	}
	if missing := delegation.MissingPreconditions(parent, req); len(missing) > 0 {
		return fmt.Errorf("cannot create workpool: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func normalizeRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func normalizePoolSize(size int) (int, error) {
	if size == 0 {
		return defaultPoolSize, nil
	}
	if size < 1 || size > maxPoolSize {
		return 0, fmt.Errorf("pool_size_out_of_range")
	}
	return size, nil
}

func normalizeLeaseTTL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultLeaseTTL, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return "", fmt.Errorf("lease_ttl_invalid")
	}
	return value, nil
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

var secretAssignmentPattern = regexp.MustCompile(`(?i)\b(token|secret|password|api[_-]?key|access[_-]?key)\s*[:=]\s*["']?([^\s"',}]+)`)

func redactInstructions(instructions string) string {
	return strings.TrimSpace(secretAssignmentPattern.ReplaceAllString(instructions, "$1=<redacted>"))
}
