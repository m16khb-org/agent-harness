package projectdocs

import (
	"fmt"
	projectdoc "issueops/internal/domain/projectdoc"
	"path/filepath"
	"strings"
)

// familyTitles maps a family root document to its index heading.
var familyTitles = map[string]string{
	"ADR.md":          "Architecture Decision Records",
	"ARCHITECTURE.md": "Architecture",
	"CAUTIONS.md":     "Cautions",
	"CONVENTIONS.md":  "Conventions",
	"OPERATIONS.md":   "Operations",
	"TESTING.md":      "Testing",
}

// familyOverviewBodies maps a family root document to the detailed starter
// body renderer that used to be the whole root document in the flat layout.
// The detail moves into the module starter; the root becomes a short index.
func familyOverviewBody(root string, signals projectdoc.ProjectSignals) string {
	switch root {
	case "ADR.md":
		return renderADR()
	case "ARCHITECTURE.md":
		return renderArchitecture(signals)
	case "CAUTIONS.md":
		return renderCautions(signals)
	case "CONVENTIONS.md":
		return renderConventions(signals)
	case "OPERATIONS.md":
		return renderOperations(signals)
	case "TESTING.md":
		return renderTesting(signals)
	}
	return ""
}

// renderFamilyIndex renders the short root index for one family: frontmatter,
// a one-line responsibility statement, and a link into the module directory.
// The project-docs-optimize checker requires this link.
func renderFamilyIndex(f projectdoc.DocFamily) string {
	title := familyTitles[f.Root]
	overviewRel := f.OverviewRel()
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "%s. This root is the family index; focused detail lives under [%s/](%s/).\n\n", capitalize(f.Responsibility), f.ModuleDir, f.ModuleDir)
	fmt.Fprintf(&b, "- [%s overview](%s)\n\n", title, overviewRel)
	b.WriteString("## Appending knowledge\n\n")
	b.WriteString("- Append new dated records with MCP `project_docs_append` or `issueops project append`.\n")
	b.WriteString("- Records are written as one file per record inside the module directory, so this index stays small.\n")
	b.WriteString("- Revise this index only to add links to new curated modules; keep it within the manifest line budget.\n")
	return b.String()
}

// renderFamilyOverview renders the module starter document with explicit
// frontmatter and a back-link to the owning root index. The back-link depth
// follows the module directory nesting (operations/guides needs ../../).
func renderFamilyOverview(f projectdoc.DocFamily, signals projectdoc.ProjectSignals) string {
	title := familyTitles[f.Root]
	body := familyOverviewBody(f.Root, signals)
	depth := strings.Count(f.ModuleDir, "/") + 1
	up := strings.Repeat("../", depth)
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: overview\ndescription: Family module overview: %s.\n---\n\n", f.Responsibility)
	fmt.Fprintf(&b, "# %s — Overview\n\n", title)
	fmt.Fprintf(&b, "Canonical index: [%s](%s%s)\n\n", f.Root, up, f.Root)
	b.WriteString(strings.TrimPrefix(body, fmt.Sprintf("# %s\n\n", title)))
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// renderFamilyModuleDocs returns the module starter entries for all families.
func renderFamilyModuleDocs(signals projectdoc.ProjectSignals) map[string]string {
	out := map[string]string{}
	for _, f := range projectdoc.DocFamilies() {
		rel := filepath.ToSlash(filepath.Join(ProjectDocsDir, f.OverviewRel()))
		out[rel] = renderFamilyOverview(f, signals)
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
