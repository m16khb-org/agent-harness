package hookprompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/workpool"
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
	for _, line := range orchestrationPoolReminders(record) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func StopOrchestrationRelayFacts(repo string) string {
	record, ok := boundOrchestrationCycle(repo)
	if !ok {
		return ""
	}
	facts := orchestrationChildMissingKeys(record)
	for _, pool := range linkedActivePoolSummaries(record) {
		if pool.taskFiles > 0 {
			facts = append(facts, "pool_incomplete:"+pool.id)
		}
	}
	if len(facts) == 0 {
		return ""
	}
	sort.Strings(facts)
	return "bound_cycle:" + record.ID + "; missing: " + strings.Join(facts, ", ")
}

func boundOrchestrationCycle(repo string) (issueops.IssueOpsRecord, bool) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return issueops.IssueOpsRecord{}, false
	}
	id := strings.TrimSpace(issueops.ActiveSessionCycleID(repo))
	if id == "" {
		return issueops.IssueOpsRecord{}, false
	}
	record, err := issueops.ReadIssueOps(issueops.IssueOpsStateRoot(), id)
	if err != nil || !record.OK {
		return issueops.IssueOpsRecord{}, false
	}
	return record, true
}

func orchestrationChildrenReminder(record issueops.IssueOpsRecord) string {
	total := len(record.ChildCycles)
	if total == 0 {
		return ""
	}
	done := 0
	unvalidated := 0
	for _, ref := range record.ChildCycles[:min(total, orchestrationChildReadLimit)] {
		child, ok := readBoundChild(ref.CycleID)
		if !ok || child.Phase != issueops.IssueOpsPhaseDone {
			continue
		}
		done++
		if strings.TrimSpace(ref.ValidationVerdict) == "" {
			unvalidated++
		}
	}
	return "children: " + itoa(done) + "/" + itoa(total) + " done, " + itoa(unvalidated) + " unvalidated - issueops child status --parent " + record.ID
}

func orchestrationChildMissingKeys(record issueops.IssueOpsRecord) []string {
	missing := []string{}
	for _, ref := range record.ChildCycles[:min(len(record.ChildCycles), orchestrationChildReadLimit)] {
		id := strings.TrimSpace(ref.CycleID)
		if id == "" {
			continue
		}
		child, ok := readBoundChild(id)
		if !ok || child.Phase != issueops.IssueOpsPhaseDone {
			missing = append(missing, "child_incomplete:"+id)
			continue
		}
		if strings.TrimSpace(ref.ValidationVerdict) == "" {
			missing = append(missing, "child_unvalidated:"+id)
		}
	}
	return missing
}

func readBoundChild(id string) (issueops.IssueOpsRecord, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return issueops.IssueOpsRecord{}, false
	}
	child, err := issueops.ReadIssueOps(issueops.IssueOpsStateRoot(), id)
	if err != nil || !child.OK {
		return issueops.IssueOpsRecord{}, false
	}
	return child, true
}

func orchestrationPoolReminders(record issueops.IssueOpsRecord) []string {
	summaries := linkedActivePoolSummaries(record)
	lines := make([]string, 0, len(summaries))
	for _, pool := range summaries {
		lines = append(lines, "pool "+pool.name+": 0 leased / "+itoa(pool.taskFiles)+" pending / 0 expired - workpool status")
	}
	return lines
}

type orchestrationPoolSummary struct {
	id        string
	name      string
	taskFiles int
}

func linkedActivePoolSummaries(record issueops.IssueOpsRecord) []orchestrationPoolSummary {
	entries, err := os.ReadDir(workpool.StateRoot())
	if err != nil {
		return nil
	}
	summaries := []orchestrationPoolSummary{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "wp-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		pool, ok := readPoolManifestOnly(filepath.Join(workpool.StateRoot(), name))
		if !ok || strings.TrimSpace(pool.ParentCycleID) != record.ID || strings.TrimSpace(pool.Status) == "closed" {
			continue
		}
		displayName := strings.TrimSpace(pool.Name)
		if displayName == "" {
			displayName = pool.ID
		}
		summaries = append(summaries, orchestrationPoolSummary{
			id:        pool.ID,
			name:      displayName,
			taskFiles: countPoolTaskFiles(pool.ID),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].name == summaries[j].name {
			return summaries[i].id < summaries[j].id
		}
		return summaries[i].name < summaries[j].name
	})
	return summaries
}

func readPoolManifestOnly(path string) (workpool.WorkPool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return workpool.WorkPool{}, false
	}
	var pool workpool.WorkPool
	if err := json.Unmarshal(data, &pool); err != nil {
		return workpool.WorkPool{}, false
	}
	if strings.TrimSpace(pool.ID) == "" {
		return workpool.WorkPool{}, false
	}
	return pool, true
}

func countPoolTaskFiles(poolID string) int {
	entries, err := os.ReadDir(filepath.Join(workpool.StateRoot(), poolID))
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "task-") && strings.HasSuffix(name, ".json") {
			count++
		}
	}
	return count
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
