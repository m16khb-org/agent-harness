package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC-07: go:embed 템플릿과 prompt-engineering 설계 문서의 PROMPT 블록은 byte 단위로
// 일치해야 한다. 이 parity가 없으면 문서와 실제 dispatch prompt가 조용히
// 갈라진다(design-review 1차 리뷰가 지적한 drift 리스크).
func TestExecutionOwnerPromptMatchesKarpathyDoc(t *testing.T) {
	docPath := filepath.Join("..", "..", "..", ".issueops", "prompt-engineering", "prompts", "issueops-v1-owner-execution-v1.md")
	doc, err := os.ReadFile(docPath)
	if err != nil {
		t.Skipf("prompt-engineering prompt doc unavailable: %v", err)
	}
	const fence = "```text\n"
	start := strings.Index(string(doc), fence)
	if start < 0 {
		t.Fatal("prompt-engineering doc has no ```text prompt block")
	}
	rest := string(doc)[start+len(fence):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("prompt-engineering doc prompt block is unterminated")
	}
	docPrompt := strings.TrimRight(rest[:end], "\n")
	embedded := strings.TrimRight(executionOwnerPromptTemplate, "\n")
	if docPrompt != embedded {
		t.Fatalf("owner prompt template drifted from the prompt-engineering doc PROMPT block:\n--- doc ---\n%s\n--- embed ---\n%s", firstDiffContext(docPrompt, embedded), firstDiffContext(embedded, docPrompt))
	}
}

func firstDiffContext(a, b string) string {
	limit := min(len(a), len(b))
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			lo := max(0, i-80)
			hi := min(len(a), i+80)
			return a[lo:hi]
		}
	}
	if len(a) > limit {
		return a[max(0, limit-80):]
	}
	return "(prefix identical; length differs)"
}
