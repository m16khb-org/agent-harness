package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func init() {
	if len(os.Args) != 5 || os.Args[1] != "-C" || os.Args[3] != "app-server" || os.Args[4] != "--stdio" {
		return
	}
	mode, err := os.ReadFile(filepath.Join(os.Args[2], ".codex-hooks-list-test-mode"))
	if err != nil || strings.TrimSpace(string(mode)) != "protocol-order" {
		return
	}
	os.Exit(runHooksListProtocolOrderHelper(os.Args[2]))
}

func TestHooksListTransportBoundsAreFixed(t *testing.T) {
	if hooksListTimeout != 15*time.Second || hooksListStdoutLimit != 1<<20 || hooksListStderrLimit != 32<<10 || hooksListLineLimit != 512<<10 || hooksListObjectLimit != 256 {
		t.Fatalf("unexpected transport bounds: timeout=%s stdout=%d stderr=%d line=%d objects=%d", hooksListTimeout, hooksListStdoutLimit, hooksListStderrLimit, hooksListLineLimit, hooksListObjectLimit)
	}
	if hooksListHookLimit != 1024 || hooksListMessageLimit != 256 || hooksListStringLimit != 4096 || hooksListDepthLimit != 32 || hooksListNodeLimit != 32<<10 || hooksListEncodedLimit != 1<<20 {
		t.Fatalf("unexpected result bounds: hooks=%d messages=%d strings=%d depth=%d nodes=%d encoded=%d", hooksListHookLimit, hooksListMessageLimit, hooksListStringLimit, hooksListDepthLimit, hooksListNodeLimit, hooksListEncodedLimit)
	}
}

func TestParseHooksListResultBoundsUnknownFieldsAndRejectsSecretLikeKeys(t *testing.T) {
	workerRoot := "/tmp/issueops-worker"
	tests := []struct {
		name   string
		result map[string]any
		want   string
	}{
		{
			name: "oversized unknown field",
			result: map[string]any{
				"data":   []any{map[string]any{"cwd": workerRoot, "hooks": []any{}, "warnings": []any{}, "errors": []any{}}},
				"future": strings.Repeat("x", hooksListStringLimit+1),
			},
			want: "string limit",
		},
		{
			name: "secret-like map key",
			result: map[string]any{
				"data": []any{map[string]any{
					"cwd":      workerRoot,
					"hooks":    []any{map[string]any{"api_key=secret-value": true}},
					"warnings": []any{},
					"errors":   []any{},
				}},
			},
			want: "unsafe map key",
		},
		{
			name: "secret-bearing map key with opaque value",
			result: map[string]any{
				"data": []any{map[string]any{
					"cwd": workerRoot,
					"hooks": []any{map[string]any{
						"metadata": map[string]any{"api_key": "review-secret-value"},
					}},
					"warnings": []any{},
					"errors":   []any{},
				}},
			},
			want: "secret-bearing map key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatal(err)
			}
			_, err = parseHooksListResult(raw, workerRoot)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("parseHooksListResult error = %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "review-secret-value") {
				t.Fatalf("parseHooksListResult reflected a secret: %v", err)
			}
		})
	}
}

