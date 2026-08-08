package smoke

import (
	inspect "agent-harness/internal/adapter/inspect"
	docs "agent-harness/internal/contract/docs"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateInspectWithDepsCoversCommandAndContractBranches(t *testing.T) {
	root := t.TempDir()
	calls := []string{}
	runner := func(dir, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		if dir != root || label != "inspect smoke" || timeout != 30*time.Second || stdin != "" {
			t.Fatalf("unexpected inspect command envelope: dir=%q label=%q timeout=%s stdin=%q", dir, label, timeout, stdin)
		}
		out, err := json.Marshal(inspect.InspectInfo{
			OK:     true,
			Skills: []inspect.SkillInfo{{Name: "self-verify", HasSkillMD: true}},
			Integration: inspect.IntegrationStatus{
				ProjectClaudeMCPConfig: true,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return StepResult{Label: label, Command: calls[len(calls)-1], OK: true, Stdout: string(out)}
	}
	step := validateInspectWithDeps("bin/agent-harness", root, runner)
	if !step.OK || len(calls) != 1 || calls[0] != "bin/agent-harness inspect --json" {
		t.Fatalf("unexpected inspect success: step=%+v calls=%v", step, calls)
	}

	failed := validateInspectWithDeps("bin", root, func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "inspect smoke", OK: false, Error: "boom"}
	})
	if failed.OK || failed.Error != "boom" {
		t.Fatalf("unexpected failed inspect passthrough: %+v", failed)
	}

	badJSON := validateInspectWithDeps("bin", root, func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "inspect smoke", OK: true, Stdout: "{"}
	})
	if badJSON.OK || badJSON.Error == "" {
		t.Fatalf("expected inspect JSON parse failure, got %+v", badJSON)
	}

	missing := validateInspectWithDeps("bin", root, func(string, string, time.Duration, string, string, ...string) StepResult {
		out, err := json.Marshal(inspect.InspectInfo{OK: false})
		if err != nil {
			t.Fatal(err)
		}
		return StepResult{Label: "inspect smoke", OK: true, Stdout: string(out)}
	})
	if missing.OK || !strings.Contains(missing.Error, "inspect ok=false") || !strings.Contains(missing.Error, "no skills listed") || !strings.Contains(missing.Error, "project Claude MCP config missing") {
		t.Fatalf("unexpected inspect contract failure: %+v", missing)
	}
}

