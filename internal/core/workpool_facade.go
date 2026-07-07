package core

import "agent-harness/internal/core/workpool"

type WorkPool = workpool.WorkPool
type WorkTask = workpool.WorkTask
type WorkPoolCreateRequest = workpool.CreatePoolRequest
type WorkPoolAddTaskRequest = workpool.AddTaskRequest
type WorkPoolClaimResult = workpool.ClaimResult
type WorkPoolStatusResult = workpool.StatusResult

func CreateWorkPool(req WorkPoolCreateRequest) (WorkPool, error) {
	return workpool.CreatePool(req)
}

func AddWorkPoolTask(poolID string, req WorkPoolAddTaskRequest) (WorkTask, error) {
	return workpool.AddTask(poolID, req)
}

func ClaimWorkPool(poolID, workerID string) (WorkPoolClaimResult, error) {
	return workpool.Claim(poolID, workerID)
}

func HeartbeatWorkPool(poolID, taskID, workerID string) (WorkTask, error) {
	return workpool.Heartbeat(poolID, taskID, workerID)
}

func SubmitWorkPool(poolID, taskID, workerID string, evidence []string, branch, worktreePath string) (WorkTask, error) {
	return workpool.Submit(poolID, taskID, workerID, evidence, branch, worktreePath)
}

func AcceptWorkPool(poolID, taskID string, evidence []string) (WorkTask, error) {
	return workpool.Accept(poolID, taskID, evidence)
}

func RejectWorkPool(poolID, taskID, reason string, requeue bool) (WorkTask, error) {
	return workpool.Reject(poolID, taskID, reason, requeue)
}

func ReapWorkPool(poolID string) ([]WorkTask, error) {
	return workpool.Reap(poolID)
}

func StatusWorkPool(poolID string) (WorkPoolStatusResult, error) {
	return workpool.Status(poolID)
}

func CloseWorkPool(poolID string, force bool, reason string) (WorkPool, error) {
	return workpool.Close(poolID, force, reason)
}
