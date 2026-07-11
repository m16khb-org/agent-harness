package skillcontract

import (
	"os"
	"os/exec"
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
		// Repo-local promotion boundary: promote never writes outside the repo.
		"it never writes outside the repo",
		"Never write into external wikis or companion tools",
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

func TestAtomicCommitPushSkillPinsStagingAndPushSafetyContract(t *testing.T) {
	assertSkillContains(t, "atomic-commit-push", []string{
		// Broad-staging guardrail.
		"Never use `git add .` or `git commit -a`",
		// Secret-blocker guardrail.
		"as blockers until inspected or excluded",
		// Force-push guardrail.
		"Never force-push unless explicitly requested",
	})
}

func TestGitlabUsecaseSkillPinsAssigneeContract(t *testing.T) {
	assertSkillContains(t, "gitlab-usecase", []string{
		// Concrete-assignee guardrail (no `@me` placeholder).
		"Do not use `@me`",
	})
}

func TestSelfVerifySkillPinsGateContract(t *testing.T) {
	body := readSkillForTest(t, "self-verify")
	for _, want := range []string{
		// QA-gate boundary: this loop does not pick improvements itself.
		"This skill is a QA gate; it does not choose improvements by itself.",
		// Promote safety: confirmed promote refuses a failed source snapshot.
		"Confirmed promote refuses a source snapshot that did not pass the gate",
		// Runtime contract: the opt-in mode renders evidence but cannot complete
		// an external judgement in the current implementation.
		"only renders the read-only evaluator prompt",
		"No Z.AI request is sent",
		"`gate` therefore returns a non-passing `llm_eval` result",
		"pass explicit `--llm-eval=false`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("self-verify SKILL.md missing contract phrase %q", want)
		}
	}
	for _, hostSpecificRecipe := range []string{
		"./cmd/harness/hookcli/hookinput",
		"bun scripts/smoke-gjc-native-hook.ts",
		"Do not use a literal `--host gjc` grep",
	} {
		if strings.Contains(body, hostSpecificRecipe) {
			t.Fatalf("self-verify SKILL.md must keep host-specific handoff recipe in IssueOps/Turing: %q", hostSpecificRecipe)
		}
	}
	if strings.Contains(body, "to run the Z.AI Coding Plan") {
		t.Fatal("self-verify SKILL.md must not claim that prompt-only evaluation invokes Z.AI")
	}
}

func TestVerificationDocsPinHandoffProbeCommands(t *testing.T) {
	for _, relPath := range []string{
		filepath.Join(".agent-harness", "TESTING.md"),
		filepath.Join(".agent-harness", "operations", "verification.md"),
	} {
		body, err := os.ReadFile(filepath.Join("..", "..", "..", relPath))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"./cmd/harness/hookcli/hookinput",
			"bun scripts/smoke-gjc-native-hook.ts",
		} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("%s missing verification probe %q", relPath, want)
			}
		}
		for _, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, "go test ") && strings.Contains(line, "./internal/core/hookinput") {
				t.Fatalf("%s must not execute nonexistent hookinput package: %s", relPath, line)
			}
		}
	}
}

func TestSelfAugmentSkillPinsImplementationContract(t *testing.T) {
	assertSkillContains(t, "self-augment", []string{
		// Augmentation must produce a real change, not a report.
		"A report-only analysis or test-only run is not enough.",
		// Cosmetic-only edits do not satisfy the loop.
		"Cosmetic-only changes do not count.",
	})
}

func TestProjectBootstrapSkillPinsSafetyContract(t *testing.T) {
	assertSkillContains(t, "project-bootstrap", []string{
		// Never clobber an existing AGENTS.md.
		"Never overwrite an existing `AGENTS.md` wholesale.",
		// Generated docs are evidence-backed drafts, not authoritative.
		"Treat generated docs as evidence-backed drafts.",
	})
}

// TestAllSkillsFrontmatterValidates closes the gap where only a handful of
// skills were pinned by phrase: it runs scripts/validate-skill.py over every
// directory under skills/ so every SKILL.md's frontmatter is validated on each
// `go test` run (including CI's `go test ./...`).
func TestAllSkillsFrontmatterValidates(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 not available: %v", err)
	}
	repoRoot := filepath.Join("..", "..", "..")
	validator := filepath.Join(repoRoot, "scripts", "validate-skill.py")
	if _, err := os.Stat(validator); err != nil {
		t.Fatalf("validate-skill.py not found: %v", err)
	}
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(skillsDir, entry.Name())
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
			continue
		}
		checked++
		out, err := exec.Command(python, validator, skillDir).CombinedOutput()
		if err != nil {
			t.Errorf("validate-skill.py failed for skills/%s: %v\n%s", entry.Name(), err, out)
		}
	}
	if checked == 0 {
		t.Fatal("no skills/* directory with a SKILL.md was validated")
	}
}
