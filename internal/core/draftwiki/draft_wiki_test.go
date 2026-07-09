package draftwiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitDraftWikiCreatesReviewStaging(t *testing.T) {
	root := t.TempDir()

	dry, err := InitDraftWiki(DraftWikiInitRequest{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || !dry.DryRun || dry.Write {
		t.Fatalf("unexpected dry-run result: %+v", dry)
	}
	if _, err := os.Stat(filepath.Join(root, DraftWikiDir)); !os.IsNotExist(err) {
		t.Fatalf("dry-run created draft wiki dir or unexpected stat error: %v", err)
	}

	written, err := InitDraftWiki(DraftWikiInitRequest{RepoRoot: root, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if !written.OK || written.DryRun || !written.Write {
		t.Fatalf("unexpected write result: %+v", written)
	}
	for _, rel := range []string{
		filepath.ToSlash(filepath.Join(DraftWikiDir, "README.md")),
		filepath.ToSlash(filepath.Join(DraftWikiDir, "draft", ".gitkeep")),
		filepath.ToSlash(filepath.Join(DraftWikiDir, "approved", ".gitkeep")),
		filepath.ToSlash(filepath.Join(DraftWikiDir, "rejected", ".gitkeep")),
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
}

func TestListDraftWikiReadsRepoLocalDrafts(t *testing.T) {
	root := t.TempDir()
	draft := filepath.Join(root, DraftWikiDir, "draft", "2026-05-31-hook-policy.md")
	mustWrite(t, draft, `---
title: "Hook policy"
source: "agent-notes"
target_wiki: "agent-harness"
target_type: "notes"
summary: "Reusable hook policy note."
---

# Hook policy

Body.
`)

	result, err := ListDraftWiki(DraftWikiListRequest{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.DraftDir == "" || len(result.Drafts) != 1 {
		t.Fatalf("unexpected list result: %+v", result)
	}
	got := result.Drafts[0]
	if got.Status != "draft" || got.Title != "Hook policy" || got.TargetWiki != "agent-harness" || got.TargetType != "notes" {
		t.Fatalf("unexpected draft metadata: %+v", got)
	}
	if got.RelPath != ".agent-harness/draft-wiki/draft/2026-05-31-hook-policy.md" {
		t.Fatalf("RelPath=%q", got.RelPath)
	}
}

func TestApproveDraftWikiMovesDraftCandidate(t *testing.T) {
	root := t.TempDir()
	draft := filepath.Join(root, DraftWikiDir, "draft", "candidate.md")
	mustWrite(t, draft, "# Candidate\n")

	result, err := ApproveDraftWiki(DraftWikiMoveRequest{RepoRoot: root, Path: draft})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.From.Status != "draft" || result.To.Status != "approved" {
		t.Fatalf("unexpected approve result: %+v", result)
	}
	if _, err := os.Stat(draft); !os.IsNotExist(err) {
		t.Fatalf("draft file still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, DraftWikiDir, "approved", "candidate.md")); err != nil {
		t.Fatalf("approved file missing: %v", err)
	}
}

func TestRejectDraftWikiMovesCandidateToRejected(t *testing.T) {
	root := t.TempDir()
	draft := filepath.Join(root, DraftWikiDir, "draft", "candidate.md")
	mustWrite(t, draft, "# Candidate\n")

	result, err := RejectDraftWiki(DraftWikiMoveRequest{RepoRoot: root, Path: draft})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Kind != "draft_wiki_reject" || result.From.Status != "draft" || result.To.Status != "rejected" {
		t.Fatalf("unexpected reject result: %+v", result)
	}
	if _, err := os.Stat(draft); !os.IsNotExist(err) {
		t.Fatalf("draft file still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, DraftWikiDir, "rejected", "candidate.md")); err != nil {
		t.Fatalf("rejected file missing: %v", err)
	}
}

func TestPromoteDraftWikiExportsApprovedDraftOnConfirm(t *testing.T) {
	root := t.TempDir()
	approved := filepath.Join(root, DraftWikiDir, "approved", "candidate.md")
	mustWrite(t, approved, `---
title: "Candidate"
target_wiki: "agent-harness"
target_type: "notes"
---

# Candidate
`)

	dry, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || !dry.DryRun || dry.Confirm || dry.Executed {
		t.Fatalf("unexpected dry-run promote result: %+v", dry)
	}
	if dry.ExportRel != filepath.ToSlash(filepath.Join(DraftWikiDir, "exported", "candidate.md")) {
		t.Fatalf("unexpected dry-run export target: %+v", dry)
	}
	if _, err := os.Stat(approved); err != nil {
		t.Fatalf("dry-run moved approved file: %v", err)
	}

	confirmed, err := PromoteDraftWiki(DraftWikiPromoteRequest{
		RepoRoot: root,
		Path:     approved,
		Confirm:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Confirm || !confirmed.Executed {
		t.Fatalf("unexpected confirmed promote result: %+v", confirmed)
	}
	if confirmed.To == nil || confirmed.To.Status != "exported" || confirmed.ExportRel != filepath.ToSlash(filepath.Join(DraftWikiDir, "exported", "candidate.md")) {
		t.Fatalf("unexpected local export result: %+v", confirmed)
	}
	exported, err := os.ReadFile(confirmed.ExportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exported), "# Candidate") {
		t.Fatalf("exported draft missing body: %s", string(exported))
	}
	if _, err := os.Stat(approved); !os.IsNotExist(err) {
		t.Fatalf("approved file should move to exported, got %v", err)
	}
	logText, err := os.ReadFile(confirmed.ExportLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logText), "export | Candidate") {
		t.Fatalf("log missing export entry: %s", string(logText))
	}
}

func TestGeneratedDraftFrontmatterUsesDefaultsAndProvidedValues(t *testing.T) {
	defaults := generatedDraftFrontmatter("", "", "")
	for _, want := range []string{
		`title: "Draft wiki candidate"`,
		`target_wiki: "dev-fundamentals"`,
		`target_type: "notes"`,
	} {
		if !strings.Contains(defaults, want) {
			t.Fatalf("default frontmatter missing %q:\n%s", want, defaults)
		}
	}

	custom := generatedDraftFrontmatter("Hook policy", "agent-harness", "runbook")
	for _, want := range []string{
		`title: "Hook policy"`,
		`target_wiki: "agent-harness"`,
		`target_type: "runbook"`,
	} {
		if !strings.Contains(custom, want) {
			t.Fatalf("custom frontmatter missing %q:\n%s", want, custom)
		}
	}
}

func TestFailDraftWikiQueueEventMarksFailureAndRedactsError(t *testing.T) {
	event := DraftWikiQueueEvent{
		ID:             "draft-wiki-1",
		RepoRoot:       "/repo",
		SourceMaterial: "keep source material",
		Status:         WorkerStatusRunning,
		UpdatedAt:      "before",
	}

	failed := failDraftWikiQueueEvent(event, fmt.Errorf("OPENAI_API_KEY=sk-123456789012345678901234"))

	if failed.Status != WorkerStatusFailed || failed.ID != event.ID || failed.RepoRoot != event.RepoRoot || failed.SourceMaterial != event.SourceMaterial {
		t.Fatalf("unexpected failed event fields: %+v", failed)
	}
	if failed.Error != "<redacted>" {
		t.Fatalf("expected redacted error, got %q", failed.Error)
	}
	if failed.UpdatedAt == "" || failed.UpdatedAt == "before" {
		t.Fatalf("expected updated timestamp, got %q", failed.UpdatedAt)
	}
}
