package issueopsdecision

import (
	"fmt"
	"strings"
	"time"

	issueopsdecisioncontract "agent-harness/internal/contract/issueopsdecision"
	"agent-harness/internal/domain/secretdetection"
)

func Build(
	request issueopsdecisioncontract.Request,
	now time.Time,
) (issueopsdecisioncontract.Decision, error) {
	kind := strings.TrimSpace(request.Kind)
	if !validKind(kind) {
		return issueopsdecisioncontract.Decision{}, fmt.Errorf(
			"invalid decision kind %q; must be one of: product, architecture, implementation, test, review, scope, follow-up",
			kind,
		)
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return issueopsdecisioncontract.Decision{}, fmt.Errorf("decision title is required")
	}
	if len(title) > 512 {
		return issueopsdecisioncontract.Decision{}, fmt.Errorf(
			"decision title must not exceed 512 bytes",
		)
	}
	body := strings.TrimSpace(request.Body)
	if body == "" {
		return issueopsdecisioncontract.Decision{}, fmt.Errorf("decision body is required")
	}
	if len(body) > 65536 {
		return issueopsdecisioncontract.Decision{}, fmt.Errorf(
			"decision body must not exceed 64 KiB",
		)
	}
	if secretdetection.Contains(body) {
		return issueopsdecisioncontract.Decision{}, fmt.Errorf(
			"decision body appears to contain secrets or credentials; redact them before storing",
		)
	}
	for _, artifact := range request.AffectedArtifacts {
		if !validArtifact(artifact) {
			return issueopsdecisioncontract.Decision{}, fmt.Errorf(
				"invalid affected artifact %q",
				artifact,
			)
		}
	}
	return issueopsdecisioncontract.Decision{
		Title:              title,
		Body:               body,
		Kind:               kind,
		Rationale:          strings.TrimSpace(request.Rationale),
		Alternatives:       stableStrings(request.Alternatives),
		AffectedIssueLinks: stableStrings(request.AffectedIssueLinks),
		AffectedArtifacts:  stableStrings(request.AffectedArtifacts),
		CreatedAt:          now.UTC().Format(time.RFC3339Nano),
	}, nil
}

func validKind(kind string) bool {
	switch kind {
	case "product", "architecture", "implementation", "test", "review", "scope", "follow-up":
		return true
	default:
		return false
	}
}

func validArtifact(artifact string) bool {
	switch artifact {
	case "issue", "plan", "test", "implementation", "review", "pr_mr", "follow-up":
		return true
	default:
		return false
	}
}

func stableStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string(nil), values...)
}
