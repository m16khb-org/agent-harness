package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	linkedbranch "agent-harness/internal/domain/issueopslinkedbranch"
)

// awaitFixture는 GitHub Orca 시작 창에 있는 record를 심는다: link 미검증,
// 봉인된 branch/base 있음.
func awaitFixture(t *testing.T, linkVerified bool) string {
	t.Helper()
	root := t.TempDir()
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: 1, ID: "io-await1", Repo: t.TempDir(), Phase: issueopscontract.IssueOpsPhaseImplement,
		IssueURL: lbIssueURL, CreatedAt: "2026-08-09T00:00:00Z", UpdatedAt: "2026-08-09T00:00:00Z",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: lbIssueURL, Branch: lbBranch,
			BaseBranch: "main", BaseSHA: lbSealedBase, LinkVerified: linkVerified,
			CreatedAt: "2026-08-09T00:00:00Z",
		},
	}
	if _, err := WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	return root
}

// awaitDeps는 관측을 회차별로 바꾸고, 잠든 시간을 가상 시계로 흘린다.
type awaitDeps struct {
	rounds   [][]linkedbranch.Node
	remote   []string
	errs     []error
	observed int
	slept    []time.Duration
	clock    time.Time
}

func (d *awaitDeps) build() AwaitBranchLinkDeps {
	d.clock = time.Unix(1786200000, 0).UTC()
	return AwaitBranchLinkDeps{
		Git: func(_ context.Context, _ string, args ...string) (int, string) {
			index := min(d.observed-1, len(d.remote)-1)
			if len(args) == 0 || args[0] != "ls-remote" || index < 0 || d.remote[index] == "" {
				return 0, ""
			}
			return 0, d.remote[index] + "\trefs/heads/" + lbBranch + "\n"
		},
		ObserveLinkedBranches: func(context.Context, string) (linkedbranch.Observation, error) {
			round := min(d.observed, len(d.rounds)-1)
			d.observed++
			if round < len(d.errs) && d.errs[round] != nil {
				return linkedbranch.Observation{}, d.errs[round]
			}
			nodes := d.rounds[round]
			return linkedbranch.Observation{TotalCount: len(nodes), Nodes: nodes}, nil
		},
		Sleep: func(_ context.Context, wait time.Duration) error {
			d.slept = append(d.slept, wait)
			d.clock = d.clock.Add(wait)
			return nil
		},
		Now: func() time.Time { return d.clock },
	}
}

func healthyNodes() []linkedbranch.Node {
	return []linkedbranch.Node{{ID: "LB_live", RefName: lbBranch, RefOID: lbSealedBase}}
}

// TestAwaitBranchLinkWaitsThroughTheCoordinatorStartupWindow는 #319의 핵심을
// 고정한다.
//
// GitHub Orca 경로에서 owner는 링크가 **아직 없는** 시점에 시작한다 — prepare가
// link 미검증 상태에서 owner를 띄우고 coordinator의 createLinkedBranch가 그 뒤에
// 오기 때문이다. 시작 시점의 부재를 terminal 실패로 다루면 이 경로는 구조적으로
// 완주할 수 없다. 실측된 실패가 정확히 그것이었다(task_e3946ef93086).
func TestAwaitBranchLinkWaitsThroughTheCoordinatorStartupWindow(t *testing.T) {
	root := awaitFixture(t, false)
	deps := &awaitDeps{
		rounds: [][]linkedbranch.Node{nil, nil, healthyNodes()},
		remote: []string{"", "", lbSealedBase},
	}
	result, err := AwaitBranchLink(context.Background(), root,
		issueopscontract.AwaitBranchLinkRequest{ID: "io-await1"}, deps.build())
	if err != nil {
		t.Fatalf("링크가 나타나면 성공해야 한다: %v", err)
	}
	if !result.Linked || result.ObservedOID != lbSealedBase || result.Attempts != 3 {
		t.Fatalf("result=%#v", result)
	}
	if len(deps.slept) != 2 {
		t.Fatalf("관측 사이에만 기다려야 한다: %v", deps.slept)
	}
}

