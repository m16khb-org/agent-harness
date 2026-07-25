package issueopscli

import "testing"

// 판정은 core가 하지만, CLI가 owner 조회 표면을 실제로 넘기는지는 이 층에서만
// 확인된다. 주입이 빠지면 게이트가 항상 거부로 떨어져 orca 사이클의 abandon이
// 통째로 막힌다 — fail-closed 방향이지만 그것도 결함이다(#136).
func TestCleanupDepsCarryTheOrcaOwnerInspector(t *testing.T) {
	deps := issueOpsFeedbackCleanupDeps()
	if deps.OrcaOwner == nil {
		t.Fatal("cleanup deps must carry the orca owner inspector for the residue gate")
	}
	if deps.OrcaIntent == nil {
		t.Fatal("cleanup deps must keep the pending-intent inspector wired")
	}
}
