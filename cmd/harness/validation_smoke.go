package main

import (
	"encoding/json"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type validationCommandRunner func(dir, label string, timeout time.Duration, stdin, name string, args ...string) StepResult

func validateInspect(binary, root string) StepResult {
	return validateInspectWithDeps(binary, root, runCommandStep)
}

func validateInspectWithDeps(binary, root string, run validationCommandRunner) StepResult {
	step := run(root, "inspect smoke", 30*time.Second, "", binary, "inspect", "--json")
	if !step.OK {
		return step
	}
	var info core.InspectInfo
	if err := json.Unmarshal([]byte(step.Stdout), &info); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := []string{}
	if !info.OK {
		errs = append(errs, "inspect ok=false")
	}
	if len(info.Skills) == 0 {
		errs = append(errs, "no skills listed")
	}
	if !info.Integration.ProjectClaudeMCPConfig {
		errs = append(errs, "project Claude MCP config missing")
	}
	if containsForbiddenLegacyOutsideRuntimePaths(step.Stdout, root) {
		errs = append(errs, "inspect output contains legacy "+"m"+"16 name")
	}
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func validateDocsIndex(binary, root string) StepResult {
	return validateDocsIndexWithDeps(binary, root, runCommandStep)
}

func validateDocsIndexWithDeps(binary, root string, run validationCommandRunner) StepResult {
	step := run(root, "docs index smoke", 30*time.Second, "", binary, "docs", "--json")
	if !step.OK {
		return step
	}
	var index core.DocsIndexResult
	if err := json.Unmarshal([]byte(step.Stdout), &index); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := []string{}
	if !index.OK {
		errs = append(errs, "docs index ok=false")
	}
	if index.HarnessRoot != root {
		errs = append(errs, "docs index harness root mismatch")
	}
	if len(index.Docs) == 0 {
		errs = append(errs, "no docs indexed")
	}
	wantDocs := []string{"AGENTS.md", "CLAUDE.md", "GENIUS_THINK.md", ".agent-harness/COMMIT_POLICY.md", "skills/self-augment/SELF_AUGMENTATION.md", "skills/self-verify/SKILL.md", ".agent-harness/OPERATIONS.md"}
	for _, want := range wantDocs {
		if !docIndexContains(index.Docs, want) {
			errs = append(errs, "missing doc "+want)
		}
	}
	for _, doc := range index.Docs {
		if doc.Title == "" {
			errs = append(errs, "missing title for "+doc.RelPath)
			break
		}
		if strings.Contains(doc.RelPath, "m"+"16") || strings.Contains(doc.Title, "m"+"16") {
			errs = append(errs, "docs index contains legacy "+"m"+"16 name")
			break
		}
	}
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func docIndexContains(docs []core.DocIndexInfo, relPath string) bool {
	for _, doc := range docs {
		if doc.RelPath == relPath {
			return true
		}
	}
	return false
}
