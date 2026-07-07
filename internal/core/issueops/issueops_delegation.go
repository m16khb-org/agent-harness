package issueops

import (
	"fmt"
	"os"
	"sort"
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

func IssueOpsChildStatus(stateRoot, parentID string, repair bool) (IssueOpsChildStatusResult, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return IssueOpsChildStatusResult{OK: false}, fmt.Errorf("parent_id is required")
	}
	var parent IssueOpsRecord
	err := withIssueOpsLock(stateRoot, parentID, func() error {
		var readErr error
		parent, readErr = ReadIssueOps(stateRoot, parentID)
		return readErr
	})
	if err != nil {
		return IssueOpsChildStatusResult{OK: false, ParentID: parentID}, err
	}

	scanned, err := scanIssueOpsChildrenForParent(stateRoot, parent)
	if err != nil {
		return IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	result := buildIssueOpsChildStatus(parent, scanned)
	if repair {
		appended, repairErr := repairIssueOpsChildIndex(stateRoot, parent.ID, scanned)
		if repairErr != nil {
			return result, repairErr
		}
		result.RepairAppended = appended
		result.Repaired = len(appended) > 0
	}
	return result, nil
}

func AcceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string) (IssueOpsChildValidationResult, error) {
	evidence = cleanIssueOpsTextValues(evidence)
	if len(evidence) == 0 {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("validation_evidence is required")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	if child.Phase != IssueOpsPhaseDone {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: child.ID}, fmt.Errorf("child_not_done: %s", child.ID)
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "accepted", "", evidence)
}

func RejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string) (IssueOpsChildValidationResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("reason must be at least 10 characters")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "rejected", reason, cleanIssueOpsTextValues(evidence))
}

func DropIssueOpsChild(stateRoot, parentID, childID, reason string) (IssueOpsChildValidationResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("reason must be at least 10 characters")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "dropped", reason, nil)
}

func scanIssueOpsChildrenForParent(stateRoot string, parent IssueOpsRecord) (map[string]IssueOpsRecord, error) {
	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		return nil, err
	}
	children := map[string]IssueOpsRecord{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "io-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		child, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			continue
		}
		if strings.TrimSpace(child.Repo) != strings.TrimSpace(parent.Repo) {
			continue
		}
		if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parent.ID {
			continue
		}
		children[child.ID] = child
	}
	return children, nil
}

func buildIssueOpsChildStatus(parent IssueOpsRecord, scanned map[string]IssueOpsRecord) IssueOpsChildStatusResult {
	result := IssueOpsChildStatusResult{OK: true, ParentID: parent.ID}
	seen := map[string]bool{}
	for _, ref := range parent.ChildCycles {
		entry := childStatusEntryFromRef(ref)
		entry.Indexed = true
		if child, ok := scanned[ref.CycleID]; ok {
			mergeChildStatusRecord(&entry, child)
			entry.Scanned = true
		} else {
			entry.Orphaned = true
			result.Orphaned = append(result.Orphaned, ref.CycleID)
		}
		result.Children = append(result.Children, entry)
		seen[ref.CycleID] = true
	}
	for _, child := range scanned {
		if seen[child.ID] {
			continue
		}
		entry := childStatusEntryFromChild(child)
		entry.Scanned = true
		result.Children = append(result.Children, entry)
	}
	sort.Slice(result.Children, func(i, j int) bool {
		return result.Children[i].CycleID < result.Children[j].CycleID
	})
	sort.Strings(result.Orphaned)
	return result
}

func repairIssueOpsChildIndex(stateRoot, parentID string, scanned map[string]IssueOpsRecord) ([]string, error) {
	appended := []string{}
	err := withIssueOpsLock(stateRoot, parentID, func() error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		seen := map[string]bool{}
		for _, ref := range parent.ChildCycles {
			seen[ref.CycleID] = true
		}
		childIDs := make([]string, 0, len(scanned))
		for childID := range scanned {
			childIDs = append(childIDs, childID)
		}
		sort.Strings(childIDs)
		for _, childID := range childIDs {
			if seen[childID] {
				continue
			}
			parent.ChildCycles = append(parent.ChildCycles, parentRefFromChild(scanned[childID]))
			appended = append(appended, childID)
		}
		if len(appended) == 0 {
			return nil
		}
		_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
		return writeErr
	})
	return appended, err
}

