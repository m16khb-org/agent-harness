package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

// remoteBranchAdvancedTipGit는 원격 tip이 기록된 머지 head보다 전진했고 base
// ancestry로도 판정되지 않는 상태를 만든다 — #323이 보고한 그 상황이다.
func remoteBranchAdvancedTipGit() *fakeRemoteBranchGit {
	git := remoteBranchGit()
	git.remoteOID = "f65d110f65d110f65d110f65d110f65d110f65d1"
	return git
}

// TestCleanupRemoteBranchAcceptsAVerifiedSupersedingArtifact는 #323을 고정한다.
// 원격 tip이 기록된 머지 head보다 전진했고 ancestry로도 판정할 수 없을 때,
// 후속 merged artifact의 provider readback이 유일한 근거가 된다.
func TestCleanupRemoteBranchAcceptsAVerifiedSupersedingArtifact(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	deps := remoteBranchDeps(remoteBranchAdvancedTipGit())
	deps.ObserveArtifact = func(url string) (issueopsdomain.ArtifactObservation, error) {
		return issueopsdomain.ArtifactObservation{
			URL: url, Provider: "github", Merged: true, State: "MERGED",
			Body: "Supersedes " + record.RemoteArtifact.URL,
		}, nil
	}
	req := CleanupRemoteBranchRequest{ID: record.ID, SupersededBy: "https://github.com/acme/repo/pull/307"}

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, req, deps)
	if err != nil {
		t.Fatalf("검증된 replacement 증거는 전진한 tip 게이트를 충족해야 한다: %v missing=%v supersedeError=%q",
			err, result.Missing, result.SupersedeError)
	}
	if result.SupersededBy != req.SupersededBy {
		t.Fatalf("무엇을 근거로 통과했는지 결과에 남아야 한다: %+v", result)
	}
}

// TestCleanupRemoteBranchAcceptsSupersedingArtifactForUnmergedOriginal keeps
// remote-branch cleanup aligned with cleanup finish: a closed-unmerged child PR
// may be cleaned when a merged replacement explicitly supersedes it.
func TestCleanupRemoteBranchAcceptsSupersedingArtifactForUnmergedOriginal(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	deps.VerifyMergedArtifact = func(issueops.IssueOpsRemoteArtifactVerification) (issueops.CleanupRemoteBranchArtifactHead, error) {
		return issueops.CleanupRemoteBranchArtifactHead{}, errors.New("remote artifact is not verified merged")
	}
	deps.ObserveArtifact = func(url string) (issueopsdomain.ArtifactObservation, error) {
		return issueopsdomain.ArtifactObservation{
			URL: url, Provider: "github", Merged: true, State: "MERGED",
			Body: "Supersedes " + record.RemoteArtifact.URL,
		}, nil
	}
	replacement := "https://github.com/acme/repo/pull/307"

	result, err := CleanupRemoteBranch(context.Background(), stateRoot, CleanupRemoteBranchRequest{
		ID: record.ID, SupersededBy: replacement,
	}, deps)
	if err != nil {
		t.Fatalf("verified replacement must satisfy an unmerged original artifact gate: %v missing=%v supersedeError=%q",
			err, result.Missing, result.SupersedeError)
	}
	if result.SupersededBy != replacement || result.Fingerprint == "" {
		t.Fatalf("replacement evidence must be fingerprinted: %+v", result)
	}
	wantNext := "issueops cleanup remote-branch --id " + record.ID +
		" --apply --confirm --fingerprint " + result.Fingerprint +
		" --superseded-by '" + replacement + "' --json"
	if result.NextCommand != wantNext {
		t.Fatalf("generated apply command must preserve replacement evidence:\n got: %s\nwant: %s", result.NextCommand, wantNext)
	}

	other := "https://github.com/acme/repo/pull/308"
	otherResult, err := CleanupRemoteBranch(context.Background(), stateRoot, CleanupRemoteBranchRequest{
		ID: record.ID, SupersededBy: other,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if otherResult.Fingerprint == result.Fingerprint {
		t.Fatal("different replacement URLs must produce different fingerprints")
	}

	applied, err := CleanupRemoteBranch(context.Background(), stateRoot, CleanupRemoteBranchRequest{
		ID: record.ID, SupersededBy: replacement, Apply: true, Confirm: true, Fingerprint: result.Fingerprint,
	}, deps)
	if err != nil || !applied.Deleted || git.pushes != 1 {
		t.Fatalf("generated replacement evidence and fingerprint must apply once: err=%v result=%+v pushes=%d", err, applied, git.pushes)
	}
}

// TestCleanupRemoteBranchRejectsUnverifiableSupersedingArtifacts는 전진한 tip을
// 아무 증거로나 지울 수 없음을 고정한다.
func TestCleanupRemoteBranchRejectsUnverifiableSupersedingArtifacts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		observe  func(string) (issueopsdomain.ArtifactObservation, error)
		wantHint string
	}{
		{"증거 없음", nil, ""},
		{
			"관측 실패",
			func(string) (issueopsdomain.ArtifactObservation, error) {
				return issueopsdomain.ArtifactObservation{}, errors.New("gh pr view failed")
			},
			"could not be observed",
		},
		{
			"머지되지 않음",
			func(url string) (issueopsdomain.ArtifactObservation, error) {
				return issueopsdomain.ArtifactObservation{URL: url, Merged: false, State: "OPEN", Body: "Supersedes x"}, nil
			},
			"is not merged",
		},
		{
			"supersede 선언 없음",
			func(url string) (issueopsdomain.ArtifactObservation, error) {
				return issueopsdomain.ArtifactObservation{URL: url, Merged: true, State: "MERGED", Body: "unrelated"}, nil
			},
			"does not declare",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := remoteBranchTestRecord(t)
			deps := remoteBranchDeps(remoteBranchAdvancedTipGit())
			deps.ObserveArtifact = tc.observe
			req := CleanupRemoteBranchRequest{ID: record.ID}
			if tc.name != "증거 없음" {
				req.SupersededBy = "https://github.com/acme/repo/pull/307"
			}

			result, err := CleanupRemoteBranch(context.Background(), stateRoot, req, deps)
			if err == nil {
				t.Fatal("검증되지 않은 replacement는 거부돼야 한다")
			}
			if !containsString(result.Missing, "remote_tip_equals_merged_head") {
				t.Fatalf("전진한 tip 게이트가 남아야 한다: %v", result.Missing)
			}
			if tc.wantHint != "" && !strings.Contains(result.SupersedeError, tc.wantHint) {
				t.Fatalf("거부 사유가 표면화돼야 한다: got %q want contains %q", result.SupersedeError, tc.wantHint)
			}
		})
	}
}
