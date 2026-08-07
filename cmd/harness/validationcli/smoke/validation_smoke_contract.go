package smoke

import (
	docs "agent-harness/internal/adapter/docs"
	inspect "agent-harness/internal/adapter/inspect"
	"os"
	"path/filepath"
	"strings"
)

func inspectSmokeValidationErrors(info inspect.InspectInfo, stdout, root string) []string {
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
	if containsForbiddenLegacyOutsideRuntimePaths(stdout, root) {
		errs = append(errs, "inspect output contains legacy "+"m"+"16 name")
	}
	return errs
}

func docsIndexSmokeValidationErrors(index docs.DocsIndexResult, root string) []string {
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
	return errs
}

func docIndexContains(docs []docs.DocIndexInfo, relPath string) bool {
	for _, doc := range docs {
		if doc.RelPath == relPath {
			return true
		}
	}
	return false
}

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	sanitized := allowCurrentOwnerHandle(text)
	replacements := []string{}
	if abs, err := filepath.Abs(root); err == nil {
		replacements = append(replacements, abs)
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements = append(replacements, home)
	}
	for _, runtimePath := range replacements {
		if runtimePath == "" || runtimePath == string(filepath.Separator) {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, runtimePath, "$RUNTIME_PATH")
	}
	for _, needle := range forbiddenLegacyNeedles() {
		if strings.Contains(sanitized, needle) {
			return true
		}
	}
	return false
}

func forbiddenLegacyNeedles() []string {
	return []string{"m" + "16kh", "m" + "16h", "M" + "16H", "m" + "16"}
}

func currentOwnerHandle() string {
	return "m" + "16khb"
}

func allowCurrentOwnerHandle(text string) string {
	return strings.ReplaceAll(text, currentOwnerHandle(), "$CURRENT_OWNER")
}
