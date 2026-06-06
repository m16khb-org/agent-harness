package main

import (
	"testing"
)

func TestLintMermaidBlocksEnforcesGeniusThinkRules(t *testing.T) {
	good := "```mermaid\nflowchart LR\n    A[\"한글 노드<br/>설명\"] --> B[\"Next\"]\n    subgraph \"계획 레이어\"\n    end\n```\n"
	if issues := lintMermaidBlocks("good.md", good); len(issues) != 0 {
		t.Fatalf("valid mermaid was rejected: %+v", issues)
	}

	bad := "```mermaid\nflowchart LR\n    A[한글 노드<br>설명] --> B[Next]\n    subgraph 계획 레이어\n    end\n```\n"
	issues := lintMermaidBlocks("bad.md", bad)
	for _, want := range []string{"bad.md:3 mermaid uses <br>; use <br/>", "bad.md:3 mermaid node text must start with a quote", "bad.md:4 mermaid subgraph title must be quoted"} {
		if !containsString(issues, want) {
			t.Fatalf("missing %q in issues: %+v", want, issues)
		}
	}

	documentedBadExample := "## 잘못된 예시 (파싱 에러 발생)\n\n```mermaid\nflowchart LR\n    A[한글 노드<br>설명]\n```\n"
	if issues := lintMermaidBlocks("genius.md", documentedBadExample); len(issues) != 0 {
		t.Fatalf("documented bad example should be ignored: %+v", issues)
	}
}
