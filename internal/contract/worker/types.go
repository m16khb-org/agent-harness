// Package worker는 worker capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package worker

import policycontract "agent-harness/internal/contract/policy"

type WorkerJob struct {
	OK           bool                             `json:"ok"`
	ID           string                           `json:"id"`
	Kind         string                           `json:"kind"`
	Status       string                           `json:"status"`
	Payload      string                           `json:"payload,omitempty"`
	CreatedAt    string                           `json:"created_at"`
	UpdatedAt    string                           `json:"updated_at"`
	StartedAt    string                           `json:"started_at,omitempty"`
	PID          int                              `json:"pid,omitempty"`
	WorkerDir    string                           `json:"worker_dir"`
	NoShell      bool                             `json:"no_shell"`
	SafetyNotice string                           `json:"safety_notice"`
	Command      []string                         `json:"command,omitempty"`
	Result       *policycontract.CommandRunResult `json:"result,omitempty"`
}

// WorkerQueueStats is a status histogram over all worker jobs (A2/G6). Depth is
// the saturation signal — only non-terminal work (queued+running) — so a backlog
// is visible before jobs time out, unlike the raw per-status counts alone.
type WorkerQueueStats struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
	Total     int `json:"total"`
	Depth     int `json:"depth"`
}

type WorkerListResult struct {
	OK        bool        `json:"ok"`
	WorkerDir string      `json:"worker_dir"`
	Jobs      []WorkerJob `json:"jobs"`
	// Queue is the saturation histogram; populated by ListWorkerJobs (the full
	// listing) and left nil by DetectStuckWorkerJobs, whose Jobs is only the
	// fixed subset and would make a histogram misleading.
	Queue *WorkerQueueStats `json:"queue,omitempty"`
}

const (
	WorkerStatusQueued    = "queued"
	WorkerStatusRunning   = "running"
	WorkerStatusSucceeded = "succeeded"
	WorkerStatusFailed    = "failed"
	WorkerStatusCancelled = "cancelled"
)
