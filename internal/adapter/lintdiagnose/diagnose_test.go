package lintdiagnose

import (
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	"strings"
	"testing"
)

func TestDiagnoseCommandRendersPromptForFailedCommand(t *testing.T) {
	root := t.TempDir()

	result, err := DiagnoseCommand(lintdiagnosecontract.LintDiagnoseRequest{
		RepoRoot:    root,
		CommandArgv: []string{"/bin/sh", "-c", "echo lint failed >&2; exit 2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Failed {
		t.Fatalf("expected failed command result: %+v", result)
	}
	for _, want := range []string{"Execution Failure Output", "lint failed", "diagnosis"} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, result.Prompt)
		}
	}
}