func readIssueOpsChildForValidation(stateRoot, parentID, childID string) (IssueOpsRecord, error) {
	parentID = strings.TrimSpace(parentID)
	childID = strings.TrimSpace(childID)
	if parentID == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("parent_id is required")
	}
	if childID == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("child_id is required")
	}
	var child IssueOpsRecord
	err := withIssueOpsLock(stateRoot, childID, func() error {
		var readErr error
		child, readErr = ReadIssueOps(stateRoot, childID)
		return readErr
	})
	if err != nil {
		return IssueOpsRecord{OK: false, ID: childID}, err
	}
	if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parentID {
		return child, fmt.Errorf("child_parent_mismatch: %s", childID)
	}
	return child, nil
}

func recordIssueOpsChildVerdict(stateRoot, parentID string, child IssueOpsRecord, verdict, reason string, evidence []string) (IssueOpsChildValidationResult, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var updated IssueOpsChildCycleRef
	err := withIssueOpsLock(stateRoot, strings.TrimSpace(parentID), func() error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if strings.TrimSpace(parent.Repo) != strings.TrimSpace(child.Repo) {
			return fmt.Errorf("child_repo_mismatch: %s", child.ID)
		}
		found := false
		for i := range parent.ChildCycles {
			if parent.ChildCycles[i].CycleID != child.ID {
				continue
			}
			parent.ChildCycles[i].ValidationVerdict = verdict
			parent.ChildCycles[i].ValidationReason = reason
			parent.ChildCycles[i].ValidationEvidence = evidence
			parent.ChildCycles[i].ValidatedAt = now
			updated = parent.ChildCycles[i]
			found = true
			break
		}
		if !found {
			updated = parentRefFromChild(child)
			updated.ValidationVerdict = verdict
			updated.ValidationReason = reason
			updated.ValidationEvidence = evidence
			updated.ValidatedAt = now
			parent.ChildCycles = append(parent.ChildCycles, updated)
		}
		_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
		return writeErr
	})
	if err != nil {
		return IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: child.ID}, err
	}
	return IssueOpsChildValidationResult{OK: true, ParentID: strings.TrimSpace(parentID), ChildID: child.ID, ParentRef: updated}, nil
}

func childStatusEntryFromRef(ref IssueOpsChildCycleRef) IssueOpsChildStatusEntry {
	return IssueOpsChildStatusEntry{
		CycleID:            ref.CycleID,
		Branch:             ref.Branch,
		Title:              ref.Title,
		ChildIssueURL:      ref.ChildIssueURL,
		ValidationVerdict:  ref.ValidationVerdict,
		ValidationReason:   ref.ValidationReason,
		ValidationEvidence: append([]string{}, ref.ValidationEvidence...),
		ValidatedAt:        ref.ValidatedAt,
	}
}

func childStatusEntryFromChild(child IssueOpsRecord) IssueOpsChildStatusEntry {
	entry := IssueOpsChildStatusEntry{CycleID: child.ID}
	mergeChildStatusRecord(&entry, child)
	return entry
}

func mergeChildStatusRecord(entry *IssueOpsChildStatusEntry, child IssueOpsRecord) {
	entry.CycleID = child.ID
	entry.Branch = child.Branch
	entry.Phase = child.Phase
	entry.LastActiveAt = IssueOpsLastActiveAt(child)
	entry.WorktreePath = strings.TrimSpace(child.WorktreePath)
	if child.Delegation != nil && entry.ChildIssueURL == "" {
		entry.ChildIssueURL = strings.TrimSpace(child.Delegation.ChildIssueURL)
	}
}

func parentRefFromChild(child IssueOpsRecord) IssueOpsChildCycleRef {
	ref := IssueOpsChildCycleRef{
		CycleID:   child.ID,
		Branch:    child.Branch,
		Title:     child.Branch,
		CreatedAt: child.CreatedAt,
	}
	if child.Delegation != nil {
		ref.ChildIssueURL = strings.TrimSpace(child.Delegation.ChildIssueURL)
		if delegatedAt := strings.TrimSpace(child.Delegation.DelegatedAt); delegatedAt != "" {
			ref.CreatedAt = delegatedAt
		}
	}
	return ref
}
