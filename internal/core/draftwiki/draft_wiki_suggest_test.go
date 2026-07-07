package draftwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSuggestDraftWikiRendersHostAgentPrompt(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "memory.md")
	mustWrite(t, input, "Hook policy should stay bookkeeping-only.\n")

	result, err := SuggestDraftWiki(DraftWikiSuggestRequest{
		RepoRoot:   root,
		InputPath:  input,
		Title:      "Hook policy memory",
		TargetWiki: "agent-harness",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Executed || result.Prompt == "" {
		t.Fatalf("unexpected suggest result: %+v", result)
	}
	for _, want := range []string{"Host-Agent Judgement Response Schema", "body_markdown", "Hook policy should stay bookkeeping-only", "suggester: \"host-agent\""} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, result.Prompt)
		}
	}
	if result.Command != "host-agent judgement result file" {
		t.Fatalf("unexpected command label %q", result.Command)
	}
}

func TestSubmitDraftWikiRecordsHostAuthoredDraft(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "host-draft.md")
	mustWrite(t, input, `---
title: "Host-authored memory"
source: "host-agent"
target_wiki: "agent-harness"
target_type: "notes"
summary: "The host agent writes the draft and harness records it."
suggester: "host-agent"
---

# Host-authored memory

Harness records supplied drafts without calling an external service.`)

	result, err := SubmitDraftWiki(DraftWikiSubmitRequest{
		RepoRoot:  root,
		DraftPath: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Draft.Status != "draft" || result.Draft.TargetWiki != "agent-harness" {
		t.Fatalf("unexpected submit result: %+v", result)
	}
	wantDraftName := time.Now().Format(time.DateOnly) + "-host-authored-memory.md"
	if _, err := os.Stat(filepath.Join(root, DraftWikiDir, "draft", wantDraftName)); err != nil {
		t.Fatalf("draft file missing: %v", err)
	}
}

func TestDraftWikiQueueWorkerRendersPrompt(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()

	queued, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		Tool:           "Bash",
		Command:        "claude-mem export observations",
		SourceMaterial: "Main agents should enqueue judged work and a host agent should write drafts.",
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
		RepoRoot: root,
		Limit:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !processed.OK || processed.Processed != 1 || processed.Succeeded != 1 {
		t.Fatalf("unexpected processed result: %+v", processed)
	}
	if len(processed.Events) != 1 || processed.Events[0].Status != WorkerStatusSucceeded || processed.Events[0].Prompt == "" {
		t.Fatalf("queue event did not render prompt: %+v", processed.Events)
	}
	if strings.Contains(processed.Events[0].Prompt, "Z."+"AI") {
		t.Fatalf("prompt should not mention external provider:\n%s", processed.Events[0].Prompt)
	}
}
