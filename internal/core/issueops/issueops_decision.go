package issueops

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token|password|passwd|credential|private[_-]?key)\s*[:=]\s*\S+`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(ghp|gho|ghu|ghs|ghr|glpat|gldt|glft)_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`),
}

func containsSecretPattern(s string) bool {
	for _, pat := range secretPatterns {
		if pat.MatchString(s) {
			return true
		}
	}
	return false
}

var validDecisionKinds = map[string]bool{
	"product":        true,
	"architecture":   true,
	"implementation": true,
	"test":           true,
	"review":         true,
	"scope":          true,
	"follow-up":      true,
}

var validDecisionArtifacts = map[string]bool{
	"issue":          true,
	"plan":           true,
	"test":           true,
	"implementation": true,
	"review":         true,
	"pr_mr":          true,
	"follow-up":      true,
}

func AddIssueOpsDecision(stateRoot, id string, req IssueOpsDecisionRecordRequest) (IssueOpsRecord, error) {
	return addIssueOpsDecision(stateRoot, id, req, nil)
}

// AddIssueOpsDecisionWithActor records a durable decision from the sealed
// preparation session when an execution workspace is present.
func AddIssueOpsDecisionWithActor(stateRoot, id string, req IssueOpsDecisionRecordRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return addIssueOpsDecision(stateRoot, id, req, &actor)
}

func addIssueOpsDecision(stateRoot, id string, req IssueOpsDecisionRecordRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = addIssueOpsDecisionLocked(stateRoot, id, req)
		return e
	})
	return rec, err
}

func addIssueOpsDecisionLocked(stateRoot, id string, req IssueOpsDecisionRecordRequest) (IssueOpsRecord, error) {
	kind := strings.TrimSpace(req.Kind)
	if !validDecisionKinds[kind] {
		return IssueOpsRecord{OK: false}, fmt.Errorf("invalid decision kind %q; must be one of: product, architecture, implementation, test, review, scope, follow-up", kind)
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision title is required")
	}
	if len(title) > 512 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision title must not exceed 512 bytes")
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision body is required")
	}
	if len(body) > 65536 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision body must not exceed 64 KiB")
	}
	if containsSecretPattern(body) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision body appears to contain secrets or credentials; redact them before storing")
	}
	for _, art := range req.AffectedArtifacts {
		if !validDecisionArtifacts[art] {
			return IssueOpsRecord{OK: false}, fmt.Errorf("invalid affected artifact %q", art)
		}
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	alt := req.Alternatives
	if alt == nil {
		alt = []string{}
	}
	links := req.AffectedIssueLinks
	if links == nil {
		links = []string{}
	}
	arts := req.AffectedArtifacts
	if arts == nil {
		arts = []string{}
	}
	record.Decisions = append(record.Decisions, IssueOpsDecision{
		Title:              title,
		Body:               body,
		Kind:               kind,
		Rationale:          strings.TrimSpace(req.Rationale),
		Alternatives:       alt,
		AffectedIssueLinks: links,
		AffectedArtifacts:  arts,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339Nano),
	})
	return touchAndWriteIssueOps(stateRoot, record)
}
