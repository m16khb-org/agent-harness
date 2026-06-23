package skillcontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSkillForTest(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", name, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertSkillContains(t *testing.T, skillName string, phrases []string) {
	t.Helper()
	body := readSkillForTest(t, skillName)
	for _, want := range phrases {
		if !strings.Contains(body, want) {
			t.Fatalf("%s SKILL.md missing contract phrase %q", skillName, want)
		}
	}
}

func TestKarpathySkillPinsPrivacyAndProportionalityContract(t *testing.T) {
	assertSkillContains(t, "karpathy", []string{
		// CoT privacy guardrail (the holdout-fixed boundary).
		"hidden/private chain-of-thought",
		// Tool-truth guardrail.
		"labeling them illustrative",
		// One-shot lightweight mode (proportionality).
		"One-shot / orchestration prompt",
		"Skip the formal test-suite, A/B, and versioning ceremony",
	})
}

func TestDraftWikiPromoterSkillPinsHookAndPromotionContract(t *testing.T) {
	assertSkillContains(t, "draft-wiki-promoter", []string{
		// Hook boundary.
		"Never run external LLM calls inside PostToolUse hooks",
		// Promotion gate.
		"only approved drafts may be promoted",
		// Operational-measurement fixes (DWP-O findings).
		"`queue` and `prune` subcommands",
		"Failure boundary when upstream is absent",
	})
}

func TestStabilityAuditSkillPinsSafetyModelContract(t *testing.T) {
	assertSkillContains(t, "stability-audit", []string{
		// Process-safety model (the STA-B boundary).
		"Never kill active `codex`, `claude`, `tmux`, or unrelated MCP processes",
		"evidence-first audit",
		// Operational-measurement fixes (STA-O findings).
		"compatibility alias for `bootstrap`",
		"intended dogfood setup",
	})
}

func TestBernersLeeSkillPrefersHarnessWebFetchContract(t *testing.T) {
	assertSkillContains(t, "berners-lee", []string{
		"`web_fetch_resilient`",
		"`agent-harness web-fetch fetch`",
		"Report `auth_required`, `paywalled`, `challenge`, or `blocked`",
		"Do not add host-specific fictional tools",
	})
}
