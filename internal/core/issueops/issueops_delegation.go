package issueops

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/delegation"
)

func StartIssueOpsChild(stateRoot string, req IssueOpsChildStartRequest) (IssueOpsChildStartResult, error) {
	parentID := strings.TrimSpace(req.ParentID)
	if parentID == "" {
		return IssueOpsChildStartResult{OK: false}, fmt.Errorf("parent_id is required")
	}
	req.ParentID = parentID
	req.Branch = strings.TrimSpace(req.Branch)
	req.Title = strings.TrimSpace(req.Title)
	req.TaskScope = strings.TrimSpace(req.TaskScope)
	req.ParentPlanPath = strings.TrimSpace(req.ParentPlanPath)
	req.ChildIssueURL = strings.TrimSpace(req.ChildIssueURL)
	if req.Branch == "" {
		return IssueOpsChildStartResult{OK: false}, fmt.Errorf("branch is required")
	}
	if req.TaskScope == "" {
		return IssueOpsChildStartResult{OK: false}, fmt.Errorf("task_scope is required")
	}
	if len(cleanIssueOpsTextValues(req.AcceptanceCriteria)) == 0 {
		return IssueOpsChildStartResult{OK: false}, fmt.Errorf("acceptance_criteria requires at least one entry")
	}

	parent, err := readIssueOpsChildParentForStart(stateRoot, req)
	if err != nil {
		return IssueOpsChildStartResult{OK: false}, err
	}
	child, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: parent.Repo, Branch: req.Branch})
	if err != nil {
		return IssueOpsChildStartResult{OK: false}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	child, err = stampIssueOpsChildDelegation(stateRoot, parent, child.ID, req, now)
	if err != nil {
		return IssueOpsChildStartResult{OK: false}, err
	}
	ref, err := appendIssueOpsChildRef(stateRoot, parent.ID, child, req, now)
	if err != nil {
		return IssueOpsChildStartResult{OK: false}, err
	}
	result := IssueOpsChildStartResult{
		OK:        true,
		ParentID:  parent.ID,
		Child:     child,
		ParentRef: ref,
		Guidance:  "base_branch=" + parent.Branch + "; create an isolated worktree for " + child.Branch + " and export HARNESS_EXPECTED_WORKTREE after linking it",
	}
	if req.ChildIssueURL != "" {
		if _, linkErr := LinkIssueOpsChild(stateRoot, parent.ID, req.ChildIssueURL, req.Title); linkErr != nil {
			result.ChildLinkWarning = linkErr.Error()
		}
	}
	return result, nil
}

func readIssueOpsChildParentForStart(stateRoot string, req IssueOpsChildStartRequest) (IssueOpsRecord, error) {
	var parent IssueOpsRecord
	err := withIssueOpsLock(stateRoot, req.ParentID, func() error {
		var readErr error
		parent, readErr = ReadIssueOps(stateRoot, req.ParentID)
		if readErr != nil {
			return readErr
		}
		if missing := delegation.MissingPreconditions(parent, req); len(missing) > 0 {
			return fmt.Errorf("cannot start issueops child: missing %s", strings.Join(missing, ", "))
		}
		return nil
	})
	return parent, err
}

func stampIssueOpsChildDelegation(stateRoot string, parent IssueOpsRecord, childID string, req IssueOpsChildStartRequest, now string) (IssueOpsRecord, error) {
	var child IssueOpsRecord
	err := withIssueOpsLock(stateRoot, childID, func() error {
		var readErr error
		child, readErr = ReadIssueOps(stateRoot, childID)
		if readErr != nil {
			return readErr
		}
		child = delegation.BuildDelegatedProfile(parent, child, req, now)
		var writeErr error
		child, writeErr = touchAndWriteIssueOps(stateRoot, child)
		return writeErr
	})
	return child, err
}

func appendIssueOpsChildRef(stateRoot, parentID string, child IssueOpsRecord, req IssueOpsChildStartRequest, now string) (IssueOpsChildCycleRef, error) {
	ref := delegation.ParentRef(child, req, now)
	err := withIssueOpsLock(stateRoot, parentID, func() error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		for _, existing := range parent.ChildCycles {
			if existing.CycleID == ref.CycleID {
				ref = existing
				return nil
			}
		}
		parent.ChildCycles = append(parent.ChildCycles, ref)
		_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
		return writeErr
	})
	return ref, err
}
