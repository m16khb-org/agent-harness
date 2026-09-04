package issueopscli

import (
	"errors"
	"testing"

	"issueops/internal/port"
)

// #97: cleanup finish orca_remove 단계의 멱등 정규화. typed not_found 계열은
// 리소스 이미 부재 = 성공이고, 그 외 오류는 실패로 보존되어야 finish의
// resumable 계약이 성립한다. 이 정규화는 기존에 테스트가 없어 비영 종료
// 경로의 코드 유실이 조용히 통과했다.
func TestNormalizeOrcaRemoveWorktreeErr(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantNil bool
	}{
		{name: "nil", err: nil, wantNil: true},
		{name: "typed selector_not_found", err: &port.OrcaError{Code: "selector_not_found"}, wantNil: true},
		{name: "typed not_found", err: &port.OrcaError{Code: "not_found"}, wantNil: true},
		{name: "prose fallback", err: errors.New("worktree not found"), wantNil: true},
		{name: "command_failed stays", err: &port.OrcaError{Code: "command_failed", Detail: "stdout: selector_not_found in diagnostic"}, wantNil: false},
		{name: "other typed error stays", err: &port.OrcaError{Code: "orca_rejected"}, wantNil: false},
		{name: "plain error stays", err: errors.New("relay handshake refused"), wantNil: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeOrcaRemoveWorktreeErr(tc.err)
			if (got == nil) != tc.wantNil {
				t.Fatalf("normalizeOrcaRemoveWorktreeErr(%v) = %v, wantNil=%v", tc.err, got, tc.wantNil)
			}
		})
	}
}
