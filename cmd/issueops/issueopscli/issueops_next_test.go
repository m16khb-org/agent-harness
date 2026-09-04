package issueopscli

import (
	"strings"
	"testing"
)

// next는 사이클이 없어도 성공해야 한다. 어느 단계에서 실행하든 라우터가 이
// 명령 하나로 진입점을 찾기 때문이다.
func TestIssueOpsNextDispatchesThroughRegistry(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"next", "--json"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops next dispatch: %v", runErr)
	}
	for _, key := range []string{"\"stage\"", "\"cwd_role\"", "\"exits\"", "\"next_command\""} {
		if !strings.Contains(out, key) {
			t.Fatalf("issueops next --json output missing %s:\n%s", key, out)
		}
	}
	text := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"next"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops next text dispatch: %v", runErr)
	}
	if !strings.HasPrefix(text, "stage ") {
		t.Fatalf("issueops next text output missing the stage line:\n%s", text)
	}
}

// 배선이 없으면 조용히 빈 결과를 내지 않고 실패한다.
func TestIssueOpsNextFailsClosedWithoutRuntime(t *testing.T) {
	previous := issueOpsCLIDeps
	issueOpsCLIDeps = neutralIssueOpsCLIDeps()
	defer func() { issueOpsCLIDeps = previous }()
	if err := runIssueOps([]string{"next", "--json"}); err == nil {
		t.Fatal("an unconfigured runtime must fail closed")
	}
}

// --cwd는 관측 지점을 바꾼다. 저장소 밖이면 사이클을 고르지 않는다.
func TestIssueOpsNextClassifiesAnUnrelatedDirectory(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"next", "--cwd", t.TempDir(), "--json"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops next --cwd dispatch: %v", runErr)
	}
	if !strings.Contains(out, "\"key\": \"none\"") {
		t.Fatalf("an unrelated directory has no cycle to continue:\n%s", out)
	}
}
