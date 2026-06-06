package mcpcli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestMCPIssueOpsRecordsIntentAndDesignReview(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intent := callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent",
		"interpreted_intent": "Persist main-agent intent before planning",
		"success_criteria":   []string{"intent is recorded"},
		"constraints":        []string{"keep state durable"},
		"ambiguities":        []string{"none"},
		"non_goals":          []string{"do not implement from hook recommendation alone"},
	})
	if _, ok := intent["intent"].(map[string]any); !ok {
		t.Fatalf("MCP intent record should persist intent payload: %#v", intent)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://github.com/example/repo/issues/1",
	})
	design := callMCPToolForIssueOpsTest(t, "issueops_review_design", map[string]any{
		"id":              id,
		"problem_summary": "IssueOps needs a design gate",
		"proposed_design": "Require approved design before implementation",
		"refactor_plan":   "Keep changes in IssueOps core and adapters",
		"alternatives":    []string{"docs-only guidance"},
		"risks":           []string{"legacy tests need explicit setup"},
		"verification":    []string{"go test ./cmd/harness/mcpcli"},
		"approved":        true,
	})
	if review, ok := design["design_review"].(map[string]any); !ok || review["approved"] != true {
		t.Fatalf("MCP design review should persist approval payload: %#v", design)
	}
}

func TestMCPIssueOpsIntentAndDesignRejectInvalidInputs(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intentErr := callMCPToolForIssueOpsTestError(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent",
		"interpreted_intent": "Persist main-agent intent before planning",
	})
	if intentErr == nil || !strings.Contains(fmt.Sprint(intentErr.Data), "success_criteria is required") {
		t.Fatalf("expected MCP intent validation error, got %+v", intentErr)
	}
	missingVerificationErr := callMCPToolForIssueOpsTestError(t, "issueops_review_design", map[string]any{
		"id":              id,
		"problem_summary": "IssueOps needs a design gate",
		"proposed_design": "Require approved design before implementation",
	})
	if missingVerificationErr == nil || !strings.Contains(fmt.Sprint(missingVerificationErr.Data), "verification is required") {
		t.Fatalf("expected MCP design missing verification error, got %+v", missingVerificationErr)
	}
	designErr := callMCPToolForIssueOpsTestError(t, "issueops_review_design", map[string]any{
		"id":              id,
		"problem_summary": "IssueOps needs a design gate",
		"proposed_design": "Require approved design before implementation",
		"verification":    []string{"go test ./cmd/harness/mcpcli"},
		"open_questions":  []string{"which design?"},
		"approved":        true,
	})
	if designErr == nil || !strings.Contains(fmt.Sprint(designErr.Data), "open_questions") {
		t.Fatalf("expected MCP design validation error, got %+v", designErr)
	}
}

func TestMCPIssueOpsApprovedDesignRequiresRefactorReviewEvidence(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent before refactoring",
		"interpreted_intent": "Persist main-agent intent and design evidence before implementation",
		"success_criteria":   []string{"approved design includes refactor evidence"},
	})
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://github.com/example/repo/issues/1",
	})
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "missing refactor plan",
			args: map[string]any{
				"id":              id,
				"problem_summary": "IssueOps needs a design gate",
				"proposed_design": "Require approved design before implementation",
				"verification":    []string{"go test ./cmd/harness/mcpcli"},
				"approved":        true,
			},
			want: "refactor_plan",
		},
		{
			name: "missing alternatives",
			args: map[string]any{
				"id":              id,
				"problem_summary": "IssueOps needs a design gate",
				"proposed_design": "Require approved design before implementation",
				"refactor_plan":   "Keep changes scoped to IssueOps core and adapters",
				"verification":    []string{"go test ./cmd/harness/mcpcli"},
				"approved":        true,
			},
			want: "alternatives",
		},
		{
			name: "missing risks",
			args: map[string]any{
				"id":              id,
				"problem_summary": "IssueOps needs a design gate",
				"proposed_design": "Require approved design before implementation",
				"refactor_plan":   "Keep changes scoped to IssueOps core and adapters",
				"alternatives":    []string{"docs-only guidance"},
				"verification":    []string{"go test ./cmd/harness/mcpcli"},
				"approved":        true,
			},
			want: "risks",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errPayload := callMCPToolForIssueOpsTestError(t, "issueops_review_design", tc.args)
			if errPayload == nil || !strings.Contains(fmt.Sprint(errPayload.Data), tc.want) {
				t.Fatalf("expected MCP design validation error %q, got %+v", tc.want, errPayload)
			}
		})
	}
}

func TestMCPIssueOpsIntentRedactsSecretLikeFreeform(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intent := callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "token=secret-value",
		"interpreted_intent": "api_key=secret-value",
		"success_criteria":   []string{"password=secret-value"},
	})
	payload, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if strings.Contains(text, "secret-value") || (!strings.Contains(text, "<redacted>") && !strings.Contains(text, `\u003credacted\u003e`)) {
		t.Fatalf("MCP intent response should redact secret-like values:\n%s", text)
	}
}
