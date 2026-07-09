package workpool

import (
	"fmt"
	"strings"
	"time"
)

type ClaimResult struct {
	OK     bool     `json:"ok"`
	Task   WorkTask `json:"task"`
	Prompt string   `json:"prompt"`
}

func Claim(poolID, workerID string) (ClaimResult, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return ClaimResult{OK: false}, err
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return ClaimResult{OK: false}, fmt.Errorf("worker_id is required")
	}
	var result ClaimResult
	err = withPoolLock(poolID, func() error {
		pool, err := ReadPool(poolID)
		if err != nil {
			return err
		}
		if pool.Status == "closed" {
			return fmt.Errorf("pool_closed")
		}
		if pool.Status == "draining" {
			return fmt.Errorf("pool_draining")
		}
		if _, err := reapExpiredLeasesLocked(pool); err != nil {
			return err
		}
		tasks, err := ListTasks(poolID)
		if err != nil {
			return err
		}
		leased := 0
		var pending *WorkTask
		for i := range tasks {
			switch tasks[i].Status {
			case "leased":
				leased++
			case "pending":
				if pending == nil {
					pending = &tasks[i]
				}
			}
		}
		if pool.PilotRequired {
			if pool.PilotTaskID == "" {
				return fmt.Errorf("pool_pilot_unassigned")
			}
			pilot := findTaskByID(tasks, pool.PilotTaskID)
			if pilot == nil {
				return fmt.Errorf("pool_pilot_pending")
			}
			if pilot.Status == "dropped" {
				return fmt.Errorf("pool_pilot_dropped")
			}
			if pilot.Status != "accepted" {
				if pilot.Status != "pending" {
					return fmt.Errorf("pool_pilot_pending")
				}
				pending = pilot
			}
		}
		if leased >= pool.Size {
			return fmt.Errorf("pool_saturated")
		}
		if pending == nil {
			return fmt.Errorf("pool_no_pending_tasks")
		}
		now := timestampNow()
		ttl, err := time.ParseDuration(pool.LeaseTTL)
		if err != nil || ttl <= 0 {
			return fmt.Errorf("lease_ttl_invalid")
		}
		task := *pending
		task.Status = "leased"
		task.WorkerID = workerID
		task.LeaseExpiresAt = workPoolNow().UTC().Add(ttl).Format(time.RFC3339Nano)
		task.LastHeartbeatAt = now
		task.Branch = recommendedTaskBranch(pool, task)
		task.UpdatedAt = now
		var writeErr error
		task, writeErr = writeTask(task)
		if writeErr != nil {
			return writeErr
		}
		result = ClaimResult{
			OK:     true,
			Task:   task,
			Prompt: renderClaimPrompt(pool, task),
		}
		return nil
	})
	return result, err
}

func findTaskByID(tasks []WorkTask, taskID string) *WorkTask {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i]
		}
	}
	return nil
}

func Heartbeat(poolID, taskID, workerID string) (WorkTask, error) {
	return mutateLeasedTask(poolID, taskID, workerID, func(pool WorkPool, task WorkTask) (WorkTask, error) {
		ttl, err := time.ParseDuration(pool.LeaseTTL)
		if err != nil || ttl <= 0 {
			return task, fmt.Errorf("lease_ttl_invalid")
		}
		task.LastHeartbeatAt = timestampNow()
		task.LeaseExpiresAt = workPoolNow().UTC().Add(ttl).Format(time.RFC3339Nano)
		task.UpdatedAt = task.LastHeartbeatAt
		return writeTask(task)
	})
}

func Submit(poolID, taskID, workerID string, evidence []string, branch, worktree string) (WorkTask, error) {
	evidence = cleanStrings(evidence)
	if len(evidence) == 0 {
		return WorkTask{OK: false}, fmt.Errorf("evidence_required")
	}
	return mutateLeasedTask(poolID, taskID, workerID, func(_ WorkPool, task WorkTask) (WorkTask, error) {
		task.Status = "submitted"
		task.Evidence = evidence
		task.Branch = strings.TrimSpace(branch)
		task.WorktreePath = strings.TrimSpace(worktree)
		task.SubmittedAt = timestampNow()
		task.UpdatedAt = task.SubmittedAt
		return writeTask(task)
	})
}

func Accept(poolID, taskID string, evidence []string) (WorkTask, error) {
	evidence = cleanStrings(evidence)
	if len(evidence) == 0 {
		return WorkTask{OK: false}, fmt.Errorf("evidence_required")
	}
	return mutateTaskIfPoolOpen(poolID, taskID, func(task WorkTask, pool WorkPool) (WorkTask, error) {
		if task.Status != "submitted" {
			return task, fmt.Errorf("task_not_submitted")
		}
		task.Status = "accepted"
		task.Evidence = evidence
		task.UpdatedAt = timestampNow()
		return writeTask(task)
	})
}

func Reject(poolID, taskID, reason string, requeue bool) (WorkTask, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return WorkTask{OK: false}, fmt.Errorf("reject_reason_too_short")
	}
	return mutateTaskIfPoolOpen(poolID, taskID, func(task WorkTask, pool WorkPool) (WorkTask, error) {
		if task.Status != "submitted" && task.Status != "leased" && task.Status != "rejected" {
			return task, fmt.Errorf("task_not_rejectable")
		}
		task.Attempts++
		task.RejectReason = reason
		task.WorkerID = ""
		task.LeaseExpiresAt = ""
		task.LastHeartbeatAt = ""
		task.SubmittedAt = ""
		if requeue {
			if task.Attempts >= pool.MaxAttempts {
				task.Status = "dropped"
			} else {
				task.Status = "pending"
			}
		} else {
			task.Status = "rejected"
		}
		task.UpdatedAt = timestampNow()
		return writeTask(task)
	})
}

