package looprun

import (
	"fmt"
	"sort"
	"strings"
)

type RepoGateSummary struct {
	Active    int
	Exhausted int
}

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

func RepoGateSummaryFor(repo string) (RepoGateSummary, []string) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return RepoGateSummary{}, nil
	}
	normalizedRepo, err := normalizeRepo(repo)
	if err != nil {
		return RepoGateSummary{}, []string{"failed to resolve loop repo: " + err.Error()}
	}
	ids, err := ListLoopIDs()
	if err != nil {
		return RepoGateSummary{}, []string{"failed to scan loop runs: " + err.Error()}
	}
	summary := RepoGateSummary{}
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

func loopIncomplete(loop LoopRun) bool {
	switch strings.TrimSpace(loop.Status) {
	case "active", "exhausted":
		return true
	default:
		return false
	}
}
