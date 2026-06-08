package issueops

import (
	"fmt"
	"strings"
	"time"
)

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
	kind := strings.TrimSpace(req.Kind)
	if !validDecisionKinds[kind] {
		return IssueOpsRecord{OK: false}, fmt.Errorf("invalid decision kind %q; must be one of: product, architecture, implementation, test, review, scope, follow-up", kind)
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision title is required")
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("decision body is required")
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
