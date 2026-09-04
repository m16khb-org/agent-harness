package issueops

import (
	"context"
	"fmt"
	"strings"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// 폐기의 원격 효과다. 미머지 사이클의 원격 정리는 이 경로만 할 수 있다 —
// `remote close-issue`와 `cleanup remote-branch`는 머지 증적을 하드 게이트로
// 요구하므로 폐기에는 쓸 수 없다.
const (
	cleanupAbandonEffectClosePR            = "close_pr"
	cleanupAbandonEffectCloseIssue         = "close_issue"
	cleanupAbandonEffectRemoteBranchDelete = "remote_branch_delete"
	cleanupAbandonEffectAlreadyClosed      = ":already_closed"
	cleanupAbandonEffectAbsent             = ":absent"
)

func cleanupAbandonRemoteRequested(req CleanupAbandonRequest) bool {
	return req.ClosePR || req.CloseIssue || req.DeleteRemoteBranch
}

// cleanupAbandonObserveRemote는 요청된 효과에 대해서만 원격을 읽는다. 플래그가
// 없으면 provider를 호출하지 않으므로 기존 폐기의 동작과 출력이 그대로다.
//
// 관측한 상태는 fingerprint에 넣지 않는다. 일시적 원격 오류가 preview 재발급
// 루프를 만들기 때문이며, 이는 ArtifactUnmerged가 이미 지키는 규율이다. 대신
// 삭제 CAS에 필요한 원격 브랜치 OID만 봉인한다(`cleanup remote-branch` 선례).
func cleanupAbandonObserveRemote(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	req CleanupAbandonRequest,
	deps CleanupAbandonDeps,
	inventory cleanupAbandonInventory,
	result *CleanupAbandonResult,
) (cleanupAbandonInventory, []string) {
	if !cleanupAbandonRemoteRequested(req) {
		return inventory, nil
	}
	inventory.ClosePR, inventory.CloseIssue, inventory.DeleteRemoteBranch =
		req.ClosePR, req.CloseIssue, req.DeleteRemoteBranch
	missing := []string{}
	effects := []string{}
	if req.ClosePR {
		switch {
		case record.RemoteArtifact == nil:
			missing = append(missing, "remote_artifact_required")
		case deps.Remote == nil:
			missing = append(missing, "remote_provider_unavailable")
		default:
			state, err := cleanupAbandonArtifactState(ctx, deps.Remote, record.Repo,
				strings.TrimSpace(record.RemoteArtifact.Kind), strings.TrimSpace(record.RemoteArtifact.URL))
			if err != nil {
				missing = append(missing, "remote_artifact_readable")
			} else {
				result.RemoteArtifactState = state
				switch {
				case cleanupAbandonStateIs(state, "merged"):
					// 게이트 ④와 같은 판정이다. 머지된 아티팩트의 정답은
					// reflect→finish이지 폐기가 아니다.
					missing = append(missing, "remote_artifact_unmerged")
				case cleanupAbandonStateIs(state, "closed"):
					effects = append(effects, cleanupAbandonEffectClosePR+cleanupAbandonEffectAlreadyClosed)
				default:
					effects = append(effects, cleanupAbandonEffectClosePR)
				}
			}
		}
	}
	if req.CloseIssue {
		switch {
		case strings.TrimSpace(record.IssueURL) == "":
			missing = append(missing, "issue_url_required")
		case deps.Remote == nil:
			missing = append(missing, "remote_provider_unavailable")
		default:
			state, err := cleanupAbandonArtifactState(ctx, deps.Remote, record.Repo, "issue", strings.TrimSpace(record.IssueURL))
			if err != nil {
				missing = append(missing, "issue_readable")
			} else {
				result.IssueState = state
				if cleanupAbandonStateIs(state, "closed") {
					effects = append(effects, cleanupAbandonEffectCloseIssue+cleanupAbandonEffectAlreadyClosed)
				} else {
					effects = append(effects, cleanupAbandonEffectCloseIssue)
				}
			}
		}
	}
	if req.DeleteRemoteBranch {
		switch {
		case inventory.Branch == "":
			missing = append(missing, "branch_recorded")
		case record.BranchPrepare != nil && inventory.Branch == strings.TrimSpace(record.BranchPrepare.BaseBranch):
			missing = append(missing, "branch_not_base")
		default:
			code, out := deps.Git(record.Repo, "ls-remote", "--heads", "origin", "refs/heads/"+inventory.Branch)
			if code != 0 {
				missing = append(missing, "remote_branch_readable")
			} else if fields := strings.Fields(strings.TrimSpace(out)); len(fields) > 0 {
				inventory.RemoteBranchOID = fields[0]
				result.RemoteBranchOID = inventory.RemoteBranchOID
				effects = append(effects, cleanupAbandonEffectRemoteBranchDelete)
			} else {
				effects = append(effects, cleanupAbandonEffectRemoteBranchDelete+cleanupAbandonEffectAbsent)
			}
		}
	}
	// 원격 관측 자체가 결격이면 계획을 비운다 — 실행되지 않을 효과를 나열하면
	// 사용자가 승인한 것과 일어날 일이 어긋난다. 다른 게이트(자식 미해소 등)가
	// 막는 경우에는 관측 결과를 그대로 보여 준다. 그때 `ok:false`와 missing이
	// 이미 "지금은 실행되지 않는다"를 말하고, 관측값은 무엇을 먼저 정리해야
	// 하는지 알려 준다.
	if len(missing) == 0 {
		result.RemoteEffects = effects
	}
	return inventory, missing
}

// cleanupAbandonApplyRemote는 로컬 삭제보다 먼저 실행된다. 여기서 멈추면
// 레코드와 워크트리가 그대로 남아 사람이 다시 결정할 수 있다.
func cleanupAbandonApplyRemote(
	ctx context.Context,
	stateRoot string,
	record issueops.IssueOpsRecord,
	req CleanupAbandonRequest,
	deps CleanupAbandonDeps,
	inventory cleanupAbandonInventory,
	fingerprint string,
	result *CleanupAbandonResult,
) error {
	if !cleanupAbandonRemoteRequested(req) {
		return nil
	}
	applied := []string{}
	fail := func(step string, cause error) error {
		result.OK = false
		result.FailedStep = step
		result.RemoteEffects = applied
		receiptErr := recordCleanupAbandonFailure(stateRoot, record.ID, step, cause, fingerprint, inventory)
		result.NextCommand = cleanupAbandonPreviewCommand(record.ID, result.Reason, req)
		return cleanupAbandonApplyError(
			fmt.Sprintf("cleanup abandon %s failed (record, worktree, and remaining remote state preserved): %v", step, cause), receiptErr)
	}
	if req.ClosePR {
		closer, ok := deps.Remote.(port.IssueProviderPullRequestCloser)
		if !ok {
			return fail(issueops.CleanupFailureStepClosePR, fmt.Errorf("provider does not support closing a pull request"))
		}
		closed, err := closer.ClosePullRequest(port.IssueProviderClosePullRequestRequest{
			Repo: record.Repo, ArtifactURL: record.RemoteArtifact.URL,
			Kind: strings.TrimSpace(record.RemoteArtifact.Kind), Confirm: true,
		})
		switch {
		case err != nil:
			return fail(issueops.CleanupFailureStepClosePR, err)
		case closed.Merged:
			// preview 이후 머지됐다는 뜻이다. 폐기를 계속하면 머지 증적을 가진
			// 레코드를 지우게 된다.
			return fail(issueops.CleanupFailureStepClosePR,
				fmt.Errorf("pull request was merged after the preview; run reflect-completion and cleanup finish instead"))
		case closed.AlreadyClosed:
			applied = append(applied, cleanupAbandonEffectClosePR+cleanupAbandonEffectAlreadyClosed)
		default:
			applied = append(applied, cleanupAbandonEffectClosePR)
		}
		result.RemoteArtifactState = closed.State
		result.PRClosed = closed.Closed
	}
	if req.CloseIssue {
		closed, err := deps.Remote.CloseIssue(port.IssueProviderCloseIssueRequest{
			Repo: record.Repo, IssueURL: record.IssueURL, Reason: "not_planned", Confirm: true,
		})
		if err != nil {
			return fail(issueops.CleanupFailureStepCloseIssue, err)
		}
		if closed.AlreadyClosed {
			applied = append(applied, cleanupAbandonEffectCloseIssue+cleanupAbandonEffectAlreadyClosed)
		} else {
			applied = append(applied, cleanupAbandonEffectCloseIssue)
		}
		result.IssueState = closed.State
		result.IssueClosed = closed.Closed
	}
	if req.DeleteRemoteBranch {
		if inventory.RemoteBranchOID == "" {
			// 부재가 삭제의 목표 상태다. 멱등 성공으로 정규화한다.
			applied = append(applied, cleanupAbandonEffectRemoteBranchDelete+cleanupAbandonEffectAbsent)
		} else if code, out := deleteRemoteBranchRef(
			func(args ...string) (int, string) { return deps.Git(record.Repo, args...) },
			inventory.Branch, inventory.RemoteBranchOID); code != 0 {
			return fail(issueops.CleanupFailureStepRemoteBranchDelete, fmt.Errorf("%s", strings.TrimSpace(out)))
		} else {
			applied = append(applied, cleanupAbandonEffectRemoteBranchDelete)
			result.RemoteBranchDeleted = true
		}
	}
	result.RemoteEffects = applied
	_ = ctx
	return nil
}

// cleanupAbandonArtifactState는 provider의 body reader로 아티팩트 상태만 읽는다.
// 상태 전용 관측기를 새로 만들지 않는 이유는 두 provider가 이미 같은 readback을
// 이 인터페이스로 노출하기 때문이다.
func cleanupAbandonArtifactState(ctx context.Context, provider port.IssueProvider, repo, kind, url string) (string, error) {
	reader, ok := provider.(port.IssueProviderArtifactBodyReader)
	if !ok {
		return "", fmt.Errorf("provider does not support reading artifact state")
	}
	if strings.TrimSpace(kind) == "" {
		kind = "issue"
	}
	body, err := reader.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{Repo: repo, Kind: kind, URL: url})
	if err != nil {
		return "", err
	}
	return body.State, nil
}

func cleanupAbandonStateIs(state, want string) bool {
	return strings.EqualFold(strings.TrimSpace(state), want)
}

// deleteRemoteBranchRef는 fully-qualified ref와 force-with-lease로 원격 브랜치를
// 지운다. ref 한정은 동명 태그를 배제하고, lease는 관측 이후 push된 커밋이
// 있으면 서버가 거부하게 한다. `cleanup remote-branch`와 abandon이 공유한다.
func deleteRemoteBranchRef(git func(args ...string) (int, string), branch, expectedOID string) (int, string) {
	ref := "refs/heads/" + branch
	return git("push", "origin", "--delete", ref, "--force-with-lease="+ref+":"+expectedOID)
}
