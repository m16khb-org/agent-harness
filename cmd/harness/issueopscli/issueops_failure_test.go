package issueopscli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRunIssueOpsLinkChildRequiresLiveIssue(t *testing.T) {
	stubIssueOpsChildIssueVerifier(t, func(_ string) error {
		return errors.New("child issue not found")
	})
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "child-live")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-child-live", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	if err := runIssueOps([]string{"link-child", "--id", id, "--child-url", "https://github.com/example/repo/issues/2", "--title", "missing child", "--json"}); err == nil || !strings.Contains(err.Error(), "child issue not found") {
		t.Fatalf("link-child should require live child issue verification, got %v", err)
	}
}

func TestRunIssueOpsPhaseFailureWithJSONEmitsStructuredError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "json-failure")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "456-provider-linked-branch", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"phase", "--id", id, "--to", "pr", "--json"})
	})
	if err == nil {
		t.Fatalf("pr phase without readiness should still return an error")
	}
	var failure map[string]any
	if unmarshalErr := json.Unmarshal([]byte(out), &failure); unmarshalErr != nil {
		t.Fatalf("phase failure with --json should emit JSON stdout: %v\n%s", unmarshalErr, out)
	}
	errorText, _ := failure["error"].(string)
	if failure["ok"] != false || !strings.Contains(errorText, "cannot enter pr phase") {
		t.Fatalf("unexpected structured failure payload: %#v", failure)
	}
}

func TestRunIssueOpsIntentAndDesignFailuresWithJSONEmitStructuredErrors(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "intent-design-json-failure")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-intent-design", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	intentOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{
			"intent", "record",
			"--id", id,
			"--raw-request", "refactor issueops flow",
			"--interpreted-intent", "keep intent evidence",
			"--json",
		})
	})
	if err == nil {
		t.Fatalf("intent record without success criteria should fail")
	}
	assertIssueOpsStructuredFailure(t, intentOut, "success_criteria is required")

	recordIssueOpsCLIIntentForTest(t, id)
	designOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{
			"design", "review",
			"--id", id,
			"--problem-summary", "IssueOps needs a design gate",
			"--proposed-design", "Require approved design before implementation",
			"--open-question", "which design?",
			"--approved",
			"--json",
		})
	})
	if err == nil {
		t.Fatalf("approved design with open questions and missing verification should fail")
	}
	assertIssueOpsStructuredFailure(t, designOut, "verification is required")

	openQuestionOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{
			"design", "review",
			"--id", id,
			"--problem-summary", "IssueOps needs a design gate",
			"--proposed-design", "Require approved design before implementation",
			"--verification", "go test ./cmd/harness/issueopscli",
			"--open-question", "which design?",
			"--approved",
			"--json",
		})
	})
	if err == nil {
		t.Fatalf("approved design with open questions should fail")
	}
	assertIssueOpsStructuredFailure(t, openQuestionOut, "open_questions")
}

func TestRunIssueOpsIntentRedactsSecretLikeFreeform(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "intent-redaction")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-intent-redaction", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"intent", "record",
			"--id", id,
			"--raw-request", "token=secret-value",
			"--interpreted-intent", "api_key=secret-value",
			"--success-criteria", "password=secret-value",
			"--json",
		})
	})
	if strings.Contains(out, "secret-value") || (!strings.Contains(out, "<redacted>") && !strings.Contains(out, `\u003credacted\u003e`)) {
		t.Fatalf("intent JSON should redact secret-like freeform values:\n%s", out)
	}
}