// TestAwaitBranchLinkIsBounded는 무한 대기를 막는다. coordinator가 멈췄으면
// owner도 멈춰야 하고, 그 사실이 진단으로 남아야 한다.
func TestAwaitBranchLinkIsBounded(t *testing.T) {
	root := awaitFixture(t, false)
	deps := &awaitDeps{rounds: [][]linkedbranch.Node{nil}}
	result, err := AwaitBranchLink(context.Background(), root,
		issueopscontract.AwaitBranchLinkRequest{ID: "io-await1", Timeout: "1m"}, deps.build())
	if err == nil || !result.TimedOut {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	for _, needle := range []string{lbBranch, lbSealedBase, "coordinator"} {
		if !strings.Contains(err.Error(), needle) {
			t.Fatalf("무엇이 없어서 멈췄는지 밝혀야 한다: %v", err)
		}
	}
	// 1분/15초 → 관측 5회, 대기 4회. 경계가 값으로 고정돼 있음을 확인한다.
	if result.Attempts != 5 || len(deps.slept) != 4 {
		t.Fatalf("attempts=%d slept=%v", result.Attempts, deps.slept)
	}
}

// TestAwaitBranchLinkStopsOnAMismatchInsteadOfWaiting는 기다려도 낫지 않는
// 상태를 구분한다. 링크는 있는데 봉인값과 다르면 사람이 봐야 하고, 기다리면
// 진단만 늦어진다.
func TestAwaitBranchLinkStopsOnAMismatchInsteadOfWaiting(t *testing.T) {
	root := awaitFixture(t, false)
	deps := &awaitDeps{
		rounds: [][]linkedbranch.Node{{{ID: "LB_live", RefName: lbBranch, RefOID: "deadbeef"}}},
		remote: []string{lbSealedBase},
	}
	_, err := AwaitBranchLink(context.Background(), root,
		issueopscontract.AwaitBranchLinkRequest{ID: "io-await1"}, deps.build())
	if err == nil || !strings.Contains(err.Error(), "sealed identity") {
		t.Fatalf("err=%v", err)
	}
	if len(deps.slept) != 0 {
		t.Fatalf("불일치는 기다리지 않는다: %v", deps.slept)
	}
}

// TestAwaitBranchLinkTreatsAReadFailureAsNotYet는 관측 실패를 부재로도
// 종료로도 다루지 않음을 고정한다. 네트워크 오류는 다음 주기에 다시 읽는다.
func TestAwaitBranchLinkTreatsAReadFailureAsNotYet(t *testing.T) {
	root := awaitFixture(t, false)
	deps := &awaitDeps{
		rounds: [][]linkedbranch.Node{nil, healthyNodes()},
		remote: []string{"", lbSealedBase},
		errs:   []error{errors.New("transient gh failure")},
	}
	result, err := AwaitBranchLink(context.Background(), root,
		issueopscontract.AwaitBranchLinkRequest{ID: "io-await1"}, deps.build())
	if err != nil || !result.Linked {
		t.Fatalf("일시적 실패 뒤에도 수렴해야 한다: result=%#v err=%v", result, err)
	}
}

// TestAwaitBranchLinkIsIdempotentWhenAlreadyRecorded는 이미 기록된 경우
// 기다리지 않음을 고정한다.
func TestAwaitBranchLinkIsIdempotentWhenAlreadyRecorded(t *testing.T) {
	root := awaitFixture(t, true)
	deps := &awaitDeps{rounds: [][]linkedbranch.Node{nil}}
	result, err := AwaitBranchLink(context.Background(), root,
		issueopscontract.AwaitBranchLinkRequest{ID: "io-await1"}, deps.build())
	if err != nil || !result.AlreadyVerified || !result.Linked {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if deps.observed != 0 {
		t.Fatal("이미 기록됐으면 관측하지 않는다")
	}
}

// TestAwaitBranchLinkRejectsAnUnboundedTimeout는 상한을 고정한다. 이보다
// 오래 기다려야 한다면 그것은 대기 문제가 아니라 coordinator가 멈춘 것이다.
func TestAwaitBranchLinkRejectsAnUnboundedTimeout(t *testing.T) {
	root := awaitFixture(t, false)
	for _, raw := range []string{"0s", "-1m", "31m", "forever"} {
		deps := &awaitDeps{rounds: [][]linkedbranch.Node{nil}}
		if _, err := AwaitBranchLink(context.Background(), root,
			issueopscontract.AwaitBranchLinkRequest{ID: "io-await1", Timeout: raw}, deps.build()); err == nil {
			t.Fatalf("--timeout %q는 거부해야 한다", raw)
		}
	}
}

// TestAwaitBranchLinkIsGitHubOnly는 표면 경계를 고정한다. GitLab은 prepare
// 시점에 이미 link_verified를 요구하므로 이 시작 창 자체가 없다.
func TestAwaitBranchLinkIsGitHubOnly(t *testing.T) {
	root := awaitFixture(t, false)
	record, err := ReadIssueOps(root, "io-await1")
	if err != nil {
		t.Fatal(err)
	}
	record.BranchPrepare.Provider = "gitlab"
	if _, err := WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	deps := &awaitDeps{rounds: [][]linkedbranch.Node{nil}}
	result, err := AwaitBranchLink(context.Background(), root,
		issueopscontract.AwaitBranchLinkRequest{ID: "io-await1"}, deps.build())
	if err == nil || !containsString(result.Missing, "branch_await_link_is_github_only") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if deps.observed != 0 {
		t.Fatal("게이트에서 막힌 요청은 외부를 관측하지 않는다")
	}
}
