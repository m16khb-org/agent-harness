package hookinput

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/commandstep"
)

func TestBasicHookInputFieldsUseTopLevelAndNestedValues(t *testing.T) {
	input := []byte(`{"repo":" /repo ","source":"COMPACT","allow":true,"hook_input":{"cwd":"/nested","fallback":true}}`)
	if got := RepoFromHookInput(input); got != "/repo" {
		t.Fatalf("RepoFromHookInput = %q", got)
	}
	if got := SourceFromHookInput(input); got != "compact" {
		t.Fatalf("SourceFromHookInput = %q", got)
	}
	if !Bool(input, "allow") || !Bool(input, "fallback") || Bool(input, "missing") {
		t.Fatal("unexpected Bool extraction")
	}
	if RepoFromHookInput([]byte(`not json`)) != "" || SourceFromHookInput([]byte(`not json`)) != "" {
		t.Fatal("invalid JSON should produce empty fields")
	}
	if got := RepoFromHookInput([]byte(`{"hook_input":{"workspace_root":" /nested "}}`)); got != "/nested" {
		t.Fatalf("nested repo = %q", got)
	}
}

func TestHookInputParsesCodexClaudeNativeSessionIdentity(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "codex", input: `{"cwd":" /repo.worktrees/16-demo ","session_id":" codex-session ","agent_id":" worker-1 ","host":" CODEX "}`},
		{name: "claude nested", input: `{"hook_input":{"cwd":" /repo.worktrees/16-demo ","sessionId":" claude-session ","agent_type":" subagent ","host":" CLAUDE "}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(tt.input)
			if got := CWDFromHookInput(input); got != "/repo.worktrees/16-demo" {
				t.Fatalf("cwd=%q", got)
			}
			if got := SessionIDFromHookInput(input); !strings.HasSuffix(got, "-session") {
				t.Fatalf("session=%q", got)
			}
			if got := AgentIDFromHookInput(input); got == "" {
				t.Fatal("agent identity missing")
			}
			if got := HostFromHookInput(input); got != strings.Split(tt.name, " ")[0] {
				t.Fatalf("host=%q", got)
			}
		})
	}
}

func TestPathsFromHookInputCollectsExplicitPatchAndInlinePaths(t *testing.T) {
	// #100 계약: 키 기반 추출(top-level "path", nested "file"/"filename")은
	// 의도적으로 보존한다 — 재평가 대상이 아니다. 반면 내용 heuristic
	// (patch 스캔·인라인 경로 문자열)은 tool_input subtree 또는 command/cmd
	// 키 아래에서만 수행된다.
	input := []byte(`{
	  "path":"a.go",
	  "nested":{"file":"b.go","items":[{"filename":"testdata/case.json"}]},
	  "patch":"*** Begin Patch\n*** Add File: c.go\n*** Update File: .agent-harness/ADR.md\n*** Delete File: d.go\n*** Move to: e.go\n*** End Patch",
	  "note":"internal/core/foo.go",
	  "duplicate":"a.go"
	}`)
	got := PathsFromHookInput(input)
	for _, want := range []string{"a.go", "b.go", "testdata/case.json"} {
		if !containsString(got, want) {
			t.Fatalf("expected key-based path %q in %#v", want, got)
		}
	}
	for _, unwanted := range []string{"c.go", ".agent-harness/ADR.md", "d.go", "e.go", "internal/core/foo.go"} {
		if containsString(got, unwanted) {
			t.Fatalf("non tool_input content path %q must not be a mutation target: %#v", unwanted, got)
		}
	}
	// 동일 추출이 tool_input 내부에서는 보존됨을 증명한다.
	insideToolInput := []byte(`{
	  "tool_input":{
	    "patch":"*** Begin Patch\n*** Add File: c.go\n*** Update File: .agent-harness/ADR.md\n*** Delete File: d.go\n*** Move to: e.go\n*** End Patch",
	    "note":"internal/core/foo.go"
	  }
	}`)
	gotInside := PathsFromHookInput(insideToolInput)
	for _, want := range []string{"c.go", ".agent-harness/ADR.md", "d.go", "e.go", "internal/core/foo.go"} {
		if !containsString(gotInside, want) {
			t.Fatalf("tool_input content path %q missing from mutation targets: %#v", want, gotInside)
		}
	}
	var out []string
	seen := map[string]bool{}
	if !addPatchPathsFromHookString(&out, seen, "*** Begin Patch\n*** Add File: z.go") {
		t.Fatal("expected patch string detection")
	}
	addHookPath(&out, seen, "z.go")
	if countString(out, "z.go") != 1 {
		t.Fatalf("expected de-duplicated paths, got %#v", out)
	}
}

func TestPathsFromHookInputIgnoresHookTranscriptMetadata(t *testing.T) {
	input := []byte(`{
	  "transcript_path":"/outside/codex-session.jsonl",
	  "agent_transcript_path":"/outside/subagent-session.jsonl",
	  "hook_input":{"transcript_path":"/outside/nested-session.jsonl"},
	  "tool_input":{
	    "file_path":"/repo/internal/core/owned.go",
	    "transcript_path":"/repo/tool-owned.jsonl",
	    "patch":"*** Begin Patch\n*** Add File: /repo/.agent-harness/research/evidence.md\n+evidence\n*** End Patch"
	  }
	}`)
	got := PathsFromHookInput(input)
	for _, unwanted := range []string{
		"/outside/codex-session.jsonl",
		"/outside/subagent-session.jsonl",
		"/outside/nested-session.jsonl",
	} {
		if containsString(got, unwanted) {
			t.Fatalf("hook metadata path %q must not be a mutation target: %#v", unwanted, got)
		}
	}
	for _, want := range []string{
		"/repo/internal/core/owned.go",
		"/repo/tool-owned.jsonl",
		"/repo/.agent-harness/research/evidence.md",
	} {
		if !containsString(got, want) {
			t.Fatalf("tool input path %q missing from mutation targets: %#v", want, got)
		}
	}
}

func TestPathsFromHookInputCollectsFilesystemSourceDestinationAliasesOnlyInsideToolInput(t *testing.T) {
	input := []byte(`{
	  "source":"hook-metadata",
	  "destination":"hook-metadata-destination",
	  "tool_input":{
	    "source":"/repo/source.txt",
	    "destination":"/repo/destination.txt",
	    "src":"/repo/src.txt",
	    "dst":"/repo/dst.txt",
	    "target":"/repo/target.txt"
	  }
	}`)
	got := PathsFromHookInput(input)
	for _, want := range []string{"/repo/source.txt", "/repo/destination.txt", "/repo/src.txt", "/repo/dst.txt", "/repo/target.txt"} {
		if !containsString(got, want) {
			t.Fatalf("filesystem alias %q missing from mutation targets: %#v", want, got)
		}
	}
	for _, unwanted := range []string{"hook-metadata", "hook-metadata-destination"} {
		if containsString(got, unwanted) {
			t.Fatalf("top-level hook metadata alias %q must not become a mutation target: %#v", unwanted, got)
		}
	}
}

func TestToolCommandAndProjectPathExtraction(t *testing.T) {
	input := []byte(`{"tool_name":"gitlab_create_mr","tool_input":{"flags":{"title":" Add feature ","description":"Body","labels":["bug",2],"assignee_id":[7],"copy_issue_labels":true},"issue_iid":"12","projectPath":"/repo"}}`)
	if got := ToolNameFromHookInput(input); got != "gitlab_create_mr" {
		t.Fatalf("ToolNameFromHookInput = %q", got)
	}
	command := CommandFromHookInput(input)
	for _, want := range []string{`glab mr create`, `--title "Add feature"`, `--description "Body"`, `--label "bug"`, `--label "2"`, `--copy-issue-labels`, `--related-issue "12"`, `--assignee-id "7"`} {
		if !strings.Contains(command, want) {
			t.Fatalf("expected command to contain %q, got %q", want, command)
		}
	}
	if got := ProjectPathFromHookInput(input); got != "/repo" {
		t.Fatalf("ProjectPathFromHookInput = %q", got)
	}
	if got := CommandFromHookInput([]byte(`{"tool_input":{"query":" symbols "}}`)); got != "symbols" {
		t.Fatalf("query fallback command = %q", got)
	}
}

func TestToolCommandExtractionForStructuredGlabAPI(t *testing.T) {
	input := []byte(`{"tool_name":"mcp__glab__glab_api","tool_input":{"endpoint":"projects/1/issues/2/links","method":"POST","flags":{"target_issue_iid":3,"link_type":"relates_to","note":"child task relation","private_token":"redacted"}}}`)
	command := CommandFromHookInput(input)
	for _, want := range []string{`glab api "projects/1/issues/2/links"`, `-X "POST"`, `-f "link_type=relates_to"`, `-f "note=child task relation"`, `-f "target_issue_iid=3"`} {
		if !strings.Contains(command, want) {
			t.Fatalf("expected structured glab API command to contain %q, got %q", want, command)
		}
	}
	if strings.Contains(command, "private_token") || strings.Contains(command, "redacted") {
		t.Fatalf("structured glab API command must not include token-like fields, got %q", command)
	}
}

func TestTranscriptHelpersExtractAssistantText(t *testing.T) {
	input := []byte(`{"lastAssistantMessage":" done ","transcriptPath":" /tmp/t.jsonl "}`)
	if got := LastAssistantMessageFromHookInput(input); got != "done" {
		t.Fatalf("LastAssistantMessageFromHookInput = %q", got)
	}
	if got := TranscriptPathFromHookInput(input); got != "/tmp/t.jsonl" {
		t.Fatalf("TranscriptPathFromHookInput = %q", got)
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := strings.Join([]string{
		`{"role":"user","content":"ignore"}`,
		`{"role":"assistant","content":[{"type":"tool_use","text":"skip"},{"type":"text","text":"hello"},{"content":"world"}]}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadLastAssistantMessageFromTranscript(path); got != "hello\nworld" {
		t.Fatalf("ReadLastAssistantMessageFromTranscript = %q", got)
	}
	if ReadLastAssistantMessageFromTranscript("") != "" || ReadLastAssistantMessageFromTranscript(filepath.Join(t.TempDir(), "missing")) != "" {
		t.Fatal("empty or missing transcript should return empty text")
	}
}

