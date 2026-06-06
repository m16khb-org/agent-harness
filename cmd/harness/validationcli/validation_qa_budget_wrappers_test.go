package validationcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationQABudgetWrappersUseDefaultSurfaces(t *testing.T) {
	root := t.TempDir()
	writeQAGateWrapperFixture(t, root)

	redaction := ValidateRedactionAudit(root)
	if !redaction.OK || redaction.Label != "redaction audit" {
		t.Fatalf("expected redaction audit wrapper success, got %#v", redaction)
	}

	qa := ValidateQAGate(root)
	if !qa.OK || qa.Label != "QA gate" {
		t.Fatalf("expected QA gate wrapper success, got %#v", qa)
	}

	binary := writeStepBudgetFakeBinary(t, t.TempDir())
	budget := ValidateStepBudgetBaseline(binary, root, 808)
	if !budget.OK || budget.Label != "step budget baseline" {
		t.Fatalf("expected step budget wrapper success, got %#v", budget)
	}
	if !strings.Contains(budget.Command, "self-verify compare") || !strings.Contains(budget.Stdout, "step_budget:docs index smoke_p95_increased_by_30.00_pct") {
		t.Fatalf("expected step budget wrapper to exercise compare CLI surface, got command=%q stdout=%q", budget.Command, budget.Stdout)
	}
}

func writeQAGateWrapperFixture(t *testing.T, root string) {
	t.Helper()
	writeFileForWrapperTest(t, filepath.Join(root, "GENIUS_THINK.md"), "# Thinking\n\n천재적 사고\nMermaid\n\n```mermaid\ngraph TD\n  A[\"OK\"]\n```\n")
	writeFileForWrapperTest(t, filepath.Join(root, ".agent-harness", "TESTING.md"), "# Testing\n\nWell-structured tests\nPoorly-structured tests\n")
	writeFileForWrapperTest(t, filepath.Join(root, "skills", "self-augment", "SELF_AUGMENTATION.md"), "# Self Augment\n\nSelf-augmentation 95\n")
	for _, skill := range []string{"atomic-commit-push", "self-augment", "self-verify"} {
		writeFileForWrapperTest(t, filepath.Join(root, "skills", skill, "SKILL.md"), "---\nname: "+skill+"\ndescription: fixture\n---\nSelf-verification 95\n")
		writeFileForWrapperTest(t, filepath.Join(root, "skills", skill, "agents", "openai.yaml"), "name: "+skill+"\n")
	}
}

func writeStepBudgetFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	resultJSON := mustMarshalStepBudgetTest(t, validStepBudgetCompareResult())
	body := `#!/bin/sh
set -eu
case "$*" in
  self-verify\ compare*)
    printf '%s\n' '` + resultJSON + `'
    ;;
  *)
    echo "unexpected fake harness args: $*" >&2
    exit 2
    ;;
esac
`
	path := filepath.Join(dir, "fake-agent-harness")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
