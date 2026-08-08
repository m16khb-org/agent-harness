package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	linkedbranch "agent-harness/internal/domain/issueopslinkedbranch"
)

// AwaitBranchLinkDeps는 외부 표면 주입점이다. 이 경로는 **읽기 전용**이다 —
// 어떤 분기에서도 provider나 git에 쓰지 않는다.
type AwaitBranchLinkDeps struct {
	Git                   func(ctx context.Context, dir string, args ...string) (int, string)
	ObserveLinkedBranches func(ctx context.Context, issueURL string) (linkedbranch.Observation, error)
	// Sleep과 Now는 테스트가 실제 시간을 기다리지 않게 하는 주입점이다.
	Sleep func(ctx context.Context, d time.Duration) error
	Now   func() time.Time
}

const (
	// awaitBranchLinkInterval은 재관측 간격이다. provider readback은 네트워크
	// 호출이므로 촘촘히 돌면 rate limit에 닿는다.
	awaitBranchLinkInterval = 15 * time.Second
	// awaitBranchLinkDefaultTimeout은 coordinator가 createLinkedBranch를
	// 수행하는 데 걸릴 만한 시간이다. 무한 대기는 owner를 영원히 붙잡는다.
	awaitBranchLinkDefaultTimeout = 10 * time.Minute
	// awaitBranchLinkMaxTimeout은 요청할 수 있는 상한이다. 이보다 오래
	// 기다려야 한다면 그것은 대기 문제가 아니라 coordinator가 멈춘 것이다.
	awaitBranchLinkMaxTimeout = 30 * time.Minute
)

