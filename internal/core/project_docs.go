package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const ProjectDocsDir = ".agent-harness"

var projectDocNames = []string{
	"ARCHITECTURE.md",
	"CAUTIONS.md",
	"COMMIT_POLICY.md",
	"CONSTITUTION.md",
	"CONVENTIONS.md",
	"TECH_STACK.md",
	"TESTING.md",
	"OPEN_API_SPEC.md",
	"ADR.md",
	"OPERATIONS.md",
	"AGENT_WORKFLOW.md",
}

const agentsStartMarker = "<!-- AGENT_HARNESS:START -->"
const agentsEndMarker = "<!-- AGENT_HARNESS:END -->"

const behavioralGuidelines = `# AGENTS.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
~~~text
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
~~~

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
`

const solidDesignPatternGuidance = `## SOLID / Design Pattern guidance

Apply SOLID, YAGNI, and KISS together. SOLID does not mean adding interfaces and layers by default; it means clarifying responsibility and dependency direction only where a real axis of change exists. Use a design pattern only when the name explains the problem and reduces maintenance cost.

### Good cases

- Apply existing patterns such as Adapter, Strategy, Factory, or Repository consistently to the same kind of problem.
- Put interfaces/ports at boundaries with real substitutability, such as external hosts, SDKs, filesystems, processes, or networks.
- Use dependency inversion where there are multiple implementations or a test double is needed.
- When introducing a pattern, record the problem, chosen pattern, rejected simpler alternatives, and cost in ADR.md.

### Bad cases

- Creating an interface, factory, registry, or plugin layer for a single use site.
- Adding abstraction or configurability based only on hypothetical future extension.
- Expanding a simple function call into a class/object graph just to match a pattern name.
- Duplicating core policy in host adapters or per-host implementations in the name of SOLID.

### Rules

- Start with the simplest implementation; introduce patterns only after a real variation point is confirmed.
- Add a new abstraction only when there are at least two use sites, a clear test boundary, or an external technology boundary.
- If a pattern turns a 50-line solution into a 200-line structure, revert and simplify.
`

type ProjectDocsBootstrapRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
	Sync     bool   `json:"sync"`
}

type ProjectDocsBootstrapResult struct {
	OK             bool                      `json:"ok"`
	Kind           string                    `json:"kind"`
	RepoRoot       string                    `json:"repo_root"`
	DocsDir        string                    `json:"docs_dir"`
	Write          bool                      `json:"write"`
	Sync           bool                      `json:"sync"`
	DryRun         bool                      `json:"dry_run"`
	GeneratedAt    string                    `json:"generated_at"`
	Signals        ProjectSignals            `json:"signals"`
	Files          []ProjectDocsPlannedFile  `json:"files"`
	LifecycleState ProjectLifecycleStatePlan `json:"lifecycle_state"`
	Warnings       []string                  `json:"warnings,omitempty"`
}

type ProjectDocsPlannedFile struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Action  string `json:"action"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
	Reason  string `json:"reason"`
}

type ProjectSignals struct {
	Files               []string          `json:"files"`
	Languages           []string          `json:"languages"`
	PackageManagers     []string          `json:"package_managers"`
	Profile             ProjectProfile    `json:"profile"`
	TestCommands        []EvidenceCommand `json:"test_commands"`
	BuildCommands       []EvidenceCommand `json:"build_commands"`
	LintCommands        []EvidenceCommand `json:"lint_commands"`
	ExistingAgentDocs   []string          `json:"existing_agent_docs"`
	GitHubWorkflows     []string          `json:"github_workflows"`
	DetectedConventions []string          `json:"detected_conventions"`
}

type EvidenceCommand struct {
	Command    string   `json:"command"`
	Evidence   []string `json:"evidence"`
	Confidence string   `json:"confidence"`
}

type ProjectProfile struct {
	VCS             ProjectVCSProfile `json:"vcs"`
	Languages       []string          `json:"languages"`
	PackageManagers []string          `json:"package_managers,omitempty"`
	ProjectTypes    []string          `json:"project_types,omitempty"`
	Frameworks      []string          `json:"frameworks,omitempty"`
	Monorepo        bool              `json:"monorepo"`
	Evidence        []string          `json:"evidence,omitempty"`
}

type ProjectVCSProfile struct {
	Provider   string `json:"provider"`
	Hosting    string `json:"hosting"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemoteName string `json:"remote_name,omitempty"`
}

type ProjectDocsRouteResult struct {
	OK          bool                   `json:"ok"`
	Kind        string                 `json:"kind"`
	RepoRoot    string                 `json:"repo_root"`
	Task        string                 `json:"task"`
	GeneratedAt string                 `json:"generated_at"`
	Docs        []ProjectDocRouteEntry `json:"docs"`
	Warnings    []string               `json:"warnings,omitempty"`
}

type ProjectDocRouteEntry struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	Exists  bool   `json:"exists"`
}

type ProjectDocsRecordRequest struct {
	RepoRoot     string   `json:"repo_root"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Context      string   `json:"context,omitempty"`
	Resolution   string   `json:"resolution,omitempty"`
	Decision     string   `json:"decision,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Consequences string   `json:"consequences,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type ProjectDocsRecordResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RecordKind    string   `json:"record_kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	GeneratedAt   string   `json:"generated_at"`
	BytesAppended int      `json:"bytes_appended"`
	SHA256        string   `json:"sha256"`
	Warnings      []string `json:"warnings,omitempty"`
}

type ProjectDocsReadResult struct {
	OK          bool     `json:"ok"`
	Kind        string   `json:"kind"`
	RepoRoot    string   `json:"repo_root"`
	RelPath     string   `json:"rel_path"`
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Content     string   `json:"content,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	Warnings    []string `json:"warnings,omitempty"`
}

