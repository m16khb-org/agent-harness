package draftwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
