package issueops

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"context"

	"agent-harness/internal/adapter/issueops/delegation"
	"agent-harness/internal/contract/issueops"
)

func StartIssueOpsChild(stateRoot string, req issueops.IssueOpsChildStartRequest) (issueops.IssueOpsChildStartResult, error) {
	return startIssueOpsChild(stateRoot, req, nil)
}

func StartIssueOpsChildWithActor(stateRoot string, req issueops.IssueOpsChildStartRequest, actor IssueOpsActor) (issueops.IssueOpsChildStartResult, error) {
	return startIssueOpsChild(stateRoot, req, &actor)
}

func startIssueOpsChild(stateRoot string, req issueops.IssueOpsChildStartRequest, actor *IssueOpsActor) (issueops.IssueOpsChildStartResult, error) {
	parentID := strings.TrimSpace(req.ParentID)
	if parentID == "" {
		return issueops.IssueOpsChildStartResult{OK: false}, fmt.Errorf("parent_id is required")
	}
	req.ParentID = parentID
	req.Branch = strings.TrimSpace(req.Branch)
	req.Title = strings.TrimSpace(req.Title)
	req.TaskScope = strings.TrimSpace(req.TaskScope)
	req.ParentPlanPath = strings.TrimSpace(req.ParentPlanPath)
	req.ChildIssueURL = strings.TrimSpace(req.ChildIssueURL)
	if req.Branch == "" {
		return issueops.IssueOpsChildStartResult{OK: false}, fmt.Errorf("branch is required")
	}
	if req.TaskScope == "" {
		return issueops.IssueOpsChildStartResult{OK: false}, fmt.Errorf("task_scope is required")
	}
	if len(cleanIssueOpsTextValues(req.AcceptanceCriteria)) == 0 {
		return issueops.IssueOpsChildStartResult{OK: false}, fmt.Errorf("acceptance_criteria requires at least one entry")
	}

	parent, err := readIssueOpsChildParentForStart(stateRoot, req, actor)
	if err != nil {
		return issueops.IssueOpsChildStartResult{OK: false}, err
	}
	child, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: parent.Repo, Branch: req.Branch})
	if err != nil {
		return issueops.IssueOpsChildStartResult{OK: false}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	child, err = stampIssueOpsChildDelegation(stateRoot, parent, child.ID, req, now)
	if err != nil {
		return issueops.IssueOpsChildStartResult{OK: false}, err
	}
	ref, err := appendIssueOpsChildRef(stateRoot, parent.ID, child, req, now, actor)
	if err != nil {
		return issueops.IssueOpsChildStartResult{OK: false}, err
	}
	result := issueops.IssueOpsChildStartResult{
		OK:        true,
		ParentID:  parent.ID,
		Child:     child,
		ParentRef: ref,
		Guidance:  "base_branch=" + parent.Branch + "; create an isolated worktree for " + child.Branch + " and export HARNESS_EXPECTED_WORKTREE after linking it",
	}
	if req.ChildIssueURL != "" {
		var linkErr error
		if actor == nil {
			_, linkErr = LinkIssueOpsChild(stateRoot, parent.ID, req.ChildIssueURL, req.Title)
		} else {
			_, linkErr = LinkIssueOpsChildWithActor(stateRoot, parent.ID, req.ChildIssueURL, req.Title, *actor)
		}
		if linkErr != nil {
			result.ChildLinkWarning = linkErr.Error()
		}
	}
	return result, nil
}

func readIssueOpsChildParentForStart(stateRoot string, req issueops.IssueOpsChildStartRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var parent issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, req.ParentID, func(context.Context) error {
		var readErr error
		parent, readErr = ReadIssueOps(stateRoot, req.ParentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
		}
		if missing := delegation.MissingPreconditions(parent, req); len(missing) > 0 {
			return fmt.Errorf("cannot start issueops child: missing %s", strings.Join(missing, ", "))
		}
		return nil
	})
	return parent, err
}

