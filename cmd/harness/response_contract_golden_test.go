package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core"
)

func TestResponseContractsGolden(t *testing.T) {
	stateDir := t.TempDir()
	workspaceDir := t.TempDir()
	gitRepoDir := makeGitRepoForContract(t)
	homeDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", filepath.Join(homeDir, ".codex"))

	replacements := map[string]string{
		stateDir:      "$STATE_DIR",
		workspaceDir:  "$WORKSPACE",
		gitRepoDir:    "$GIT_REPO",
		harnessRoot(): "$HARNESS_ROOT",
		homeDir:       "$HOME",
	}
	addEvalSymlinkReplacement(t, replacements, stateDir, "$STATE_DIR")
	addEvalSymlinkReplacement(t, replacements, workspaceDir, "$WORKSPACE")
	addEvalSymlinkReplacement(t, replacements, gitRepoDir, "$GIT_REPO")
	addEvalSymlinkReplacement(t, replacements, harnessRoot(), "$HARNESS_ROOT")

	snapshot := map[string]any{}
	cliSnapshot := map[string]any{}
	snapshot["cli"] = cliSnapshot
	cliSnapshot["inspect"] = runCLIJSONContract(t, replacements, func() error {
		return runInspect([]string{"--json", "--repo", workspaceDir})
	})
	cliSnapshot["docs_index"] = runCLIJSONContract(t, replacements, func() error {
		return runDocs([]string{"--json"})
	})
	cliSnapshot["preflight"] = runCLIJSONContract(t, replacements, func() error {
		return runPreflight([]string{"--json", gitRepoDir})
	})
	cliSnapshot["policy_check"] = runCLIJSONContract(t, replacements, func() error {
		return runPolicy([]string{"check", "--workspace-root", workspaceDir, "--cwd", workspaceDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["state_write"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"write", "--key", "current", "--value", "current content", "--json"})
	})
	cliSnapshot["state_read"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"read", "--key", "current", "--json"})
	})
	cliSnapshot["state_list"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"list", "--json"})
	})

	old := mustStateReadForContract(t, "current")
	old.Record.Key = "old"
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	mustWriteStateRecordForContract(t, stateDir, "old", old.Record)
	cliSnapshot["state_prune_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--json"})
	})
	cliSnapshot["state_prune_confirm"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--confirm", "--json"})
	})

	legacy := core.StateRecord{
		Key:       "legacy",
		Content:   "legacy content",
		UpdatedAt: "2000-01-01T00:00:00Z",
		Bytes:     len([]byte("legacy content")),
	}
	mustWriteStateRecordForContract(t, stateDir, "legacy", legacy)
	cliSnapshot["state_migrate_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"migrate", "--json"})
	})
	cliSnapshot["state_migrate_confirm"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"migrate", "--confirm", "--json"})
	})
	cliSnapshot["state_doctor_healthy"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"doctor", "--json"})
	})

	if err := os.WriteFile(filepath.Join(stateDir, "corrupt.json"), []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cliSnapshot["state_doctor_corrupt"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"doctor", "--json"})
	})

	writeSelfAugmentCompareFixturesForContract(t, stateDir)
	cliSnapshot["self_augment_plan"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfAugment([]string{"--target-score", "95", "--json"})
	})
	cliSnapshot["self_augment_lesson"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfAugment([]string{"lesson", "--candidate", "reflexion-state-memory", "--lesson", "Contract lesson", "--next-action", "Check stored lesson before next cycle", "--state-key", "self-augment-lesson-contract", "--json"})
	})
	cliSnapshot["self_verify_candidates"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"candidates", "--save-state", "--state-key", "self-verify-candidates-contract", "--json"})
	})
	cliSnapshot["self_verify_compare"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"compare", "--baseline-key", "self-verify-baseline", "--candidate-key", "self-verify-candidate", "--json"})
	})
	cliSnapshot["self_verify_promote_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"promote", "--from-key", "self-verify-candidate", "--baseline-key", "self-verify-promoted", "--json"})
	})
	cliSnapshot["self_verify_history"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"history", "--prefix", "self-verify", "--json"})
	})

	mcpSnapshot := map[string]any{}
	snapshot["mcp"] = mcpSnapshot
	mcpSnapshot["harness_inspect"] = runMCPToolContract(t, replacements, "harness_inspect", map[string]any{
		"repo": workspaceDir,
	})
	mcpSnapshot["docs_index"] = runMCPToolContract(t, replacements, "docs_index", map[string]any{})
	mcpSnapshot["atomic_commit_preflight"] = runMCPToolContract(t, replacements, "atomic_commit_preflight", map[string]any{
		"path": gitRepoDir,
	})
	mcpSnapshot["command_policy_check"] = runMCPToolContract(t, replacements, "command_policy_check", map[string]any{
		"workspace_root": workspaceDir,
		"cwd":            workspaceDir,
		"argv":           []string{"git", "status", "--short"},
	})
	mcpSnapshot["state_prune"] = runMCPToolContract(t, replacements, "state_prune", map[string]any{
		"max_age": "1h",
	})
	mcpSnapshot["state_doctor"] = runMCPToolContract(t, replacements, "state_doctor", map[string]any{})
	mcpSnapshot["state_migrate"] = runMCPToolContract(t, replacements, "state_migrate", map[string]any{})
	mcpSnapshot["self_augment"] = runMCPToolContract(t, replacements, "self_augment", map[string]any{
		"target_score": 95,
	})
	mcpSnapshot["self_augment_lesson"] = runMCPToolContract(t, replacements, "self_augment_lesson", map[string]any{
		"candidate_id": "reflexion-state-memory",
		"lesson":       "MCP lesson",
		"next_action":  "Check MCP lesson before next cycle",
		"state_key":    "self-augment-lesson-mcp",
	})
	mcpSnapshot["self_verify_candidates"] = runMCPToolContract(t, replacements, "self_verify_candidates", map[string]any{
		"save_state": true,
		"state_key":  "self-verify-candidates-mcp",
	})
	mcpSnapshot["self_verify_compare"] = runMCPToolContract(t, replacements, "self_verify_compare", map[string]any{
		"baseline_key":  "self-verify-baseline",
		"candidate_key": "self-verify-candidate",
	})
	mcpSnapshot["self_verify_promote"] = runMCPToolContract(t, replacements, "self_verify_promote", map[string]any{
		"from_key":     "self-verify-candidate",
		"baseline_key": "self-verify-promoted",
	})
	mcpSnapshot["self_verify_history"] = runMCPToolContract(t, replacements, "self_verify_history", map[string]any{
		"prefix": "self-verify",
	})

	assertJSONGolden(t, "response_contracts.golden.json", snapshot)
}

