package issueops

import (
	"context"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	linkedbranch "agent-harness/internal/domain/issueopslinkedbranch"
)

const (
	lbSealedBase = "5480568a4178d5ea46d5486b97d0ff5223f1c24c"
	lbBranch     = "304-completion-reseed-stale-receipt"
	lbIssueURL   = "https://github.com/m16khb/agent-harness/issues/304"
	lbOrphanID   = "LB_kwDPAAAAAS0v3kvOAOdU6g"
)

// lbFixture는 branch prepare까지 진행된 record를 state root에 심는다.
func lbFixture(t *testing.T) (string, issueopscontract.IssueOpsRecord) {
	t.Helper()
	root := t.TempDir()
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: 1, ID: "io-lb01", Repo: t.TempDir(), Phase: "execution",
		IssueURL: lbIssueURL, CreatedAt: "2026-08-08T00:00:00Z", UpdatedAt: "2026-08-08T00:00:00Z",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: lbIssueURL, Branch: lbBranch,
			BaseBranch: "main", BaseSHA: lbSealedBase, CreatedAt: "2026-08-08T00:00:00Z",
		},
	}
	written, err := WriteIssueOps(root, record)
	if err != nil {
		t.Fatalf("fixture record: %v", err)
	}
	return root, written
}

// lbDeps는 관측 결과를 고정하고 삭제 호출을 센다.
type lbDeps struct {
	nodes     []linkedbranch.Node
	total     int
	remote    string
	observed  int
	deleted   []string
	deleteErr error
}

func (d *lbDeps) build() CleanupLinkedBranchDeps {
	return CleanupLinkedBranchDeps{
		Git: func(_ context.Context, _ string, args ...string) (int, string) {
			if len(args) > 0 && args[0] == "ls-remote" && d.remote != "" {
				return 0, d.remote + "\trefs/heads/" + lbBranch + "\n"
			}
			return 0, ""
		},
		ObserveLinkedBranches: func(context.Context, string) (linkedbranch.Observation, error) {
			d.observed++
			total := d.total
			if total == 0 {
				total = len(d.nodes)
			}
			return linkedbranch.Observation{TotalCount: total, Nodes: d.nodes}, nil
		},
		DeleteLinkedBranch: func(_ context.Context, _, nodeID string) error {
			if d.deleteErr != nil {
				return d.deleteErr
			}
			d.deleted = append(d.deleted, nodeID)
			return nil
		},
	}
}

// TestCleanupLinkedBranchPreviewSealsTheNodeAndRefusesToDelete는 AC-04의
// 경계를 고정한다. preview는 관측만 하고 아무것도 지우지 않으며, 확정한 노드
// id가 결속된 fingerprint와 정확한 다음 명령을 돌려준다.
func TestCleanupLinkedBranchPreviewSealsTheNodeAndRefusesToDelete(t *testing.T) {
	root, _ := lbFixture(t)
	deps := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}

	result, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lb01"}, deps.build())
	if err != nil {
		t.Fatalf("고아 하나가 확정되면 preview는 성공해야 한다: %v", err)
	}
	if result.State != string(linkedbranch.StateOrphan) || result.LinkedBranchID != lbOrphanID {
		t.Fatalf("result=%#v", result)
	}
	if result.Fingerprint == "" || !strings.Contains(result.NextCommand, "--fingerprint "+result.Fingerprint) {
		t.Fatalf("다음 명령이 fingerprint를 실어야 한다: %q", result.NextCommand)
	}
	if len(deps.deleted) != 0 {
		t.Fatalf("preview는 지우지 않는다: %v", deps.deleted)
	}
}

// TestCleanupLinkedBranchApplyDeletesOnlyTheSealedNode는 삭제가 확정된 노드
// 하나에만 닿는지 고정한다.
func TestCleanupLinkedBranchApplyDeletesOnlyTheSealedNode(t *testing.T) {
	root, _ := lbFixture(t)
	deps := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}
	built := deps.build()

	preview, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lb01"}, built)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupLinkedBranch(context.Background(), root, issueopscontract.CleanupLinkedBranchRequest{
		ID: "io-lb01", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint}, built)
	if err != nil {
		t.Fatalf("apply가 실패했다: %v", err)
	}
	if !applied.Deleted || applied.DeletedAt == "" {
		t.Fatalf("result=%#v", applied)
	}
	if len(deps.deleted) != 1 || deps.deleted[0] != lbOrphanID {
		t.Fatalf("확정된 노드 하나만 지워야 한다: %v", deps.deleted)
	}
	// apply는 preview의 관측을 재사용하지 않고 매번 새로 읽는다 — 그것이
	// stale 검사의 근거다.
	if deps.observed != 2 {
		t.Fatalf("apply는 다시 관측해야 한다: observed=%d", deps.observed)
	}
}

// TestCleanupLinkedBranchApplyRequiresConfirm는 confirm 없는 apply를 막는다.
func TestCleanupLinkedBranchApplyRequiresConfirm(t *testing.T) {
	root, _ := lbFixture(t)
	deps := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}
	_, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lb01", Apply: true}, deps.build())
	if err == nil || !strings.Contains(err.Error(), "requires --confirm") {
		t.Fatalf("err=%v", err)
	}
	if len(deps.deleted) != 0 {
		t.Fatal("confirm 없이 지우면 안 된다")
	}
}

