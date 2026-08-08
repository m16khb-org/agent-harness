// Package commitsuggest는 commitsuggest capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package commitsuggest

// CommitSuggestRequest는 commit message 제안을 구성한다.
type CommitSuggestRequest struct {
	RepoRoot string `json:"repo_root"`
	Staged   bool   `json:"staged"`
}

type CommitSuggestResult struct {
	OK            bool   `json:"ok"`
	Executed      bool   `json:"executed"`
	RepoRoot      string `json:"repo_root"`
	Staged        bool   `json:"staged"`
	CommitMessage string `json:"commit_message,omitempty"`
	Prompt        string `json:"prompt,omitempty"`
}
