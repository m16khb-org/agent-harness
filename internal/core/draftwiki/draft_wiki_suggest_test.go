package draftwiki

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-harness/internal/core/externalllm"
)

func TestSuggestDraftWikiUsesZAIWithConfiguredModel(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "memory.md")
	mustWrite(t, input, "Hook policy should stay bookkeeping-only.\n")
	requests := withFakeDraftWikiZAI(t, draftWikiLLMJSONForTest(t, `---
title: "Hook policy memory"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "Main agents should explicitly queue work instead of hooks running long LLM calls inline."
---

# Hook policy memory

Main agents should explicitly queue judged material and leave LLM summarization to a worker.`))

	result, err := SuggestDraftWiki(DraftWikiSuggestRequest{
		RepoRoot:   root,
		InputPath:  input,
		Title:      "Hook policy memory",
		TargetWiki: "agent-harness",
		Model:      "glm-5-turbo",
		Write:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.DryRun || !result.Executed || result.Model != "glm-5-turbo" {
		t.Fatalf("unexpected suggest result: %+v", result)
	}
	if got := atomic.LoadInt32(requests); got != 1 {
		t.Fatalf("expected one Z.AI request, got %d", got)
	}
	if result.Draft == nil || result.Draft.Status != "draft" || result.Draft.TargetWiki != "agent-harness" {
		t.Fatalf("unexpected draft metadata: %+v", result.Draft)
	}
	if !strings.Contains(result.Command, "zai:") {
		t.Fatalf("expected zai preview command, got %q", result.Command)
	}
	wantDraftName := time.Now().Format(time.DateOnly) + "-hook-policy-memory.md"
	if _, err := os.Stat(filepath.Join(root, DraftWikiDir, "draft", wantDraftName)); err != nil {
		t.Fatalf("draft file missing: %v", err)
	}
}

func TestSuggestDraftWikiStripsModeBanner(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "memory.md")
	mustWrite(t, input, "Hook policy should stay bookkeeping-only.\n")
	withFakeDraftWikiZAI(t, "ULTRAWORK MODE ENABLED!\n\n"+draftWikiLLMJSONForTest(t, `---
title: "Banner-safe draft"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "Model mode banners are not part of the draft."
---

# Banner-safe draft

The useful draft body remains.`))

	result, err := SuggestDraftWiki(DraftWikiSuggestRequest{
		RepoRoot:   root,
		InputPath:  input,
		TargetWiki: "agent-harness",
		Write:      true,
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
		t.Fatalf("mode banner leaked into draft: %s", string(body))
	}
}

func TestDraftWikiQueueWorkerRunsZAIAndWritesDraft(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	requests := withFakeDraftWikiZAI(t, draftWikiLLMJSONForTest(t, `---
title: "Explicit queued memory"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "The main agent explicitly queues draft-wiki work and the worker performs LLM summarization."
---

# Explicit queued memory

The main agent explicitly queues draft-wiki work; the worker calls the Z.AI LLM outside the hook critical path.`))

	queued, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		Tool:           "Bash",
		Command:        "claude-mem export observations",
		SourceMaterial: "Main agents should enqueue judged work and a worker should call Z.AI.",
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
		Model:    "glm-5-turbo",
		Limit:    1,
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
	if got := atomic.LoadInt32(requests); got != 1 {
		t.Fatalf("expected one Z.AI request, got %d", got)
	}
	drafts, err := ListDraftWiki(DraftWikiListRequest{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts.Drafts) != 1 || drafts.Drafts[0].Status != "draft" || drafts.Drafts[0].Title != "Explicit queued memory" {
		t.Fatalf("worker did not write draft-wiki/draft candidate: %+v", drafts)
	}
}

func withFakeDraftWikiZAI(t *testing.T, content string) *int32 {
	t.Helper()
	t.Setenv("Z_AI_API_KEY", "test-key")
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		atomic.AddInt32(&requests, 1)
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, content)
	}))
	t.Cleanup(server.Close)
	previous := externalllm.SetBaseURL(server.URL)
	t.Cleanup(func() { externalllm.SetBaseURL(previous) })
	return &requests
}
