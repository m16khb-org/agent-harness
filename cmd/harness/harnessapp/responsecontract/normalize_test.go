package responsecontract

import "testing"

func TestNormalizeContractValueRewritesDynamicFields(t *testing.T) {
	input := map[string]any{
		"updated_at":                "2026-06-13T12:00:00Z",
		"audit_log_id":              "audit-123",
		"project_claude_skill":      true,
		"project_codex_skill":       false,
		"project_gjc_skill":         true,
		"project_reasonix_skill":    true,
		"project_reasonix_settings": true,
		"id":                        "job-123",
		"pid":                       1234,
		"head":                      "abcdef1",
		"message":                   "/tmp/repo changed",
		"commit":                    "1234567 subject",
		"command":                   map[string]any{"executed": true, "exit_code": float64(0), "started_at": "x", "finished_at": "y", "duration_ms": float64(17), "policy": map[string]any{}},
		"checkpoint":                map[string]any{"state_dir": "/state", "path": "x", "key": "k", "bytes": float64(10)},
		"record":                    map[string]any{"key": "self-verify-baseline", "schema_version": float64(1), "updated_at": "x", "bytes": float64(20)},
		"history":                   map[string]any{"key": "self-verify-candidate", "generated_at": "x", "updated_at": "y", "bytes": float64(30)},
		"items":                     []any{"2026-06-13T12:00:00Z"},
		"score":                     89.23999999999998,
	}
	got := NormalizeContractValue(input, map[string]string{"/tmp/repo": "$REPO"}).(map[string]any)
	assertEqual(t, got["updated_at"], "$TIMESTAMP")
	assertEqual(t, got["audit_log_id"], "$AUDIT_ID")
	assertEqual(t, got["project_claude_skill"], "$PROJECT_SKILL_PRESENCE")
	assertEqual(t, got["project_codex_skill"], "$PROJECT_SKILL_PRESENCE")
	assertEqual(t, got["project_gjc_skill"], "$PROJECT_SKILL_PRESENCE")
	assertEqual(t, got["project_reasonix_skill"], "$PROJECT_SKILL_PRESENCE")
	assertEqual(t, got["project_reasonix_settings"], "$PROJECT_SETTINGS_PRESENCE")
	assertEqual(t, got["id"], "$WORKER_JOB_ID")
	assertEqual(t, got["pid"], "$PID")
	assertEqual(t, got["head"], "$GIT_SHA")
	assertEqual(t, got["message"], "$REPO changed")
	assertEqual(t, got["commit"], "$GIT_SHA subject")
	assertEqual(t, got["command"].(map[string]any)["duration_ms"], "$DURATION_MS")
	assertEqual(t, got["checkpoint"].(map[string]any)["bytes"], "$STATE_BYTES")
	assertEqual(t, got["record"].(map[string]any)["bytes"], "$STATE_RECORD_BYTES")
	assertEqual(t, got["history"].(map[string]any)["bytes"], "$STATE_RECORD_BYTES")
	assertEqual(t, got["items"].([]any)[0], "$TIMESTAMP")
	assertEqual(t, got["score"], 89.24)
}

func TestNormalizeContractValueIssueOpsIDAndStringReplacements(t *testing.T) {
	got := NormalizeContractValue(map[string]any{
		"id":   "io-123",
		"path": "/repo/sub",
	}, map[string]string{"/repo/sub": "$SUB", "/repo": "$REPO"}).(map[string]any)
	assertEqual(t, got["id"], "$ISSUEOPS_ID")
	assertEqual(t, got["path"], "$SUB")
	if normalizeContractString("not a timestamp", nil) != "not a timestamp" {
		t.Fatal("unexpected normal string rewrite")
	}
}

func TestNormalizeContractValueRewritesGitHubAuthDrift(t *testing.T) {
	cases := []string{
		"IssueOps remote reflect-devils-advocate failed: gh issue view failed: To use GitHub CLI in a GitHub Actions workflow, set the GH_TOKEN environment variable.",
		"IssueOps remote reflect-devils-advocate failed: gh issue view failed: You are not logged into any GitHub hosts. To log in, run: gh auth login",
	}
	for _, input := range cases {
		got := NormalizeContractValue(input, nil)
		assertEqual(t, got, "IssueOps remote reflect-devils-advocate failed: gh issue view failed: $GH_AUTH_ERROR")
	}
}

func TestNormalizeMCPTextJSONRewritesNestedJSONText(t *testing.T) {
	input := map[string]any{
		"content": []any{
			map[string]any{"text": `{"updated_at":"2026-06-13T12:00:00Z","pid":123}`},
			map[string]any{"text": "plain"},
		},
	}
	got := NormalizeMCPTextJSON(input, nil).(map[string]any)
	content := got["content"].([]any)
	nested := content[0].(map[string]any)["json"].(map[string]any)
	assertEqual(t, nested["updated_at"], "$TIMESTAMP")
	assertEqual(t, nested["pid"], "$PID")
	assertEqual(t, content[1].(map[string]any)["text"], "plain")
}

func TestDynamicClassifierHelpers(t *testing.T) {
	if !looksLikeDynamicStateRecord(map[string]any{"key": "self-augment-lesson-a", "schema_version": 1, "updated_at": "x", "bytes": 1}) {
		t.Fatal("expected dynamic state record")
	}
	if looksLikeDynamicStateRecord(map[string]any{"key": "other", "schema_version": 1, "updated_at": "x", "bytes": 1}) {
		t.Fatal("unexpected dynamic state record")
	}
	if !looksLikeDynamicStateHistoryEntry(map[string]any{"key": "self-verify-promoted", "generated_at": "x", "updated_at": "y", "bytes": 1}) {
		t.Fatal("expected dynamic state history entry")
	}
	if !looksLikeStateCheckpoint(map[string]any{"state_dir": "s", "path": "p", "key": "k", "bytes": 1}) {
		t.Fatal("expected checkpoint")
	}
	if !looksLikeCommandRunResult(map[string]any{"executed": true, "exit_code": 0, "started_at": "x", "finished_at": "y", "duration_ms": 1, "policy": map[string]any{}}) {
		t.Fatal("expected command result")
	}
	if !isDynamicTimeKey("cutoff") || isDynamicTimeKey("name") {
		t.Fatal("unexpected dynamic time key classification")
	}
	if !isRFC3339Like("2026-06-13T12:00:00Z") || isRFC3339Like("not-time") {
		t.Fatal("unexpected RFC3339 classification")
	}
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
