package projectdocs

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/repopath"
)

func RouteProjectDocs(repoRoot, task string) (ProjectDocsRouteResult, error) {
	root, err := repopath.NormalizeRoot(repoRoot)
	if err != nil {
		return ProjectDocsRouteResult{}, err
	}
	normalizedTask := strings.ToLower(strings.TrimSpace(task))
	if normalizedTask == "" {
		normalizedTask = "general"
	}
	rels := routeDocsForTask(normalizedTask)
	entries := make([]ProjectDocRouteEntry, 0, len(rels))
	for _, rd := range rels {
		path := filepath.Join(root, filepath.FromSlash(rd.rel))
		_, err := os.Stat(path)
		entries = append(entries, ProjectDocRouteEntry{RelPath: rd.rel, Path: path, Reason: rd.reason, Exists: err == nil})
	}
	warnings := []string{}
	missingProjectDocs := true
	if _, err := os.Stat(filepath.Join(root, ProjectDocsDir)); err == nil {
		missingProjectDocs = false
	}
	if missingProjectDocs {
		warnings = append(warnings, "project docs are missing; run agent-harness project bootstrap to create AGENTS.md routing, .agent-harness docs, and repo metadata")
	}
	return ProjectDocsRouteResult{
		OK:          true,
		Kind:        "project_docs_route",
		RepoRoot:    root,
		Task:        normalizedTask,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Docs:        entries,
		Warnings:    warnings,
	}, nil
}

func routeDocsForTask(task string) []routeDoc {
	base := []routeDoc{{"AGENTS.md", "repo-level agent entrypoint and document router"}}
	p := func(name, reason string) routeDoc {
		return routeDoc{filepath.ToSlash(filepath.Join(ProjectDocsDir, name)), reason}
	}
	extra := []routeDoc{}
	if strings.Contains(task, "gitlab") || strings.Contains(task, "github") ||
		strings.Contains(task, "glab") || strings.Contains(task, "gh issue") ||
		strings.Contains(task, "vcs") || strings.Contains(task, "merge request") ||
		strings.Contains(task, "pull request") || strings.Contains(task, "remote issue") {
		extra = append(extra, p("VCS.md", "verified VCS provider capabilities, exact request recipes, and CLI fallbacks"))
	}
	add := func(names ...routeDoc) []routeDoc {
		result := append([]routeDoc(nil), base...)
		result = append(result, extra...)
		return append(result, names...)
	}
	if strings.Contains(task, "conflict") || strings.Contains(task, "constitution") || strings.Contains(task, "principle") || strings.Contains(task, "instruction") || strings.Contains(task, "session") {
		return add(p("CONSTITUTION.md", "SessionStart baseline and source-of-truth priority"), p("CAUTIONS.md", "risks that may affect the decision"))
	}
	if strings.Contains(task, "caution") || strings.Contains(task, "risk") || strings.Contains(task, "false") || strings.Contains(task, "failure") || strings.Contains(task, "regression") {
		return add(p("CAUTIONS.md", "known false cases, repeated failures, and risk notes"), p("TESTING.md", "test design rules and verification checks to prevent recurrence"), p("ADR.md", "decision context if the false case was caused by architecture"))
	}
	if strings.Contains(task, "adr") || strings.Contains(task, "decision") || strings.Contains(task, "alternative") || strings.Contains(task, "why") {
		return add(p("ADR.md", "architecture decision rationale, rejected alternatives, and consequences"), p("ARCHITECTURE.md", "current structure affected by the decision"), p("CONSTITUTION.md", "principles that constrain decisions"))
	}
	if strings.Contains(task, "execution") || strings.Contains(task, "finish") || strings.Contains(task, "complete") || strings.Contains(task, "workflow") {
		return add(p("AGENT_WORKFLOW.md", "agent start/work/verify/finish procedure"), p("TESTING.md", "verification evidence before completion"))
	}
	if strings.Contains(task, "commit") || strings.Contains(task, "pr") || strings.Contains(task, "push") {
		return add(p("COMMIT_POLICY.md", "commit message, staging, and verification policy"), p("TESTING.md", "checks to run before commit/PR"), p("CAUTIONS.md", "project-specific commit risks"))
	}
	if strings.Contains(task, "openapi") || strings.Contains(task, "swagger") || strings.Contains(task, "endpoint") || strings.Contains(task, "controller") || strings.Contains(task, "dto") || strings.Contains(task, "api doc") || strings.Contains(task, "api spec") {
		return add(p("OPEN_API_SPEC.md", "project-specific OpenAPI/Swagger static and agent review prompt"), p("TESTING.md", "API documentation check commands and static-vs-agent boundary"), p("AGENT_WORKFLOW.md", "verification workflow"), p("CAUTIONS.md", "known API documentation risks"))
	}
	if strings.Contains(task, "test") || strings.Contains(task, "testing") || strings.Contains(task, "spec") || strings.Contains(task, "verify") || strings.Contains(task, "ci") {
		return add(p("TESTING.md", "well/poorly structured test guidance plus test/build/lint command candidates"), p("TECH_STACK.md", "toolchain evidence"), p("AGENT_WORKFLOW.md", "verification workflow"), p("CAUTIONS.md", "known verification risks"))
	}
	if strings.Contains(task, "architecture") || strings.Contains(task, "design") || strings.Contains(task, "refactor") {
		return add(p("ARCHITECTURE.md", "system structure and boundaries"), p("ADR.md", "past structure decisions and rejected alternatives"), p("CONSTITUTION.md", "decision priority and invariants"), p("CONVENTIONS.md", "editing and structure conventions"))
	}
	if strings.Contains(task, "dependency") || strings.Contains(task, "package") || strings.Contains(task, "upgrade") || strings.Contains(task, "stack") {
		return add(p("TECH_STACK.md", "detected stack and package manager evidence"), p("CONVENTIONS.md", "dependency addition rules"), p("TESTING.md", "test design rules and checks after dependency changes"))
	}
	if strings.Contains(task, "run") || strings.Contains(task, "deploy") || strings.Contains(task, "env") || strings.Contains(task, "local") || strings.Contains(task, "operate") {
		return add(p("OPERATIONS.md", "local development, environment, and deployment guidance"), p("TECH_STACK.md", "toolchain evidence"), p("CAUTIONS.md", "operational risks"))
	}
	return add(p("CONSTITUTION.md", "source-of-truth and operating principles"), p("AGENT_WORKFLOW.md", "default start/work/verify/finish workflow"), p("CONVENTIONS.md", "general editing rules"), p("CAUTIONS.md", "known project risks"), p("TESTING.md", "default test design and verification guidance"))
}
