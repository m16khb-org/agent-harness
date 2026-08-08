// Package looprun는 looprun capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package looprun

type LoopRun struct {
	OK            bool          `json:"ok"`
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	Repo          string        `json:"repo"`
	Name          string        `json:"name"`
	Goal          string        `json:"goal"`
	VerifyArgv    []string      `json:"verify_argv,omitempty"`
	MaxAttempts   int           `json:"max_attempts"`
	Status        string        `json:"status"`
	Attempts      []LoopAttempt `json:"attempts,omitempty"`
	StopReason    string        `json:"stop_reason,omitempty"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
}

type LoopAttempt struct {
	Seq      int      `json:"seq"`
	Verdict  string   `json:"verdict"`
	Evidence []string `json:"evidence"`
	At       string   `json:"at"`
}

type StartLoopRequest struct {
	Repo        string
	Name        string
	Goal        string
	VerifyArgv  []string
	MaxAttempts int
}

type RecordAttemptRequest struct {
	Verdict  string
	Evidence []string
}

type StatusResult struct {
	OK           bool    `json:"ok"`
	Loop         LoopRun `json:"loop"`
	AttemptCount int     `json:"attempt_count"`
	LastVerdict  string  `json:"last_verdict,omitempty"`
}