func Reap(poolID string) ([]WorkTask, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return nil, err
	}
	var reaped []WorkTask
	err = withPoolLock(poolID, func() error {
		pool, err := ReadPool(poolID)
		if err != nil {
			return err
		}
		reaped, err = reapExpiredLeasesLocked(pool)
		return err
	})
	return reaped, err
}

func Status(poolID string) (StatusResult, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return StatusResult{OK: false}, err
	}
	result := StatusResult{OK: true, Counts: map[string]int{}}
	err = withPoolLock(poolID, func() error {
		pool, err := ReadPool(poolID)
		if err != nil {
			return err
		}
		reaped, err := reapExpiredLeasesLocked(pool)
		if err != nil {
			return err
		}
		tasks, err := ListTasks(poolID)
		if err != nil {
			return err
		}
		counts := map[string]int{}
		for _, task := range tasks {
			counts[task.Status]++
		}
		result = StatusResult{
			OK:     true,
			Pool:   pool,
			Tasks:  tasks,
			Counts: counts,
			Reaped: reaped,
		}
		return nil
	})
	if err != nil {
		result.OK = false
	}
	return result, err
}

func Close(poolID string, force bool, reason string) (WorkPool, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return WorkPool{OK: false}, err
	}
	var pool WorkPool
	err = withPoolLock(poolID, func() error {
		var readErr error
		pool, readErr = ReadPool(poolID)
		if readErr != nil {
			return readErr
		}
		if pool.Status == "closed" {
			return nil
		}
		tasks, err := ListTasks(poolID)
		if err != nil {
			return err
		}
		if !allTasksTerminal(tasks) {
			if !force {
				return fmt.Errorf("pool_not_terminal")
			}
			if len(strings.TrimSpace(reason)) < 10 {
				return fmt.Errorf("force_close_reason_required")
			}
		}
		pool.Status = "closed"
		pool.UpdatedAt = timestampNow()
		var writeErr error
		pool, writeErr = writePool(pool)
		return writeErr
	})
	return pool, err
}

func mutateLeasedTask(poolID, taskID, workerID string, fn func(WorkPool, WorkTask) (WorkTask, error)) (WorkTask, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return WorkTask{OK: false}, fmt.Errorf("worker_id is required")
	}
	return mutateTaskIfPoolOpen(poolID, taskID, func(task WorkTask, pool WorkPool) (WorkTask, error) {
		if task.Status != "leased" {
			return task, fmt.Errorf("lease_not_held")
		}
		if task.WorkerID != workerID {
			return task, fmt.Errorf("worker_mismatch")
		}
		if leaseExpired(task) {
			return task, fmt.Errorf("lease_expired")
		}
		return fn(pool, task)
	})
}

func mutateTaskIfPoolOpen(poolID, taskID string, fn func(WorkTask, WorkPool) (WorkTask, error)) (WorkTask, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return WorkTask{OK: false}, err
	}
	taskID, err = normalizeTaskID(taskID)
	if err != nil {
		return WorkTask{OK: false, PoolID: poolID}, err
	}
	var task WorkTask
	err = withTaskLock(poolID, taskID, func() error {
		pool, err := ReadPool(poolID)
		if err != nil {
			return err
		}
		if pool.Status == "closed" {
			return fmt.Errorf("pool_closed")
		}
		task, err = ReadTask(poolID, taskID)
		if err != nil {
			return err
		}
		var writeErr error
		task, writeErr = fn(task, pool)
		return writeErr
	})
	return task, err
}

func reapExpiredLeasesLocked(pool WorkPool) ([]WorkTask, error) {
	tasks, err := ListTasks(pool.ID)
	if err != nil {
		return nil, err
	}
	reaped := []WorkTask{}
	for _, task := range tasks {
		if task.Status != "leased" || !leaseExpired(task) {
			continue
		}
		task.Attempts++
		task.WorkerID = ""
		task.LeaseExpiresAt = ""
		task.LastHeartbeatAt = ""
		if task.Attempts >= pool.MaxAttempts {
			task.Status = "dropped"
		} else {
			task.Status = "pending"
		}
		task.UpdatedAt = timestampNow()
		written, err := writeTask(task)
		if err != nil {
			return reaped, err
		}
		reaped = append(reaped, written)
	}
	return reaped, nil
}

func leaseExpired(task WorkTask) bool {
	expires := strings.TrimSpace(task.LeaseExpiresAt)
	if expires == "" {
		return true
	}
	parsed, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return true
	}
	return !workPoolNow().UTC().Before(parsed)
}

func allTasksTerminal(tasks []WorkTask) bool {
	for _, task := range tasks {
		if task.Status != "accepted" && task.Status != "dropped" {
			return false
		}
	}
	return true
}

func renderClaimPrompt(pool WorkPool, task WorkTask) string {
	return "workpool task " + task.ID +
		"\nbranch: " + recommendedTaskBranch(pool, task) +
		"\nprepare an isolated worktree, then heartbeat with workpool heartbeat and submit with workpool submit"
}

func recommendedTaskBranch(pool WorkPool, task WorkTask) string {
	return "pool/" + pool.Name + "/" + task.ID
}
