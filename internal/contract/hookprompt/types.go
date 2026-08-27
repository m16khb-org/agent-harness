// Package hookprompt는 hookprompt capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package hookprompt

import projectdoc "agent-harness/internal/domain/projectdoc"

type HookUserPromptRequest struct {
	Prompt               string `json:"prompt"`
	Repo                 string `json:"repo,omitempty"`
	Host                 string `json:"host,omitempty"`
	SessionID            string `json:"session_id,omitempty"`
	EnableLLMHints       bool   `json:"enable_llm_hints,omitempty"`
	DisableKarpathyFirst bool   `json:"disable_karpathy_first,omitempty"`
}

type HookUserPromptResult struct {
	OK                bool                                `json:"ok"`
	Kind              string                              `json:"kind"`
	GeneratedAt       string                              `json:"generated_at"`
	ShouldInject      bool                                `json:"should_inject"`
	AdditionalContext string                              `json:"additional_context,omitempty"`
	Hints             []Hint                              `json:"hints,omitempty"`
	ProjectDocs       []projectdoc.ProjectDocCatalogEntry `json:"project_docs,omitempty"`
	KarpathyFirst     bool                                `json:"karpathy_first,omitempty"`
	UserNotice        string                              `json:"user_notice,omitempty"`
}

type ProjectDocCatalogContext struct {
	ShouldInject bool                                `json:"should_inject"`
	ProjectDocs  []projectdoc.ProjectDocCatalogEntry `json:"project_docs,omitempty"`
	Compact      string                              `json:"compact,omitempty"`
	UserView     string                              `json:"user_view,omitempty"`
}

type Hint struct {
	Tool     string `json:"tool"`
	Reason   string `json:"reason"`
	Priority string `json:"priority,omitempty"`
}
