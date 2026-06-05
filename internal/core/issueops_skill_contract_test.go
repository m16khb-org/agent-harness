package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsSkillRoutesPhasesToLazyCodexFeatures(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	for _, want := range []string{
		"LazyCodex/OMO Phase Assist Map",
		"deep-interview",
		"problem",
		"grill",
		"issue",
		"issue-preflight",
		"PROMPT.md",
		"ideal issue prompt",
		"raw user request",
		"ambiguity ledger",
		"plan",
		"implement",
		"ai-slop-clean",
		"feedback",
		"pr",
		"cleanup",
		"omo:ulw-plan",
		"omo:programming",
		"omo:debugging",
		"omo:lsp",
		"omo:ulw-loop",
		"omo:remove-ai-slops",
		"omo:comment-checker",
		"omo:review-work",
		"lazycodex-ai",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill missing LazyCodex phase routing contract %q", want)
		}
	}
}

func readIssueOpsSkillForTest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "skills", "issueops", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
