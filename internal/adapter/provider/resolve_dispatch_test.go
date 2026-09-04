package provider

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/port"
)

// Resolve는 provider 이름 디스패치다. 지원 목록 에러가 정확한 이름을
// 안내하는지 잠근다.
func TestResolveDispatch(t *testing.T) {
	for name := range map[string]bool{"github": true, "gitlab": true} {
		resolved, err := Resolve(name)
		if err != nil || resolved == nil {
			t.Fatalf("Resolve(%q) = %v, %v", name, resolved, err)
		}
	}
	if _, err := Resolve("bitbucket"); err == nil || !strings.Contains(err.Error(), "supported: github, gitlab") {
		t.Fatalf("unknown provider error must list supported providers: %v", err)
	}
}

// ReadExecutionIssueSnapshot은 resolve 실패와 reader 미구현을 구분해서
// 거부해야 한다.
func TestReadExecutionIssueSnapshotFailsClosed(t *testing.T) {
	if _, err := ReadExecutionIssueSnapshot(context.Background(), "unknown", port.ExecutionIssueSnapshotRequest{URL: "u"}); err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("unknown provider must fail: %v", err)
	}
	// 실 provider는 스냅샷 리더를 구현한다. 실제 원격 호출 없이 타입
	// 주장 경로만 검증하기 위해 resolve 성공 후 리더 인터페이스가 되는지는
	// github/gitlab 패키지 테스트가 담당한다. 여기서는 컨텍스트 취소로
	// 원격 호출 전 fail-closed를 확인한다.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadExecutionIssueSnapshot(ctx, "github", port.ExecutionIssueSnapshotRequest{URL: "https://github.com/x/y/issues/1"}); err == nil {
		t.Fatal("cancelled context must fail before any remote call")
	}
}
