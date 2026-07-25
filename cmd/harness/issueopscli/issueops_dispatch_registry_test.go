package issueopscli

import (
	"strings"
	"testing"
)

// TestIssueOpsListDispatchesThroughRegistry는 #93의 회귀 증거다: usage와
// commandparse에 존재하는 list가 dispatch registry에서 빠지면 사용자 가시
// 경로가 unknown subcommand로 실패한다.
func TestIssueOpsListDispatchesThroughRegistry(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"list", "--json"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops list dispatch: %v", runErr)
	}
	if !strings.Contains(out, "\"entries\"") && !strings.Contains(out, "\"scanned_records\"") {
		t.Fatalf("issueops list --json output missing aggregate fields:\n%s", out)
	}
	text := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"list"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops list text dispatch: %v", runErr)
	}
	if !strings.HasPrefix(text, "cycles:") {
		t.Fatalf("issueops list text output missing cycles summary:\n%s", text)
	}
}

// TestIssueOpsUsageRegistryBidirectionalParity는 usage 텍스트와 dispatch
// registry의 subcommand 집합 동등성을 registry에서 파생해 단언한다. 하드코딩
// fragment 스냅샷과 달리 어느 방향의 누락(#93 list, usage의 devils-advocate)도
// 즉시 실패시킨다.
func TestIssueOpsUsageRegistryBidirectionalParity(t *testing.T) {
	const prefix = "agent-harness issueops "
	usageKeys := map[string]bool{}
	for _, line := range strings.Split(issueOpsUsageText(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, prefix))
		if len(fields) == 0 {
			continue
		}
		usageKeys[fields[0]] = true
	}
	for key := range issueOpsSubcommands {
		if !usageKeys[key] {
			t.Errorf("registry subcommand %q is missing from issueOpsUsageText()", key)
		}
	}
	for key := range usageKeys {
		if _, ok := issueOpsSubcommands[key]; !ok {
			t.Errorf("usage subcommand %q is not registered in issueOpsSubcommands", key)
		}
	}
}