type ProjectDocsUpdateRequest struct {
	RepoRoot       string   `json:"repo_root"`
	RelPath        string   `json:"rel_path"`
	Content        string   `json:"content"`
	ExpectedSHA256 string   `json:"expected_sha256,omitempty"`
	Summary        string   `json:"summary"`
	Evidence       []string `json:"evidence,omitempty"`
	Confirm        bool     `json:"confirm"`
}

type ProjectDocsUpdateResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	Action        string   `json:"action"`
	Confirmed     bool     `json:"confirmed"`
	DryRun        bool     `json:"dry_run"`
	GeneratedAt   string   `json:"generated_at"`
	CurrentSHA256 string   `json:"current_sha256,omitempty"`
	NextSHA256    string   `json:"next_sha256"`
	Bytes         int      `json:"bytes"`
	Summary       string   `json:"summary"`
	Evidence      []string `json:"evidence,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

func ProjectDocNames() []string {
	out := append([]string(nil), projectDocNames...)
	return out
}

func BootstrapProjectDocs(req ProjectDocsBootstrapRequest) (ProjectDocsBootstrapResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	signals := AnalyzeProjectSignals(root)
	lifecycleState, err := InitProjectLifecycleState(root, req.Write, signals.Profile)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	files := []ProjectDocsPlannedFile{}
	warnings := append([]string{}, lifecycleState.Warnings...)
	contents := renderProjectDocs(root, signals)
	contents["AGENTS.md"] = renderAgentsWithBlock(root, contents["AGENTS.md"])

	for _, rel := range append([]string{"AGENTS.md"}, prefixedProjectDocNames()...) {
		content := contents[rel]
		if content == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		action := plannedFileAction(path, content)
		shouldWrite := req.Write && action != "unchanged" && (req.Sync || action == "create" || rel == "AGENTS.md")
		if shouldWrite {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return ProjectDocsBootstrapResult{}, err
			}
		} else if req.Write && action == "update" && !req.Sync {
			warnings = appendUnique(warnings, "sync_available: existing project docs were preserved; pass --sync to refresh them from current templates and repo evidence")
		}
		files = append(files, ProjectDocsPlannedFile{
			RelPath: filepath.ToSlash(rel),
			Path:    path,
			Action:  action,
			Bytes:   len([]byte(content)),
			SHA256:  sha256Hex(content),
			Reason:  projectDocReason(rel),
		})
	}
	draftWiki, err := InitDraftWiki(DraftWikiInitRequest{RepoRoot: root, Write: req.Write})
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	files = append(files, draftWiki.Files...)
	// Ensure every standard project doc carries its canonical meta frontmatter,
	// preserving body content. This runs on bootstrap and --sync alike so even
	// preserved (non-synced) docs declare their category, fixed by doc name.
	if req.Write {
		for _, rel := range prefixedProjectDocNames() {
			path := filepath.Join(root, filepath.FromSlash(rel))
			existing, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			ensured := ensureDocMetaFrontmatter(filepath.Base(rel), string(existing))
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
		DocsDir:        filepath.Join(root, ProjectDocsDir),
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

func RouteProjectDocs(repoRoot, task string) (ProjectDocsRouteResult, error) {
	root, err := normalizeRepoRoot(repoRoot)
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

func AppendProjectDocsRecord(req ProjectDocsRecordRequest) (ProjectDocsRecordResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsRecordResult{}, err
	}
	recordKind := strings.ToLower(strings.TrimSpace(req.Kind))
	recordKind = strings.ReplaceAll(recordKind, "_", "-")
	recordKind = strings.ReplaceAll(recordKind, " ", "-")
	var rel string
	switch recordKind {
	case "caution", "cautions", "false-case", "failure", "problem":
		recordKind = "caution"
		rel = filepath.ToSlash(filepath.Join(ProjectDocsDir, "CAUTIONS.md"))
	case "adr", "decision", "architecture-decision":
		recordKind = "adr"
		rel = filepath.ToSlash(filepath.Join(ProjectDocsDir, "ADR.md"))
	default:
		return ProjectDocsRecordResult{}, fmt.Errorf("unsupported record kind %q: use caution or adr", req.Kind)
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return ProjectDocsRecordResult{}, fmt.Errorf("title is required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return ProjectDocsRecordResult{}, fmt.Errorf("summary is required")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ProjectDocsRecordResult{}, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		seed := "# " + map[string]string{"caution": "Cautions", "adr": "Architecture Decision Records"}[recordKind] + "\n\n"
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			return ProjectDocsRecordResult{}, err
		}
	}
	entry := renderProjectDocsRecordEntry(recordKind, req, time.Now())
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return ProjectDocsRecordResult{}, err
	}
	if _, err := f.WriteString(entry); err != nil {
		_ = f.Close()
		return ProjectDocsRecordResult{}, err
	}
	if err := f.Close(); err != nil {
		return ProjectDocsRecordResult{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ProjectDocsRecordResult{}, err
	}
	return ProjectDocsRecordResult{
		OK:            true,
		Kind:          "project_docs_record",
		RecordKind:    recordKind,
		RepoRoot:      root,
		RelPath:       rel,
		Path:          path,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		BytesAppended: len([]byte(entry)),
		SHA256:        sha256Hex(string(b)),
	}, nil
}

func ReadProjectDoc(repoRoot, relPath string) (ProjectDocsReadResult, error) {
	root, err := normalizeRepoRoot(repoRoot)
	if err != nil {
		return ProjectDocsReadResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(relPath)
	if err != nil {
		return ProjectDocsReadResult{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	result := ProjectDocsReadResult{
		OK:          true,
		Kind:        "project_docs_read",
		RepoRoot:    root,
		RelPath:     rel,
		Path:        path,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		result.Exists = false
		result.Warnings = []string{"document_missing: run project_docs_bootstrap_plan or agent-harness project bootstrap first"}
		return result, nil
	}
	if err != nil {
		return ProjectDocsReadResult{}, err
	}
	result.Exists = true
	result.Content = string(b)
	result.SHA256 = sha256Hex(result.Content)
	return result, nil
}

func UpdateProjectDoc(req ProjectDocsUpdateRequest) (ProjectDocsUpdateResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsUpdateResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(req.RelPath)
	if err != nil {
		return ProjectDocsUpdateResult{}, err
	}
	content := strings.TrimRight(req.Content, "\n") + "\n"
	if strings.TrimSpace(content) == "" {
		return ProjectDocsUpdateResult{}, fmt.Errorf("content is required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return ProjectDocsUpdateResult{}, fmt.Errorf("summary is required")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	current := ""
	currentSHA := ""
	if b, err := os.ReadFile(path); err == nil {
		current = string(b)
		currentSHA = sha256Hex(current)
	} else if !os.IsNotExist(err) {
		return ProjectDocsUpdateResult{}, err
	}
	if currentSHA != "" {
		expected := strings.TrimSpace(req.ExpectedSHA256)
		if expected == "" {
			return ProjectDocsUpdateResult{}, fmt.Errorf("expected_sha256 is required when updating an existing project doc; call project_docs_read first")
		}
		if expected != currentSHA {
			return ProjectDocsUpdateResult{}, fmt.Errorf("expected_sha256 mismatch for %s: current %s", rel, currentSHA)
		}
	}
	action := "create"
	if current != "" {
		action = plannedFileAction(path, content)
	}
	nextSHA := sha256Hex(content)
	warnings := []string{}
	if !req.Confirm {
		warnings = append(warnings, "dry_run_only: pass confirm=true to write the updated .agent-harness document")
	} else if action != "unchanged" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return ProjectDocsUpdateResult{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return ProjectDocsUpdateResult{}, err
		}
	}
	return ProjectDocsUpdateResult{
		OK:            true,
		Kind:          "project_docs_update",
		RepoRoot:      root,
		RelPath:       rel,
		Path:          path,
		Action:        action,
		Confirmed:     req.Confirm,
		DryRun:        !req.Confirm,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		CurrentSHA256: currentSHA,
		NextSHA256:    nextSHA,
		Bytes:         len([]byte(content)),
		Summary:       summary,
		Evidence:      nonEmptyStrings(req.Evidence),
		Warnings:      warnings,
	}, nil
}

func renderProjectDocsRecordEntry(kind string, req ProjectDocsRecordRequest, now time.Time) string {
	var b strings.Builder
	stamp := now.Format("2006-01-02")
	fmt.Fprintf(&b, "\n## %s — %s\n\n", stamp, strings.TrimSpace(req.Title))
	fmt.Fprintf(&b, "- Kind: `%s`\n", kind)
	if source := strings.TrimSpace(req.Source); source != "" {
		fmt.Fprintf(&b, "- Source: %s\n", source)
	}
	fmt.Fprintf(&b, "- Summary: %s\n", strings.TrimSpace(req.Summary))
	if v := strings.TrimSpace(req.Context); v != "" {
		fmt.Fprintf(&b, "- Context: %s\n", v)
	}
	if kind == "caution" {
		if v := strings.TrimSpace(req.Resolution); v != "" {
			fmt.Fprintf(&b, "- Resolution: %s\n", v)
		}
	} else {
		if v := strings.TrimSpace(req.Decision); v != "" {
			fmt.Fprintf(&b, "- Decision: %s\n", v)
		}
		if v := strings.TrimSpace(req.Consequences); v != "" {
			fmt.Fprintf(&b, "- Consequences: %s\n", v)
		}
	}
	if len(req.Evidence) > 0 {
		b.WriteString("- Evidence:\n")
		for _, ev := range req.Evidence {
			if ev = strings.TrimSpace(ev); ev != "" {
				fmt.Fprintf(&b, "  - %s\n", ev)
			}
		}
	}
	if len(req.Alternatives) > 0 {
		b.WriteString("- Alternatives / rejected options:\n")
		for _, alt := range req.Alternatives {
			if alt = strings.TrimSpace(alt); alt != "" {
				fmt.Fprintf(&b, "  - %s\n", alt)
			}
		}
	}
	return b.String()
}

func AnalyzeProjectSignals(root string) ProjectSignals {
	files := listInterestingFiles(root)
	s := ProjectSignals{Files: files}
	addLang := func(v string) { s.Languages = appendUnique(s.Languages, v) }
	addPM := func(v string) { s.PackageManagers = appendUnique(s.PackageManagers, v) }
	addConvention := func(v string) { s.DetectedConventions = appendUnique(s.DetectedConventions, v) }
	for _, rel := range files {
		switch rel {
		case "go.mod":
			addLang("Go")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "go test ./...", Evidence: []string{"go.mod"}, Confidence: "high"})
			s.BuildCommands = append(s.BuildCommands, EvidenceCommand{Command: "go build ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
			s.LintCommands = append(s.LintCommands, EvidenceCommand{Command: "go vet ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
		case "package.json":
			addLang("JavaScript/TypeScript")
			addPM("npm-compatible")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "npm test", Evidence: []string{"package.json"}, Confidence: "medium"})
		case "pnpm-lock.yaml":
			addPM("pnpm")
		case "yarn.lock":
			addPM("yarn")
		case "pyproject.toml":
			addLang("Python")
			addPM("pyproject")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "pytest", Evidence: []string{"pyproject.toml"}, Confidence: "medium"})
		case "Cargo.toml":
			addLang("Rust")
			addPM("cargo")
			s.TestCommands = append(s.TestCommands, EvidenceCommand{Command: "cargo test", Evidence: []string{"Cargo.toml"}, Confidence: "high"})
			s.BuildCommands = append(s.BuildCommands, EvidenceCommand{Command: "cargo build", Evidence: []string{"Cargo.toml"}, Confidence: "high"})
		case "Makefile":
			addConvention("Makefile exists; inspect targets before inventing commands")
		case "Taskfile.yml", "Taskfile.yaml":
			addConvention("Taskfile exists; prefer documented task targets when present")
		case "AGENTS.md", "CLAUDE.md":
			s.ExistingAgentDocs = appendUnique(s.ExistingAgentDocs, rel)
		}
		switch {
		case rel != "go.mod" && strings.HasSuffix(rel, "/go.mod"):
			addLang("Go")
		case rel != "package.json" && strings.HasSuffix(rel, "/package.json"):
			addLang("JavaScript/TypeScript")
			addPM("npm-compatible")
		case rel != "pyproject.toml" && strings.HasSuffix(rel, "/pyproject.toml"):
			addLang("Python")
			addPM("pyproject")
		case rel != "Cargo.toml" && strings.HasSuffix(rel, "/Cargo.toml"):
			addLang("Rust")
			addPM("cargo")
		}
		if strings.HasPrefix(rel, ".github/workflows/") {
			s.GitHubWorkflows = appendUnique(s.GitHubWorkflows, rel)
		}
	}
	sort.Strings(s.Languages)
	sort.Strings(s.PackageManagers)
	sort.Strings(s.ExistingAgentDocs)
	sort.Strings(s.GitHubWorkflows)
	sort.Strings(s.DetectedConventions)
	s.Profile = inferProjectProfile(root, s)
	return s
}

func inferProjectProfile(root string, signals ProjectSignals) ProjectProfile {
	profile := ProjectProfile{
		VCS:             inferProjectVCS(root),
		Languages:       append([]string{}, signals.Languages...),
		PackageManagers: append([]string{}, signals.PackageManagers...),
		Evidence:        []string{},
	}
	addEvidence := func(v string) { profile.Evidence = appendUnique(profile.Evidence, v) }
	if profile.VCS.RemoteHost != "" || profile.VCS.Provider == "git" || profile.VCS.Provider == "local" {
		addEvidence("git remote/config")
	}
	for _, rel := range signals.Files {
		switch {
		case rel == "go.mod" || strings.HasSuffix(rel, "/go.mod"):
			addEvidence(rel)
		case rel == "package.json" || strings.HasSuffix(rel, "/package.json"):
			addEvidence(rel)
		case rel == "pyproject.toml" || strings.HasSuffix(rel, "/pyproject.toml"):
			addEvidence(rel)
		case rel == "Cargo.toml" || strings.HasSuffix(rel, "/Cargo.toml"):
			addEvidence(rel)
		case rel == "pnpm-workspace.yaml", rel == "turbo.json", rel == "nx.json", rel == "lerna.json":
			addEvidence(rel)
		}
	}
	profile.Frameworks = detectFrameworks(root, signals.Files, addEvidence)
	profile.Monorepo = detectMonorepo(root, signals.Files, addEvidence)
	profile.ProjectTypes = inferProjectTypes(root, signals, profile.Frameworks, profile.Monorepo, addEvidence)
	sort.Strings(profile.Frameworks)
	sort.Strings(profile.ProjectTypes)
	sort.Strings(profile.Evidence)
	return profile
}

func inferProjectVCS(root string) ProjectVCSProfile {
	origin := readGitOriginURL(root)
	if origin == "" {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			return ProjectVCSProfile{Provider: "git", Hosting: "local", RemoteName: "origin"}
		}
		return ProjectVCSProfile{Provider: "none", Hosting: "local"}
	}
	host := remoteHost(origin)
	provider := "git"
	hosting := "self-hosted"
	switch strings.ToLower(host) {
	case "github.com":
		provider, hosting = "github", "managed"
	case "gitlab.com":
		provider, hosting = "gitlab", "managed"
	case "bitbucket.org":
		provider, hosting = "bitbucket", "managed"
	default:
		lowerHost := strings.ToLower(host)
		switch {
		case strings.Contains(lowerHost, "gitlab"):
			provider = "gitlab"
		case strings.Contains(lowerHost, "github"):
			provider = "github"
		case strings.Contains(lowerHost, "bitbucket"):
			provider = "bitbucket"
		}
	}
	if host == "" {
		hosting = "unknown"
	}
	return ProjectVCSProfile{Provider: provider, Hosting: hosting, RemoteHost: host, RemoteName: "origin"}
}

func remoteHost(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err == nil {
			return strings.ToLower(u.Hostname())
		}
	}
	if at := strings.Index(remote, "@"); at >= 0 {
		rest := remote[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			return strings.ToLower(rest[:colon])
		}
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return strings.ToLower(rest[:slash])
		}
	}
	return ""
}

func detectFrameworks(root string, files []string, addEvidence func(string)) []string {
	frameworks := []string{}
	addFramework := func(name, evidence string) {
		frameworks = appendUnique(frameworks, name)
		if evidence != "" {
			addEvidence(evidence)
		}
	}
	for _, rel := range files {
		base := filepath.Base(rel)
		switch base {
		case "next.config.js", "next.config.mjs", "next.config.ts":
			addFramework("Next.js", rel)
		case "vite.config.js", "vite.config.mjs", "vite.config.ts":
			addFramework("Vite", rel)
		case "nuxt.config.js", "nuxt.config.ts":
			addFramework("Nuxt", rel)
		case "astro.config.js", "astro.config.mjs", "astro.config.ts":
			addFramework("Astro", rel)
		case "nest-cli.json":
			addFramework("NestJS", rel)
		}
		if rel == "go.mod" {
			for _, mod := range readGoModules(root) {
				switch {
				case strings.Contains(mod, "github.com/spf13/cobra"):
					addFramework("Cobra", "go.mod:github.com/spf13/cobra")
				case strings.Contains(mod, "github.com/gin-gonic/gin"):
					addFramework("Gin", "go.mod:github.com/gin-gonic/gin")
				case strings.Contains(mod, "github.com/go-chi/chi"):
					addFramework("chi", "go.mod:github.com/go-chi/chi")
				case strings.Contains(mod, "github.com/labstack/echo"):
					addFramework("Echo", "go.mod:github.com/labstack/echo")
				}
			}
		}
		if rel == "package.json" {
			for dep := range readPackageDependencies(filepath.Join(root, rel)) {
				switch dep {
				case "react":
					addFramework("React", "package.json:react")
				case "next":
					addFramework("Next.js", "package.json:next")
				case "vite":
					addFramework("Vite", "package.json:vite")
				case "vue":
					addFramework("Vue", "package.json:vue")
				case "svelte":
					addFramework("Svelte", "package.json:svelte")
				case "@angular/core":
					addFramework("Angular", "package.json:@angular/core")
				case "express":
					addFramework("Express", "package.json:express")
				case "@nestjs/core":
					addFramework("NestJS", "package.json:@nestjs/core")
				case "fastify":
					addFramework("Fastify", "package.json:fastify")
				case "prisma", "@prisma/client":
					addFramework("Prisma", "package.json:"+dep)
				}
			}
		}
	}
	return frameworks
}

func detectMonorepo(root string, files []string, addEvidence func(string)) bool {
	for _, rel := range files {
		switch rel {
		case "pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json":
			addEvidence(rel)
			return true
		}
		if strings.Contains(rel, "/") && (strings.HasSuffix(rel, "/package.json") || strings.HasSuffix(rel, "/go.mod") || strings.HasSuffix(rel, "/pyproject.toml") || strings.HasSuffix(rel, "/Cargo.toml")) {
			addEvidence(rel)
			return true
		}
	}
	if workspaces := readPackageWorkspaces(filepath.Join(root, "package.json")); len(workspaces) > 0 {
		addEvidence("package.json:workspaces")
		return true
	}
	return false
}

func inferProjectTypes(root string, signals ProjectSignals, frameworks []string, monorepo bool, addEvidence func(string)) []string {
	types := []string{}
	addType := func(v, evidence string) {
		types = appendUnique(types, v)
		if evidence != "" {
			addEvidence(evidence)
		}
	}
	if monorepo {
		addType("monorepo", "")
	}
	frontend := containsAnyString(frameworks, "React", "Next.js", "Vite", "Vue", "Svelte", "Angular", "Nuxt", "Astro")
	backend := containsAnyString(frameworks, "Express", "NestJS", "Fastify", "Gin", "chi", "Echo") || containsAnyString(signals.Languages, "Go")
	cli := containsAnyString(frameworks, "Cobra")
	if frontend {
		addType("frontend", "")
	}
	if backend {
		addType("backend", "")
	}
	if frontend && backend {
		addType("fullstack", "")
	}
	if cli || dirExists(filepath.Join(root, "cmd")) {
		addType("cli", "cmd/")
	}
	if len(types) == 0 && len(signals.Languages) > 0 {
		addType("library", "")
	}
	return types
}

func readPackageDependencies(path string) map[string]bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	var pkg struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return map[string]bool{}
	}
	out := map[string]bool{}
	for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies, pkg.OptionalDependencies} {
		for dep := range deps {
			out[dep] = true
		}
	}
	return out
}

func readPackageWorkspaces(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	var direct []string
	if err := json.Unmarshal(raw["workspaces"], &direct); err == nil && len(direct) > 0 {
		return direct
	}
	var object struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw["workspaces"], &object); err == nil {
		return object.Packages
	}
	return nil
}

func readGoModules(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}

func containsAnyString(items []string, wants ...string) bool {
	set := map[string]bool{}
	for _, item := range items {
		set[item] = true
	}
	for _, want := range wants {
		if set[want] {
			return true
		}
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func normalizeRepoRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repo root is not a directory: %s", abs)
	}
	return abs, nil
}

func prefixedProjectDocNames() []string {
	out := make([]string, 0, len(projectDocNames))
	for _, name := range projectDocNames {
		out = append(out, filepath.ToSlash(filepath.Join(ProjectDocsDir, name)))
	}
	return out
}

func normalizeProjectDocRelPath(relPath string) (string, error) {
	rel := filepath.ToSlash(strings.TrimSpace(relPath))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" {
		return "", fmt.Errorf("rel_path is required")
	}
	if strings.HasPrefix(rel, ProjectDocsDir+"/") {
		rel = strings.TrimPrefix(rel, ProjectDocsDir+"/")
	}
	allowed := map[string]bool{}
	for _, name := range projectDocNames {
		allowed[name] = true
	}
	if !allowed[rel] {
		return "", fmt.Errorf("unsupported project doc %q: use one of %s", relPath, strings.Join(projectDocNames, ", "))
	}
	return filepath.ToSlash(filepath.Join(ProjectDocsDir, rel)), nil
}

func nonEmptyStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func listInterestingFiles(root string) []string {
	interesting := map[string]bool{}
	maxDepth := 4
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" || base == ".agent-harness" {
				return filepath.SkipDir
			}
			if len(parts) > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(rel)
		if len(parts) <= maxDepth && (isProjectSignalFile(base) || strings.HasPrefix(rel, ".github/workflows/") || strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.ts")) {
			interesting[rel] = true
		}
		return nil
	})
	out := make([]string, 0, len(interesting))
	for rel := range interesting {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func isProjectSignalFile(base string) bool {
	switch base {
	case "AGENTS.md", "CLAUDE.md", "README.md", "go.mod", "go.sum", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", "yarn.lock", "package-lock.json", "pyproject.toml", "requirements.txt", "Cargo.toml", "Cargo.lock", "Makefile", "Taskfile.yml", "Taskfile.yaml", "Dockerfile", "docker-compose.yml", "docker-compose.yaml", "next.config.js", "next.config.mjs", "next.config.ts", "vite.config.js", "vite.config.mjs", "vite.config.ts", "nuxt.config.js", "nuxt.config.ts", "astro.config.js", "astro.config.mjs", "astro.config.ts", "tailwind.config.js", "tailwind.config.ts", "tsconfig.json", "turbo.json", "nx.json", "lerna.json", "nest-cli.json":
		return true
	default:
		return false
	}
}

func renderProjectDocs(root string, signals ProjectSignals) map[string]string {
	out := map[string]string{}
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "ARCHITECTURE.md"))] = renderArchitecture(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CAUTIONS.md"))] = renderCautions(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "COMMIT_POLICY.md"))] = renderCommitPolicy()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CONSTITUTION.md"))] = renderConstitution()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CONVENTIONS.md"))] = renderConventions(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "TECH_STACK.md"))] = renderTechStack(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "TESTING.md"))] = renderTesting(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPEN_API_SPEC.md"))] = renderOpenAPISpec()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "ADR.md"))] = renderADR()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPERATIONS.md"))] = renderOperations(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "AGENT_WORKFLOW.md"))] = renderAgentWorkflow()
	// Prepend canonical meta frontmatter so created/synced docs declare what
	// category of information they hold. Same doc name => same metadata.
	for rel, content := range out {
		out[rel] = ensureDocMetaFrontmatter(filepath.Base(rel), content)
	}
	return out
}

func renderAgentsWithBlock(root, _ string) string {
	block := strings.TrimSpace(fmt.Sprintf(`%s
