package draftwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromoteExportsApprovedDraftLocally(t *testing.T) {
	root := t.TempDir()
	approved := filepath.Join(root, DraftWikiDir, "approved", "candidate.md")
	mustWrite(t, approved, `---
title: "Candidate"
target_wiki: "agent-harness"
target_type: "notes"
summary: "A durable note."
---

# Candidate

Durable local export.
`)

	dry, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || !dry.DryRun || dry.Confirm || dry.Executed {
		t.Fatalf("unexpected dry-run promote result: %+v", dry)
	}
	if dry.ExportRel != ".agent-harness/draft-wiki/exported/candidate.md" {
		t.Fatalf("dry-run should report local export target, got %+v", dry)
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
	if confirmed.From.Status != "approved" || confirmed.To == nil || confirmed.To.Status != "exported" {
		t.Fatalf("promote should move approved draft to exported: %+v", confirmed)
	}
	if confirmed.ExportRel != ".agent-harness/draft-wiki/exported/candidate.md" || confirmed.ExportPath == "" {
		t.Fatalf("unexpected export path: %+v", confirmed)
	}
	if _, err := os.Stat(approved); !os.IsNotExist(err) {
		t.Fatalf("approved file should be moved out after confirm, got %v", err)
	}
	exported, err := os.ReadFile(confirmed.ExportPath)
	if err != nil {
		t.Fatal(err)
	}
	exportedText := string(exported)
	for _, want := range []string{`title: "Candidate"`, "# Candidate", "Durable local export."} {
		if !strings.Contains(exportedText, want) {
			t.Fatalf("exported draft missing %q:\n%s", want, exportedText)
		}
	}
	logText, err := os.ReadFile(confirmed.ExportLogPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"export | Candidate", ".agent-harness/draft-wiki/approved/candidate.md", ".agent-harness/draft-wiki/exported/candidate.md"} {
		if !strings.Contains(string(logText), want) {
			t.Fatalf("export log missing %q:\n%s", want, string(logText))
		}
	}
}

func TestPromoteRefusesUnapprovedDraft(t *testing.T) {
	root := t.TempDir()
	draft := filepath.Join(root, DraftWikiDir, "draft", "candidate.md")
	mustWrite(t, draft, `---
title: "Candidate"
---

# Candidate
`)

	_, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: draft, Confirm: true})
	if err == nil || !strings.Contains(err.Error(), `has status "draft"; promote requires approved`) {
		t.Fatalf("expected unapproved refusal, got %v", err)
	}
	if _, statErr := os.Stat(draft); statErr != nil {
		t.Fatalf("refused promote should leave draft in place: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, DraftWikiDir, "exported", "candidate.md")); !os.IsNotExist(statErr) {
		t.Fatalf("refused promote should not export draft, got %v", statErr)
	}
}
