package issueops

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

func keepRemoteRequest(id string, apply bool, fingerprint string) CleanupFinishRequest {
	req := finishRequest(id, apply, fingerprint)
	req.KeepRemoteBranch = true
	return req
}

// 원격 브랜치를 지울 수도(cleanup remote-branch 게이트 ⑩) 남길 수도
// (remote_branch_absent) 없으면 사이클은 하네스 안에서 끝나지 않는다. 남기는
// 것은 명시적 선택으로만 열리고, finish는 레코드를 지우므로 무엇을 남겼는지가
// 결과와 감사 라인 양쪽에 남아야 그 브랜치를 다시 찾을 수 있다.
func TestCleanupFinishKeepsSurvivingRemoteBranchWhenRequested(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123", remoteBranchOID: "f00dcafe"}
	deps := finishDeps(git)
	audit := ""
	deps.ReflectAudit = func(_ issueops.IssueOpsRecord, _ portCompletionSection, line string) error {
		audit = line
		return nil
	}

	preview, err := CleanupFinish(context.Background(), stateRoot, keepRemoteRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("명시적으로 남기기로 한 원격 브랜치는 finish를 막지 않는다: %v %v", err, preview.Missing)
	}
	kept := preview.KeptRemoteBranch
	if kept == nil || kept.Branch != "80-finish" || kept.RemoteOID != "f00dcafe" ||
		kept.State != issueops.CleanupKeptRemoteBranchPresent {
		t.Fatalf("preview는 무엇이 남는지 관측해 보고해야 한다: %+v", kept)
	}

	result, err := CleanupFinish(context.Background(), stateRoot, keepRemoteRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RecordDeleted {
		t.Fatalf("사이클은 끝나야 한다: %+v", result)
	}
	if result.KeptRemoteBranch == nil || result.KeptRemoteBranch.RemoteOID != "f00dcafe" {
		t.Fatalf("apply 결과도 남긴 브랜치를 담아야 한다: %+v", result.KeptRemoteBranch)
	}
	if !strings.Contains(audit, "remote_branch_kept=") || !strings.Contains(audit, "f00dcafe") {
		t.Fatalf("감사 라인은 레코드 삭제 뒤 남는 유일한 흔적이다: %q", audit)
	}
}

// 원격을 읽지 못하는 상태도 같은 교착을 만든다. origin이 사라졌거나 프로젝트가
// 옮겨지면 ls-remote는 영원히 실패하고, 잔존 여부를 확정할 수 없다는 이유로
// 사이클이 갇힌다. 남기기로 한 이상 잔존과 같게 다루되 그 사실을 구분해 적는다.
func TestCleanupFinishKeepsRemoteBranchWhenRemoteIsUnreadable(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123", lsRemoteFail: true}

	preview, err := CleanupFinish(context.Background(), stateRoot, keepRemoteRequest(record.ID, false, ""), finishDeps(git))
	if err != nil {
		t.Fatalf("관측 불가는 남기기 선택을 막지 않는다: %v %v", err, preview.Missing)
	}
	kept := preview.KeptRemoteBranch
	if kept == nil || kept.State != issueops.CleanupKeptRemoteBranchUnreadable || kept.RemoteOID != "" {
		t.Fatalf("관측하지 못한 것을 관측한 것처럼 적으면 안 된다: %+v", kept)
	}
}

// 남긴 것이 없으면 흔적도 없어야 한다. 플래그를 습관적으로 붙였다는 이유로
// 존재하지 않는 원격 브랜치를 이슈 본문에 기록하면 그 기록이 거짓이 된다.
func TestCleanupFinishReportsNothingKeptWhenRemoteBranchAlreadyAbsent(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}

	preview, err := CleanupFinish(context.Background(), stateRoot, keepRemoteRequest(record.ID, false, ""), finishDeps(git))
	if err != nil {
		t.Fatal(err)
	}
	if preview.KeptRemoteBranch != nil {
		t.Fatalf("남긴 것이 없으면 nil이다: %+v", preview.KeptRemoteBranch)
	}
}