## agent-harness project docs

This repository uses agent-harness project docs. Read existing AGENTS.md rules first, then read only the additional documents relevant to the task.

- Architecture or large design changes: %[2]s/ARCHITECTURE.md, %[2]s/CONSTITUTION.md
- Testing or verification changes: %[2]s/TESTING.md
- Endpoint/DTO/OpenAPI changes: %[2]s/OPEN_API_SPEC.md
- Commit or PR work: %[2]s/COMMIT_POLICY.md
- Code style or structure changes: %[2]s/CONVENTIONS.md
- Dependency or tech-stack changes: %[2]s/TECH_STACK.md
- Run, deploy, environment, or local development: %[2]s/OPERATIONS.md
- Agent start, verification, and completion workflow: %[2]s/AGENT_WORKFLOW.md
- Risky or recurring-failure work: %[2]s/CAUTIONS.md
- Structural rationale, alternatives, and decisions: %[2]s/ADR.md
- Session start, instruction conflicts, and principle decisions: %[2]s/CONSTITUTION.md
%s`, agentsStartMarker, ProjectDocsDir, agentsEndMarker)) + "\n"
	path := filepath.Join(root, "AGENTS.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimRight(behavioralGuidelines, "\n") + "\n\n---\n\n" + block + "\n"
	}
	text := ensureBehavioralGuidelinesAtTop(string(b))
	start := strings.Index(text, agentsStartMarker)
	end := strings.Index(text, agentsEndMarker)
	if start >= 0 && end > start {
		end += len(agentsEndMarker)
		return strings.TrimRight(text[:start], "\n") + "\n\n" + block + strings.TrimLeft(text[end:], "\n")
	}
	return strings.TrimRight(text, "\n") + "\n\n" + block
}

func ensureBehavioralGuidelinesAtTop(text string) string {
	trimmed := strings.TrimLeft(text, "\ufeff\n\r\t ")
	if strings.HasPrefix(trimmed, "# AGENTS.md\n\nBehavioral guidelines to reduce common LLM coding mistakes.") {
		return text
	}
	return strings.TrimRight(behavioralGuidelines, "\n") + "\n\n---\n\n" + strings.TrimLeft(text, "\n")
}

func renderOpenAPISpec() string {
	return `# OpenAPI Spec Guidance

