package projectdoc

import "strings"

// docMetaDescriptions is the canonical, name-keyed metadata for standard project
// docs: a fixed one-line description of WHAT CATEGORY of information each doc
// holds (not a summary of its current content). Same doc name => same
// description in every repo, and it stays fixed across bootstrap and
// bootstrap --sync. It is rendered as SKILL.md-style YAML frontmatter at the top
// of each doc so both humans and the project-doc catalog read the same source.
var docMetaDescriptions = map[string]string{
	"ARCHITECTURE.md":   "System structure, component boundaries, and responsibilities.",
	"ADR.md":            "Structural decisions, rationale, and rejected alternatives.",
	"CONSTITUTION.md":   "Instruction priority, safety, and accuracy principles.",
	"CONVENTIONS.md":    "Coding conventions, package structure, and layer boundaries.",
	"TECH_STACK.md":     "Chosen languages, runtimes, tools, and rationale.",
	"TESTING.md":        "Verification standards, test practices, and required checks.",
	"COMMIT_POLICY.md":  "Commit message format, scope, and decision-record rules.",
	"CAUTIONS.md":       "Recurring mistakes, operational cautions, and avoidance guidance.",
	"OPERATIONS.md":     "Operations quick-start, reference map, and runtime procedures.",
	"OPEN_API_SPEC.md":  "Endpoint, DTO, and OpenAPI documentation gate rules.",
	"AGENT_WORKFLOW.md": "Agent start, execution, verification, and completion flow.",
	"VCS.md":            "Verified VCS provider capabilities, request recipes, identity checks, and CLI fallbacks.",
}

// DocMetaDescription returns the canonical metadata description for a standard
// project doc filename, and whether one exists.
func DocMetaDescription(name string) (string, bool) {
	desc, ok := docMetaDescriptions[name]
	return desc, ok
}

// parseDocFrontmatter extracts a leading SKILL.md-style frontmatter block
// (--- ... ---) from the very top of content. It returns the name and
// description fields, the body after the block, and whether a block was present.
func ParseFrontmatter(content string) (name, description, body string, ok bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", content, false
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return "", "", content, false
	}
	for _, line := range lines[1:closeIdx] {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.TrimSpace(value)
		case "description":
			description = strings.TrimSpace(value)
		}
	}
	body = strings.TrimLeft(strings.Join(lines[closeIdx+1:], "\n"), "\n")
	return name, description, body, true
}

// renderDocMetaFrontmatter renders the canonical frontmatter block for a doc.
func renderDocMetaFrontmatter(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n"
}

// ensureDocMetaFrontmatter guarantees content begins with the canonical meta
// frontmatter for the given doc name while preserving the existing body. An
// existing frontmatter block is replaced; otherwise one is prepended. Content is
// returned unchanged when the doc has no canonical metadata. The operation is
// idempotent: applying it twice yields identical output.
func EnsureMetaFrontmatter(name, content string) string {
	desc, ok := DocMetaDescription(name)
	if !ok {
		return content
	}
	_, _, body, hadFrontmatter := ParseFrontmatter(content)
	if !hadFrontmatter {
		body = content
	}
	block := renderDocMetaFrontmatter(name, desc)
	if strings.TrimSpace(body) == "" {
		return block
	}
	return block + "\n" + strings.TrimLeft(body, "\n")
}
