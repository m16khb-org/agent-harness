package mcpcli

import (
	"strings"
	"testing"

	daemonlog "issueops/internal/domain/daemonlog"
)

// 데몬 경계의 실증 회귀(2026-08-21 실측): 세션 루틴 이벤트와 정상 종료
// 에러가 데몬 logFile을 채우지 않아야 한다. 판정 자체는 internal/domain/
// daemonlog가 소유하고 여기서는 데몬 스트림 연결이 그대로 쓰는지만 잠근다.
func TestDaemonStreamUsesRoutineEventFilter(t *testing.T) {
	var buf strings.Builder
	logger := daemonDiagnosticsLogger(&buf)
	for event := range daemonlog.RoutineSessionEvents {
		logger.Info(event)
	}
	logger.Info("tool call completed")
	out := buf.String()
	for event := range daemonlog.RoutineSessionEvents {
		if strings.Contains(out, "level=INFO msg=\""+event+"\"") {
			t.Fatalf("routine event %q must be demoted:\n%s", event, out)
		}
	}
	if !strings.Contains(out, `level=INFO msg="tool call completed"`) {
		t.Fatalf("ordinary INFO must pass:\n%s", out)
	}
}
