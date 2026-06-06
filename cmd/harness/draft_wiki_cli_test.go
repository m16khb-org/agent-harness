package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core"
)

func TestRunProjectDraftWikiPruneJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	for i := 0; i < 3; i++ {
		if _, err := core.AppendDraftWikiQueueEvent(core.DraftWikiQueueAppendRequest{
			RepoRoot:       root,
			SourceMaterial: "cli prune material",
		}); err != nil {
			t.Fatal(err)
		}
	}

	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"prune", "--repo", root, "--keep", "1", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("prune returned invalid JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["kind"] != "draft_wiki_queue_prune" {
		t.Fatalf("unexpected prune result: %#v", result)
	}
	if result["before"] != float64(3) || result["after"] != float64(1) || result["pruned"] != float64(2) {
		t.Fatalf("unexpected prune counts: %#v", result)
	}
	if path, _ := result["path"].(string); !strings.HasSuffix(path, "draft-wiki-queue.jsonl") {
		t.Fatalf("missing queue path in result: %#v", result)
	}
}

func TestRunProjectDraftWikiQueueAndWorkerWritesDraft(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	materialPath := filepath.Join(root, "material.md")
	if err := os.WriteFile(materialPath, []byte("The main agent judged this hook policy reusable enough for draft wiki."), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "agy-settings.json")
	if err := os.WriteFile(configPath, []byte(`{"model":"Gemini 3.5 Flash (High)"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
if [ "$1" != "--dangerously-skip-permissions" ] || [ "$2" != "-p" ]; then
  echo "missing agy flags" >&2
  exit 2
fi
cat <<'EOF'
{"body_markdown":"---\ntitle: \"Explicit queued draft\"\nsource: \"main-agent\"\ntarget_wiki: \"agent-harness\"\ntarget_type: \"notes\"\nsummary: \"The main agent explicitly queues reusable draft-wiki material.\"\n---\n\n# Explicit queued draft\n\nThe main agent, not a heuristic hook, decides whether material is worth queueing."}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}

	queuedOut := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"queue", "--repo", root, "--input", materialPath, "--target-wiki", "agent-harness", "--json"})
	})
	var queued map[string]any
	if err := json.Unmarshal([]byte(queuedOut), &queued); err != nil {
		t.Fatalf("queue returned invalid JSON: %v\n%s", err, queuedOut)
	}
	if queued["ok"] != true {
		t.Fatalf("unexpected queue result: %#v", queued)
	}

	out := captureStdoutForContract(t, func() error {
		return runWorker([]string{"draft-wiki", "--repo", root, "--agy-command", fakeAgy, "--agy-settings", configPath, "--json"})
	})
	var processed map[string]any
	if err := json.Unmarshal([]byte(out), &processed); err != nil {
		t.Fatalf("worker output is not JSON: %q: %v", out, err)
	}
	if processed["processed"] != float64(1) || processed["succeeded"] != float64(1) {
		t.Fatalf("worker did not process explicitly queued draft-wiki item: %+v", processed)
	}
	wantDraftName := time.Now().Format(time.DateOnly) + "-explicit-queued-draft.md"
	if _, err := os.Stat(filepath.Join(root, ".agent-harness", "draft-wiki", "draft", wantDraftName)); err != nil {
		t.Fatalf("draft-wiki draft file missing after explicit queue+worker: %v", err)
	}
}

func TestRunProjectDraftWikiQueueReadsStdin(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})
	if _, err := w.WriteString("Heredoc material judged reusable by the main agent."); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"queue", "--repo", root, "--stdin", "--target-wiki", "agent-harness", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("queue returned invalid JSON: %v\n%s", err, out)
	}
	if result["ok"] != true {
		t.Fatalf("unexpected queue result: %#v", result)
	}
	path, ok := result["path"].(string)
	if !ok || path == "" {
		t.Fatalf("missing queue path in result: %#v", result)
	}
	rawQueue, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(rawQueue, &event); err != nil {
		t.Fatalf("queue event is not valid JSON: %v\n%s", err, rawQueue)
	}
	if event["source"] != "main-agent" || event["source_material"] != "Heredoc material judged reusable by the main agent." {
		t.Fatalf("stdin queue did not preserve main-agent source/material: %+v", event)
	}
}

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
	if !strings.Contains(promoteText, "draft-wiki promote dry-run") || !strings.Contains(promoteText, "@wiki ingest") {
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
			stderr, err := captureProjectCLIStderr(func() error {
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

func writeDraftWikiCLIDraft(t *testing.T, root, status, name, title, targetWiki string) string {
	t.Helper()
	path := filepath.Join(root, ".agent-harness", "draft-wiki", status, name)
	body := "---\n" +
		"title: \"" + title + "\"\n" +
		"target_wiki: \"" + targetWiki + "\"\n" +
		"target_type: \"notes\"\n" +
		"---\n\n# " + title + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
