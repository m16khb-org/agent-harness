package issueops

import (
	"fmt"
	"sort"
	"strings"

	"issueops/internal/contract/issueops"
)

func issueOpsChildPRGateMissing(stateRoot string, parent issueops.IssueOpsRecord) ([]string, []string) {
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

func issueOpsActiveChildIDs(stateRoot string, parent issueops.IssueOpsRecord) ([]string, error) {
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

func issueOpsChildStatusWithoutParentLock(stateRoot string, parent issueops.IssueOpsRecord) (issueops.IssueOpsChildStatusResult, map[string]issueops.IssueOpsRecord, error) {
	scanned, err := scanIssueOpsChildrenForParent(stateRoot, parent)
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, nil, err
	}
	return buildIssueOpsChildStatus(parent, scanned), scanned, nil
}

func issueOpsChildPRGateKey(entry issueops.IssueOpsChildStatusEntry, child issueops.IssueOpsRecord) string {
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

func issueOpsChildTerminal(entry issueops.IssueOpsChildStatusEntry, _ issueops.IssueOpsRecord) bool {
	return entry.Phase == IssueOpsPhaseDone
}

func issueOpsChildDropped(entry issueops.IssueOpsChildStatusEntry) bool {
	return strings.TrimSpace(entry.ValidationVerdict) == "dropped" &&
		len(strings.TrimSpace(entry.ValidationReason)) >= 10 &&
		strings.TrimSpace(entry.ValidatedAt) != ""
}
