package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"

	issueopsdomain "agent-harness/internal/domain/issueops"
)

// absentRefFinishGit는 worktree 제거가 local branch ref를 함께 없애는 실제
// 순서를 재현한다. 그 뒤의 `update-ref -d`는 Git이 ref를 찾지 못해 실패한다.
type absentRefFinishGit struct {
	branchOID     string
	refRemoved    bool
	updateRefRuns int
}

func (g *absentRefFinishGit) run(_ string, args ...string) (int, string) {
	switch args[0] {
	case "ls-remote":
		return 0, ""
	case "status":
		return 0, ""
	case "rev-parse":
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	case "worktree":
		// Orca/Git worktree 제거가 linked branch ref까지 회수한다.
		g.refRemoved = true
		return 0, ""
	case "show-ref", "for-each-ref":
		if g.refRemoved {
			return 1, ""
		}
		return 0, "abc123 refs/heads/80-finish\n"
	case "update-ref":
		g.updateRefRuns++
		if g.refRemoved {
			return 1, "error: cannot lock ref 'refs/heads/80-finish': unable to resolve reference 'refs/heads/80-finish'"
		}
		g.branchOID = ""
		return 0, ""
	}
	return 0, ""
}

// TestCleanupFinishConvergesWhenWorktreeRemovalAlreadyDroppedTheBranchRef는
// #291을 고정한다. preview 시점에는 branch OID가 관측되지만 worktree 제거가
// 그 ref를 함께 없애므로, 뒤이은 branch delete는 대상 부재로 실패한다. exact
// target의 부재는 idempotent success이며 첫 apply 하나로 record 삭제까지
// 수렴해야 한다.
func TestCleanupFinishConvergesWhenWorktreeRemovalAlreadyDroppedTheBranchRef(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &absentRefFinishGit{branchOID: "abc123"}
	deps := CleanupFinishDeps{Git: git.run, InspectProcesses: func(string) ([]string, error) { return nil, nil }}

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatalf("worktree 제거가 이미 없앤 ref는 idempotent success여야 한다: %v (failed_step=%s)", err, result.FailedStep)
	}
	if result.FailedStep != "" {
		t.Fatalf("첫 apply에서 실패 단계가 남으면 안 된다: %+v", result)
	}
	if !result.RecordDeleted {
		t.Fatalf("첫 apply 하나로 record 삭제까지 수렴해야 한다: %+v", result)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err == nil {
		t.Fatal("record가 남아 있다")
	}
}

// TestCleanupFinishStillFailsOnRealBranchDeleteErrors는 위 정규화가 진짜
// 오류까지 삼키지 않음을 고정한다. permission/lock contention/OID drift는
// 부재가 아니므로 계속 실패해야 한다.
func TestCleanupFinishStillFailsOnRealBranchDeleteErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
	}{
		{"permission denied", "error: cannot lock ref 'refs/heads/80-finish': Unable to create lock file: Permission denied"},
		{"lock contention", "error: cannot lock ref 'refs/heads/80-finish': is at 0000 but expected 1111"},
		{"OID drift", "error: cannot lock ref 'refs/heads/80-finish': reference already exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record, _ := finishTestRecord(t, true)
			git := &realErrorFinishGit{branchOID: "abc123", updateRefOutput: tc.output}
			deps := CleanupFinishDeps{Git: git.run, InspectProcesses: func(string) ([]string, error) { return nil, nil }}

			preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
			if err == nil || result.FailedStep != "branch_delete" {
				t.Fatalf("진짜 branch delete 오류는 계속 실패해야 한다: err=%v result=%+v", err, result)
			}
			kept, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil || kept.CleanupFinishFailure == nil || kept.CleanupFinishFailure.Step != "branch_delete" {
				t.Fatalf("실패 지점이 보존돼야 한다: %v %+v", readErr, kept.CleanupFinishFailure)
			}
		})
	}
}

// realErrorFinishGit는 ref가 여전히 존재하는데 update-ref가 실패하는 상황을
// 재현한다.
type realErrorFinishGit struct {
	branchOID       string
	updateRefOutput string
}

func (g *realErrorFinishGit) run(_ string, args ...string) (int, string) {
	switch args[0] {
	case "ls-remote":
		return 0, ""
	case "status":
		return 0, ""
	case "rev-parse":
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	case "worktree":
		return 0, ""
	case "show-ref", "for-each-ref":
		return 0, g.branchOID + " refs/heads/80-finish\n"
	case "update-ref":
		return 1, g.updateRefOutput
	}
	return 0, ""
}

// TestCleanupAbandonConvergesWhenWorktreeRemovalAlreadyDroppedTheBranchRef는
// finish와 같은 순서 결함이 abandon 경로에도 있었음을 고정한다(#291).
func TestCleanupAbandonConvergesWhenWorktreeRemovalAlreadyDroppedTheBranchRef(t *testing.T) {
	git := &absentRefFinishGit{branchOID: "abc123"}
	// worktree 제거가 ref를 회수한 뒤의 상태를 직접 만든다.
	git.refRemoved = true

	if branchRefPresent(git.run, "/repo", "80-finish") {
		t.Fatal("전제 확인 실패: ref가 회수된 뒤에는 부재로 관측돼야 한다")
	}
	if code, _ := git.run("/repo", "update-ref", "-d", "refs/heads/80-finish", "abc123"); code == 0 {
		t.Fatal("전제 확인 실패: 부재 ref에 대한 update-ref는 실패해야 한다")
	}

	present := &realErrorFinishGit{branchOID: "abc123"}
	if !branchRefPresent(present.run, "/repo", "80-finish") {
		t.Fatal("ref가 남아 있으면 존재로 관측돼야 한다 — 진짜 오류를 삼키면 안 된다")
	}
	if branchRefPresent(nil, "/repo", "80-finish") != true {
		t.Fatal("관측이 불가능하면 fail-closed로 존재 취급해야 한다")
	}
	if branchRefPresent(present.run, "/repo", "  ") != true {
		t.Fatal("branch 이름이 비면 fail-closed로 존재 취급해야 한다")
	}
}

