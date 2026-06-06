package draftwiki

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