## Purpose

This project-specific API documentation prompt is for agents and MCP routing when endpoint, controller, handler, DTO, schema, or OpenAPI files change.

## Gate order

1. Static gate: ` + "`agent-harness api-doc static-check --json`" + `
2. Agent gate: ` + "`agent-harness api-doc review --json`" + `
3. Combined gate: ` + "`agent-harness api-doc check --json`" + `

Default scope is staged API candidate files. Scan all legacy debt only when ` + "`--all`" + ` is explicitly supplied.

## Static omissions to block

- missing route operation summary/description
- description does not follow the repo's sectioned Markdown format
- missing path/query/header/body parameter documentation
- missing 400 response when validation surface exists
- missing 401 response for private/auth endpoints
- OpenAPI decorator or optional-validation mismatch on required/optional DTO fields

## Agent review prompt

Static checks catch decorator/comment-level omissions. Agent review reads directly related business logic to detect public API contract drift.

The agent must inspect service/usecase/domain/error-mapping code called by changed endpoints. If these errors can occur, they must appear in OpenAPI responses.

- entity/resource not found → 404
- auth/session/token failure → 401
- permission/ownership/tier/role failure → 403
- validation/body/query/header problem → 400
- duplicate/state conflict/idempotency conflict → 409

Documentation must not contradict real behavior. For example, if docs say the endpoint only reads cache but it changes payment state, or docs omit 404 while a service can throw NotFound, that is a blocking issue.