// TestCleanupLinkedBranchFailsClosedWhenTheObservationMoved는 AC-05를
// 고정한다. preview 이후 링크가 수렴해 브랜치가 생기면 apply는 멈춰야 하고,
// 어떤 링크도 건드리면 안 된다.
func TestCleanupLinkedBranchFailsClosedWhenTheObservationMoved(t *testing.T) {
	root, _ := lbFixture(t)
	deps := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}
	built := deps.build()
	preview, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lb01"}, built)
	if err != nil {
		t.Fatal(err)
	}

	// 두 번째 고아가 생겼다 — 이제 어느 것이 우리 것인지 알 수 없다.
	deps.nodes = append(deps.nodes, linkedbranch.Node{ID: "LB_second"})
	_, err = CleanupLinkedBranch(context.Background(), root, issueopscontract.CleanupLinkedBranchRequest{
		ID: "io-lb01", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint}, built)
	if err == nil {
		t.Fatal("관측이 움직였으면 멈춰야 한다")
	}
	if len(deps.deleted) != 0 {
		t.Fatalf("멈춘 apply는 아무것도 지우지 않는다: %v", deps.deleted)
	}
}

// TestCleanupLinkedBranchRejectsAStaleFingerprint는 원격 브랜치가 생겨
// fingerprint 입력이 달라진 경우를 고정한다.
func TestCleanupLinkedBranchRejectsAStaleFingerprint(t *testing.T) {
	root, _ := lbFixture(t)
	deps := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}
	built := deps.build()
	_, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lb01"}, built)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CleanupLinkedBranch(context.Background(), root, issueopscontract.CleanupLinkedBranchRequest{
		ID: "io-lb01", Apply: true, Confirm: true, Fingerprint: strings.Repeat("0", 64)}, built)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("err=%v", err)
	}
	if len(deps.deleted) != 0 {
		t.Fatal("stale fingerprint는 삭제로 이어지면 안 된다")
	}
}

// TestCleanupLinkedBranchIsIdempotentWhenAlreadyAbsent는 성공한 삭제 뒤의
// 재실행이 stale로 막히지 않음을 고정한다. 멱등성과 TOCTOU 방어가 서로를
// 무효화하면 사용자는 이미 끝난 정리를 끝났다고 확인할 방법이 없다.
func TestCleanupLinkedBranchIsIdempotentWhenAlreadyAbsent(t *testing.T) {
	root, _ := lbFixture(t)
	deps := &lbDeps{}
	result, err := CleanupLinkedBranch(context.Background(), root, issueopscontract.CleanupLinkedBranchRequest{
		ID: "io-lb01", Apply: true, Confirm: true, Fingerprint: strings.Repeat("0", 64)}, deps.build())
	if err != nil || !result.AlreadyAbsent || !result.OK {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

// TestCleanupLinkedBranchRecordsEveryDispositionInTheAudit는 AC-06을
// 고정한다. 성공·이미 부재·모호성이 모두 durable record에 남아야 한다.
func TestCleanupLinkedBranchRecordsEveryDispositionInTheAudit(t *testing.T) {
	for _, tc := range []struct {
		name      string
		nodes     []linkedbranch.Node
		apply     bool
		wantState string
	}{
		{"이미 부재", nil, true, string(linkedbranch.StateAbsent)},
		{"모호", []linkedbranch.Node{{ID: lbOrphanID}, {ID: "LB_second"}}, false, string(linkedbranch.StateAmbiguous)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := lbFixture(t)
			deps := &lbDeps{nodes: tc.nodes}
			_, _ = CleanupLinkedBranch(context.Background(), root, issueopscontract.CleanupLinkedBranchRequest{
				ID: "io-lb01", Apply: tc.apply, Confirm: tc.apply, Fingerprint: strings.Repeat("0", 64)}, deps.build())

			after, err := ReadIssueOps(root, "io-lb01")
			if err != nil {
				t.Fatal(err)
			}
			if after.LinkedBranchCleanup == nil || after.LinkedBranchCleanup.State != tc.wantState {
				t.Fatalf("처분이 durable audit에 남아야 한다: %#v", after.LinkedBranchCleanup)
			}
			if after.LinkedBranchCleanup.ObservedAt == "" {
				t.Fatal("audit은 언제 관측했는지를 남겨야 한다")
			}
		})
	}
}

// TestCleanupLinkedBranchRefusesNonGitHubProviders는 표면 경계를 고정한다.
// LinkedBranch는 GitHub의 개념이고, 다른 provider에서는 무엇을 지울지가
// 정의되지 않는다.
func TestCleanupLinkedBranchRefusesNonGitHubProviders(t *testing.T) {
	root, record := lbFixture(t)
	record.BranchPrepare.Provider = "gitlab"
	if _, err := WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	deps := &lbDeps{nodes: []linkedbranch.Node{{ID: lbOrphanID}}}
	result, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lb01"}, deps.build())
	if err == nil {
		t.Fatal("github 전용 표면이다")
	}
	if !containsString(result.Missing, "linked_branch_cleanup_is_github_only") {
		t.Fatalf("missing=%v", result.Missing)
	}
	if deps.observed != 0 {
		t.Fatal("게이트에서 막힌 요청은 외부를 관측하지 않는다")
	}
}
