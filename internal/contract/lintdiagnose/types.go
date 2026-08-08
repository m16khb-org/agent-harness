// Package lintdiagnose는 lintdiagnose capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package lintdiagnose

type LintDiagnoseRequest struct {
	RepoRoot    string   `json:"repo_root"`
	CommandArgv []string `json:"command_argv"`
}

type LintDiagnoseResult struct {
	OK          bool     `json:"ok"`
	CommandArgv []string `json:"command_argv"`
	ExitCode    int      `json:"exit_code"`
	Failed      bool     `json:"failed"`
	Diagnosis   string   `json:"diagnosis,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
}
