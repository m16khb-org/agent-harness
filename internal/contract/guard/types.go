// Package guard는 guard capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package guard

type GuardCheckRequest struct {
	RepoRoot string   `json:"repo_root"`
	Staged   bool     `json:"staged"`
	All      bool     `json:"all"`
	Files    []string `json:"files,omitempty"`
}

type GuardCheckResult struct {
	OK           bool           `json:"ok"`
	RepoRoot     string         `json:"repo_root"`
	Mode         string         `json:"mode"`
	CheckedFiles []string       `json:"checked_files"`
	Findings     []GuardFinding `json:"findings"`
	Summary      GuardSummary   `json:"summary"`
	Warnings     []string       `json:"warnings,omitempty"`
}

type GuardFinding struct {
	Severity    string   `json:"severity"`
	Rule        string   `json:"rule"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Evidence    string   `json:"evidence,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type GuardSummary struct {
	Block  int `json:"block"`
	Warn   int `json:"warn"`
	Review int `json:"review"`
	Info   int `json:"info"`
}
