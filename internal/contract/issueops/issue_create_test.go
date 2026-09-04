package issueops

import (
	"strings"
	"testing"
)

func TestValidateIssueCreateTransitionRejectsRetryEnablingDowngrade(t *testing.T) {
	if err := ValidateIssueCreateTransition(IssueCreateIntentPending, IssueCreateIntentInvokedUnknown); err != nil {
		t.Fatalf("pending -> invoked_unknown: %v", err)
	}
	if err := ValidateIssueCreateTransition(IssueCreateIntentInvokedUnknown, IssueCreateIntentVerificationFailed); err != nil {
		t.Fatalf("invoked_unknown -> verification_failed: %v", err)
	}
	if err := ValidateIssueCreateTransition(IssueCreateIntentInvokedUnknown, IssueCreateIntentNotInvoked); err == nil {
		t.Fatal("invoked_unknown -> not_invoked must be rejected")
	}
	if err := ValidateIssueCreateTransition(IssueCreateIntentVerificationFailed, IssueCreateIntentInvokedUnknown); err == nil {
		t.Fatal("verification_failed -> invoked_unknown must be rejected")
	}
}

func TestValidateIssueCreateIntentRejectsUnboundedDurableFields(t *testing.T) {
	valid := IssueOpsIssueCreateIntent{
		OperationID:      "0123456789abcdef0123456789abcdef",
		Marker:           "<!-- issueops:issue-create:0123456789abcdef0123456789abcdef -->",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Title",
		BodySHA256:       strings.Repeat("a", 64),
		Status:           IssueCreateIntentInvokedUnknown,
		Attempt:          1,
		Failure:          "timeout",
		StartedAt:        "2026-08-14T00:00:00Z",
		UpdatedAt:        "2026-08-14T00:01:00Z",
	}
	if err := ValidateIssueCreateIntent(valid); err != nil {
		t.Fatalf("valid intent: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*IssueOpsIssueCreateIntent)
	}{
		{"title", func(intent *IssueOpsIssueCreateIntent) {
			intent.Title = strings.Repeat("t", MaxIssueCreateTitleBytes+1)
		}},
		{"failure", func(intent *IssueOpsIssueCreateIntent) {
			intent.Failure = strings.Repeat("f", MaxIssueCreateFailureBytes+1)
		}},
		{"canonical URL", func(intent *IssueOpsIssueCreateIntent) {
			intent.Status = IssueCreateIntentVerificationFailed
			intent.CanonicalURL = "https://github.com/acme/repo/issues/" + strings.Repeat("1", MaxIssueCreateURLBytes)
		}},
		{"labels", func(intent *IssueOpsIssueCreateIntent) { intent.Labels = make([]string, MaxIssueCreateValues+1) }},
		{"label value", func(intent *IssueOpsIssueCreateIntent) {
			intent.Labels = []string{strings.Repeat("l", MaxIssueCreateValueBytes+1)}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := valid
			test.mutate(&intent)
			if err := ValidateIssueCreateIntent(intent); err == nil {
				t.Fatal("expected durable bound error")
			}
		})
	}
}