func TestMCPValueHelpers(t *testing.T) {
	values := map[string]any{
		"s":       "a, b",
		"any":     []any{"x", float64(2), ""},
		"strings": []string{" y "},
		"num":     float64(3),
		"flag":    true,
	}
	if got := firstStringValue(values, "missing", "s"); got != "a, b" {
		t.Fatalf("firstStringValue = %q", got)
	}
	if !boolValue(values, "flag") || boolValue(values, "missing") {
		t.Fatal("unexpected boolValue")
	}
	if got := strings.Join(stringListValue(values, "s"), "|"); got != "a|b" {
		t.Fatalf("string list from string = %q", got)
	}
	if got := strings.Join(stringListValue(values, "any"), "|"); got != "x|2" {
		t.Fatalf("string list from []any = %q", got)
	}
	if got := strings.Join(stringListValue(values, "strings"), "|"); got != "y" {
		t.Fatalf("string list from []string = %q", got)
	}
	if got := strings.Join(stringListValue(values, "num"), "|"); got != "3" {
		t.Fatalf("string list from number = %q", got)
	}
	if shellQuoteArg("a b") != `"a b"` {
		t.Fatal("unexpected shell quote")
	}
	_ = commandstep.StepResult{}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func countString(items []string, want string) int {
	count := 0
	for _, item := range items {
		if item == want {
			count++
		}
	}
	return count
}
