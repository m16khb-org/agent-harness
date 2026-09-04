package feedbackcleanup

import (
	"context"
	"strings"

	provenanceapp "issueops/internal/application/issueopsprovenance"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

// bindCleanupNextCommand는 cleanup이 낸 다음 명령에 provenance를 결속한다.
//
// generation 0은 아직 lease가 없다는 뜻이다 — execution을 가진 적 없는
// record(예: problem 단계에 머문 사이클)의 cleanup preview가 그 경우다.
// provenance는 "이 generation의 lease에 결속된 명령"을 표현하므로 결속할
// 대상이 없으면 붙일 것이 없다. 억지로 붙이면 Validate가 generation 0을
// 거부해 preview 자체가 실패하고, 그 record는 정리할 수단을 잃는다.
//
// executioncmd는 #411에서 이미 이 규칙을 갖췄다. 두 경로가 같은 질문에 같은
// 답을 해야 한다 — 한쪽만 lease 없는 명령을 다룰 수 있으면 사용자는 어느
// 표면을 쓰느냐에 따라 막힌다(#437).
func bindCleanupNextCommand(command string, generation uint64, observer provenanceport.Observer) (string, error) {
	if strings.TrimSpace(command) == "" || generation == 0 {
		return command, nil
	}
	return provenanceapp.Bind(context.Background(), command, generation, observer)
}
