package projectdoc

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// DocFamily pairs a required root index document with the module directory
// that owns its detail. Folder-first bootstrap creates both halves so the
// project-docs-optimize checker contract (root index + non-empty linked module
// dir) holds from the very first write.
type DocFamily struct {
	Root           string
	ModuleDir      string
	Responsibility string
}

var docFamilies = []DocFamily{
	{Root: "ADR.md", ModuleDir: "adr", Responsibility: "accepted architecture decisions and roadmap"},
	{Root: "ARCHITECTURE.md", ModuleDir: "architecture", Responsibility: "dependency direction and runtime topology"},
	{Root: "CAUTIONS.md", ModuleDir: "cautions", Responsibility: "known risks and incident lessons"},
	{Root: "CONVENTIONS.md", ModuleDir: "conventions", Responsibility: "implementation and interface conventions"},
	{Root: "OPERATIONS.md", ModuleDir: "operations/guides", Responsibility: "installation and runtime operation"},
	{Root: "TESTING.md", ModuleDir: "testing", Responsibility: "test strategy and verification gates"},
}

// DocFamilies returns the family definitions in canonical order.
func DocFamilies() []DocFamily {
	return append([]DocFamily(nil), docFamilies...)
}

// FamilyByRoot reports the family owning a required root document name such
// as "ADR.md".
func FamilyByRoot(name string) (DocFamily, bool) {
	for _, f := range docFamilies {
		if f.Root == name {
			return f, true
		}
	}
	return DocFamily{}, false
}

// FamilyByModuleDir reports the family owning a module directory such as
// "adr" or "operations/guides".
func FamilyByModuleDir(dir string) (DocFamily, bool) {
	for _, f := range docFamilies {
		if f.ModuleDir == dir {
			return f, true
		}
	}
	return DocFamily{}, false
}

// OverviewRel returns the module starter path relative to ProjectDocsDir,
// for example "adr/overview.md".
func (f DocFamily) OverviewRel() string {
	return filepath.ToSlash(filepath.Join(f.ModuleDir, "overview.md"))
}

// ManifestRelPath is the modular documentation contract location, relative to
// the repository root.
func ManifestRelPath() string {
	return filepath.ToSlash(filepath.Join(ProjectDocsDir, "documentation", "manifest.json"))
}

// familyModulePath reports whether rel is a valid family module document path
// such as "adr/overview.md" or "adr/2026-08-20-slug.md". It rejects path
// traversal and non-markdown files.
func familyModulePath(rel string) (DocFamily, bool) {
	if strings.Contains(rel, "..") || !strings.HasSuffix(rel, ".md") {
		return DocFamily{}, false
	}
	if filepath.ToSlash(filepath.Clean(rel)) != rel {
		return DocFamily{}, false
	}
	for _, f := range docFamilies {
		prefix := f.ModuleDir + "/"
		if strings.HasPrefix(rel, prefix) && len(rel) > len(prefix) {
			return f, true
		}
	}
	return DocFamily{}, false
}

// familyRecordMetaDescriptions is the canonical frontmatter description for
// append-created record files inside each family module directory.
var familyRecordMetaDescriptions = map[string]string{
	"adr":               "Accepted decision record with rationale, alternatives, and consequences.",
	"architecture":      "Architecture module detail: structure, boundaries, and dependencies.",
	"cautions":          "Caution record for a solved false case or recurring risk.",
	"conventions":       "Conventions module detail: concrete implementation and interface rules.",
	"operations/guides": "Operations guide module for one runtime or installation procedure.",
	"testing":           "Testing module detail: strategy, fixtures, and verification gates.",
}

// RecordMetaDescription returns the canonical record-file description for a
// family module directory.
func RecordMetaDescription(moduleDir string) (string, bool) {
	desc, ok := familyRecordMetaDescriptions[moduleDir]
	return desc, ok
}

// ManifestJSON renders the folder-first documentation contract seeded by
// bootstrap: six families plus the single-owner root documents. It mirrors
// the project-docs-optimize checker schema (schema_version 1).
func ManifestJSON() string {
	type familyJSON struct {
		Root           string `json:"root"`
		ModuleDir      string `json:"module_dir"`
		Responsibility string `json:"responsibility"`
	}
	families := make([]familyJSON, 0, len(docFamilies))
	for _, f := range docFamilies {
		families = append(families, familyJSON{
			Root:           filepath.ToSlash(filepath.Join(ProjectDocsDir, f.Root)),
			ModuleDir:      filepath.ToSlash(filepath.Join(ProjectDocsDir, f.ModuleDir)),
			Responsibility: f.Responsibility,
		})
	}
	manifest := struct {
		SchemaVersion     int               `json:"schema_version"`
		MaxRootLines      int               `json:"max_root_lines"`
		MaxModuleLines    int               `json:"max_module_lines"`
		Families          []familyJSON      `json:"families"`
		SingleOwnerTopics map[string]string `json:"single_owner_topics"`
	}{
		SchemaVersion:  1,
		MaxRootLines:   250,
		MaxModuleLines: 250,
		Families:       families,
		SingleOwnerTopics: map[string]string{
			"commit formatting":                  filepath.ToSlash(filepath.Join(ProjectDocsDir, "COMMIT_POLICY.md")),
			"constitutional priority and safety": filepath.ToSlash(filepath.Join(ProjectDocsDir, "CONSTITUTION.md")),
			"OpenAPI requirements":               filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPEN_API_SPEC.md")),
			"technology selection":               filepath.ToSlash(filepath.Join(ProjectDocsDir, "TECH_STACK.md")),
			"agent execution sequence":           filepath.ToSlash(filepath.Join(ProjectDocsDir, "AGENT_WORKFLOW.md")),
		},
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
}
