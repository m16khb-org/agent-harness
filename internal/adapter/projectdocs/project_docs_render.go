package projectdocs

import (
	projectdoc "agent-harness/internal/domain/projectdoc"
	"os"
	"path/filepath"
	"strings"
)

func RenderProjectDocs(root string, signals projectdoc.ProjectSignals) map[string]string {
	out := map[string]string{}
	// Family roots become short indexes; their previous detail bodies move
	// into module starter documents created alongside them (folder-first).
	for _, f := range projectdoc.DocFamilies() {
		out[filepath.ToSlash(filepath.Join(ProjectDocsDir, f.Root))] = renderFamilyIndex(f)
	}
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "COMMIT_POLICY.md"))] = renderCommitPolicy()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CONSTITUTION.md"))] = renderConstitution()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "TECH_STACK.md"))] = renderTechStack(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPEN_API_SPEC.md"))] = renderOpenAPISpec()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "AGENT_WORKFLOW.md"))] = renderAgentWorkflow()
	// DESIGN.md exists only in repositories with a client surface (web
	// frontend or desktop client): a design system contract is meaningless
	// for backend/CLI-only projects.
	if hasClientSurface(signals) {
		out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "DESIGN.md"))] = renderDesignDoc(root, signals)
	}
	// Prepend canonical meta frontmatter so created/synced docs declare what
	// category of information they hold. Same doc name => same metadata.
	for rel, content := range out {
		out[rel] = ensureDocMetaFrontmatter(filepath.Base(rel), content)
	}
	// Module starters carry their own explicit frontmatter and back-links.
	for rel, content := range renderFamilyModuleDocs(signals) {
		out[rel] = content
	}
	return out
}

func renderArchitecture(signals projectdoc.ProjectSignals) string {
	return "# Architecture\n\n## Purpose\n\nThis is an architecture draft generated from project files by agent-harness. Mark weak inferences with Confidence; current code and command output are authoritative.\n\n## Detected structure\n\n" + bulletListWithFallback(signals.Files, "Not enough project signal files were detected.") + "\n## Guidance\n\n- Before large design changes, inspect current entrypoints, package/module boundaries, and data flow.\n- Add new abstractions only after existing patterns and test boundaries are confirmed.\n- Name the architecture style actually observed (layered, hexagonal/ports-and-adapters, onion, clean architecture, modular monolith, microservices) and back it with the owning files.\n"
}

func renderCautions(signals projectdoc.ProjectSignals) string {
	items := []string{"Generated docs are drafts; directly verify weak evidence.", "Do not commit secrets, credentials, local state, or generated artifacts."}
	if len(signals.GitHubWorkflows) > 0 {
		items = append(items, "CI workflows exist; compare local verification with CI behavior.")
	}
	return "# Cautions\n\n" + bulletListWithFallback(items, "No cautions recorded.")
}

func renderCommitPolicy() string {
	return "# Commit Policy\n\n" +
		"## Default\n\n" +
		"- Prefer small atomic commits.\n" +
		"- Run verification appropriate to the change scope before committing.\n" +
		"- Use Conventional Commit format unless the project has stricter rules.\n\n" +
		"~~~text\n<type>(<scope>): <summary>\n\nWhy: <why this change exists>\nTested: <commands run>\nNot-tested: <known verification gaps>\n~~~\n\n" +
		"## Safety\n\n" +
		"- Do not stage unrelated changes.\n" +
		"- Manually inspect secret-like paths or credential changes before committing.\n"
}

func renderConstitution() string {
	return `# Constitution

## SessionStart contract

This project-specific constitution should be read at session start. Follow the general LLM coding behavior guidelines at the top of AGENTS.md; this document adds harness structure, security, and verification invariants. Treat it as the baseline principle document for MCP routing.

## Source of truth

1. Latest explicit user/system instructions
2. Current repo AGENTS.md or a nearer nested AGENTS.md
3. .agent-harness/*.md
4. Current files and command output

## Principles

- Host adapters must not bypass core policy.
- Never put raw secrets in docs, logs, test fixtures, or MCP/CLI responses.
- Preserve explicit workspace-root and command-policy boundaries.
- Harness results observed from Codex and Claude Code should match.
`
}

func renderConventions(signals projectdoc.ProjectSignals) string {
	lines := []string{"# Conventions\n\n## Detected conventions\n\n"}
	if len(signals.DetectedConventions) == 0 {
		lines = append(lines, "- Few conventions were auto-detected. Inspect README, config, and existing files first.\n")
	} else {
		lines = append(lines, bulletListWithFallback(signals.DetectedConventions, "No detected conventions."))
	}
	lines = append(lines, "\n## Editing rules\n\n- Follow existing style first.\n- Do not run repo-wide formatting unless explicitly requested.\n- Add new dependencies only after documenting the need and alternatives.\n")
	lines = append(lines, "\n", solidDesignPatternGuidance)
	lines = append(lines, "\n", engineeringStandardsChecklist)
	return strings.Join(lines, "")
}

func renderTechStack(signals projectdoc.ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Tech Stack\n\n## Detected languages\n\n")
	b.WriteString(bulletListWithFallback(signals.Languages, "Could not auto-confirm languages."))
	b.WriteString("\n## Package managers\n\n")
	b.WriteString(bulletListWithFallback(signals.PackageManagers, "Could not auto-confirm package managers."))
	b.WriteString("\n## Evidence files\n\n")
	b.WriteString(bulletListWithFallback(signals.Files, "No evidence files."))
	return b.String()
}