## Clean Swagger style

- Operation summary should be short and client-oriented.
- Prefer sectioned Markdown plus bullets for descriptions, such as ` + "`### Purpose`" + `, ` + "`### Request Rules`" + `/` + "`### Processing`" + `, and ` + "`### Auth/Notes`" + `.
- Path/query/header/body parameters should include name, requiredness, format, and example.
- Responses should include client-handled failure statuses with schema/description, not success-only docs.
- Document single-object responses as top-level objects without unnecessary wrapper objects. Exceptions: pagination/list envelopes, explicit metadata contracts, backward compatibility, and standard error envelopes.
- If public/admin/internal docs are separated, filter paths/schemas for the intended audience.
`
}

func renderArchitecture(signals ProjectSignals) string {
	return "# Architecture\n\n## Purpose\n\nThis is an architecture draft generated from project files by agent-harness. Mark weak inferences with Confidence; current code and command output are authoritative.\n\n## Detected structure\n\n" + bulletListWithFallback(signals.Files, "Not enough project signal files were detected.") + "\n## Guidance\n\n- Before large design changes, inspect current entrypoints, package/module boundaries, and data flow.\n- Add new abstractions only after existing patterns and test boundaries are confirmed.\n"
}

func renderCautions(signals ProjectSignals) string {
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

func renderConventions(signals ProjectSignals) string {
	lines := []string{"# Conventions\n\n## Detected conventions\n\n"}
	if len(signals.DetectedConventions) == 0 {
		lines = append(lines, "- Few conventions were auto-detected. Inspect README, config, and existing files first.\n")
	} else {
		lines = append(lines, bulletListWithFallback(signals.DetectedConventions, "No detected conventions."))
	}
	lines = append(lines, "\n## Editing rules\n\n- Follow existing style first.\n- Do not run repo-wide formatting unless explicitly requested.\n- Add new dependencies only after documenting the need and alternatives.\n")
	lines = append(lines, "\n", solidDesignPatternGuidance)
	return strings.Join(lines, "")
}

func renderTechStack(signals ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Tech Stack\n\n## Detected languages\n\n")
	b.WriteString(bulletListWithFallback(signals.Languages, "Could not auto-confirm languages."))
	b.WriteString("\n## Package managers\n\n")
	b.WriteString(bulletListWithFallback(signals.PackageManagers, "Could not auto-confirm package managers."))
	b.WriteString("\n## Evidence files\n\n")
	b.WriteString(bulletListWithFallback(signals.Files, "No evidence files."))
	return b.String()
}

func renderTesting(signals ProjectSignals) string {
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

func renderADR() string {
	return `# Architecture Decision Records

