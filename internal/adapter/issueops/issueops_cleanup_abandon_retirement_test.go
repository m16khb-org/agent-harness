package issueops

import (
	"context"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// finish는 remote_artifact_merged를 요구하고 abandon은 artifact 부재를 요구했다.
// 그 사이에 "artifact가 있지만 머지되지 않은" 정상적인 결말이 통째로 빠졌다.
// superseded로 닫힌 PR, base 브랜치 삭제로 자동 close된 PR, 다른 브랜치로 복원되어
// 통합된 작업이 모두 여기에 해당하며, 어느 경로로도 은퇴하지 못했다(#342).
func TestAbandonAllowsDoneCycleWithUnmergedArtifact(t *testing.T) {
	stateRoot, record := abandonRetirementRecord(t)
	req := abandonRequest(record.ID, false, "")
	req.ArtifactUnmerged = true

	result, err := CleanupAbandon(context.Background(), stateRoot, req, abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
	if err != nil {
		t.Fatalf("a done cycle with an unmerged artifact must retire through abandon: %v (%v)", err, result.Missing)
	}
}

// 머지 증적을 가진 레코드의 정답은 여전히 reflect→finish다. abandon이 이를
// 삼키면 머지 증적 보존 경로를 우회하는 탈출구가 된다.
func TestAbandonRejectsMergedArtifact(t *testing.T) {
	stateRoot, record := abandonRetirementRecord(t)
	req := abandonRequest(record.ID, false, "")
	req.ArtifactUnmerged = false

	result, err := CleanupAbandon(context.Background(), stateRoot, req, abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
	if err == nil {
		t.Fatal("a record whose artifact was not observed as unmerged must stay blocked")
	}
	if !containsString(result.Missing, "remote_artifact_unmerged") {
		t.Fatalf("the gate must name itself: %v", result.Missing)
	}
}

// 관측하지 못한 것은 통과가 아니다. 원격 조회에 실패하면 ArtifactUnmerged가
// false로 남고 게이트는 닫힌 상태를 유지한다.
func TestAbandonKeepsArtifactGateClosedWithoutObservation(t *testing.T) {
	stateRoot, record := abandonRetirementRecord(t)

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
	if err == nil || !containsString(result.Missing, "remote_artifact_unmerged") {
		t.Fatalf("an unobserved artifact must keep abandon fail-closed: %v %v", err, result.Missing)
	}
}

// artifact가 아예 없으면 finish는 remote_artifact_merged를 만족할 수 없다.
// 그 레코드의 유일한 은퇴 경로가 abandon이므로 phase가 done이어도 열려야 한다.
func TestAbandonAllowsDoneCycleWithoutArtifact(t *testing.T) {
	stateRoot, record := abandonRetirementRecord(t)
	record.RemoteArtifact = nil
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
	if err != nil {
		t.Fatalf("a done cycle without any artifact must retire through abandon: %v (%v)", err, result.Missing)
	}
}

// abandonRetirementRecord는 phase=done이고 미머지 artifact를 가진 사이클을 만든다.
// 로컬 잔여물은 없으므로 이 이슈가 다루는 두 게이트만 판정에 남는다.
func abandonRetirementRecord(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot, record := abandonTestRecord(t)
	record.Phase = IssueOpsPhaseDone
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr",
		URL:        "https://github.com/example/agent-harness/pull/241",
		VerifiedAt: "2026-08-02T15:54:07Z",
	}
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}