func stampIssueOpsChildDelegation(stateRoot string, parent issueops.IssueOpsRecord, childID string, req issueops.IssueOpsChildStartRequest, now string) (issueops.IssueOpsRecord, error) {
	var child issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, childID, func(context.Context) error {
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

func appendIssueOpsChildRef(stateRoot, parentID string, child issueops.IssueOpsRecord, req issueops.IssueOpsChildStartRequest, now string, actor *IssueOpsActor) (issueops.IssueOpsChildCycleRef, error) {
	ref := delegation.ParentRef(child, req, now)
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
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

func IssueOpsChildStatus(stateRoot, parentID string, repair bool) (issueops.IssueOpsChildStatusResult, error) {
	return issueOpsChildStatus(stateRoot, parentID, repair, nil)
}

func IssueOpsChildStatusWithActor(stateRoot, parentID string, repair bool, actor IssueOpsActor) (issueops.IssueOpsChildStatusResult, error) {
	return issueOpsChildStatus(stateRoot, parentID, repair, &actor)
}

func issueOpsChildStatus(stateRoot, parentID string, repair bool, actor *IssueOpsActor) (issueops.IssueOpsChildStatusResult, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return issueops.IssueOpsChildStatusResult{OK: false}, fmt.Errorf("parent_id is required")
	}
	var parent issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		var readErr error
		parent, readErr = ReadIssueOps(stateRoot, parentID)
		return readErr
	})
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parentID}, err
	}

	scanned, err := scanIssueOpsChildrenForParent(stateRoot, parent)
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	result := buildIssueOpsChildStatus(parent, scanned)
	if repair {
		appended, repairErr := repairIssueOpsChildIndex(stateRoot, parent.ID, scanned, actor)
		if repairErr != nil {
			return result, repairErr
		}
		result.RepairAppended = appended
		result.Repaired = len(appended) > 0
	}
	return result, nil
}

func AcceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string) (issueops.IssueOpsChildValidationResult, error) {
	return acceptIssueOpsChild(stateRoot, parentID, childID, evidence, nil)
}

func AcceptIssueOpsChildWithActor(stateRoot, parentID, childID string, evidence []string, actor IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	return acceptIssueOpsChild(stateRoot, parentID, childID, evidence, &actor)
}

func acceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	evidence = cleanIssueOpsTextValues(evidence)
	if len(evidence) == 0 {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("validation_evidence is required")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return acceptArchivedIssueOpsChild(stateRoot, parentID, childID, evidence, actor)
		}
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	if child.Phase != IssueOpsPhaseDone {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: child.ID}, fmt.Errorf("child_not_done: %s", child.ID)
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "accepted", "", evidence, actor)
}

func acceptArchivedIssueOpsChild(stateRoot, parentID, childID string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	parentID = strings.TrimSpace(parentID)
	childID = strings.TrimSpace(childID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var updated issueops.IssueOpsChildCycleRef
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
		}
		for i := range parent.ChildCycles {
			if parent.ChildCycles[i].CycleID != childID {
				continue
			}
			parent.ChildCycles[i].ValidationVerdict = "accepted"
			parent.ChildCycles[i].ValidationReason = ""
			parent.ChildCycles[i].ValidationEvidence = evidence
			parent.ChildCycles[i].ValidatedAt = now
			updated = parent.ChildCycles[i]
			_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
			return writeErr
		}
		return fmt.Errorf("child_not_indexed: %s", childID)
	})
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: parentID, ChildID: childID}, err
	}
	return issueops.IssueOpsChildValidationResult{OK: true, ParentID: parentID, ChildID: childID, ParentRef: updated}, nil
}

func RejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string) (issueops.IssueOpsChildValidationResult, error) {
	return rejectIssueOpsChild(stateRoot, parentID, childID, reason, evidence, nil)
}

func RejectIssueOpsChildWithActor(stateRoot, parentID, childID, reason string, evidence []string, actor IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	return rejectIssueOpsChild(stateRoot, parentID, childID, reason, evidence, &actor)
}

func rejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("reason must be at least 10 characters")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "rejected", reason, cleanIssueOpsTextValues(evidence), actor)
}

func DropIssueOpsChild(stateRoot, parentID, childID, reason string) (issueops.IssueOpsChildValidationResult, error) {
	return dropIssueOpsChild(stateRoot, parentID, childID, reason, nil)
}

func DropIssueOpsChildWithActor(stateRoot, parentID, childID, reason string, actor IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	return dropIssueOpsChild(stateRoot, parentID, childID, reason, &actor)
}

func dropIssueOpsChild(stateRoot, parentID, childID, reason string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("reason must be at least 10 characters")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "dropped", reason, nil, actor)
}

