package selfworkflow

import (
	"io"
	"time"
)

type SelfVerifyProgressEvent struct {
	Event       string `json:"event"`
	LoopKind    string `json:"loop_kind,omitempty"`
	Iteration   int    `json:"iteration,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`
	Seed        int64  `json:"seed,omitempty"`
	StepIndex   int    `json:"step_index,omitempty"`
	StepCount   int    `json:"step_count,omitempty"`
	Step        string `json:"step,omitempty"`
	OK          *bool  `json:"ok,omitempty"`
	ElapsedMS   int64  `json:"elapsed_ms"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
	LastSuccess string `json:"last_success,omitempty"`
	Error       string `json:"error,omitempty"`
}

type SelfVerifyProgressReporter struct {
	mode        string
	writer      io.Writer
	started     time.Time
	lastSuccess string
}
