package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// ancestryRemoteBranchGit은 base 조상 관계까지 흉내낸다. 실제 게이트는
// `git merge-base --is-ancestor <tip> <base>`의 종료 코드로 판정한다.
type ancestryRemoteBranchGit struct {
	*fakeRemoteBranchGit
	// ancestors는 base(refs/remotes/origin/main)의 조상으로 인정할 OID 집합이다.
	ancestors map[string]bool
	// mergeBaseFail은 조회 자체가 실패하는 상황을 만든다.
	mergeBaseFail bool
	mergeBaseArgs []string
}

func (g *ancestryRemoteBranchGit) run(ctx context.Context, dir string, args ...string) (int, string) {
	if args[0] == "merge-base" {
		g.mergeBaseArgs = append([]string{}, args...)
		if g.mergeBaseFail {
			return 128, "fatal: Not a valid object name"
		}
		for _, arg := range args {
			if g.ancestors[arg] {
				return 0, ""
			}
		}
		return 1, ""
	}
	return g.fakeRemoteBranchGit.run(ctx, dir, args...)
}

func ancestryGit() *ancestryRemoteBranchGit {
	return &ancestryRemoteBranchGit{
		fakeRemoteBranchGit: remoteBranchGit(),
		ancestors:           map[string]bool{},
	}
}

func ancestryDeps(git *ancestryRemoteBranchGit) CleanupRemoteBranchDeps {
	deps := remoteBranchDeps(git.fakeRemoteBranchGit)
	deps.Git = git.run
	return deps
}

// 한 사이클이 PR을 두 개 낳으면 레코드의 단일 아티팩트에는 첫 PR의 head만 담긴다.
// 두 번째 PR이 머지되어 그 커밋이 이미 base에 있어도, OID CAS만 보는 게이트는
// 영구히 차단한다. 그리고 lease가 released라 아티팩트 갱신 경로도 닫혀 있어
// 하네스 안에서 회복할 수 없다 — #149에서 실측하고 하네스 밖에서 우회했다.
//
// 원격 tip이 base의 조상이면 잃을 커밋이 없다. 그것이 이 게이트가 지키려는
// 실제 조건이다.
func TestRemoteBranchDeleteAcceptsTipAlreadyInBase(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := ancestryGit()
	// 두 번째 PR로 머지된 커밋이 원격 tip이고, 그것이 base에 도달해 있다.
	git.remoteOID = remoteBranchTestPushedOID
	git.ancestors[remoteBranchTestPushedOID] = true

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), ancestryDeps(git))
	if err != nil {
		t.Fatalf("base에 이미 도달한 tip은 삭제를 막을 이유가 없다: %v (missing=%v)", err, result.Missing)
	}
	if result.Fingerprint == "" {
		t.Fatalf("통과한 preview는 fingerprint를 발급해야 한다: %+v", result)
	}
	// 판정 근거가 무엇이었는지 결과에 남아야 한다(#154 계약).
	if !result.RemoteTipReachedBase {
		t.Fatalf("ancestry로 통과했다면 그 사실을 밝혀야 한다: %+v", result)
	}
}

// ancestry 경로는 OID CAS를 대체하지 않는다. tip이 base에 없으면 여전히 막는다 —
// 그것이 이 게이트가 막으려는 실제 손해(머지되지 않은 커밋 유실)다.
func TestRemoteBranchDeleteStillBlocksTipOutsideBase(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := ancestryGit()
	git.remoteOID = remoteBranchTestPushedOID // base에 도달하지 않은 커밋

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), ancestryDeps(git))
	if err == nil || !containsString(result.Missing, "remote_tip_equals_merged_head") {
		t.Fatalf("머지되지 않은 커밋이 남은 브랜치는 여전히 차단해야 한다: %v %v", err, result.Missing)
	}
	if result.RemoteTipReachedBase {
		t.Fatal("ancestry가 성립하지 않았는데 도달했다고 보고하면 안 된다")
	}
}

// ancestry 조회 자체가 실패하면 새 경로를 성립시키지 않는다. 관측하지 못한 것을
// 통과 근거로 쓰면 커밋을 잃는다.
func TestRemoteBranchDeleteFailsClosedWhenAncestryIsUnobservable(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := ancestryGit()
	git.remoteOID = remoteBranchTestPushedOID
	git.ancestors[remoteBranchTestPushedOID] = true
	git.mergeBaseFail = true

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), ancestryDeps(git))
	if err == nil || !containsString(result.Missing, "remote_tip_equals_merged_head") {
		t.Fatalf("관측 불가는 통과가 아니다: %v %v", err, result.Missing)
	}
}

// OID가 일치하는 기존 경로는 ancestry를 조회하지 않는다. squash 머지에서 원본
// 커밋은 base의 조상이 아니므로, 그 경로가 ancestry에 의존하게 되면 squash된
// 브랜치를 영구히 못 지운다 — 주석의 brooks B3 기각이 지키려던 것이다.
func TestRemoteBranchDeleteKeepsOIDPathIndependentOfAncestry(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := ancestryGit()
	// remoteOID는 아티팩트 head와 같고(기본값), base 조상은 아니다 — squash 머지.

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), ancestryDeps(git))
	if err != nil {
		t.Fatalf("OID 일치는 종전대로 통과해야 한다: %v (missing=%v)", err, result.Missing)
	}
	if len(git.mergeBaseArgs) != 0 {
		t.Fatalf("OID가 일치하면 ancestry를 물을 필요가 없다: %v", git.mergeBaseArgs)
	}
	if result.RemoteTipReachedBase {
		t.Fatal("OID 경로로 통과한 것을 ancestry 통과로 보고하면 근거가 흐려진다")
	}
}

// base 브랜치를 모르면 무엇과 비교할지 정할 수 없다. 추측하지 않고 기존 판정을
// 남긴다.
func TestRemoteBranchDeleteSkipsAncestryWithoutPreparedBase(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.BranchPrepare.BaseBranch = ""
	})
	git := ancestryGit()
	git.remoteOID = remoteBranchTestPushedOID
	git.ancestors[remoteBranchTestPushedOID] = true

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), ancestryDeps(git))
	if err == nil || !containsString(result.Missing, "remote_tip_equals_merged_head") {
		t.Fatalf("base를 모르면 새 경로를 시도하지 않는다: %v %v", err, result.Missing)
	}
	if len(git.mergeBaseArgs) != 0 {
		t.Fatalf("base 이름 없이 ancestry를 묻지 않는다: %v", git.mergeBaseArgs)
	}
}

// ancestry는 준비된 base의 remote-tracking ref와 비교한다. 로컬 브랜치나 다른
// ref를 보면 로컬 상태에 따라 판정이 흔들린다.
func TestRemoteBranchAncestryComparesAgainstRemoteTrackingBase(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := ancestryGit()
	git.remoteOID = remoteBranchTestPushedOID
	git.ancestors[remoteBranchTestPushedOID] = true

	if _, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), ancestryDeps(git)); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(git.mergeBaseArgs, " ")
	for _, want := range []string{"--is-ancestor", remoteBranchTestPushedOID, "refs/remotes/origin/main"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ancestry 질의 %q가 %q를 포함해야 한다", joined, want)
		}
	}
}
