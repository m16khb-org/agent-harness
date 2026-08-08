package looprun

import (
	loopruncontract "agent-harness/internal/contract/looprun"
	"fmt"
	"sort"
	"strings"
)

func RepoGateMissing(repo string) ([]string, []string) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, nil
	}
	normalizedRepo, err := normalizeRepo(repo)
	if err != nil {
		return []string{"loops_complete"}, []string{"failed to resolve loop repo: " + err.Error()}
	}
	ids, err := ListLoopIDs()
	if err != nil {
		return []string{"loops_complete"}, []string{"failed to scan loop runs: " + err.Error()}
	}
	missing := []string{}
	warnings := []string{}
	for _, id := range ids {
		loop, err := ReadLoopExisting(id)
		if err != nil {
			missing = append(missing, "loop_incomplete:"+id)
			warnings = append(warnings, "failed to inspect loop run "+id+": "+err.Error())
			continue
		}
		if strings.TrimSpace(loop.Repo) != normalizedRepo {
			continue
		}
		if loopIncomplete(loop) {
			missing = append(missing, "loop_incomplete:"+loop.ID)
		}
	}
	sort.Strings(missing)
	return missing, warnings
}

func RepoGateSummaryFor(repo string) (loopruncontract.RepoGateSummary, []string) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return loopruncontract.RepoGateSummary{}, nil
	}
	normalizedRepo, err := normalizeRepo(repo)
	if err != nil {
		return loopruncontract.RepoGateSummary{}, []string{"failed to resolve loop repo: " + err.Error()}
	}
	ids, err := ListLoopIDs()
	if err != nil {
		return loopruncontract.RepoGateSummary{}, []string{"failed to scan loop runs: " + err.Error()}
	}
	summary := loopruncontract.RepoGateSummary{}
	warnings := []string{}
	for _, id := range ids {
		loop, err := ReadLoopExisting(id)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to inspect loop run %s: %v", id, err))
			continue
		}
		if strings.TrimSpace(loop.Repo) != normalizedRepo {
			continue
		}
		switch strings.TrimSpace(loop.Status) {
		case "active":
			summary.Active++
		case "exhausted":
			summary.Exhausted++
		}
	}
	return summary, warnings
}

func loopIncomplete(loop loopruncontract.LoopRun) bool {
	switch strings.TrimSpace(loop.Status) {
	case "active", "exhausted":
		return true
	default:
		return false
	}
}
