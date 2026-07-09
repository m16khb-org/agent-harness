package draftwikicli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunProjectDraftWikiInitAndListCoverTextAndJSON(t *testing.T) {
	root := t.TempDir()

	initText := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"init", "--repo", root, "--dry-run"})
	})
	if !strings.Contains(initText, "draft-wiki would initialize") {
		t.Fatalf("unexpected init dry-run text:\n%s", initText)
	}

	initJSON := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"init", "--repo", root, "--json"})
	})
	var initResult map[string]any
	if err := json.Unmarshal([]byte(initJSON), &initResult); err != nil {
		t.Fatalf("init returned invalid JSON: %v\n%s", err, initJSON)
	}
	if initResult["ok"] != true || initResult["kind"] != "draft_wiki_init" {
		t.Fatalf("unexpected init result: %#v", initResult)
	}

	draftPath := writeDraftWikiCLIDraft(t, root, "draft", "candidate.md", "Candidate", "agent-harness")
	listText := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"list", "--repo", root})
	})
	if !strings.Contains(listText, "draft-wiki: 1 drafts") || !strings.Contains(listText, "candidate.md") {
		t.Fatalf("unexpected list text:\n%s", listText)
	}

	listJSON := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"list", "--repo", root, "--json"})
	})
	var listResult map[string]any
	if err := json.Unmarshal([]byte(listJSON), &listResult); err != nil {
		t.Fatalf("list returned invalid JSON: %v\n%s", err, listJSON)
	}
	if listResult["ok"] != true {
		t.Fatalf("unexpected list result: %#v", listResult)
	}
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("list should not move draft: %v", err)
	}
}

func TestRunProjectDraftWikiApproveRejectAndPromoteDryRun(t *testing.T) {
	root := t.TempDir()
	approvePath := writeDraftWikiCLIDraft(t, root, "draft", "approve.md", "Approve", "agent-harness")

	approveJSON := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"approve", "--repo", root, "--json", approvePath})
	})
	var approveResult map[string]any
	if err := json.Unmarshal([]byte(approveJSON), &approveResult); err != nil {
		t.Fatalf("approve returned invalid JSON: %v\n%s", err, approveJSON)
	}
	if approveResult["ok"] != true || approveResult["kind"] != "draft_wiki_approve" {
		t.Fatalf("unexpected approve result: %#v", approveResult)
	}
	approvedPath := filepath.Join(root, ".agent-harness", "draft-wiki", "approved", "approve.md")
	if _, err := os.Stat(approvedPath); err != nil {
		t.Fatalf("approved draft missing: %v", err)
	}

	promoteText := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"promote", "--repo", root, approvedPath})
	})
	if !strings.Contains(promoteText, "draft-wiki promote dry-run") || !strings.Contains(promoteText, "exported/approve.md") {
		t.Fatalf("unexpected promote dry-run text:\n%s", promoteText)
	}

	rejectPath := writeDraftWikiCLIDraft(t, root, "draft", "reject.md", "Reject", "agent-harness")
	rejectText := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"reject", "--repo", root, rejectPath})
	})
	if !strings.Contains(rejectText, "rejected draft:") || !strings.Contains(rejectText, "rejected/reject.md") {
		t.Fatalf("unexpected reject text:\n%s", rejectText)
	}
	if _, err := os.Stat(filepath.Join(root, ".agent-harness", "draft-wiki", "rejected", "reject.md")); err != nil {
		t.Fatalf("rejected draft missing: %v", err)
	}
}

func TestRunProjectDraftWikiRejectsMissingAndUnknownSubcommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing", args: nil, wantErr: "missing draft-wiki subcommand"},
		{name: "unknown", args: []string{"missing-command"}, wantErr: `unknown draft-wiki subcommand "missing-command"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr, err := captureProjectCLIStderr(t, func() error {
				return runProjectDraftWiki(tt.args)
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
			if !strings.Contains(stderr, "agent-harness project draft-wiki init") {
				t.Fatalf("expected draft-wiki usage on stderr, got:\n%s", stderr)
			}
		})
	}
}

func TestParseDraftWikiPathFlagsRejectsMissingPath(t *testing.T) {
	_, _, _, err := parseDraftWikiPathFlags("project draft-wiki approve", []string{"--repo", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "exactly one draft path is required") {
		t.Fatalf("expected missing path error, got %v", err)
	}
}
