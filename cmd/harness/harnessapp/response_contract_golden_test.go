package harnessapp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestResponseContractsGolden(t *testing.T) {
	stubIssueOpsChildIssueVerifier(t, nil)
	stateDir := t.TempDir()
	workspaceDir := t.TempDir()
	gitRepoDir := makeGitRepoForContract(t)
	homeDir := t.TempDir()
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	workerDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_DAEMON_DIR", filepath.Join(stateDir, "daemon"))
	t.Setenv("HARNESS_AUDIT_LOG", auditLog)
	t.Setenv("HARNESS_WORKER_DIR", workerDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", filepath.Join(homeDir, ".codex"))

	replacements := map[string]string{
		stateDir:      "$STATE_DIR",
		workspaceDir:  "$WORKSPACE",
		gitRepoDir:    "$GIT_REPO",
		harnessRoot(): "$HARNESS_ROOT",
		homeDir:       "$HOME",
		auditLog:      "$AUDIT_LOG",
		workerDir:     "$WORKER_DIR",
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	executableBytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	executableHash := sha256.Sum256(executableBytes)
	replacements[executable] = "$HARNESS_EXECUTABLE"
	replacements[hex.EncodeToString(executableHash[:])] = "$HARNESS_EXECUTABLE_SHA256"
	addEvalSymlinkReplacement(t, replacements, stateDir, "$STATE_DIR")
	addEvalSymlinkReplacement(t, replacements, workspaceDir, "$WORKSPACE")
	addEvalSymlinkReplacement(t, replacements, gitRepoDir, "$GIT_REPO")
	addEvalSymlinkReplacement(t, replacements, harnessRoot(), "$HARNESS_ROOT")
	addEvalSymlinkReplacement(t, replacements, workerDir, "$WORKER_DIR")
	addEvalSymlinkReplacement(t, replacements, filepath.Dir(auditLog), "$AUDIT_DIR")

	snapshot := map[string]any{
		"cli": buildCLIResponseContractSnapshot(t, replacements, stateDir, workspaceDir, gitRepoDir),
		"mcp": buildMCPResponseContractSnapshot(t, replacements, workspaceDir, gitRepoDir),
	}
	normalized := normalizeDocsCountsForGolden(snapshot)
	assertJSONGolden(t, "response_contracts.golden.json", normalizeIssueOpsStateKeysForGolden(normalized))
}

const docsCountGoldenPlaceholder = "$DOCS_COUNT"
const issueOpsStateKeyGoldenPlaceholder = "$ISSUEOPS_STATE_KEY"

var issueOpsStateKeyPattern = regexp.MustCompile(`/issueops-v1/[0-9a-f]{16}/`)

// IssueOps lifecycle ID에서 파생한 state key는 fixture마다 달라지므로 경로의
// 구조만 golden에 남긴다. 실제 CLI 응답은 치환하지 않는다.
func normalizeIssueOpsStateKeysForGolden(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = normalizeIssueOpsStateKeysForGolden(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			out = append(out, normalizeIssueOpsStateKeysForGolden(child))
		}
		return out
	case string:
		return issueOpsStateKeyPattern.ReplaceAllStringFunc(typed, func(string) string {
			return "/issueops-v1/" + issueOpsStateKeyGoldenPlaceholder + "/"
		})
	default:
		return value
	}
}

// normalizeDocsCountsForGolden은 tracked markdown 계수를 placeholder로 치환해
// golden이 "오늘 저장소에 문서가 몇 개인가"가 아니라 응답의 형태를 기록하게
// 한다(#109 — plan 문서 커밋마다 수동 동기화하던 함정의 구조적 제거). 수치일
// 때만 치환한다: 비수치 docs_count는 계약 회귀라 golden이 그대로 잡아야 한다.
func normalizeDocsCountsForGolden(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if isDocsCountGoldenKey(key) && isDocsCountGoldenNumber(child) {
				out[key] = docsCountGoldenPlaceholder
				continue
			}
			out[key] = normalizeDocsCountsForGolden(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			out = append(out, normalizeDocsCountsForGolden(child))
		}
		return out
	default:
		return value
	}
}

func isDocsCountGoldenKey(key string) bool {
	return key == "docs_count" || key == "docs_indexed"
}

func isDocsCountGoldenNumber(value any) bool {
	switch value.(type) {
	case int, int32, int64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func assertJSONGolden(t *testing.T, name string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, name, append(b, '\n'))
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./cmd/harness/harnessapp -run Golden -update)", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s: %s", name, firstJSONDifference(t, want, got, "$"))
	}
}

func firstJSONDifference(t *testing.T, want, got []byte, path string) string {
	t.Helper()
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal expected golden JSON: %v", err)
	}
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal actual golden JSON: %v", err)
	}
	return firstContractValueDifference(wantValue, gotValue, path)
}

func firstContractValueDifference(want, got any, path string) string {
	if reflect.DeepEqual(want, got) {
		return ""
	}
	switch wantValue := want.(type) {
	case map[string]any:
		gotValue, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s type: got %T, want %T", path, got, want)
		}
		wantKeys := make([]string, 0, len(wantValue))
		for key := range wantValue {
			wantKeys = append(wantKeys, key)
		}
		sort.Strings(wantKeys)
		for _, key := range wantKeys {
			wantChild := wantValue[key]
			gotChild, exists := gotValue[key]
			if !exists {
				return fmt.Sprintf("%s.%s missing from actual value", path, key)
			}
			if difference := firstContractValueDifference(wantChild, gotChild, path+"."+key); difference != "" {
				return difference
			}
		}
		gotKeys := make([]string, 0, len(gotValue))
		for key := range gotValue {
			gotKeys = append(gotKeys, key)
		}
		sort.Strings(gotKeys)
		for _, key := range gotKeys {
			if _, exists := wantValue[key]; !exists {
				return fmt.Sprintf("%s.%s unexpected in actual value", path, key)
			}
		}
	case []any:
		gotValue, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s type: got %T, want %T", path, got, want)
		}
		if len(wantValue) != len(gotValue) {
			return fmt.Sprintf("%s length: got %d, want %d", path, len(gotValue), len(wantValue))
		}
		for index, wantChild := range wantValue {
			if difference := firstContractValueDifference(wantChild, gotValue[index], fmt.Sprintf("%s[%d]", path, index)); difference != "" {
				return difference
			}
		}
	default:
		return fmt.Sprintf("%s: got %#v, want %#v", path, got, want)
	}
	return fmt.Sprintf("%s differs", path)
}
