package projectdocs

import (
	projectdocdomain "agent-harness/internal/domain/projectdoc"
	projectdocscontract "agent-harness/internal/contract/projectdocs"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func AppendProjectDocsEntry(req projectdocscontract.ProjectDocsAppendRequest) (projectdocscontract.ProjectDocsAppendResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	recordKind := strings.ToLower(strings.TrimSpace(req.Kind))
	recordKind = strings.ReplaceAll(recordKind, "_", "-")
	recordKind = strings.ReplaceAll(recordKind, " ", "-")
	switch recordKind {
	case "caution", "cautions", "false-case", "failure", "problem":
		recordKind = "caution"
	case "adr", "decision", "architecture-decision":
		recordKind = "adr"
	default:
		return projectdocscontract.ProjectDocsAppendResult{}, fmt.Errorf("unsupported record kind %q: use caution or adr", req.Kind)
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return projectdocscontract.ProjectDocsAppendResult{}, fmt.Errorf("title is required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return projectdocscontract.ProjectDocsAppendResult{}, fmt.Errorf("summary is required")
	}
	// Single path: records always land as one dated file inside the family
	// module directory; the family root index is never touched (no root SHA
	// churn, checker-safe in bootstrapped repositories).
	return appendModularRecord(root, recordKind, req, time.Now())
}

// appendModularRecord writes one dated record file into the family module
// directory (adr/ or cautions/) with canonical frontmatter. The root index is
// never touched, so append cannot churn the root SHA other agents may hold.
func appendModularRecord(root, recordKind string, req projectdocscontract.ProjectDocsAppendRequest, now time.Time) (projectdocscontract.ProjectDocsAppendResult, error) {
	moduleDir := "cautions"
	if recordKind == "adr" {
		moduleDir = "adr"
	}
	family, ok := projectdocdomain.FamilyByModuleDir(moduleDir)
	if !ok {
		return projectdocscontract.ProjectDocsAppendResult{}, fmt.Errorf("no family owns module dir %q", moduleDir)
	}
	slug := recordSlug(req.Title)
	date := now.Format("2006-01-02")
	base := date + "-" + slug + ".md"
	dirPath := filepath.Join(root, filepath.FromSlash(filepath.Join(ProjectDocsDir, family.ModuleDir)))
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	path := filepath.Join(dirPath, base)
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dirPath, fmt.Sprintf("%s-%s-%d.md", date, slug, n))
	}
	rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, root), "/"))
	description, _ := projectdocdomain.RecordMetaDescription(family.ModuleDir)
	content := renderProjectDocsAppendRecordFile(recordKind, strings.TrimSuffix(filepath.Base(path), ".md"), description, req, now)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	return projectdocscontract.ProjectDocsAppendResult{
		OK:            true,
		Kind:          "project_docs_append",
		RecordKind:    recordKind,
		RepoRoot:      root,
		RelPath:       rel,
		Path:          path,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		BytesAppended: len([]byte(content)),
		SHA256:        sha256Hex(content),
	}, nil
}

func recordSlug(title string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen && b.Len() > 0:
			b.WriteByte('-')
			lastHyphen = true
		}
		if b.Len() >= 60 {
			break
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "record"
	}
	return slug
}
