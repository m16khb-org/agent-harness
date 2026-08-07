package projectbootstrap

import (
	"os"
	"path/filepath"
	"time"

	"agent-harness/internal/adapter/draftwiki"
	"agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectdoc"
	"agent-harness/internal/adapter/projectdocs"
	"agent-harness/internal/adapter/repopath"
)

func BootstrapProjectDocs(req ProjectDocsBootstrapRequest) (ProjectDocsBootstrapResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	signals := projectdocs.AnalyzeProjectSignals(root)
	lifecycleState, err := lifecycle.InitProjectLifecycleState(root, req.Write, signals.Profile)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	files := []projectdoc.ProjectDocsPlannedFile{}
	warnings := append([]string{}, lifecycleState.Warnings...)
	contents := projectdocs.RenderProjectDocs(root, signals)
	contents["AGENTS.md"] = projectdocs.RenderAgentsWithBlock(root, contents["AGENTS.md"])

	for _, rel := range append([]string{"AGENTS.md"}, projectdoc.PrefixedProjectDocNames()...) {
		content := contents[rel]
		if content == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		action := projectdoc.PlannedFileAction(path, content)
		shouldWrite := req.Write && action != "unchanged" && (req.Sync || action == "create" || rel == "AGENTS.md")
		if shouldWrite {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
		} else if req.Write && action == "update" && !req.Sync {
			warnings = projectdoc.AppendUnique(warnings, "sync_available: existing project docs were preserved; pass --sync to refresh them from current templates and repo evidence")
		}
		files = append(files, projectdoc.ProjectDocsPlannedFile{
			RelPath: filepath.ToSlash(rel),
			Path:    path,
			Action:  action,
			Bytes:   len([]byte(content)),
			SHA256:  projectdoc.SHA256Hex(content),
			Reason:  projectDocReason(rel),
		})
	}
	draftWiki, err := draftwiki.InitDraftWiki(draftwiki.DraftWikiInitRequest{RepoRoot: root, Write: req.Write})
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	files = append(files, draftWiki.Files...)
	// Ensure every standard project doc carries its canonical meta frontmatter,
	// preserving body content. This runs on bootstrap and --sync alike so even
	// preserved (non-synced) docs declare their category, fixed by doc name.
	if req.Write {
		for _, rel := range projectdoc.PrefixedProjectDocNames() {
			path := filepath.Join(root, filepath.FromSlash(rel))
			existing, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			ensured := projectdoc.EnsureMetaFrontmatter(filepath.Base(rel), string(existing))
			if ensured != string(existing) {
				if err := os.WriteFile(path, []byte(ensured), 0o644); err != nil {
					return ProjectDocsBootstrapResult{}, err
				}
			}
		}
	}
	if !req.Write {
		warnings = append(warnings, "dry_run_only: rerun without --dry-run to create missing AGENTS.md/.agent-harness docs and repo metadata; add --sync to refresh existing docs")
	}
	return ProjectDocsBootstrapResult{
		OK:             true,
		Kind:           "project_docs_bootstrap",
		RepoRoot:       root,
		DocsDir:        filepath.Join(root, projectdoc.ProjectDocsDir),
		Write:          req.Write,
		Sync:           req.Sync,
		DryRun:         !req.Write,
		GeneratedAt:    time.Now().Format(time.RFC3339),
		Signals:        signals,
		Files:          files,
		LifecycleState: lifecycleState,
		Warnings:       warnings,
	}, nil
}

func projectDocReason(rel string) string {
	if rel == "AGENTS.md" {
		return "agent entrypoint and routing block"
	}
	return "project-specific agent operating document"
}
