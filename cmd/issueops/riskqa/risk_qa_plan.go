package riskqa

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func Plan(root string) RiskQATierPlan {
	paths, warnings := gitChangedPaths(root)
	plan := PlanFromPaths(paths)
	plan.Reasons = append(plan.Reasons, warnings...)
	sort.Strings(plan.Reasons)
	return plan
}

func PlanFromPaths(paths []string) RiskQATierPlan {
	plan := RiskQATierPlan{Tier: "standard", ChangedPaths: uniqueSortedStrings(paths), Reasons: []string{}, Commands: []string{}}
	if len(plan.ChangedPaths) == 0 {
		plan.Reasons = append(plan.Reasons, "working tree has no local changes")
		return plan
	}
	goChanged := false
	sensitive := false
	for _, path := range plan.ChangedPaths {
		if strings.HasSuffix(path, ".go") {
			goChanged = true
		}
		if isRiskSensitivePath(path) {
			sensitive = true
		}
	}
	if goChanged {
		plan.Tier = "static"
		plan.Reasons = append(plan.Reasons, "go changes detected")
		plan.Commands = append(plan.Commands, "go vet ./...")
	}
	if goChanged && sensitive {
		plan.Tier = "elevated"
		plan.Reasons = append(plan.Reasons, "go changes touch policy, MCP, adapter, daemon, state, or harness orchestration surfaces")
		plan.Commands = append([]string{"go test -race ./... -count=1"}, plan.Commands...)
	}
	if !goChanged {
		plan.Reasons = append(plan.Reasons, "no Go changes detected; race/static tier skipped")
	}
	sort.Strings(plan.Reasons)
	return plan
}

func isRiskSensitivePath(path string) bool {
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "cmd/issueops/") || strings.HasPrefix(path, "internal/") {
		return true
	}
	for _, token := range []string{"daemon", "worker", "policy", "state", "mcp", "adapter", "install", "hook", "self_augment", "self-augment"} {
		if strings.Contains(path, token) {
			return true
		}
	}
	return false
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(filepath.ToSlash(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func PlanJSON(plan RiskQATierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}