// TestCleanupFinishAcceptsAVerifiedSupersedingArtifact는 #283의 통과 경로를
// 고정한다. 원 artifact가 unmerged여도 후속 artifact가 provider readback으로
// merged·같은 프로젝트·명시적 supersede를 만족하면 정리할 수 있어야 한다.
func TestCleanupFinishAcceptsAVerifiedSupersedingArtifact(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &realErrorFinishGit{branchOID: ""}
	deps := CleanupFinishDeps{
		Git:              git.run,
		InspectProcesses: func(string) ([]string, error) { return nil, nil },
		ObserveArtifact: func(url string) (issueopsdomain.ArtifactObservation, error) {
			return issueopsdomain.ArtifactObservation{
				URL: url, Provider: "github", Merged: true, State: "MERGED",
				Body: "Supersedes " + record.RemoteArtifact.URL,
			}, nil
		},
	}
	req := finishRequest(record.ID, false, "")
	req.Merged = false
	req.SupersededBy = "https://github.com/acme/repo/pull/999"

	result, err := CleanupFinish(context.Background(), stateRoot, req, deps)
	if err != nil {
		t.Fatalf("검증된 replacement 증거는 merged 게이트를 충족해야 한다: %v missing=%v supersedeError=%q",
			err, result.Missing, result.SupersedeError)
	}
	if result.SupersededBy != req.SupersededBy {
		t.Fatalf("무엇을 근거로 통과했는지 결과에 남아야 한다: %+v", result)
	}
	wantNext := "agent-harness issueops cleanup finish --id " + record.ID +
		" --apply --confirm --fingerprint " + result.Fingerprint +
		" --superseded-by '" + req.SupersededBy + "' --json"
	if result.NextCommand != wantNext {
		t.Fatalf("generated apply command must preserve replacement evidence:\n got: %s\nwant: %s", result.NextCommand, wantNext)
	}

	req.Apply, req.Confirm, req.Fingerprint = true, true, result.Fingerprint
	applied, err := CleanupFinish(context.Background(), stateRoot, req, deps)
	if err != nil || !applied.RecordDeleted {
		t.Fatalf("generated replacement evidence and fingerprint must finish cleanup: err=%v result=%+v", err, applied)
	}
}

// TestCleanupFinishRejectsUnverifiableSupersedingArtifacts는 이 경로가 아무
// 머지된 PR로 아무 record나 지우는 문을 열지 않음을 고정한다.
func TestCleanupFinishRejectsUnverifiableSupersedingArtifacts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		observe  func(string) (issueopsdomain.ArtifactObservation, error)
		wantHint string
	}{
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
		{
			"cross-repo",
			func(string) (issueopsdomain.ArtifactObservation, error) {
				return issueopsdomain.ArtifactObservation{
					URL: "https://github.com/other/repo/pull/999", Merged: true, State: "MERGED",
					Body: "Supersedes everything",
				}, nil
			},
			"not the original project",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record, _ := finishTestRecord(t, true)
			deps := CleanupFinishDeps{
				Git:              (&realErrorFinishGit{}).run,
				InspectProcesses: func(string) ([]string, error) { return nil, nil },
				ObserveArtifact:  tc.observe,
			}
			req := finishRequest(record.ID, false, "")
			req.Merged = false
			req.SupersededBy = "https://github.com/acme/repo/pull/999"

			result, err := CleanupFinish(context.Background(), stateRoot, req, deps)
			if err == nil {
				t.Fatal("검증되지 않은 replacement는 거부돼야 한다")
			}
			if !containsString(result.Missing, "remote_artifact_merged") {
				t.Fatalf("merged 게이트가 남아야 한다: %v", result.Missing)
			}
			if !strings.Contains(result.SupersedeError, tc.wantHint) {
				t.Fatalf("거부 사유가 표면화돼야 한다: got %q want contains %q", result.SupersedeError, tc.wantHint)
			}
		})
	}
}

// TestCleanupFinishRequiresObservationForSupersedeEvidence는 관측이 주입되지
// 않으면 증거를 인정하지 않음을 고정한다.
func TestCleanupFinishRequiresObservationForSupersedeEvidence(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	deps := CleanupFinishDeps{
		Git:              (&realErrorFinishGit{}).run,
		InspectProcesses: func(string) ([]string, error) { return nil, nil },
	}
	req := finishRequest(record.ID, false, "")
	req.Merged = false
	req.SupersededBy = "https://github.com/acme/repo/pull/999"

	result, _ := CleanupFinish(context.Background(), stateRoot, req, deps)
	if !strings.Contains(result.SupersedeError, "provider observation is not configured") {
		t.Fatalf("관측 없이 증거를 인정하면 안 된다: %q", result.SupersedeError)
	}
}
