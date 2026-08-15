package skillcontract

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"fmt"
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
		if !strings.Contains(body, want) && !strings.Contains(strings.ReplaceAll(body, "`", ""), strings.ReplaceAll(want, "`", "")) {
			t.Fatalf("%s SKILL.md missing contract phrase %q", skillName, want)
		}
	}
}

func readRepoFileForTest(t *testing.T, relPath string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", relPath))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestP1PioneerCorrectnessContracts(t *testing.T) {
	assertSkillContains(t, "berners-lee", []string{
		"high-volume-exploration",
		"devils-advocate-review",
		"parallel-independent-research",
		"cross-verification-consensus",
	})
	assertSkillContains(t, "boehm", []string{
		"verified high-coverage analysis",
		"full-page-ocr",
		"region-ocr",
		"claimed-but-unverified",
		"validate_analysis_report.py",
	})
	assertSkillContains(t, "brooks", []string{
		"## IssueOps Integration",
		"agent-harness issueops devils-advocate review",
		"issueops_record_devils_advocate_review",
	})
	assertSkillContains(t, "karpathy", []string{
		"Shannon measures generated code artifacts, not prompt quality.",
	})
	assertSkillContains(t, "turing", []string{
		"skills/issueops/references/execution.md",
		"current host's available browser tool",
		"AppleScript on macOS",
		"`xdotool` on Linux only",
	})
	turing := readSkillForTest(t, "turing")
	if strings.Contains(turing, "Chrome / agent-browser") {
		t.Fatal("turing SKILL.md must not name the nonexistent agent-browser tool")
	}
	rebase := readRepoFileForTest(t, filepath.Join("skills", "torvalds", "references", "rebase-protocol.md"))
	for _, want := range []string{"Backup refs persist until explicitly deleted.", "git branch -D <backup-ref>"} {
		if !strings.Contains(rebase, want) {
			t.Fatalf("rebase protocol missing P1 retention phrase %q", want)
		}
	}
	vonNeumann := readSkillForTest(t, "von-neumann")
	if strings.Contains(vonNeumann, "task(subagent_type=") {
		t.Fatal("von-neumann SKILL.md must not prescribe a host-specific task pseudo-API")
	}
	if !strings.Contains(vonNeumann, "current host's delegation tool") {
		t.Fatal("von-neumann SKILL.md must use host-neutral delegation wording")
	}
	assertSkillContains(t, "hopper", []string{"Four Strategies", "self-verify-progress-heartbeat", "Strategy D: Snapshot/Golden Diff"})
	dijkstra := readSkillForTest(t, "dijkstra")
	if !strings.Contains(dijkstra, "```text\n   Equivalent in any language") {
		t.Fatal("dijkstra SKILL.md must keep scaling-test interpretation inside its fenced block")
	}

	fixtures, err := issueopscore.LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	pioneerCount := 0
	for _, fixture := range fixtures {
		if fixture.PioneerSkillTarget != "" {
			pioneerCount++
		}
	}
	dashboard := readRepoFileForTest(t, filepath.Join(".agent-harness", "operations", "quality-dashboard.md"))
	for _, want := range []string{
		"historical 2026-06-16 isolated-rubric cohort: 9 skills",
		fmt.Sprintf("current IssueOps benchmark fixture loader: %d pioneer-targeted fixtures", pioneerCount),
	} {
		if !strings.Contains(dashboard, want) {
			t.Fatalf("quality dashboard missing P1 count contract %q", want)
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

func TestStabilityAuditSkillPinsSafetyModelContract(t *testing.T) {
	assertSkillContains(t, "stability-audit", []string{
		// Process-safety model (the STA-B boundary).
		"Never kill active `codex`, `claude`, `tmux`, or unrelated MCP processes",
		"evidence-first audit",
		// Operational-measurement fixes (STA-O findings).
		"`./bin/agent-harness install --dry-run --json`",
		"`./bin/agent-harness install --json` only for full install tasks",
		"intended dogfood setup",
		"exact current-v1 state write/read/doctor",
	})
	body := readSkillForTest(t, "stability-audit")
	for _, retired := range []string{
		strings.Join([]string{"state", "migrate"}, " "),
		strings.Join([]string{"agent-harness", "install-native"}, " "),
	} {
		if strings.Contains(body, retired) {
			t.Fatalf("stability-audit skill still instructs agents to run retired command %q", retired)
		}
	}
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

func TestGitLabSnapshotSkillsPinPortableVCSContract(t *testing.T) {
	assertSkillContains(t, "gitlab-usecase", []string{
		".agent-harness/VCS.md",
		"glab_api",
		"flags.hostname",
		"server namespace",
		"개인 wrapper",
		"project_docs_read",
		"project_docs_update",
		"glab api",
		"successful exact-identity MCP evidence를 얻지 못했을 때만",
		"이미 공급한 invalid evidence는 CLI fallback하지 않고 fail-closed한다.",
		"OpenWiki 자동 update",
	})
	execution := readRepoFileForTest(t, filepath.Join("skills", "issueops", "references", "execution.md"))
	for _, want := range []string{
		".agent-harness/VCS.md",
		"glab_api",
		"flags.hostname",
		"issue_snapshot",
		"--issue-snapshot-file",
		"glab_mcp",
		"glab_cli",
		"project_docs_read",
		"project_docs_update",
		"glab api",
		"successful exact-identity MCP evidence를 얻지 못했을 때만",
		"이미 공급한 invalid evidence는 CLI fallback하지 않고 fail-closed한다.",
		"OpenWiki 자동 update",
	} {
		if !strings.Contains(execution, want) {
			t.Fatalf("IssueOps execution reference missing portable snapshot contract %q", want)
		}
	}
	for _, relPath := range []string{
		filepath.Join("skills", "gitlab-usecase", "SKILL.md"),
		filepath.Join("skills", "issueops", "references", "execution.md"),
		filepath.Join(".agent-harness", "OPERATIONS.md"),
		filepath.Join(".agent-harness", "AGENT_WORKFLOW.md"),
	} {
		body := readRepoFileForTest(t, relPath)
		privateHome := "/Users/" + "ha" + "bin"
		for _, privateIdentity := range []string{privateHome, "glab-mcp-wrapper"} {
			if strings.Contains(body, privateIdentity) {
				t.Fatalf("%s must not hardcode private VCS tool identity %q", relPath, privateIdentity)
			}
		}
	}
}

func TestSelfVerifySkillPinsGateContract(t *testing.T) {
	body := readSkillForTest(t, "self-verify")
	for _, want := range []string{
		"First-party hosts are exactly Codex, Claude Code, and Omo native.",
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
	} {
		if strings.Contains(body, hostSpecificRecipe) {
			t.Fatalf("self-verify SKILL.md must keep host-specific handoff recipe in IssueOps/Turing: %q", hostSpecificRecipe)
		}
	}
	if strings.Contains(body, "to run the Z.AI Coding Plan") {
		t.Fatal("self-verify SKILL.md must not claim that prompt-only evaluation invokes Z.AI")
	}
	assertRetiredHostsAbsent(t, "self-verify SKILL.md", body)
}

func TestVerificationDocsPinHandoffProbeCommands(t *testing.T) {
	testingIndex := readRepoFileForTest(t, filepath.Join(".agent-harness", "TESTING.md"))
	if !strings.Contains(testingIndex, "testing/issueops-execution.md") {
		t.Fatal(".agent-harness/TESTING.md must route handoff probes to testing/issueops-execution.md")
	}
	for _, relPath := range []string{
		filepath.Join(".agent-harness", "testing", "issueops-execution.md"),
		filepath.Join(".agent-harness", "operations", "verification.md"),
	} {
		body := readRepoFileForTest(t, relPath)
		for _, want := range []string{
			"./cmd/harness/hookcli/hookinput",
			"Codex",
			"Claude",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing verification probe %q", relPath, want)
			}
		}
		for line := range strings.SplitSeq(body, "\n") {
			if strings.Contains(line, "go test ") && strings.Contains(line, "./internal/core/hookinput") {
				t.Fatalf("%s must not execute nonexistent hookinput package: %s", relPath, line)
			}
		}
		assertRetiredHostsAbsent(t, relPath, body)
	}
}

func TestTuringSkillPinsThreeHostExecutionContract(t *testing.T) {
	body := readSkillForTest(t, "turing")
	if !strings.Contains(body, "First-party hosts are exactly Codex, Claude Code, and Omo native.") {
		t.Fatal("turing SKILL.md must state the exact first-party host set")
	}
	assertRetiredHostsAbsent(t, "turing SKILL.md", body)
}

func assertRetiredHostsAbsent(t *testing.T, name, body string) {
	t.Helper()
	for _, host := range []string{strings.Join([]string{"g", "jc"}, ""), strings.Join([]string{"reason", "ix"}, "")} {
		if strings.Contains(strings.ToLower(body), host) {
			t.Fatalf("%s retains retired host %q", name, host)
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

func TestAllSkillShellFencesValidate(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 not available: %v", err)
	}
	repoRoot := filepath.Join("..", "..", "..")
	validator := filepath.Join(repoRoot, "scripts", "verify-skill-shell.py")
	skillsDir := filepath.Join(repoRoot, "skills")
	out, err := exec.Command(python, validator, skillsDir).CombinedOutput()
	if err != nil {
		t.Fatalf("verify-skill-shell.py failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "skill shell verification passed") {
		t.Fatalf("unexpected verifier output: %s", out)
	}
}
