// Package hookfailure는 hookfailure capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package hookfailure

type HookFailureEvent struct {
	Timestamp      string   `json:"timestamp,omitempty"`
	Hook           string   `json:"hook"`
	Host           string   `json:"host,omitempty"`
	Repo           string   `json:"repo,omitempty"`
	CWD            string   `json:"cwd,omitempty"`
	Tool           string   `json:"tool,omitempty"`
	Argv           []string `json:"argv,omitempty"`
	CommandSnippet string   `json:"command_snippet,omitempty"`
	Error          string   `json:"error"`
}

type HookFailureRecordResult struct {
	OK    bool             `json:"ok"`
	Path  string           `json:"path"`
	Event HookFailureEvent `json:"event"`
}

type HookFailureListResult struct {
	OK     bool               `json:"ok"`
	Path   string             `json:"path"`
	Events []HookFailureEvent `json:"events"`
}

type HookFailurePruneResult struct {
	OK     bool   `json:"ok"`
	Path   string `json:"path"`
	Pruned int    `json:"pruned"`
	Kept   int    `json:"kept"`
}

// HookFailureStats aggregates the failure JSONL into the hook-health metrics
// the quality program's Q2 item requires: failure counts (total, per hook,
// recent windows) plus log-health fields for the rotation policy.
type HookFailureStats struct {
	OK        bool           `json:"ok"`
	Path      string         `json:"path"`
	Total     int            `json:"total"`
	ByHook    map[string]int `json:"by_hook,omitempty"`
	Last24h   int            `json:"last_24h"`
	Last7d    int            `json:"last_7d"`
	SizeBytes int64          `json:"size_bytes"`
	Oldest    string         `json:"oldest,omitempty"`
	Newest    string         `json:"newest,omitempty"`
}
