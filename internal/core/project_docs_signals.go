package core

import (
	"sort"
	"strings"
)

func AnalyzeProjectSignals(root string) ProjectSignals {
	files := listInterestingFiles(root)
	s := ProjectSignals{Files: files}
	addLang := func(v string) { s.Languages = appendUnique(s.Languages, v) }
	addPM := func(v string) { s.PackageManagers = appendUnique(s.PackageManagers, v) }
	addConvention := func(v string) { s.DetectedConventions = appendUnique(s.DetectedConventions, v) }
	for _, rel := range files {
		switch rel {
		case "go.mod":
			addLang("Go")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "go test ./...", Evidence: []string{"go.mod"}, Confidence: "high"})
			s.BuildCommands = append(s.BuildCommands, EvidenceCommand{Command: "go build ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
			s.LintCommands = append(s.LintCommands, EvidenceCommand{Command: "go vet ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
		case "package.json":
			addLang("JavaScript/TypeScript")
			addPM("npm-compatible")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "npm test", Evidence: []string{"package.json"}, Confidence: "medium"})
		case "pnpm-lock.yaml":
			addPM("pnpm")
		case "yarn.lock":
			addPM("yarn")
		case "pyproject.toml":
			addLang("Python")
			addPM("pyproject")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "pytest", Evidence: []string{"pyproject.toml"}, Confidence: "medium"})
		case "Cargo.toml":
			addLang("Rust")
			addPM("cargo")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "cargo test", Evidence: []string{"Cargo.toml"}, Confidence: "high"})
			s.BuildCommands = append(s.BuildCommands, EvidenceCommand{Command: "cargo build", Evidence: []string{"Cargo.toml"}, Confidence: "high"})
		case "Makefile":
			addConvention("Makefile exists; inspect targets before inventing commands")
		case "Taskfile.yml", "Taskfile.yaml":
			addConvention("Taskfile exists; prefer documented task targets when present")
		case "AGENTS.md", "CLAUDE.md":
			s.ExistingAgentDocs = appendUnique(s.ExistingAgentDocs, rel)
		}
		switch {
		case rel != "go.mod" && strings.HasSuffix(rel, "/go.mod"):
			addLang("Go")
		case rel != "package.json" && strings.HasSuffix(rel, "/package.json"):
			addLang("JavaScript/TypeScript")
			addPM("npm-compatible")
		case rel != "pyproject.toml" && strings.HasSuffix(rel, "/pyproject.toml"):
			addLang("Python")
			addPM("pyproject")
		case rel != "Cargo.toml" && strings.HasSuffix(rel, "/Cargo.toml"):
			addLang("Rust")
			addPM("cargo")
		}
		if strings.HasPrefix(rel, ".github/workflows/") {
			s.GitHubWorkflows = appendUnique(s.GitHubWorkflows, rel)
		}
	}
	sort.Strings(s.Languages)
	sort.Strings(s.PackageManagers)
	sort.Strings(s.ExistingAgentDocs)
	sort.Strings(s.GitHubWorkflows)
	sort.Strings(s.DetectedConventions)
	s.Profile = inferProjectProfile(root, s)
	return s
}
