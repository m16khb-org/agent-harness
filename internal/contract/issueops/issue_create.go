package issueops

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	IssueCreateIntentPending            = "pending"
	IssueCreateIntentNotInvoked         = "not_invoked"
	IssueCreateIntentInvokedUnknown     = "invoked_unknown"
	IssueCreateIntentURLObserved        = "url_observed"
	IssueCreateIntentVerificationFailed = "verification_failed"
	IssueCreateIntentReceiptFailed      = "receipt_failed"
	IssueCreateIntentCompleted          = "completed"

	MaxIssueCreateTitleBytes     = 512
	MaxIssueCreateAuthorityBytes = 512
	MaxIssueCreateURLBytes       = 2048
	MaxIssueCreateFailureBytes   = 2048
	MaxIssueCreateValues         = 64
	MaxIssueCreateValueBytes     = 256
)

type IssueOpsIssueCreateIntent struct {
	OperationID      string   `json:"operation_id"`
	Marker           string   `json:"marker"`
	Provider         string   `json:"provider"`
	ProjectAuthority string   `json:"project_authority"`
	Title            string   `json:"title"`
	BodySHA256       string   `json:"body_sha256"`
	Labels           []string `json:"labels,omitempty"`
	Assignees        []string `json:"assignees,omitempty"`
	Status           string   `json:"status"`
	Attempt          int      `json:"attempt"`
	CanonicalURL     string   `json:"canonical_url,omitempty"`
	Failure          string   `json:"failure,omitempty"`
	StartedAt        string   `json:"started_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type IssueOpsIssueCreateIntentRequest struct {
	OperationID      string
	Provider         string
	ProjectAuthority string
	Title            string
	BodySHA256       string
	Labels           []string
	Assignees        []string
	StartedAt        string
}

type IssueOpsIssueCreateOutcome struct {
	Status       string
	CanonicalURL string
	Failure      string
	ObservedAt   string
}

type IssueOpsIssueCreateReconcileResult struct {
	OK                bool                       `json:"ok"`
	CandidateCount    int                        `json:"candidate_count"`
	CandidateURL      string                     `json:"candidate_url"`
	WouldAdopt        bool                       `json:"would_adopt"`
	IssueURL          string                     `json:"issue_url"`
	IssueCreateIntent *IssueOpsIssueCreateIntent `json:"issue_create_intent"`
}

func ValidateIssueCreateIntent(intent IssueOpsIssueCreateIntent) error {
	for field, value := range map[string]string{
		"operation_id":      intent.OperationID,
		"marker":            intent.Marker,
		"provider":          intent.Provider,
		"project_authority": intent.ProjectAuthority,
		"title":             intent.Title,
		"body_sha256":       intent.BodySHA256,
		"started_at":        intent.StartedAt,
		"updated_at":        intent.UpdatedAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("issue create intent %s is required", field)
		}
	}
	if len(intent.BodySHA256) != 64 {
		return fmt.Errorf("issue create intent body_sha256 must be 64 hex characters")
	}
	if len(intent.OperationID) != 32 ||
		intent.OperationID != strings.ToLower(intent.OperationID) {
		return fmt.Errorf("issue create intent operation_id must be 32 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(intent.OperationID); err != nil {
		return fmt.Errorf("issue create intent operation_id must be 32 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(intent.BodySHA256); err != nil {
		return fmt.Errorf("issue create intent body_sha256 must be lowercase hexadecimal")
	}
	if intent.BodySHA256 != strings.ToLower(intent.BodySHA256) {
		return fmt.Errorf("issue create intent body_sha256 must be lowercase hexadecimal")
	}
	if intent.Provider != "github" && intent.Provider != "gitlab" {
		return fmt.Errorf("issue create intent provider is invalid")
	}
	if len(intent.ProjectAuthority) > MaxIssueCreateAuthorityBytes {
		return fmt.Errorf("issue create intent project_authority exceeds %d bytes", MaxIssueCreateAuthorityBytes)
	}
	if len(intent.Title) > MaxIssueCreateTitleBytes {
		return fmt.Errorf("issue create intent title exceeds %d bytes", MaxIssueCreateTitleBytes)
	}
	if len(intent.CanonicalURL) > MaxIssueCreateURLBytes {
		return fmt.Errorf("issue create intent canonical_url exceeds %d bytes", MaxIssueCreateURLBytes)
	}
	if len(intent.Failure) > MaxIssueCreateFailureBytes {
		return fmt.Errorf("issue create intent failure exceeds %d bytes", MaxIssueCreateFailureBytes)
	}
	if len(intent.StartedAt) > 64 || len(intent.UpdatedAt) > 64 {
		return fmt.Errorf("issue create intent timestamps exceed 64 bytes")
	}
	if err := validateIssueCreateValues("labels", intent.Labels); err != nil {
		return err
	}
	if err := validateIssueCreateValues("assignees", intent.Assignees); err != nil {
		return err
	}
	if intent.Marker != "<!-- agent-harness:issue-create:"+intent.OperationID+" -->" {
		return fmt.Errorf("issue create intent marker does not match operation_id")
	}
	if intent.Attempt < 1 {
		return fmt.Errorf("issue create intent attempt must be positive")
	}
	switch intent.Status {
	case IssueCreateIntentPending,
		IssueCreateIntentNotInvoked,
		IssueCreateIntentInvokedUnknown,
		IssueCreateIntentURLObserved,
		IssueCreateIntentVerificationFailed,
		IssueCreateIntentReceiptFailed,
		IssueCreateIntentCompleted:
	default:
		return fmt.Errorf("unsupported issue create intent status %q", intent.Status)
	}
	if intent.Status == IssueCreateIntentCompleted && strings.TrimSpace(intent.CanonicalURL) == "" {
		return fmt.Errorf("completed issue create intent requires canonical_url")
	}
	if (intent.Status == IssueCreateIntentPending ||
		intent.Status == IssueCreateIntentNotInvoked ||
		intent.Status == IssueCreateIntentInvokedUnknown) &&
		strings.TrimSpace(intent.CanonicalURL) != "" {
		return fmt.Errorf("issue create intent status %s must not have canonical_url", intent.Status)
	}
	if intent.Status != IssueCreateIntentPending &&
		intent.Status != IssueCreateIntentCompleted &&
		strings.TrimSpace(intent.Failure) == "" {
		return fmt.Errorf("issue create intent status %s requires failure", intent.Status)
	}
	return nil
}

func ValidateIssueCreateTransition(from, to string) error {
	allowed := false
	switch from {
	case IssueCreateIntentPending:
		allowed = to == IssueCreateIntentNotInvoked ||
			to == IssueCreateIntentInvokedUnknown ||
			to == IssueCreateIntentURLObserved ||
			to == IssueCreateIntentVerificationFailed ||
			to == IssueCreateIntentReceiptFailed ||
			to == IssueCreateIntentCompleted
	case IssueCreateIntentInvokedUnknown, IssueCreateIntentURLObserved:
		allowed = to == from ||
			to == IssueCreateIntentVerificationFailed ||
			to == IssueCreateIntentReceiptFailed ||
			to == IssueCreateIntentCompleted
	case IssueCreateIntentVerificationFailed, IssueCreateIntentReceiptFailed:
		allowed = to == from ||
			to == IssueCreateIntentVerificationFailed ||
			to == IssueCreateIntentReceiptFailed ||
			to == IssueCreateIntentCompleted
	}
	if !allowed {
		return fmt.Errorf("illegal issue create intent transition %s -> %s", from, to)
	}
	return nil
}

func validateIssueCreateValues(field string, values []string) error {
	if len(values) > MaxIssueCreateValues {
		return fmt.Errorf("issue create intent %s exceeds %d values", field, MaxIssueCreateValues)
	}
	for _, value := range values {
		if len(value) > MaxIssueCreateValueBytes {
			return fmt.Errorf("issue create intent %s value exceeds %d bytes", field, MaxIssueCreateValueBytes)
		}
	}
	return nil
}
