package issueops

import (
	"fmt"
	"sort"
	"strings"
)

func issueOpsChildPRGateMissing(stateRoot string, parent IssueOpsRecord) ([]string, []string) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return nil, nil
	}
	status, scanned, err := issueOpsChildStatusWithoutParentLock(stateRoot, parent)
	if err != nil {
		return []string{"children_complete"}, []string{"failed to scan delegated children: " + err.Error()}
	}
	missing := []string{}
	for _, child := range status.Children {
		if key := issueOpsChildPRGateKey(child, scanned[child.CycleID]); key != "" {
			missing = append(missing, key+":"+child.CycleID)
		}
	}
	return missing, nil
}

func issueOpsActiveChildIDs(stateRoot string, parent IssueOpsRecord) ([]string, error) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return nil, nil
	}
	status, scanned, err := issueOpsChildStatusWithoutParentLock(stateRoot, parent)
	if err != nil {
		return nil, fmt.Errorf("children_active scan failed: %w", err)
	}
	ids := []string{}
	for _, child := range status.Children {
		if issueOpsChildDropped(child) {
			continue
		}
		if !issueOpsChildTerminal(child, scanned[child.CycleID]) {
			ids = append(ids, child.CycleID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func issueOpsChildStatusWithoutParentLock(stateRoot string, parent IssueOpsRecord) (IssueOpsChildStatusResult, map[string]IssueOpsRecord, error) {
	scanned, err := scanIssueOpsChildrenForParent(stateRoot, parent)
	if err != nil {
		return IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, nil, err
	}
	return buildIssueOpsChildStatus(parent, scanned), scanned, nil
}

func issueOpsChildPRGateKey(entry IssueOpsChildStatusEntry, child IssueOpsRecord) string {
	if issueOpsChildDropped(entry) {
		return ""
	}
	if !issueOpsChildTerminal(entry, child) {
		return "child_incomplete"
	}
	switch strings.TrimSpace(entry.ValidationVerdict) {
	case "":
		return "child_unvalidated"
	case "rejected":
		return "child_rejected_unresolved"
	default:
		return ""
	}
}

func issueOpsChildTerminal(entry IssueOpsChildStatusEntry, _ IssueOpsRecord) bool {
	return entry.Phase == IssueOpsPhaseDone
}

func issueOpsChildDropped(entry IssueOpsChildStatusEntry) bool {
	return strings.TrimSpace(entry.ValidationVerdict) == "dropped"
}

func issueOpsAppendActiveChildrenAudit(reason string, childIDs []string) string {
	if len(childIDs) == 0 {
		return strings.TrimSpace(reason)
	}
	return strings.TrimSpace(reason) + "; active_children=" + strings.Join(childIDs, ",")
}