// AwaitBranchLink는 coordinator가 만들 linked branch가 나타날 때까지 경계
// 있게 기다린다(#319).
//
// GitHub Orca 경로에는 구조적인 시작 창이 있다. `execution prepare --mode orca`는
// link 미검증 상태에서 owner를 띄우고(PrepareIssueIdentity의 GitHub 예외),
// 문서화된 순서상 coordinator의 createLinkedBranch는 그 **뒤**에 온다. Orca가
// 항상 새 branch를 만들기 때문에 원격 branch가 먼저 있으면 prepare 자체가
// 실패하므로, 이 순서는 뒤집을 수 없다.
//
// 그래서 owner가 시작 시점에 링크를 **한 번** 읽으면 답은 항상 "아직 없음"이고,
// 그것을 terminal 실패로 다루면 GitHub Orca dogfood는 구조적으로 완주할 수 없다.
// 실측된 실패가 정확히 그것이었다(lifecycle io-b3d92dc6247a, task_e3946ef93086:
// "branch_link_verification_required ... 구현 파일을 한 줄도 수정하지 않았습니다").
//
// 대기를 프롬프트 지시가 아니라 도구에 두는 이유는, 이것이 안전 계약의 일부이기
// 때문이다. "몇 번 다시 읽어라"를 에이전트의 규율에 맡기면 경계가 관측되지
// 않는다. 여기서는 간격도 상한도 값으로 고정된다.
func AwaitBranchLink(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest, deps AwaitBranchLinkDeps) (issueopscontract.AwaitBranchLinkResult, error) {
	if deps.Git == nil {
		deps.Git = defaultExecutionSyncBaseGit
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Sleep == nil {
		deps.Sleep = sleepWithContext
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return issueopscontract.AwaitBranchLinkResult{OK: false, ID: req.ID}, err
	}
	result := issueopscontract.AwaitBranchLinkResult{OK: true, ID: record.ID}

	timeout, err := awaitBranchLinkTimeout(req.Timeout)
	if err != nil {
		result.OK = false
		return result, err
	}
	result.TimeoutSeconds = int(timeout / time.Second)

	if missing := awaitBranchLinkGates(record, deps); len(missing) > 0 {
		result.OK, result.Missing = false, missing
		return result, fmt.Errorf("branch await-link is not ready: %s", strings.Join(missing, ", "))
	}
	prepare := record.BranchPrepare
	result.IssueURL, result.Branch, result.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA

	// 이미 기록돼 있으면 기다리지 않는다. 멱등 성공이다.
	if prepare.LinkVerified {
		result.AlreadyVerified, result.Linked = true, true
		return result, nil
	}

	deadline := deps.Now().Add(timeout)
	for {
		state, node, reason := awaitBranchLinkObserve(ctx, record, deps)
		result.Attempts++
		result.State, result.StateReason = string(state), reason
		if state == linkedbranch.StateHealthy {
			result.Linked, result.LinkedBranchID, result.ObservedOID = true, node.ID, node.RefOID
			result.NextCommand = awaitBranchLinkNextCommand(record.ID)
			return result, nil
		}
		// mismatched는 기다린다고 낫지 않는다. 링크는 있는데 봉인값과 다르므로
		// 사람이 봐야 한다 — 기다리면 진단만 늦어진다.
		if state == linkedbranch.StateMismatched {
			result.OK = false
			return result, fmt.Errorf("linked branch does not match the sealed identity: %s", reason)
		}
		if !deps.Now().Before(deadline) {
			result.OK, result.TimedOut = false, true
			return result, fmt.Errorf(
				"the coordinator's linked branch for %s did not appear within %s (last state: %s); "+
					"the coordinator must create it at the sealed base %s",
				prepare.Branch, timeout, state, prepare.BaseSHA)
		}
		if err := deps.Sleep(ctx, awaitBranchLinkInterval); err != nil {
			result.OK = false
			return result, err
		}
	}
}

// awaitBranchLinkObserve는 issue 링크와 원격 ref를 같은 시점에 읽어 분류한다.
// #306의 분류기를 그대로 쓴다 — "정상 링크"의 정의가 두 곳에서 갈리면 한쪽이
// 통과시킨 상태를 다른 쪽이 거부하는 교착이 다시 생긴다.
func awaitBranchLinkObserve(ctx context.Context, record issueopscontract.IssueOpsRecord, deps AwaitBranchLinkDeps) (linkedbranch.State, linkedbranch.Node, string) {
	prepare := record.BranchPrepare
	observation, err := deps.ObserveLinkedBranches(ctx, prepare.IssueURL)
	if err != nil {
		// 관측 실패는 부재가 아니다. 다음 주기에 다시 읽는다.
		return linkedbranch.StateAmbiguous, linkedbranch.Node{}, "linked branch readback failed: " + err.Error()
	}
	observation.IssueURL, observation.RequestedBranch, observation.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA
	observation.RemoteOID = cleanupLinkedBranchRemoteOID(ctx, record.Repo, prepare.Branch, CleanupLinkedBranchDeps{Git: deps.Git})
	return linkedbranch.Classify(observation)
}

func awaitBranchLinkGates(record issueopscontract.IssueOpsRecord, deps AwaitBranchLinkDeps) []string {
	var missing []string
	if deps.ObserveLinkedBranches == nil {
		missing = append(missing, "linked_branch_observation_unavailable")
	}
	prepare := record.BranchPrepare
	if prepare == nil {
		return append(missing, "branch_prepare_missing")
	}
	if prepare.Provider != "github" {
		// GitLab은 prepare 시점에 link_verified를 이미 요구하므로 이 창이 없다.
		missing = append(missing, "branch_await_link_is_github_only")
	}
	if strings.TrimSpace(prepare.IssueURL) == "" {
		missing = append(missing, "issue_url_missing")
	}
	if strings.TrimSpace(prepare.Branch) == "" {
		missing = append(missing, "branch_missing")
	}
	if strings.TrimSpace(prepare.BaseSHA) == "" {
		missing = append(missing, "sealed_base_missing")
	}
	return missing
}

func awaitBranchLinkTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return awaitBranchLinkDefaultTimeout, nil
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("branch await-link --timeout is not a duration: %q", raw)
	}
	if timeout <= 0 || timeout > awaitBranchLinkMaxTimeout {
		return 0, fmt.Errorf("branch await-link --timeout must be between 0 and %s, got %s", awaitBranchLinkMaxTimeout, timeout)
	}
	return timeout, nil
}

// awaitBranchLinkNextCommand는 링크가 확인된 뒤 owner가 실행할 명령을 가리킨다.
// 값 자체는 owner packet이 봉인해 두므로 여기서는 그 자리를 지목만 한다.
func awaitBranchLinkNextCommand(id string) string {
	return "record the link with the sealed verify_branch_link command from the owner packet for " + id
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
