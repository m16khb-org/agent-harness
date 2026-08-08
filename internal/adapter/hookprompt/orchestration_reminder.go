package hookprompt

import (
	"path/filepath"
	"sort"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
)

const orchestrationChildReadLimit = 16

func orchestrationReminderValue(repo string) string {
	record, ok := boundOrchestrationCycle(repo)
	if !ok {
		return ""
	}
	lines := []string{}
	if line := orchestrationChildrenReminder(record); line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func StopOrchestrationRelayFacts(repo string) string {
	record, ok := boundOrchestrationCycle(repo)
	if !ok {
		return ""
	}
	facts := orchestrationChildMissingKeys(record)
	if len(facts) == 0 {
		return ""
	}
	sort.Strings(facts)
	return "bound_cycle:" + record.ID + "; missing: " + strings.Join(facts, ", ")
}

func boundOrchestrationCycle(repo string) (issueopscontract.IssueOpsRecord, bool) {
	repo = cleanOrchestrationPath(repo)
	if repo == "" {
		return issueopscontract.IssueOpsRecord{}, false
	}
	ids, err := ListIssueOpsIDs(IssueOpsStateRoot())
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, false
	}
	var match issueopscontract.IssueOpsRecord
	for _, id := range ids {
		record, readErr := ReadIssueOps(IssueOpsStateRoot(), id)
		if readErr != nil || !record.OK || record.Phase == issueopscontract.IssueOpsPhaseDone || record.Execution == nil {
			continue
		}
		workspace := record.Execution.Workspace
		if repo != cleanOrchestrationPath(workspace.SourceRoot) && repo != cleanOrchestrationPath(workspace.Root) {
			continue
		}
		if match.ID != "" {
			return issueopscontract.IssueOpsRecord{}, false
		}
		match = record
	}
	return match, match.ID != ""
}

func cleanOrchestrationPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}

func orchestrationChildrenReminder(record issueopscontract.IssueOpsRecord) string {
	bounded, total := boundedOrchestrationChildren(record)
	if total == 0 {
		return ""
	}
	done := 0
	unvalidated := 0
	for _, ref := range bounded {
		child, ok := readBoundChild(ref.CycleID)
		if !ok || child.Phase != issueopscontract.IssueOpsPhaseDone {
			continue
		}
		done++
		if strings.TrimSpace(ref.ValidationVerdict) == "" {
			unvalidated++
		}
	}
	return "children: " + itoa(done) + "/" + itoa(total) + " done, " + itoa(unvalidated) + " unvalidated - issueops child status --parent " + record.ID
}

func orchestrationChildMissingKeys(record issueopscontract.IssueOpsRecord) []string {
	missing := []string{}
	bounded, _ := boundedOrchestrationChildren(record)
	for _, ref := range bounded {
		id := strings.TrimSpace(ref.CycleID)
		if id == "" {
			continue
		}
		child, ok := readBoundChild(id)
		if !ok || child.Phase != issueopscontract.IssueOpsPhaseDone {
			missing = append(missing, "child_incomplete:"+id)
			continue
		}
		if strings.TrimSpace(ref.ValidationVerdict) == "" {
			missing = append(missing, "child_unvalidated:"+id)
		}
	}
	return missing
}

func orchestrationChildDropped(ref issueopscontract.IssueOpsChildCycleRef) bool {
	return strings.TrimSpace(ref.ValidationVerdict) == "dropped"
}

func boundedOrchestrationChildren(record issueopscontract.IssueOpsRecord) ([]issueopscontract.IssueOpsChildCycleRef, int) {
	total := 0
	bounded := make([]issueopscontract.IssueOpsChildCycleRef, 0, orchestrationChildReadLimit)
	for _, ref := range record.ChildCycles {
		if orchestrationChildDropped(ref) {
			continue
		}
		total++
		if len(bounded) < orchestrationChildReadLimit {
			bounded = append(bounded, ref)
		}
	}
	return bounded, total
}

func readBoundChild(id string) (issueopscontract.IssueOpsRecord, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return issueopscontract.IssueOpsRecord{}, false
	}
	child, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil || !child.OK {
		return issueopscontract.IssueOpsRecord{}, false
	}
	return child, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
