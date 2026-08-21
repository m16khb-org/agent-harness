package projectbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	projectdoc "agent-harness/internal/domain/projectdoc"
	projectdocdomain "agent-harness/internal/domain/projectdoc"
)

func BootstrapProjectDocs(req ProjectDocsBootstrapRequest) (ProjectDocsBootstrapResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	signals := AnalyzeProjectSignals(root)
	lifecycleState, err := initProjectLifecycleState(root, req.Write, signals.Profile)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	files := []projectdoc.ProjectDocsPlannedFile{}
	warnings := append([]string{}, lifecycleState.Warnings...)
	contents := RenderProjectDocs(root, signals)
	contents["AGENTS.md"] = RenderAgentsWithBlock(root, contents["AGENTS.md"])
	// Folder-first rule: family roots and module starters are only ever
	// created, never updated — not even with --sync. Curated content in a
	// modular repository must flow through project_docs_revise or
	// project-docs-optimize, never a template refresh.
	manifestPath := filepath.Join(root, filepath.FromSlash(projectdocdomain.ManifestRelPath()))
	manifestExisted := fileExists(manifestPath)
	// Legacy flat layout: family root documents exist without the modular
	// contract. agent-harness is a library applied to many repositories, so
	// an in-progress repo's established flat docs stay untouched: creating
	// module starters and a manifest around curated flat roots would produce
	// a half-migrated layout that violates the optimize checker contract.
	// Restructuring belongs to project-docs-optimize, not bootstrap.
	legacyFlat := false
	if !manifestExisted {
		for _, f := range projectdocdomain.DocFamilies() {
			if fileExists(filepath.Join(root, filepath.ToSlash(filepath.Join(projectdocdomain.ProjectDocsDir, f.Root)))) {
				legacyFlat = true
				break
			}
		}
	}
	if legacyFlat {
		warnings = projectdocdomain.AppendUnique(warnings, "legacy_flat_layout_preserved: existing flat family roots were kept without partial modular scaffolding; restructure with project-docs-optimize instead of bootstrap")
	}
	familyPreserved := false
	for _, rel := range append([]string{"AGENTS.md"}, projectdocdomain.PrefixedProjectDocNames()...) {
		content := contents[rel]
		if content == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		action := PlannedFileAction(path, content)
		familyDoc := isFamilyDocRel(rel)
		shouldWrite := req.Write && action != "unchanged" && !familyDoc && (req.Sync || action == "create" || rel == "AGENTS.md")
		if familyDoc && action == "create" && !legacyFlat {
			shouldWrite = req.Write
		}
		if familyDoc && action == "update" {
			familyPreserved = true
		}
		preserved := action == "update" && (familyDoc || (rel != "AGENTS.md" && !req.Sync))
		if shouldWrite {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
		} else if action == "update" && !req.Sync && !familyDoc {
			warnings = projectdocdomain.AppendUnique(warnings, "sync_available: existing project docs were preserved; pass --sync to refresh them from current templates and repo evidence")
		}
		files = append(files, projectdoc.ProjectDocsPlannedFile{
			RelPath:   filepath.ToSlash(rel),
			Path:      path,
			Action:    action,
			Bytes:     len([]byte(content)),
			SHA256:    projectdoc.SHA256Hex(content),
			Reason:    projectDocReason(rel),
			Preserved: preserved,
		})
	}
	// Module starter documents for every family (folder-first layout).
	for _, f := range projectdocdomain.DocFamilies() {
		rel := filepath.ToSlash(filepath.Join(projectdocdomain.ProjectDocsDir, f.OverviewRel()))
		content := contents[rel]
		if content == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		action := PlannedFileAction(path, content)
		if req.Write && action == "create" && !legacyFlat {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
		} else if action == "update" {
			familyPreserved = true
		}
		if !legacyFlat {
			files = append(files, projectdoc.ProjectDocsPlannedFile{
				RelPath:   rel,
				Path:      path,
				Action:    action,
				Bytes:     len([]byte(content)),
				SHA256:    projectdoc.SHA256Hex(content),
				Reason:    projectDocReason(rel),
				Preserved: action == "update",
			})
		}
	}
	// Modular documentation contract: seeded once, then owned by the repo
	// (budgets may be tuned by hand or by project-docs-optimize; never reset).
	manifestContent := projectdocdomain.ManifestJSON()
	manifestAction := PlannedFileAction(manifestPath, manifestContent)
	if req.Write && manifestAction == "create" && !legacyFlat {
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
			return ProjectDocsBootstrapResult{}, err
		}
		if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
			return ProjectDocsBootstrapResult{}, err
		}
	}
	if !legacyFlat {
		files = append(files, projectdoc.ProjectDocsPlannedFile{
			RelPath: projectdocdomain.ManifestRelPath(),
			Path:    manifestPath,
			Action:  manifestAction,
			Bytes:   len([]byte(manifestContent)),
			SHA256:  projectdoc.SHA256Hex(manifestContent),
			Reason:  "modular documentation contract manifest",
		})
	}
	if familyPreserved {
		warnings = projectdocdomain.AppendUnique(warnings, "family_docs_preserved: modular family roots and module starters are never overwritten; revise them with project_docs_revise or reorganize with project-docs-optimize")
	}
	if manifestExisted && req.Write && req.Sync {
		warnings = projectdocdomain.AppendUnique(warnings, "manifest_preserved: documentation/manifest.json budgets are repo-owned; --sync does not reset them")
	}
	// Ensure every standard project doc carries its canonical meta frontmatter,
	// preserving body content. This runs on bootstrap and --sync alike so even
	// preserved (non-synced) docs declare their category, fixed by doc name.
	if req.Write {
		for _, rel := range projectdocdomain.PrefixedProjectDocNames() {
			path := filepath.Join(root, filepath.FromSlash(rel))
			existing, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			ensured := projectdocdomain.EnsureMetaFrontmatter(filepath.Base(rel), string(existing))
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
		DocsDir:        filepath.Join(root, projectdocdomain.ProjectDocsDir),
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
	if rel == projectdocdomain.ManifestRelPath() {
		return "modular documentation contract manifest"
	}
	stripped := strings.TrimPrefix(rel, projectdocdomain.ProjectDocsDir+"/")
	if _, ok := projectdocdomain.FamilyByRoot(stripped); ok {
		return "family root index linking its module directory"
	}
	if strings.HasPrefix(stripped, "adr/") || strings.HasPrefix(stripped, "architecture/") || strings.HasPrefix(stripped, "cautions/") || strings.HasPrefix(stripped, "conventions/") || strings.HasPrefix(stripped, "operations/") || strings.HasPrefix(stripped, "testing/") {
		return "family module starter document"
	}
	return "project-specific agent operating document"
}

func isFamilyDocRel(rel string) bool {
	stripped := strings.TrimPrefix(rel, projectdocdomain.ProjectDocsDir+"/")
	if _, ok := projectdocdomain.FamilyByRoot(stripped); ok {
		return true
	}
	for _, f := range projectdocdomain.DocFamilies() {
		if strings.HasPrefix(stripped, f.ModuleDir+"/") {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
