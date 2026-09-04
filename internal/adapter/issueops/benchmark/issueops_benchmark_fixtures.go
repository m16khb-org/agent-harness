package benchmark

import (
	"encoding/json"
	"fmt"
	issueopscontract "issueops/internal/contract/issueops"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func LoadIssueOpsBenchmarkFixtures(dir string) ([]issueopscontract.IssueOpsBenchmarkFixture, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("fixtures path is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fixtures []issueopscontract.IssueOpsBenchmarkFixture
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fixture issueopscontract.IssueOpsBenchmarkFixture
		if err := json.Unmarshal(b, &fixture); err != nil {
			return nil, fmt.Errorf("parse fixture %s: %w", path, err)
		}
		if err := validateIssueOpsBenchmarkFixture(fixture); err != nil {
			return nil, fmt.Errorf("invalid fixture %s: %w", path, err)
		}
		fixtures = append(fixtures, fixture)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no issueops benchmark fixtures in %s", dir)
	}
	return fixtures, nil
}

func validateIssueOpsBenchmarkFixture(f issueopscontract.IssueOpsBenchmarkFixture) error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(f.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(f.UserPrompt) == "" {
		return fmt.Errorf("user_prompt is required")
	}
	if strings.TrimSpace(f.RepoContext) == "" {
		return fmt.Errorf("repo_context is required")
	}
	if len(f.CriticalFailures) == 0 {
		return fmt.Errorf("critical_failures is required")
	}
	return nil
}
