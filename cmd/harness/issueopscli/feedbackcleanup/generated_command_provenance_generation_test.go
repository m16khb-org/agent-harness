package feedbackcleanup

import (
	"context"
	"strings"
	"testing"

	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type stubProvenanceObserver struct{}

func (stubProvenanceObserver) Observe(context.Context) (provenanceport.Receipt, error) {
	return provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64),
	}, nil
}

// TestCleanupNextCommandSkipsProvenanceWithoutALease는 #437의 부수 결함을
// 고정한다.
//
// generation 0은 아직 lease가 없다는 뜻이다. provenance는 "이 generation의
// lease에 결속된 명령"을 표현하므로 결속할 대상이 없으면 붙일 것이 없다.
// 억지로 붙이면 Validate가 generation 0을 거부해 명령 자체가 실패한다.
//
// executioncmd는 #411에서 이미 이 규칙을 갖췄는데 cleanup 경로에는 없었다.
// 실측: execution을 가진 적 없는 record(problem 단계)의 cleanup abandon
// preview가 generated_command_provenance_invalid로 실패했다.
func TestCleanupNextCommandSkipsProvenanceWithoutALease(t *testing.T) {
	const command = "agent-harness issueops cleanup abandon --id io-x --apply --confirm --json"

	unbound, err := bindCleanupNextCommand(command, 0, stubProvenanceObserver{})
	if err != nil {
		t.Fatalf("lease 없는 명령은 provenance 없이 통과해야 한다: %v", err)
	}
	if unbound != command {
		t.Fatalf("결속할 lease가 없으면 명령이 그대로여야 한다: %q", unbound)
	}

	bound, err := bindCleanupNextCommand(command, 3, stubProvenanceObserver{})
	if err != nil {
		t.Fatalf("lease가 있으면 결속돼야 한다: %v", err)
	}
	if !strings.Contains(bound, "--generated-for-generation 3") {
		t.Fatalf("generation이 있으면 provenance가 붙어야 한다: %q", bound)
	}
}

// TestCleanupNextCommandLeavesAnEmptyCommandAlone는 빈 명령에 provenance를
// 붙이지 않음을 고정한다.
func TestCleanupNextCommandLeavesAnEmptyCommandAlone(t *testing.T) {
	got, err := bindCleanupNextCommand("", 3, stubProvenanceObserver{})
	if err != nil || got != "" {
		t.Fatalf("빈 명령은 그대로여야 한다: %q %v", got, err)
	}
}
