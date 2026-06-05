package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
source: "claude-mem"
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

func TestPromoteDraftWikiWritesLLMWikiRawNoteOnConfirm(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
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
	if !strings.Contains(dry.HandoffCommand, "@wiki ingest") || !strings.Contains(dry.HandoffCommand, "--wiki agent-harness") {
		t.Fatalf("unexpected handoff command: %q", dry.HandoffCommand)
	}
	if _, err := os.Stat(approved); err != nil {
		t.Fatalf("dry-run moved approved file: %v", err)
	}

	confirmed, err := PromoteDraftWiki(DraftWikiPromoteRequest{
		RepoRoot:          root,
		Path:              approved,
		Confirm:           true,
		LLMWikiConfigPath: configPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Confirm || !confirmed.Executed {
		t.Fatalf("unexpected confirmed promote result: %+v", confirmed)
	}
	if confirmed.LLMWikiRoot != filepath.Join(hub, "topics", "agent-harness") {
		t.Fatalf("LLMWikiRoot=%q", confirmed.LLMWikiRoot)
	}
	wantRawSuffix := filepath.ToSlash(filepath.Join("raw", "notes", time.Now().Format(time.DateOnly)+"-candidate.md"))
	if confirmed.LLMWikiRawPath == "" || !strings.HasSuffix(filepath.ToSlash(confirmed.LLMWikiRawPath), wantRawSuffix) {
		t.Fatalf("unexpected raw path: %+v", confirmed)
	}
	raw, err := os.ReadFile(confirmed.LLMWikiRawPath)
	if err != nil {
		t.Fatal(err)
	}
	rawText := string(raw)
	if !strings.Contains(rawText, "type: notes") || !strings.Contains(rawText, "source: \"agent-harness draft-wiki:.agent-harness/draft-wiki/approved/candidate.md\"") {
		t.Fatalf("raw note missing llm-wiki frontmatter: %s", rawText)
	}
	if !strings.Contains(rawText, "# Candidate") {
		t.Fatalf("raw note missing draft body: %s", rawText)
	}
	if _, err := os.Stat(approved); err != nil {
		t.Fatalf("approved file should remain reviewable: %v", err)
	}
	logText, err := os.ReadFile(filepath.Join(hub, "topics", "agent-harness", "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logText), "ingest | Candidate") {
		t.Fatalf("log missing ingest entry: %s", string(logText))
	}
}

func TestSuggestDraftWikiUsesAgyPrintWithConfiguredModel(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "memory.md")
	mustWrite(t, input, "Hook policy should stay bookkeeping-only.\n")
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Gemini 3.5 Flash (High)"}`)
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	mustWrite(t, fakeAgy, `#!/bin/sh
if [ "$1" != "--dangerously-skip-permissions" ] || [ "$2" != "-p" ]; then
  echo "missing agy flags" >&2
  exit 2
fi
cat <<'EOF'
`+draftWikiAgyJSONForTest(t, `---
title: "Hook policy memory"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "Main agents should explicitly queue work instead of hooks running long LLM calls inline."
---

# Hook policy memory

Main agents should explicitly queue judged material and leave LLM summarization to a worker.`)+`
EOF
`)
	if err := os.Chmod(fakeAgy, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := SuggestDraftWiki(DraftWikiSuggestRequest{
		RepoRoot:        root,
		InputPath:       input,
		Title:           "Hook policy memory",
		TargetWiki:      "agent-harness",
		AgyCommand:      fakeAgy,
		AgyModel:        "Gemini 3.5 Flash (High)",
		AgySettingsPath: configPath,
		Write:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.DryRun || !result.Executed || result.ModelSelectionMethod != "settings_json" {
		t.Fatalf("unexpected suggest result: %+v", result)
	}
	if result.Draft == nil || result.Draft.Status != "draft" || result.Draft.TargetWiki != "agent-harness" {
		t.Fatalf("unexpected draft metadata: %+v", result.Draft)
	}
	if !strings.Contains(result.Command, fakeAgy+" --dangerously-skip-permissions -p") {
		t.Fatalf("expected agy print command, got %q", result.Command)
	}
	wantDraftName := time.Now().Format(time.DateOnly) + "-hook-policy-memory.md"
	if _, err := os.Stat(filepath.Join(root, DraftWikiDir, "draft", wantDraftName)); err != nil {
		t.Fatalf("draft file missing: %v", err)
	}
}

func TestSuggestDraftWikiStripsAgyModeBanner(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "memory.md")
	mustWrite(t, input, "Hook policy should stay bookkeeping-only.\n")
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Gemini 3.5 Flash (High)"}`)
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	mustWrite(t, fakeAgy, `#!/bin/sh
cat <<'EOF'
ULTRAWORK MODE ENABLED!

`+draftWikiAgyJSONForTest(t, `---
title: "Banner-safe draft"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "Agy mode banners are not part of the draft."
---

# Banner-safe draft

The useful draft body remains.`)+`
EOF
`)
	if err := os.Chmod(fakeAgy, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := SuggestDraftWiki(DraftWikiSuggestRequest{
		RepoRoot:        root,
		InputPath:       input,
		TargetWiki:      "agent-harness",
		AgyCommand:      fakeAgy,
		AgyModel:        "Gemini 3.5 Flash (High)",
		AgySettingsPath: configPath,
		Write:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || result.Draft.Title != "Banner-safe draft" || result.Draft.TargetWiki != "agent-harness" {
		t.Fatalf("unexpected draft metadata after banner strip: %+v", result.Draft)
	}
	body, err := os.ReadFile(result.Draft.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "ULTRAWORK MODE ENABLED") {
		t.Fatalf("agy mode banner leaked into draft: %s", string(body))
	}
}

func TestSuggestDraftWikiRejectsWrongAgyModel(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "memory.md")
	mustWrite(t, input, "memory\n")
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Claude Opus 4.6 (Thinking)"}`)

	_, err := SuggestDraftWiki(DraftWikiSuggestRequest{
		RepoRoot:        root,
		InputPath:       input,
		AgyCommand:      "agy",
		AgyModel:        "Gemini 3.5 Flash (High)",
		AgySettingsPath: configPath,
		Write:           true,
	})
	if err == nil || !strings.Contains(err.Error(), "agy model mismatch") {
		t.Fatalf("expected model mismatch, got %v", err)
	}
}

func TestDraftWikiQueueWorkerRunsAgyAndWritesDraft(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Gemini 3.5 Flash (High)"}`)
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	mustWrite(t, fakeAgy, `#!/bin/sh
if [ "$1" != "--dangerously-skip-permissions" ] || [ "$2" != "-p" ]; then
  echo "missing agy flags" >&2
  exit 2
fi
printf '%s\n' "$3" > prompt.txt
cat <<'EOF'
`+draftWikiAgyJSONForTest(t, `---
title: "Explicit queued memory"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "The main agent explicitly queues draft-wiki work and the worker performs agy summarization."
---

# Explicit queued memory

The main agent explicitly queues draft-wiki work; the worker calls agy -p outside the hook critical path.`)+`
EOF
`)
	if err := os.Chmod(fakeAgy, 0o755); err != nil {
		t.Fatal(err)
	}

	queued, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		Tool:           "Bash",
		Command:        "claude-mem export observations",
		SourceMaterial: "Main agents should enqueue judged work and a worker should call agy -p.",
		TargetWiki:     "agent-harness",
		TargetType:     "notes",
		Source:         "main-agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !queued.OK || queued.Event.Status != WorkerStatusQueued || queued.Path == "" {
		t.Fatalf("unexpected queued result: %+v", queued)
	}

	processed, err := ProcessDraftWikiQueue(DraftWikiQueueProcessRequest{
		RepoRoot:        root,
		AgyCommand:      fakeAgy,
		AgyModel:        "Gemini 3.5 Flash (High)",
		AgySettingsPath: configPath,
		Limit:           1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !processed.OK || processed.Processed != 1 || processed.Succeeded != 1 {
		t.Fatalf("unexpected processed result: %+v", processed)
	}
	if len(processed.Events) != 1 || processed.Events[0].Status != WorkerStatusSucceeded {
		t.Fatalf("queue event was not marked succeeded: %+v", processed.Events)
	}
	if !strings.Contains(processed.Events[0].DraftRelPath, ".agent-harness/draft-wiki/draft/") {
		t.Fatalf("missing draft rel path: %+v", processed.Events[0])
	}
	if _, err := os.Stat(filepath.Join(root, "prompt.txt")); err != nil {
		t.Fatalf("fake agy did not receive prompt: %v", err)
	}
	drafts, err := ListDraftWiki(DraftWikiListRequest{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts.Drafts) != 1 || drafts.Drafts[0].Status != "draft" || drafts.Drafts[0].Title != "Explicit queued memory" {
		t.Fatalf("worker did not write draft-wiki/draft candidate: %+v", drafts)
	}
}

func TestDraftWikiQueueAppendCapsTailAfterSlack(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()

	for i := 0; i < maxDraftWikiQueueEvents*2+1; i++ {
		if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
			RepoRoot:       root,
			SourceMaterial: fmt.Sprintf("queue material %03d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	_, path, err := draftWikiQueuePath(root, false)
	if err != nil {
		t.Fatal(err)
	}
	events, warnings, err := readDraftWikiQueueEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", warnings)
	}
	if len(events) != maxDraftWikiQueueEvents {
		t.Fatalf("queue length=%d want %d", len(events), maxDraftWikiQueueEvents)
	}
	if got, want := events[0].SourceMaterial, "queue material 201"; got != want {
		t.Fatalf("oldest retained material=%q want %q", got, want)
	}
	if got, want := events[len(events)-1].SourceMaterial, "queue material 400"; got != want {
		t.Fatalf("newest retained material=%q want %q", got, want)
	}
}

func TestPruneDraftWikiQueueKeepZeroAndTail(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()

	for i := 0; i < 5; i++ {
		if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
			RepoRoot:       root,
			SourceMaterial: fmt.Sprintf("prune material %d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	tail, err := PruneDraftWikiQueue(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !tail.OK || tail.Before != 5 || tail.After != 2 || tail.Pruned != 3 {
		t.Fatalf("unexpected tail prune result: %+v", tail)
	}
	events, warnings, err := readDraftWikiQueueEvents(tail.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(events) != 2 {
		t.Fatalf("unexpected queue after tail prune: events=%+v warnings=%+v", events, warnings)
	}
	if events[0].SourceMaterial != "prune material 3" || events[1].SourceMaterial != "prune material 4" {
		t.Fatalf("prune did not retain newest events: %+v", events)
	}

	empty, err := PruneDraftWikiQueue(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Before != 2 || empty.After != 0 || empty.Pruned != 2 {
		t.Fatalf("unexpected empty prune result: %+v", empty)
	}
	stat, err := os.Stat(empty.Path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 0 || stat.Mode().Perm() != 0o600 {
		t.Fatalf("expected empty 0600 queue file, size=%d mode=%o", stat.Size(), stat.Mode().Perm())
	}
}

func TestPruneAllDraftWikiQueues(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	rootA := t.TempDir()
	rootB := t.TempDir()

	for _, root := range []string{rootA, rootB} {
		for i := 0; i < 3; i++ {
			if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
				RepoRoot:       root,
				SourceMaterial: fmt.Sprintf("%s material %d", filepath.Base(root), i),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	result, err := PruneAllDraftWikiQueues(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || len(result.Queues) != 2 || result.Before != 6 || result.After != 2 || result.Pruned != 4 {
		t.Fatalf("unexpected prune-all result: %+v", result)
	}
	for _, queue := range result.Queues {
		events, warnings, err := readDraftWikiQueueEvents(queue.Path)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 0 || len(events) != 1 {
			t.Fatalf("unexpected queue after prune-all: queue=%+v events=%+v warnings=%+v", queue, events, warnings)
		}
		if !strings.HasSuffix(events[0].SourceMaterial, " material 2") {
			t.Fatalf("prune-all did not retain newest event: %+v", events[0])
		}
	}
}

func TestDraftWikiQueueRewritePreservesRedactionAndMaterialCap(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	longSecret := "token=secret-value\n" + strings.Repeat("x", 13000)
	queued, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		SourceMaterial: "old material",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		SourceMaterial: longSecret,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := PruneDraftWikiQueue(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != queued.Path || result.After != 1 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
	events, warnings, err := readDraftWikiQueueEvents(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(events) != 1 {
		t.Fatalf("unexpected queue after prune: events=%+v warnings=%+v", events, warnings)
	}
	material := events[0].SourceMaterial
	if strings.Contains(material, "secret-value") {
		t.Fatalf("secret leaked after rewrite: %q", material)
	}
	if !strings.Contains(material, "<redacted>") {
		t.Fatalf("redaction marker missing after rewrite: %q", material)
	}
	if len([]byte(material)) > 12020 || !strings.Contains(material, "[truncated]") {
		t.Fatalf("material cap not preserved after rewrite, bytes=%d material=%q", len([]byte(material)), material)
	}
	stat, err := os.Stat(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("queue rewrite mode=%o want 600", stat.Mode().Perm())
	}
}

func TestPromoteDraftWikiDryRunDoesNotWriteLLMWiki(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "dry-run.md")
	mustWrite(t, approved, `---
title: "Dry Run"
target_wiki: "agent-harness"
target_type: "notes"
---

# Dry Run
`)

	result, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, LLMWikiConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun || result.Confirm || result.Executed {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if result.UpstreamTool != "nvk/llm-wiki" || !strings.Contains(result.HandoffCommand, "@wiki ingest") {
		t.Fatalf("dry-run should report upstream handoff only: %+v", result)
	}
	rawDir := filepath.Join(hub, "topics", "agent-harness", "raw")
	if _, err := os.Stat(rawDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote raw directory or unexpected stat error: %v", err)
	}
}

func TestPromoteDraftWikiRejectsCollisionWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "collision.md")
	mustWrite(t, approved, `---
title: "Collision"
target_wiki: "agent-harness"
target_type: "notes"
---

# Collision
`)
	rawPath := filepath.Join(hub, "topics", "agent-harness", "raw", "notes", time.Now().Format(time.DateOnly)+"-collision.md")
	mustWrite(t, rawPath, "existing raw note\n")
	_, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, Confirm: true, LLMWikiConfigPath: configPath})
	if err == nil || !strings.Contains(err.Error(), "llm-wiki raw file already exists") {
		t.Fatalf("expected collision error, got %v", err)
	}
	got, readErr := os.ReadFile(rawPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "existing raw note\n" {
	}
}

func TestPromoteDraftWikiRejectsInvalidTargetType(t *testing.T) {
	root := t.TempDir()
	configPath, _ := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "invalid-type.md")
	mustWrite(t, approved, `---
title: "Invalid Type"
target_wiki: "agent-harness"
target_type: "badtype"
---

# Invalid Type
`)

	_, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, Confirm: true, LLMWikiConfigPath: configPath})
	if err == nil || !strings.Contains(err.Error(), `unsupported llm-wiki raw type "badtype"`) {
		t.Fatalf("expected invalid target type error, got %v", err)
	}
}

func TestPromoteDraftWikiConfirmDoesNotCreateLLMWikiIndexArtifacts(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "boundary.md")
	mustWrite(t, approved, `---
title: "Boundary"
target_wiki: "agent-harness"
target_type: "notes"
---

# Boundary
`)

	result, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, Confirm: true, LLMWikiConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Executed || result.LLMWikiRawPath == "" || result.LLMWikiLogPath == "" {
		t.Fatalf("unexpected confirm result: %+v", result)
	}
	for _, rel := range []string{"compiled", "index", "query", "embeddings"} {
		if _, err := os.Stat(filepath.Join(hub, "topics", "agent-harness", rel)); !os.IsNotExist(err) {
			t.Fatalf("promotion must not create llm-wiki %s artifact: %v", rel, err)
		}
	}
}

func TestDraftWikiQueueReportsMalformedLinesAndContinues(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Gemini 3.5 Flash (High)"}`)
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	mustWrite(t, fakeAgy, `#!/bin/sh
cat <<'EOF'
`+draftWikiAgyJSONForTest(t, `---
title: "Malformed queue still processes"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "Valid queued events continue after malformed lines."
---

# Malformed queue still processes

Valid queued events continue after malformed lines.`)+`
EOF
`)
	if err := os.Chmod(fakeAgy, 0o755); err != nil {
		t.Fatal(err)
	}

	queued, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		SourceMaterial: "valid material with secret api_key=supersecret",
		TargetWiki:     "agent-harness",
		TargetType:     "notes",
	})
	if err != nil {
		t.Fatal(err)
	}
	malformed := `{"source_material":"api_key=supersecret",`
	original, err := os.ReadFile(queued.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queued.Path, append([]byte(malformed+"\n"), original...), 0o600); err != nil {
		t.Fatal(err)
	}

	processed, err := ProcessDraftWikiQueue(DraftWikiQueueProcessRequest{
		RepoRoot:        root,
		AgyCommand:      fakeAgy,
		AgyModel:        "Gemini 3.5 Flash (High)",
		AgySettingsPath: configPath,
		Limit:           1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if processed.Processed != 1 || processed.Succeeded != 1 {
		t.Fatalf("valid event did not continue after malformed line: %+v", processed)
	}
	if len(processed.Warnings) != 1 {
		t.Fatalf("expected one malformed-line warning, got %+v", processed.Warnings)
	}
	warning := processed.Warnings[0]
	if !strings.Contains(warning, "line 1") || !strings.Contains(warning, "malformed JSONL") {
		t.Fatalf("warning lacks line number/context: %q", warning)
	}
	if strings.Contains(warning, "api_key=supersecret") || len(warning) > 240 {
		t.Fatalf("warning was not bounded/redacted: %q", warning)
	}
	encoded, err := json.Marshal(processed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "valid material") || strings.Contains(string(encoded), "source_material") {
		t.Fatalf("process response exposed source material: %s", encoded)
	}
}

func TestDraftWikiQueueRunningRewriteFailureDoesNotInvokeAgy(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Gemini 3.5 Flash (High)"}`)
	invoked := filepath.Join(root, "agy-invoked")
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	mustWrite(t, fakeAgy, `#!/bin/sh
touch "`+invoked+`"
echo should-not-run
`)
	if err := os.Chmod(fakeAgy, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{RepoRoot: root, SourceMaterial: "must persist running before agy"}); err != nil {
		t.Fatal(err)
	}
	originalRewrite := rewriteDraftWikiQueueEventsFunc
	rewriteDraftWikiQueueEventsFunc = func(string, []DraftWikiQueueEvent) error {
		return os.ErrPermission
	}
	t.Cleanup(func() { rewriteDraftWikiQueueEventsFunc = originalRewrite })

	processed, err := ProcessDraftWikiQueue(DraftWikiQueueProcessRequest{
		RepoRoot:        root,
		AgyCommand:      fakeAgy,
		AgyModel:        "Gemini 3.5 Flash (High)",
		AgySettingsPath: configPath,
		Limit:           1,
	})
	if err == nil {
		t.Fatalf("expected running-state rewrite error, got result %+v", processed)
	}
	if _, statErr := os.Stat(invoked); !os.IsNotExist(statErr) {
		t.Fatalf("fake agy was invoked despite rewrite failure: %v", statErr)
	}
}

func TestDraftWikiQueueConcurrentWorkersProcessOneEventOnce(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	configPath := filepath.Join(root, "agy-settings.json")
	mustWrite(t, configPath, `{"model":"Gemini 3.5 Flash (High)"}`)
	invocations := filepath.Join(root, "agy-invocations.log")
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	mustWrite(t, fakeAgy, `#!/bin/sh
printf 'invoke\n' >> "`+invocations+`"
sleep 0.2
cat <<'EOF'
`+draftWikiAgyJSONForTest(t, `---
title: "Concurrent queue event"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "One concurrent worker should process the queued event."
---

# Concurrent queue event

Only one worker should process this event.`)+`
EOF
`)
	if err := os.Chmod(fakeAgy, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{RepoRoot: root, SourceMaterial: "race one event"}); err != nil {
		t.Fatal(err)
	}

	results := make(chan DraftWikiQueueProcessResult, 2)
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			res, err := ProcessDraftWikiQueue(DraftWikiQueueProcessRequest{
				RepoRoot:        root,
				AgyCommand:      fakeAgy,
				AgyModel:        "Gemini 3.5 Flash (High)",
				AgySettingsPath: configPath,
				Limit:           1,
			})
			results <- res
			errs <- err
		}()
	}
	close(start)
	processedTotal := 0
	for i := 0; i < 2; i++ {
		res := <-results
		if err := <-errs; err != nil {
			t.Fatalf("worker %d returned error: %v result=%+v", i, err, res)
		}
		processedTotal += res.Processed
	}
	if processedTotal != 1 {
		t.Fatalf("expected exactly one processed event, got %d", processedTotal)
	}
	log, err := os.ReadFile(invocations)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(log), "invoke"); got != 1 {
		t.Fatalf("expected one agy invocation, got %d log=%q", got, log)
	}
	drafts, err := ListDraftWiki(DraftWikiListRequest{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts.Drafts) != 1 {
		t.Fatalf("expected one draft, got %+v", drafts.Drafts)
	}
}

func writeTestLLMWikiHub(t *testing.T) (configPath, hub string) {
	t.Helper()
	root := t.TempDir()
	hub = filepath.Join(root, "llm-wiki")
	topic := filepath.Join(hub, "topics", "agent-harness")
	mustWrite(t, filepath.Join(hub, "wikis.json"), `{
  "default": "agent-harness",
  "wikis": {
    "agent-harness": {
      "path": "topics/agent-harness",
      "description": "Agent harness memory",
      "status": "active"
    }
  }
}`)
	mustWrite(t, filepath.Join(topic, "config.md"), "# Agent Harness\n")
	mustWrite(t, filepath.Join(topic, "log.md"), "# Log\n")
	configPath = filepath.Join(root, "llm-wiki-config.json")
	mustWrite(t, configPath, `{"hub_path":"`+filepath.ToSlash(hub)+`"}`)
	return configPath, hub
}

func draftWikiAgyJSONForTest(t *testing.T, bodyMarkdown string) string {
	t.Helper()
	b, err := json.Marshal(draftWikiSuggestAgyResponse{BodyMarkdown: bodyMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	return "```json\n" + string(b) + "\n```"
}
