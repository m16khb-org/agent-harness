// Package hookprompt는 host lifecycle context hook의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package hookprompt

import projectdoc "agent-harness/internal/domain/projectdoc"

// ProjectDocCatalogContext is the static project-doc catalog a context hook
// injects: Compact is the model-facing one-line menu, UserView the readable
// list hosts may show to the user. --json prints this shape verbatim, which
// is what the Omo lifecycle extension reads.
type ProjectDocCatalogContext struct {
	ShouldInject bool                                `json:"should_inject"`
	ProjectDocs  []projectdoc.ProjectDocCatalogEntry `json:"project_docs,omitempty"`
	Compact      string                              `json:"compact,omitempty"`
	UserView     string                              `json:"user_view,omitempty"`
}