// Truncated captures must fail with an explicit truncation error BEFORE any
// JSON parse attempt — never with a misleading "invalid character" decode
// error (the docs index outgrew the old 32KB budget and broke the 95-gate).
func TestValidateSmokeStepsRejectTruncatedCapturesExplicitly(t *testing.T) {
	root := t.TempDir()
	truncatedRunner := func(label string) validationCommandRunner {
		return func(string, string, time.Duration, string, string, ...string) StepResult {
			return StepResult{
				Label:           label,
				OK:              true,
				Stdout:          "[truncated: original_bytes=38544 omitted_bytes=5829]\nrest-of-tail",
				StdoutBytes:     38544,
				StdoutTruncated: true,
			}
		}
	}
	for _, tc := range []struct {
		name string
		run  func() StepResult
	}{
		{name: "inspect", run: func() StepResult {
			return validateInspectWithDeps("bin", root, truncatedRunner("inspect smoke"))
		}},
		{name: "docs index", run: func() StepResult {
			return validateDocsIndexWithDeps("bin", root, truncatedRunner("docs index smoke"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := tc.run()
			if step.OK {
				t.Fatalf("truncated capture must fail the step: %+v", step)
			}
			if !strings.Contains(step.Error, "truncated") || !strings.Contains(step.Error, "budget") {
				t.Fatalf("expected explicit truncation error, got %q", step.Error)
			}
			if strings.Contains(step.Error, "invalid character") {
				t.Fatalf("must not surface a misleading JSON decode error: %q", step.Error)
			}
		})
	}
}

// The smoke budget must comfortably exceed the live docs index (38KB and
// growing); pin the floor so a future "optimization" cannot reintroduce the
// truncate-then-parse failure.
func TestValidateSmokeOutputBudgetCoversDocsIndexGrowth(t *testing.T) {
	if commandOutputBudgetBytes < 4*1024*1024 {
		t.Fatalf("smoke output budget %d is below the 4MB floor", commandOutputBudgetBytes)
	}
}

func TestValidateSmokeWrappersUseExecutableSurface(t *testing.T) {
	root := t.TempDir()
	binary := writeValidationSmokeFakeBinary(t, root, root)

	for _, tc := range []struct {
		name string
		run  func() StepResult
	}{
		{name: "inspect", run: func() StepResult { return validateInspect(binary, root) }},
		{name: "docs index", run: func() StepResult { return validateDocsIndex(binary, root) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if step := tc.run(); !step.OK {
				t.Fatalf("expected wrapper success, got %+v", step)
			}
		})
	}
}

func writeValidationSmokeFakeBinary(t *testing.T, dir, root string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	inspect, err := json.Marshal(inspect.InspectInfo{
		OK:     true,
		Skills: []inspect.SkillInfo{{Name: "self-verify", HasSkillMD: true}},
		Integration: inspect.IntegrationStatus{
			ProjectClaudeMCPConfig: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := json.Marshal(docs.DocsIndexResult{OK: true, HarnessRoot: root, Docs: []docs.DocIndexInfo{
		{RelPath: "AGENTS.md", Title: "Agents"},
		{RelPath: "CLAUDE.md", Title: "Claude"},
		{RelPath: "GENIUS_THINK.md", Title: "Genius"},
		{RelPath: ".agent-harness/COMMIT_POLICY.md", Title: "Commit"},
		{RelPath: "skills/self-augment/SELF_AUGMENTATION.md", Title: "Augment"},
		{RelPath: "skills/self-verify/SKILL.md", Title: "Verify"},
		{RelPath: ".agent-harness/OPERATIONS.md", Title: "Ops"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nset -eu\ncase \"$*\" in\n" +
		"  \"inspect --json\") printf '%s\\n' '" + string(inspect) + "' ;;\n" +
		"  \"docs --json\") printf '%s\\n' '" + string(docs) + "' ;;\n" +
		"  *) echo \"unexpected fake harness args: $*\" >&2; exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateDocsIndexWithDepsCoversCommandAndContractBranches(t *testing.T) {
	root := t.TempDir()
	wantDocs := []docs.DocIndexInfo{
		{RelPath: "AGENTS.md", Title: "Agents"},
		{RelPath: "CLAUDE.md", Title: "Claude"},
		{RelPath: "GENIUS_THINK.md", Title: "Genius"},
		{RelPath: ".agent-harness/COMMIT_POLICY.md", Title: "Commit"},
		{RelPath: "skills/self-augment/SELF_AUGMENTATION.md", Title: "Augment"},
		{RelPath: "skills/self-verify/SKILL.md", Title: "Verify"},
		{RelPath: ".agent-harness/OPERATIONS.md", Title: "Ops"},
	}
	calls := []string{}
	runner := func(dir, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		if dir != root || label != "docs index smoke" || timeout != 30*time.Second || stdin != "" {
			t.Fatalf("unexpected docs command envelope: dir=%q label=%q timeout=%s stdin=%q", dir, label, timeout, stdin)
		}
		out, err := json.Marshal(docs.DocsIndexResult{OK: true, HarnessRoot: root, Docs: wantDocs})
		if err != nil {
			t.Fatal(err)
		}
		return StepResult{Label: label, Command: calls[len(calls)-1], OK: true, Stdout: string(out)}
	}
	step := validateDocsIndexWithDeps("bin/agent-harness", root, runner)
	if !step.OK || len(calls) != 1 || calls[0] != "bin/agent-harness docs --json" {
		t.Fatalf("unexpected docs success: step=%+v calls=%v", step, calls)
	}
	if !docIndexContains(wantDocs, "AGENTS.md") || docIndexContains(wantDocs, "missing.md") {
		t.Fatalf("unexpected docIndexContains behavior")
	}

	badJSON := validateDocsIndexWithDeps("bin", root, func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "docs index smoke", OK: true, Stdout: "{"}
	})
	if badJSON.OK || badJSON.Error == "" {
		t.Fatalf("expected docs JSON parse failure, got %+v", badJSON)
	}

	missing := validateDocsIndexWithDeps("bin", root, func(string, string, time.Duration, string, string, ...string) StepResult {
		out, err := json.Marshal(docs.DocsIndexResult{
			OK:          false,
			HarnessRoot: root + "-other",
			Docs:        []docs.DocIndexInfo{{RelPath: "AGENTS.md", Title: ""}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return StepResult{Label: "docs index smoke", OK: true, Stdout: string(out)}
	})
	if missing.OK || !strings.Contains(missing.Error, "docs index ok=false") || !strings.Contains(missing.Error, "docs index harness root mismatch") || !strings.Contains(missing.Error, "missing doc CLAUDE.md") || !strings.Contains(missing.Error, "missing title for AGENTS.md") {
		t.Fatalf("unexpected docs contract failure: %+v", missing)
	}
}