func TestParseHooksListResultRejectsDepthNodeAndEncodedSizeAmplification(t *testing.T) {
	workerRoot := "/tmp/issueops-worker"
	parseHook := func(t *testing.T, hook map[string]any) error {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"data": []any{map[string]any{
			"cwd": workerRoot, "hooks": []any{hook}, "warnings": []any{}, "errors": []any{},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = parseHooksListResult(raw, workerRoot)
		return err
	}

	t.Run("nesting depth", func(t *testing.T) {
		var nested any = true
		for range 40 {
			nested = []any{nested}
		}
		if err := parseHook(t, map[string]any{"payload": nested}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "nesting depth") {
			t.Fatalf("deep result error = %v, want nesting depth", err)
		}
	})

	t.Run("node count", func(t *testing.T) {
		values := make([]any, 40_000)
		if err := parseHook(t, map[string]any{"payload": values}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "node limit") {
			t.Fatalf("wide result error = %v, want node limit", err)
		}
	})

	t.Run("encoded size", func(t *testing.T) {
		values := make([]any, 20_000)
		var nested any = values
		for range 25 {
			nested = []any{nested}
		}
		if err := parseHook(t, map[string]any{"payload": nested}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "encoded size") {
			t.Fatalf("amplified result error = %v, want encoded size", err)
		}
	})
}

func TestHooksListUnexpectedResponseIDDoesNotReflectAttackerInput(t *testing.T) {
	secret := "token=review-secret-value"
	envelope := map[string]json.RawMessage{"id": json.RawMessage(`"` + secret + `"`)}
	_, _, err := hooksListResponseID(envelope)
	if err == nil || !strings.Contains(err.Error(), "unexpected response id") {
		t.Fatalf("unexpected response id error = %v", err)
	}
	if strings.Contains(err.Error(), "review-secret-value") {
		t.Fatalf("unexpected response id reflected attacker input: %v", err)
	}
}

func TestListHooksWaitsForInitializeBeforeSendingHooksRequest(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	workerRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workerRoot, ".codex-hooks-list-test-mode"), []byte("protocol-order\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(binDir, "codex")); err != nil {
		t.Skipf("symlink test Codex executable: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result, err := ListHooks(context.Background(), workerRoot)
	if err != nil {
		t.Fatalf("ordered hooks/list exchange failed: %v", err)
	}
	if !result.OK || result.AuditLogID == "" || len(result.Data) != 1 || result.Data[0].CWD != workerRoot {
		t.Fatalf("unexpected ordered hooks/list result: %+v", result)
	}
}

func TestParseHooksListResultRedactsKnownStringValues(t *testing.T) {
	workerRoot := "/tmp/issueops-worker"
	secret := "review-secret-value"
	raw, err := json.Marshal(map[string]any{"data": []any{map[string]any{
		"cwd":      workerRoot,
		"hooks":    []any{map[string]any{"command": "api_key=" + secret}},
		"warnings": []any{"Authorization: Bearer " + secret},
		"errors":   []any{},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := parseHooksListResult(raw, workerRoot)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || !strings.Contains(string(encoded), "redacted") {
		t.Fatalf("known response strings were not redacted: %s", encoded)
	}
}

func runHooksListProtocolOrderHelper(workerRoot string) int {
	reader := bufio.NewReader(os.Stdin)
	first, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(first) != initializeMessage {
		fmt.Fprintln(os.Stderr, "missing exact initialize request")
		return 90
	}
	type readResult struct {
		line string
		err  error
	}
	next := make(chan readResult, 1)
	go func() {
		line, err := reader.ReadString('\n')
		next <- readResult{line: line, err: err}
	}()
	select {
	case early := <-next:
		fmt.Fprintf(os.Stderr, "request arrived before initialize response: %q (%v)\n", early.line, early.err)
		return 91
	case <-time.After(150 * time.Millisecond):
	}
	fmt.Println(`{"id":1,"result":{}}`)
	var second readResult
	select {
	case second = <-next:
	case <-time.After(2 * time.Second):
		fmt.Fprintln(os.Stderr, "initialized notification timeout")
		return 92
	}
	if second.err != nil || strings.TrimSpace(second.line) != initializedMessage {
		fmt.Fprintf(os.Stderr, "invalid initialized notification: %q (%v)\n", second.line, second.err)
		return 93
	}
	third, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "hooks/list request timeout")
		return 94
	}
	var request struct {
		Method string `json:"method"`
		ID     int    `json:"id"`
		Params struct {
			CWDs []string `json:"cwds"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(third), &request); err != nil || request.Method != "hooks/list" || request.ID != 2 || len(request.Params.CWDs) != 1 || request.Params.CWDs[0] != workerRoot {
		fmt.Fprintf(os.Stderr, "invalid hooks/list request: %q (%v)\n", third, err)
		return 95
	}
	response := map[string]any{"id": 2, "result": map[string]any{"data": []any{map[string]any{
		"cwd": workerRoot, "hooks": []any{}, "warnings": []any{}, "errors": []any{},
	}}}}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 96
	}
	return 0
}