## Purpose

Record structural choices, rejected alternatives, and decisions that affect long-term maintenance. This is not an implementation note; preserve why this structure was chosen and which alternatives should not be retried.

## When to read

- Before architecture changes, large refactors, or dependency/framework replacement
- When changing or bypassing existing structure
- When modifying code whose historical rationale is unclear

## When to append

- A new structure or boundary was chosen.
- Alternatives were considered and rejection reasons will reduce future re-analysis.
- Operations, performance, or security constraints shaped the design.

## Entry template

### YYYY-MM-DD: <decision title>

- Context: <problem and constraints>
- Decision: <chosen structure>
- Alternatives rejected:
  - <alternative>: <why rejected>
- Consequences: <tradeoffs and follow-up>
- Evidence: <files, commands, issues, docs>
`
}

func renderOperations(signals ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Operations\n\n")
	b.WriteString("## Local development\n\n")
	b.WriteString("- Check README, package scripts, Makefile/Taskfile, and CI workflows first.\n")
	if len(signals.BuildCommands) > 0 {
		b.WriteString("- Candidate build commands are listed in TESTING.md and should be verified before use.\n")
	}
	b.WriteString("\n## Environment and secrets\n\n")
	b.WriteString("- Do not put raw .env values, credentials, API keys, or local state in docs or logs.\n")
	b.WriteString("- Document only required environment variable names and purposes; use OS keychain/env references for values.\n")
	b.WriteString("\n## Deploy/release\n\n")
	b.WriteString("- Do not infer deploy procedures automatically. Verify them from CI/CD workflows and operations docs.\n")
	b.WriteString("\n## Project docs bootstrap and upkeep\n\n")
	b.WriteString("- `agent-harness project bootstrap --repo . --json` creates docs and user-state repo metadata; `--sync` refreshes them from current evidence.\n")
	b.WriteString("- After initial setup, agents should read repo evidence and keep `.agent-harness` docs current through MCP `project_docs_route` → `project_docs_read` → `project_docs_update`.\n")
	b.WriteString("- Append resolved false cases and decisions to CAUTIONS/ADR with `project_docs_record` instead of rewriting full documents.\n")
	b.WriteString("\n## UserPromptSubmit hook\n\n")
	b.WriteString("- When the host supports it, connect `agent-harness hook user-prompt` to UserPromptSubmit to inject short agent_harness MCP candidates for each user prompt.\n")
	b.WriteString("- The hook does not execute work; it only performs static keyword routing. It does not use the network or read large files.\n")
	return b.String()
}

func renderAgentWorkflow() string {
	return `# Agent Workflow