// hasClientSurface reports whether the repository exposes a client: a web
// frontend framework or a desktop client shell (Tauri, Electron).
func hasClientSurface(signals projectdoc.ProjectSignals) bool {
	for _, t := range signals.Profile.ProjectTypes {
		if t == "frontend" || t == "desktop-client" {
			return true
		}
	}
	return false
}

// renderDesignDoc renders the client design-system document. When the repo
// already maintains an authoritative design document (commonly a curated
// root DESIGN.md), this doc stays a pointer to it instead of duplicating it.
func renderDesignDoc(root string, signals projectdoc.ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Design System\n\n")
	b.WriteString("## Purpose\n\n")
	b.WriteString("Client design contract for this repository: palette, typography, spacing, motion, accessibility, and component states. Read it before UI, styling, component, or interaction work, and update it when a design decision changes.\n\n")
	b.WriteString("## Detected client surface\n\n")
	if len(signals.Profile.Frameworks) > 0 {
		b.WriteString(bulletListWithFallback(signals.Profile.Frameworks, "No client frameworks confirmed."))
	} else {
		b.WriteString("- Client surface inferred from project types: " + strings.Join(signals.Profile.ProjectTypes, ", ") + "\n")
	}
	existing := existingDesignDocRel(root)
	b.WriteString("\n## Authoritative design document\n\n")
	if existing != "" {
		b.WriteString("This repository already maintains `" + existing + "`; that document is authoritative for the design system. Keep this file as the agent-facing routing record: do not duplicate tokens or tables here, point to `" + existing + "` and record only agent workflow notes (when to read it, how changes are verified).\n")
	} else {
		b.WriteString("No standalone design document was detected at the repository root. Record confirmed design decisions here with evidence (source stylesheets, component files); mark anything unconfirmed as `Unknown / not confirmed`.\n")
	}
	b.WriteString("\n## Required sections once enriched\n\n")
	b.WriteString("- Identity and atmosphere: what the product must feel like, signature elements.\n")
	b.WriteString("- Palette: role-based tokens, not raw hex values scattered in code.\n")
	b.WriteString("- Typography: scale, weights, font stacks, line-height and tracking rules.\n")
	b.WriteString("- Spacing and layout: base unit, grid, container rules.\n")
	b.WriteString("- Motion: durations, easing, reduced-motion behavior.\n")
	b.WriteString("- Accessibility: focus, contrast, touch targets, screen-reader semantics.\n")
	b.WriteString("- Component states: idle/loading/success/error per interactive component.\n\n")
	b.WriteString("## Rule\n\n")
	b.WriteString("Stylesheets and component code are the implementation; this document is the contract. When code and this document disagree, fix one of them in the same change set and say which was wrong.\n")
	return b.String()
}

// existingDesignDocRel returns the repo-relative path of an authoritative
// design document when one exists at a conventional location.
func existingDesignDocRel(root string) string {
	for _, rel := range []string{"DESIGN.md", "docs/DESIGN.md"} {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && !info.IsDir() {
			return rel
		}
	}
	return ""
}

func renderTesting(signals projectdoc.ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Testing\n\n")
	b.WriteString("## Purpose\n\n")
	b.WriteString("This is the reference document agents should read before writing or modifying tests. It records candidate commands and the difference between well-structured and poorly structured tests.\n\n")
	b.WriteString("## When to read\n\n")
	b.WriteString("- When writing tests for a new feature or bug fix\n")
	b.WriteString("- When existing tests fail or are flaky\n")
	b.WriteString("- When proving behavior preservation after refactoring\n")
	b.WriteString("- When deciding which verification to run before completion\n\n")
	b.WriteString("## Well-structured tests\n\n")
	b.WriteString("- Directly verify changed behavior and prefer public contracts/observable behavior over implementation details.\n")
	b.WriteString("- Use assertion messages or fixture names that reveal the failure cause.\n")
	b.WriteString("- Keep tests deterministic, independently runnable, and not overly dependent on order, time, external network, or global state.\n")
	b.WriteString("- Regression tests clearly encode the recurring input/context and expected result.\n")
	b.WriteString("- Reuse existing test style/helpers and explain intent/scope for broad snapshot or golden updates.\n\n")
	b.WriteString("## Poorly-structured tests\n\n")
	b.WriteString("- Locking only internal implementation structure and blocking harmless refactors.\n")
	b.WriteString("- Adding assertions not tied to a real bug or requirement.\n")
	b.WriteString("- Depending on sleeps, real network, local machine state, or test order.\n")
	b.WriteString("- Using huge fixtures or vague snapshots that do not explain failures.\n")
	b.WriteString("- Weakening production behavior just to pass tests.\n\n")
	b.WriteString("## Candidate test commands\n\n")
	b.WriteString(commandList(signals.TestCommands))
	b.WriteString("\n## Candidate build commands\n\n")
	b.WriteString(commandList(signals.BuildCommands))
	b.WriteString("\n## Candidate lint/static checks\n\n")
	b.WriteString(commandList(signals.LintCommands))
	b.WriteString("\n## Rule\n\nVerification commands are candidates only. Before running, check package scripts, CI workflows, and README for more specific instructions. When adding tests, apply the well-structured/poorly-structured criteria above first.\n")
	return b.String()
}