func makeGitRepoForContract(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitForContract(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# contract fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, dir, "add", "README.md")
	runGitForContract(t, dir,
		"-c", "user.name=Contract Test",
		"-c", "user.email=contract@example.invalid",
		"commit", "-q",
		"-m", "docs(contract): add fixture",
		"-m", "Lore:\n- Intent: Normalize preflight contract.\n- Why: Response golden should cover git DTOs.\n- Changes:\n  - Add fixture README.\n- Verify: go test ./cmd/harness -run Golden\n- Risk: Low",
	)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runGitForContract(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func addEvalSymlinkReplacement(t *testing.T, replacements map[string]string, path, token string) {
	t.Helper()
	eval, err := filepath.EvalSymlinks(path)
	if err == nil && eval != "" {
		replacements[eval] = token
	}
}

func writeSelfAugmentCompareFixturesForContract(t *testing.T, stateDir string) {
	t.Helper()
	summary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1000},
		},
	}
	fixtures := []struct {
		key         string
		elapsed     int64
		generatedAt string
	}{
		{key: "self-verify-baseline", elapsed: 1000, generatedAt: "2000-01-01T00:00:00Z"},
		{key: "self-verify-candidate", elapsed: 1100, generatedAt: "2000-01-01T00:01:00Z"},
	}
	for _, fixture := range fixtures {
		if err := writeSelfAugmentSnapshotRecord(stateDir, fixture.key, SelfAugmentStateSnapshot{
			SchemaVersion: 1,
			Kind:          "self_verification_summary",
			OK:            true,
			Iterations:    10,
			BaseSeed:      600,
			ElapsedMS:     fixture.elapsed,
			HarnessRoot:   harnessRoot(),
			GeneratedAt:   fixture.generatedAt,
			Summary:       summary,
		}); err != nil {
			t.Fatalf("write compare fixture %s: %v", fixture.key, err)
		}
	}
}