## Start

1. Read AGENTS.md first.
2. At session start, treat .agent-harness/CONSTITUTION.md as the baseline principle document.
3. If MCP is available, send the current task to project_docs_route and select only necessary docs.
4. Verify inferred doc claims against current files and command output.

## MCP usage rule

- When the host supports it, agent-harness hook user-prompt injects MCP candidate hints for each user instruction. The hint is a reminder for judgment, not an auto-execution command.
- Use MCP when the task needs current state, repo-specific doc routing, policy decisions, state checkpoints, or durable records that the model should not rely on from memory.
- Do not use MCP for simple reasoning or summarizing already opened files.
- Avoid exposing many tools at once; narrowly use route/read/update/record/check tools that match the task.
- Do not trust tool output blindly; check paths, exists flags, warnings, and verification evidence.

## Work

Use the Simplicity First and Surgical Changes principles from AGENTS.md, plus these project record/safety rules.

- Do not overwrite existing user changes.
- Add dependencies, deploy, or perform destructive actions only with explicit instruction or strong evidence.
- If docs diverge from current code or user consensus, use project_docs_read to verify the current SHA and project_docs_update to change one document at a time.
- When a problem occurred and was resolved, record it with MCP project_docs_record(kind=caution) in .agent-harness/CAUTIONS.md.
- When a structural decision or rejected alternative matters, record it with MCP project_docs_record(kind=adr) in .agent-harness/ADR.md.

