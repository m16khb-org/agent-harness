package commandstep

type StepResult struct {
	Label           string `json:"label"`
	Command         string `json:"command,omitempty"`
	OK              bool   `json:"ok"`
	DurationMS      int64  `json:"duration_ms"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutBytes     int    `json:"stdout_bytes,omitempty"`
	StderrBytes     int    `json:"stderr_bytes,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
	Error           string `json:"error,omitempty"`
}
