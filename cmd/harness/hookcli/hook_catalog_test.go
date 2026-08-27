package hookcli

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/testsupport"
)

func hookTempRepoWithDoc(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "ARCHITECTURE.md"), []byte("# Arch\n\n## 경계\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// runHookRawCapture feeds stdinJSON to fn and returns the raw stdout so tests
// can assert on "nothing printed" as well as on JSON payloads.
func runHookRawCapture(t *testing.T, stdinJSON string, fn func() error) string {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, stdinJSON); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()
	return testsupport.CaptureStdout(t, func() error {
		if err := fn(); err != nil {
			t.Fatalf("hook: %v", err)
		}
		return nil
	})
}

func runHookCapture(t *testing.T, stdinJSON string, fn func() error) map[string]any {
	t.Helper()
	out := runHookRawCapture(t, stdinJSON, fn)
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook output is not JSON: %q: %v", out, err)
	}
	return obj
}

func hookAdditionalContext(obj map[string]any) string {
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		return ""
	}
	ctx, _ := hso["additionalContext"].(string)
	return ctx
}

func TestRunHookSessionStartInjectsCatalogClaude(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", "claude"}) })
	if hso, _ := obj["hookSpecificOutput"].(map[string]any); hso["hookEventName"] != "SessionStart" {
		t.Fatalf("SessionStart must name its event: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") || !strings.Contains(ctx, "ARCHITECTURE.md=") {
		t.Fatalf("SessionStart must inject the compact catalog: %q", ctx)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "• ARCHITECTURE.md") {
		t.Fatalf("Claude SessionStart should show the readable catalog via systemMessage: %v", obj["systemMessage"])
	}
}

// Claude Code and Codex both re-run SessionStart with source "compact" after
// compaction, and neither host accepts model-facing context on PostCompact,
// so the compact source must inject exactly like startup.
func TestRunHookSessionStartInjectsCatalogOnCompactSource(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, source := range []string{"compact", "resume", "clear"} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"`+source+`"}`, func() error { return runHook([]string{"session-start", "--host", "claude"}) })
		if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
			t.Fatalf("source %s must re-establish the catalog: %+v", source, obj)
		}
	}
}

func TestRunHookSessionStartCodexOmitsSystemMessage(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", "codex"}) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("Codex SessionStart must omit systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "• ARCHITECTURE.md") || strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("Codex SessionStart additionalContext should be the readable catalog view: %q", ctx)
	}
}

func TestRunHookContextEventsEmitNoopWithoutProjectDocs(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, host := range []string{"codex", "claude"} {
		for _, event := range []string{"session-start", "post-compact"} {
			obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{event, "--host", host}) })
			if len(obj) != 0 {
				t.Fatalf("%s/%s without docs must be an empty object, got %+v", host, event, obj)
			}
		}
	}
}

func TestRunHookPostCompactCarriesOnlyUserFacingCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, host := range []string{"codex", "claude", ""} {
		args := []string{"post-compact"}
		if host != "" {
			args = append(args, "--host", host)
		}
		obj := runHookCapture(t, `{"cwd":"`+repo+`","trigger":"auto"}`, func() error { return runHook(args) })
		if _, ok := obj["hookSpecificOutput"]; ok {
			t.Fatalf("PostCompact must not emit hookSpecificOutput (%q): %+v", host, obj)
		}
		if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
			t.Fatalf("PostCompact (%q) should carry the readable catalog via systemMessage: %+v", host, obj)
		}
	}
}

func TestRunHookJSONOutputUsesSnakeCaseFields(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, event := range []string{"session-start", "post-compact"} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHook([]string{event, "--json"}) })
		if obj["should_inject"] != true {
			t.Fatalf("%s --json must expose should_inject: %+v", event, obj)
		}
		if compact, _ := obj["compact"].(string); !strings.Contains(compact, "ARCHITECTURE.md=") {
			t.Fatalf("%s --json must expose the compact catalog: %+v", event, obj)
		}
		for _, legacy := range []string{"ShouldInject", "Compact", "UserView", "ProjectDocs"} {
			if _, ok := obj[legacy]; ok {
				t.Fatalf("%s --json must not expose PascalCase field %s: %+v", event, legacy, obj)
			}
		}
	}
}

func TestRunHookContextEventsAcrossIsolatedWorktreesDoNotCreateHarnessState(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	for _, host := range []string{"codex", "claude"} {
		for _, repo := range []string{hookTempRepoWithDoc(t), hookTempRepoWithDoc(t)} {
			for _, event := range []string{"session-start", "post-compact"} {
				event := event
				runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error {
					return runHook([]string{event, "--host", host})
				})
			}
		}
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("context hooks must not create harness state, got %v", entries)
	}
}

// HARNESS_DISABLE_HOOKS is the kill-switch for repositories the harness does
// not own: the hook must print nothing at all, not even an empty object.
func TestDisableHooksTurnsContextEventIntoSilentNoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	t.Setenv("HARNESS_DISABLE_HOOKS", "1")
	repo := hookTempRepoWithDoc(t)
	out := runHookRawCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error {
		return runHook([]string{"session-start", "--host", "claude"})
	})
	if strings.TrimSpace(out) != "" {
		t.Fatalf("disabled hooks must emit nothing: %q", out)
	}
}

func TestTopLevelHelpReturnsErrHelp(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, entry := range []string{"--help", "-h", "help"} {
		if err := runHook([]string{entry}); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("%s must return ErrHelp, got %v", entry, err)
		}
	}
	if err := runHook([]string{"session-start", "--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("subcommand --help must return ErrHelp, got %v", err)
	}
}

func TestRetiredHookSubcommandsAreRejected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if err := runHook(nil); err == nil {
		t.Fatal("missing subcommand must fail")
	}
	for _, retired := range []string{"user-prompt", "pre-tool-use", "post-tool-use", "pre-compact", "stop", "failures", "metrics"} {
		err := runHook([]string{retired})
		if err == nil || !strings.Contains(err.Error(), "unknown hook subcommand") {
			t.Fatalf("%s must be rejected as unknown, got %v", retired, err)
		}
	}
}