func scanIssueOpsChildrenForParent(stateRoot string, parent issueops.IssueOpsRecord) (map[string]issueops.IssueOpsRecord, error) {
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		return nil, err
	}
	children := map[string]issueops.IssueOpsRecord{}
	for _, id := range ids {
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

func buildIssueOpsChildStatus(parent issueops.IssueOpsRecord, scanned map[string]issueops.IssueOpsRecord) issueops.IssueOpsChildStatusResult {
	result := issueops.IssueOpsChildStatusResult{OK: true, ParentID: parent.ID}
	seen := map[string]bool{}
	for _, ref := range parent.ChildCycles {
		entry := childStatusEntryFromRef(ref)
		entry.Indexed = true
		if child, ok := scanned[ref.CycleID]; ok {
			mergeChildStatusRecord(&entry, child)
			entry.Scanned = true
		} else if !issueOpsChildRefHasAcceptedCleanupReceipt(ref) {
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
	if parent.Phase == IssueOpsPhaseDone {
		for i := range result.Children {
			if issueOpsChildPRGateKey(result.Children[i], scanned[result.Children[i].CycleID]) != "" {
				result.Children[i].ParentClosedState = "parent_closed"
			}
		}
	}
	sort.Strings(result.Orphaned)
	return result
}

func repairIssueOpsChildIndex(stateRoot, parentID string, scanned map[string]issueops.IssueOpsRecord, actor *IssueOpsActor) ([]string, error) {
	appended := []string{}
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
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

func readIssueOpsChildForValidation(stateRoot, parentID, childID string) (issueops.IssueOpsRecord, error) {
	parentID = strings.TrimSpace(parentID)
	childID = strings.TrimSpace(childID)
	if parentID == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("parent_id is required")
	}
	if childID == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("child_id is required")
	}
	var child issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, childID, func(context.Context) error {
		var readErr error
		child, readErr = ReadIssueOps(stateRoot, childID)
		return readErr
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: childID}, err
	}
	if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parentID {
		return child, fmt.Errorf("child_parent_mismatch: %s", childID)
	}
	return child, nil
}

func recordIssueOpsChildVerdict(stateRoot, parentID string, child issueops.IssueOpsRecord, verdict, reason string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var updated issueops.IssueOpsChildCycleRef
	err := withIssueOpsLock(context.Background(), stateRoot, strings.TrimSpace(parentID), func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
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
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: child.ID}, err
	}
	return issueops.IssueOpsChildValidationResult{OK: true, ParentID: strings.TrimSpace(parentID), ChildID: child.ID, ParentRef: updated}, nil
}

func childStatusEntryFromRef(ref issueops.IssueOpsChildCycleRef) issueops.IssueOpsChildStatusEntry {
	entry := issueops.IssueOpsChildStatusEntry{
		CycleID:            ref.CycleID,
		Branch:             ref.Branch,
		Title:              ref.Title,
		ChildIssueURL:      ref.ChildIssueURL,
		ValidationVerdict:  ref.ValidationVerdict,
		ValidationReason:   ref.ValidationReason,
		ValidationEvidence: append([]string{}, ref.ValidationEvidence...),
		ValidatedAt:        ref.ValidatedAt,
	}
	// accepted는 done child만 기록할 수 있으므로 parent receipt 자체가 cleanup
	// 이후에도 terminal phase의 내구성 있는 증거다.
	if strings.TrimSpace(ref.ValidationVerdict) == "accepted" && strings.TrimSpace(ref.ValidatedAt) != "" {
		entry.Phase = IssueOpsPhaseDone
	}
	return entry
}

// issueOpsChildRefHasAcceptedCleanupReceipt는 done child에만 발급되는 accepted
// parent receipt로 cleanup 이후의 정상 레코드 부재를 판정한다.
func issueOpsChildRefHasAcceptedCleanupReceipt(ref issueops.IssueOpsChildCycleRef) bool {
	return strings.TrimSpace(ref.ValidationVerdict) == "accepted" &&
		strings.TrimSpace(ref.ValidatedAt) != "" &&
		len(ref.ValidationEvidence) > 0
}

func childStatusEntryFromChild(child issueops.IssueOpsRecord) issueops.IssueOpsChildStatusEntry {
	entry := issueops.IssueOpsChildStatusEntry{CycleID: child.ID}
	mergeChildStatusRecord(&entry, child)
	return entry
}

func mergeChildStatusRecord(entry *issueops.IssueOpsChildStatusEntry, child issueops.IssueOpsRecord) {
	entry.CycleID = child.ID
	entry.Branch = child.Branch
	entry.Phase = child.Phase
	entry.LastActiveAt = LastActiveAt(child)
	entry.WorktreePath = strings.TrimSpace(child.WorktreePath)
	if child.Delegation != nil && entry.ChildIssueURL == "" {
		entry.ChildIssueURL = strings.TrimSpace(child.Delegation.ChildIssueURL)
	}
}

func parentRefFromChild(child issueops.IssueOpsRecord) issueops.IssueOpsChildCycleRef {
	ref := issueops.IssueOpsChildCycleRef{
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
