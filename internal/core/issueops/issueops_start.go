package issueops

import (
	"fmt"
	"strings"
	"time"
)

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("repo is required")
	}
	branch := strings.TrimSpace(req.Branch)
	if err := validateIssueOpsIssueBranch(branch); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	id := newIssueOpsID(repo, branch)
	if existing, err := ReadIssueOps(stateRoot, id); err == nil {
		return existing, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := IssueOpsRecord{
		OK:        true,
		ID:        id,
		Repo:      repo,
		Branch:    branch,
		Phase:     IssueOpsPhaseProblem,
		Feedback:  []IssueOpsFeedbackItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return writeIssueOps(stateRoot, record)
}
