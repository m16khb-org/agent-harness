package issueopscli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"agent-harness/internal/core"
)

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func captureProjectCLIStderr(fn func() error) (string, error) {
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()
	os.Stderr = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		return "", closeErr
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return "", err
	}
	return out.String(), callErr
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func recordIssueOpsCLIIntentForTest(t *testing.T, id string) {
	t.Helper()
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"intent", "record",
			"--id", id,
			"--raw-request", "refactor issueops flow",
			"--interpreted-intent", "keep intent and design evidence before implementation",
			"--success-criteria", "intent is recorded",
			"--success-criteria", "design is reviewed",
			"--json",
		})
	})
}

func recordIssueOpsCLIDesignForTest(t *testing.T, id string) {
	t.Helper()
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"design", "review",
			"--id", id,
			"--problem-summary", "IssueOps must preserve the work contract",
			"--proposed-design", "Gate implementation on a reviewed design contract",
			"--refactor-plan", "Keep IssueOps state and adapter changes scoped to the active cycle",
			"--alternative", "documentation-only guidance",
			"--risk", "legacy tests must create explicit design evidence",
			"--verification", "go test ./cmd/harness/issueopscli",
			"--approved",
			"--json",
		})
	})
}

func recordIssueOpsCoreIntentForCLITest(t *testing.T, id string) {
	t.Helper()
	if _, err := core.RecordIssueOpsIntent(core.IssueOpsStateRoot(), id, core.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCoreDesignForCLITest(t *testing.T, id string) {
	t.Helper()
	if _, err := core.RecordIssueOpsDesignReview(core.IssueOpsStateRoot(), id, core.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		RefactorPlan:   "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:   []string{"documentation-only guidance"},
		Risks:          []string{"legacy tests must create explicit design evidence"},
		Verification:   []string{"go test ./cmd/harness/issueopscli"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}
