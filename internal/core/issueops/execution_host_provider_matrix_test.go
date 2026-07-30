package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/preflight"
)

// GitLab branch prepare가 만든 원격 브랜치가 봉인된 base SHA 그대로면 새 작업이
// 없는 예약 브랜치다. Orca adapter가 생성 후 그 이름으로 정규화할 수 있으므로
// auto와 명시적 Orca 모두 같은 모드를 선택해야 한다.
func TestGitLabPreparedRemoteBranchAtSealedBaseUsesOrca(t *testing.T) {
	for _, mode := range []string{ExecutionModeAuto, "orca"} {
		t.Run(mode, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			record.BranchPrepare.Provider = "gitlab"
			record.BranchPrepare.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/69"
			record.IssueURL = record.BranchPrepare.IssueURL
			if _, err := writeIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			orca := readyOrcaFake()

			got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: mode, CWD: record.Repo,
				Actor: executionActor("codex", "matrix-session"), OwnerHost: "claude",
			}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})

			if err != nil || got.ResolvedMode != "orca" || got.FallbackCode != "" {
				t.Fatalf("GitLab 예약 브랜치는 Orca로 준비 가능해야 한다: result=%+v err=%v", got, err)
			}
			if orca.probeCalls != 1 || orca.prepareCalls != 0 {
				t.Fatalf("preview는 probe만 한 번 실행해야 한다: probe=%d prepare=%d", orca.probeCalls, orca.prepareCalls)
			}
		})
	}
}

// auto 폴백은 owner host와 무관하게 같아야 한다. #152 변경이
// resolveExecutionPrepareMode 안에 있고 그 함수는 host를 검사하므로(codex·claude만
// 허용), host별 동작 차이가 생기면 여기서 걸린다.
func TestAutoBranchFallbackIsIdenticalAcrossFirstPartyHosts(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			orca := readyOrcaFake()

			got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "matrix-session"), OwnerHost: host,
			}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
			if err != nil {
				t.Fatalf("%s host의 auto도 실행 가능한 모드를 골라야 한다: %v", host, err)
			}
			if got.ResolvedMode != "direct" || got.FallbackCode != "orca_branch_name_taken" {
				t.Fatalf("%s host에서 폴백 결과가 달라졌다: %+v", host, got)
			}
			if orca.prepareCalls != 0 {
				t.Fatalf("%s host에서 mutation이 일어났다: prepareCalls=%d", host, orca.prepareCalls)
			}
		})
	}
}

// 브랜치 사전 확인의 차단도 host와 무관하다. 명시적 Orca는 어느 host에서도 실패한다.
func TestExplicitOrcaBranchConflictIsIdenticalAcrossFirstPartyHosts(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			createLocalBranch(t, record.Repo, record.Branch)
			orca := readyOrcaFake()

			_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "matrix-session"), OwnerHost: host,
			}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
			if err == nil {
				t.Fatalf("%s host에서도 명시적 Orca는 브랜치 충돌로 실패해야 한다", host)
			}
			if !strings.Contains(err.Error(), record.Branch) {
				t.Fatalf("%s host 오류 %q가 충돌 브랜치를 지목해야 한다", host, err)
			}
			if orca.prepareCalls != 0 {
				t.Fatalf("%s host에서 mutation이 일어났다: prepareCalls=%d", host, orca.prepareCalls)
			}
		})
	}
}

// #153의 ancestry 경로는 git 연산이라 provider와 무관해야 한다. GitLab 아티팩트를
// 가진 사이클에서도 같은 판정이 나오는지 확인한다 — provider별로 갈리면 GitLab
// 사이클의 cleanup이 막힌다.
func TestRemoteBranchAncestryIsProviderNeutral(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			stateRoot, record := remoteBranchTestRecord(t)
			mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
				rec.BranchPrepare.Provider = provider
				rec.RemoteArtifact.Provider = provider
				if provider == "gitlab" {
					rec.RemoteArtifact.Kind = "mr"
					rec.RemoteArtifact.URL = "https://gitlab.example.com/acme/repo/-/merge_requests/117"
					rec.IssueURL = "https://gitlab.example.com/acme/repo/-/issues/116"
					rec.BranchPrepare.IssueURL = rec.IssueURL
				}
			})
			git := ancestryGit()
			git.remoteOID = remoteBranchTestPushedOID
			git.ancestors[remoteBranchTestPushedOID] = true
			if provider == "gitlab" {
				git.originURL = "https://gitlab.example.com/acme/repo.git"
			}

			result, err := CleanupRemoteBranch(context.Background(), stateRoot,
				remoteBranchRequest(record.ID, false, ""), ancestryDeps(git))
			if err != nil {
				t.Fatalf("%s: base에 도달한 tip은 provider와 무관하게 통과해야 한다: %v (missing=%v)", provider, err, result.Missing)
			}
			if !result.RemoteTipReachedBase {
				t.Fatalf("%s: ancestry 통과 근거가 밝혀져야 한다: %+v", provider, result)
			}
		})
	}
}

// GitLab 예외는 정확히 봉인된 base의 예약 브랜치에만 적용한다. 다른 SHA면 이미
// 작업이 들어간 브랜치일 수 있으므로 provider와 무관하게 fail-closed다.
func TestOrcaBranchPrecheckOnlyAllowsExactGitLabPreparedRemote(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	record.BranchPrepare.Provider = "gitlab"
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureOrcaBranchIsFree(current, current.Branch); err != nil {
		t.Fatalf("봉인된 base와 같은 GitLab 예약 브랜치는 허용해야 한다: %v", err)
	}

	if code, _, stderr := preflight.GitCmd(record.Repo, "commit", "--allow-empty", "-m", "diverged"); code != 0 {
		t.Fatalf("분기 SHA 픽스처: %s", stderr)
	}
	diverged := strings.TrimSpace(preflight.GitOut(record.Repo, "rev-parse", "HEAD"))
	if code, _, stderr := preflight.GitCmd(record.Repo, "update-ref", "refs/remotes/origin/"+record.Branch, diverged); code != 0 {
		t.Fatalf("분기 원격 ref 픽스처: %s", stderr)
	}
	if err := ensureOrcaBranchIsFree(current, current.Branch); err == nil {
		t.Fatal("봉인된 base와 다른 GitLab 원격 브랜치는 차단해야 한다")
	} else if !strings.Contains(err.Error(), "on origin") {
		t.Fatalf("오류 %q가 원격 충돌을 지목해야 한다", err)
	}
}