## Verify

Use the Goal-Driven Execution principle from AGENTS.md, plus these verification routing rules.

- Before writing or modifying tests, read the good/bad test criteria in .agent-harness/TESTING.md.
- When changing CLI/MCP/API documentation contracts, also run golden/schema/smoke verification.
- Completion reports must include test/build/static-check results and reasons for skipped verification.

## Finish

- If a commit is needed, follow .agent-harness/COMMIT_POLICY.md.
- Record resolved false cases or structural decisions with MCP project_docs_record when useful.
`
}

func bulletListWithFallback(items []string, fallback string) string {
	if len(items) == 0 {
		return "- " + fallback + "\n"
	}
	var b strings.Builder
	for _, item := range items {
		b.WriteString("- " + item + "\n")
	}
	return b.String()
}

func commandList(commands []EvidenceCommand) string {
	if len(commands) == 0 {
		return "- No auto-inferred commands. Check README, CI workflows, and package scripts.\n"
	}
	var b strings.Builder
	for _, cmd := range commands {
		b.WriteString("- `" + cmd.Command + "`\n")
		b.WriteString("  - Evidence: " + strings.Join(cmd.Evidence, ", ") + "\n")
		b.WriteString("  - Confidence: " + cmd.Confidence + "\n")
	}
	return b.String()
}

func plannedFileAction(path, content string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "create"
	}
	if string(b) == content {
		return "unchanged"
	}
	return "update"
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func projectDocReason(rel string) string {
	if rel == "AGENTS.md" {
		return "agent entrypoint and routing block"
	}
	return "project-specific agent operating document"
}

func appendUnique(items []string, v string) []string {
	for _, item := range items {
		if item == v {
			return items
		}
	}
	return append(items, v)
}

type routeDoc struct{ rel, reason string }

func routeDocsForTask(task string) []routeDoc {
	base := []routeDoc{{"AGENTS.md", "repo-level agent entrypoint and document router"}}
	add := func(names ...routeDoc) []routeDoc { return append(base, names...) }
	p := func(name, reason string) routeDoc {
		return routeDoc{filepath.ToSlash(filepath.Join(ProjectDocsDir, name)), reason}
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
	if strings.Contains(task, "handoff") || strings.Contains(task, "finish") || strings.Contains(task, "complete") || strings.Contains(task, "workflow") {
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
