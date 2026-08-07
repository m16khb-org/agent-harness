package lifecycle

import (
	"fmt"
	"os"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"

	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/remote"
	"agent-harness/internal/core/remoteartifact"
)

// sealedIssueEditBlockReason은 봉인된 owner context에 묶인 이슈의 본문 편집을
// 선제 차단한다.
//
// 이 가드가 없으면 봉인 이후의 본문 개정이 훅을 통과하고, digest drift는 owner가
// claim할 때에야 발견된다. 그 시점에는 owner가 이미 낡은 컨텍스트를 들고 대기
// 중이며 원인은 다른 세션의 편집이다.
//
// 원격 조회는 하지 않는다: 훅은 로컬 durable 레코드만 읽어 빠르고 결정적이어야
// 한다. 대상 식별자를 해석할 수 없으면 통과시킨다 — 이 가드의 목적은 봉인
// 보호이며, 미지의 명령 형태를 fail-closed로 막으면 봉인과 무관한 일상 작업이
// 깨진다.
func sealedIssueEditBlockReason(req lifecyclecontract.HookToolUseLifecycleRequest) string {
	target, ok := remoteartifact.IssueEditTargetFromCommand(req.Tool, req.Command, req.Repo)
	if !ok {
		return ""
	}
	ids, err := issueopscore.ListIssueOpsIDs(IssueOpsStateRoot())
	if err != nil {
		// 레코드를 못 읽는 상태에서 편집을 막으면 복구 작업까지 봉쇄된다.
		// 봉인 보호는 최선 노력 가드이며 claim 시점 검증이 최종 방어선이다.
		return ""
	}
	for _, id := range ids {
		record, readErr := ReadIssueOps(IssueOpsStateRoot(), id)
		if readErr != nil {
			continue
		}
		if !sealedIssueEditRecordProtects(record, target) {
			continue
		}
		return fmt.Sprintf(
			"issue %s is sealed into the owner context of IssueOps lifecycle %s (generation %d); editing its body would drift the sealed digest and permanently reject the owner claim. If the revision is intended, reseal first with `agent-harness issueops execution replace --id %s --expected-generation %d --reseed` and then edit",
			target, record.ID, record.Execution.Lease.Generation, record.ID, record.Execution.Lease.Generation,
		)
	}
	return ""
}

// sealedIssueEditRecordProtects는 세 조건을 모두 요구한다: orca 모드의 활성
// 세대이고, 그 세대의 packet 파일이 실제로 존재하며, 연결된 이슈가 편집 대상과
// 같다. packet 실존을 요구하는 이유는 봉인 전 단계의 orca 사이클에서 편집이
// 막히지 않게 하기 위함이다.
func sealedIssueEditRecordProtects(record issueopscontract.IssueOpsRecord, target string) bool {
	if record.Execution == nil || record.Execution.Mode != issueopscontract.ExecutionModeOrca {
		return false
	}
	switch record.Execution.Lease.Status {
	case issueopscontract.LeaseStatusClaimable, issueopscontract.LeaseStatusActive, issueopscontract.LeaseStatusRevoking:
	default:
		return false
	}
	if !sameSealedIssueTarget(record.IssueURL, target) {
		return false
	}
	packet := issueopscore.SealedOwnerContextPacketPath(record)
	if strings.TrimSpace(packet) == "" {
		return false
	}
	info, err := os.Lstat(packet)
	return err == nil && info.Mode().IsRegular()
}

// sameSealedIssueTarget는 번호와 URL 두 형태를 같은 이슈로 인정한다. 번호만
// 비교할 때 저장소 경계를 넘지 않도록, 훅 요청의 repo에 속한 레코드만 스캔
// 대상이 되는 상위 흐름에 의존한다.
func sameSealedIssueTarget(issueURL, target string) bool {
	issueURL, target = strings.TrimSpace(issueURL), strings.TrimSpace(target)
	if issueURL == "" || target == "" {
		return false
	}
	if strings.EqualFold(issueURL, target) {
		return true
	}
	number := strings.TrimSpace(remote.IssueNumber(issueURL))
	return number != "" && number == target
}
