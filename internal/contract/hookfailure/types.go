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
	// Termination은 hook이 자기 오류가 아니라 외부 신호로 끝났을 때 그 사유를
	// 담는다("signal:terminated" 등). 비어 있으면 통상적인 오류 종료다.
	//
	// 이 필드가 필요한 이유: host가 hook 자식을 signal로 끝내면 hook은 exit
	// code를 남기지 못하고, 호출자는 "hook exited without a status code"라는
	// 사유 없는 문구만 보게 된다. 어느 hook이 어떤 신호로 끝났는지는 죽는
	// 쪽만 알 수 있으므로 여기서 기록한다(#268).
	Termination string `json:"termination,omitempty"`
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
