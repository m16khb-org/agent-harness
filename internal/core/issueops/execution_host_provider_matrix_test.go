package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/preflight"
)

// GitLab 사이클이 Orca를 쓸 수 없는 이유는 `gitlab_issue_metadata_unsupported`다.
// #152가 `orca_branch_name_taken` 폴백을 더했지만 그것은 GitLab보다 아래에 있으므로
// 도달하지 않아야 한다 — **어느 코드로 폴백하는지가 사용자가 할 일을 정한다.** 전자는
// Orca가 GitLab 메타데이터를 봉인하지 못하는 것이고 후자는 브랜치 이름을 비우면 되는
// 것이다.
//
// 기존 `TestExecutionGitLabOrcaCapabilityFailsBeforeProbeOrMutation`은 브랜치 이름이
// 비어 있는 조합을 본다(`orcaPrepareRecord`가 원격 ref를 지운다). IssueOps 정식 순서를
// 따른 GitLab 사이클 — 브랜치가 원격에 이미 있는 상태 — 는 미검증이었다(#164).
func TestGitLabCapabilityOutranksBranchConflictRegardlessOfBranchState(t *testing.T) {
	for _, mode := range []string{ExecutionModeAuto, "orca"} {
		t.Run(mode, func(t *testing.T) {
			// 기본 픽스처는 원격 브랜치를 만들어 둔다 — 정식 순서의 상태다.
			stateRoot, record := executionPrepareRecord(t)
			record.BranchPrepare.Provider = "gitlab"
			record.BranchPrepare.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/69"
			if _, err := writeIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			orca := readyOrcaFake()

			got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: mode, CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "matrix-session"), OwnerHost: "claude",
			}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})

			if mode == ExecutionModeAuto {
				if err != nil {
					t.Fatalf("auto는 GitLab에서 direct로 폴백해야 한다: %v", err)
				}
				if got.FallbackCode != "gitlab_issue_metadata_unsupported" {
					t.Fatalf("브랜치가 있어도 GitLab 사유가 먼저다. 브랜치 충돌 코드가 나오면 사용자가 엉뚱한 조치를 한다: %q", got.FallbackCode)
				}
				return
			}
			if err == nil {
				t.Fatal("명시적 Orca는 GitLab에서 실패해야 한다")
			}
			if !strings.Contains(err.Error(), "gitlab_issue_metadata_unsupported") {
				t.Fatalf("오류 %q가 GitLab 사유를 지목해야 한다. 브랜치 충돌로 보고하면 원인이 뒤바뀐다", err)
			}
			if orca.probeCalls != 0 {
				t.Fatalf("GitLab은 Orca probe 이전에 막혀야 한다: probeCalls=%d", orca.probeCalls)
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

// GitLab 원격 브랜치 이름 규칙도 같은 검사를 받는다. `ensureOrcaBranchIsFree`가
// remote-tracking ref를 보는데 그 ref 형태는 provider와 무관하다.
func TestOrcaBranchPrecheckSeesRemoteRefRegardlessOfProvider(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			stateRoot, record := orcaPrepareRecord(t)
			mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
				rec.BranchPrepare.Provider = provider
			})
			head := strings.TrimSpace(preflight.GitOut(record.Repo, "rev-parse", "HEAD"))
			if code, _, stderr := preflight.GitCmd(record.Repo, "update-ref",
				"refs/remotes/origin/"+record.Branch, head); code != 0 {
				t.Fatalf("원격 ref 픽스처: %s", stderr)
			}

			current, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := ensureOrcaBranchIsFree(current, current.Branch); err == nil {
				t.Fatalf("%s: 원격 전용 브랜치도 이름 충돌이다", provider)
			} else if !strings.Contains(err.Error(), "on origin") {
				t.Fatalf("%s: 오류 %q가 원격을 지목해야 한다", provider, err)
			}
		})
	}
}