func runCLIJSONContract(t *testing.T, replacements map[string]string, fn func() error) any {
	t.Helper()
	stdout := captureStdoutForContract(t, fn)
	var value any
	if err := json.Unmarshal([]byte(stdout), &value); err != nil {
		t.Fatalf("unmarshal CLI JSON %q: %v", stdout, err)
	}
	return normalizeContractValue(value, replacements)
}

func runMCPToolContract(t *testing.T, replacements map[string]string, name string, arguments map[string]any) any {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := handleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("handleToolCall(%s): %+v", name, rpcErr)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		t.Fatal(err)
	}
	return normalizeMCPTextJSON(normalizeContractValue(value, replacements), replacements)
}

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func mustStateReadForContract(t *testing.T, key string) core.StateResult {
	t.Helper()
	result, err := core.StateRead(key)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustWriteStateRecordForContract(t *testing.T, stateDir, key string, record core.StateRecord) {
	t.Helper()
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, key+".json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func normalizeContractValue(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		isStateCheckpoint := looksLikeStateCheckpoint(v)
		isDynamicStateRecord := looksLikeDynamicStateRecord(v)
		for key, child := range v {
			if isDynamicTimeKey(key) {
				out[key] = "$TIMESTAMP"
				continue
			}
			if isStateCheckpoint && key == "bytes" {
				out[key] = "$STATE_BYTES"
				continue
			}
			if isDynamicStateRecord && key == "bytes" {
				out[key] = "$STATE_RECORD_BYTES"
				continue
			}
			if key == "audit_log_id" {
				out[key] = "$AUDIT_ID"
				continue
			}
			if key == "head" || key == "sha" {
				out[key] = "$GIT_SHA"
				continue
			}
			out[key] = normalizeContractValue(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, normalizeContractValue(child, replacements))
		}
		return out
	case string:
		return normalizeContractString(v, replacements)
	default:
		return v
	}
}

func looksLikeDynamicStateRecord(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || (!strings.HasPrefix(keyValue, "self-augment-lesson-") && !strings.HasPrefix(keyValue, "self-verify-candidates-")) {
		return false
	}
	if _, ok := value["schema_version"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeStateCheckpoint(value map[string]any) bool {
	if _, ok := value["state_dir"]; !ok {
		return false
	}
	if _, ok := value["path"]; !ok {
		return false
	}
	if _, ok := value["key"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

var gitSubjectPrefixRe = regexp.MustCompile(`^[0-9a-f]{7,40} `)

func normalizeContractString(value string, replacements map[string]string) string {
	keys := make([]string, 0, len(replacements))
	for from := range replacements {
		if from != "" {
			keys = append(keys, from)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, from := range keys {
		value = strings.ReplaceAll(value, from, replacements[from])
	}
	if gitSubjectPrefixRe.MatchString(value) {
		return "$GIT_SHA " + strings.TrimSpace(gitSubjectPrefixRe.ReplaceAllString(value, ""))
	}
	if isRFC3339Like(value) {
		return "$TIMESTAMP"
	}
	return value
}

func normalizeMCPTextJSON(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "text" {
				if text, ok := child.(string); ok {
					var nested any
					if err := json.Unmarshal([]byte(text), &nested); err == nil {
						out["json"] = normalizeContractValue(nested, replacements)
						continue
					}
				}
			}
			out[key] = normalizeMCPTextJSON(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, normalizeMCPTextJSON(child, replacements))
		}
		return out
	default:
		return v
	}
}

func isDynamicTimeKey(key string) bool {
	switch key {
	case "updated_at", "generated_at", "cutoff", "started_at", "finished_at":
		return true
	default:
		return false
	}
}

func isRFC3339Like(value string) bool {
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	return false
}
