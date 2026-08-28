package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func retargetActorRecord(t *testing.T, status issueops.LeaseStatus, holder *issueops.NativeActor) (issueops.IssueOpsRecord, string) {
	t.Helper()
	source := t.TempDir()
	root := filepath.Join(source+".worktrees", "2819-child")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return issueops.IssueOpsRecord{Execution: &issueops.Execution{
		Mode: issueops.ExecutionModeDirect,
		Workspace: issueops.Workspace{
			SourceRoot: source, Root: root, Branch: "2819-child", BaseHead: strings.Repeat("a", 40),
			Driver: "git", LinkedAt: "2026-08-28T00:00:00Z",
		},
		Lease: issueops.WriteLease{Generation: 1, Status: status, Holder: holder, ClaimedAt: "2026-08-28T00:00:01Z"},
	}}, root
}

// 재타깃은 사이클 중(lease 보유)에도, 완료 후 cleanup 시점(lease 해제)에도 일어난다.
// 보유 중에는 홀더만 base를 옮길 수 있어야 하고, 해제 후에는 완료 계열 표면
// (remote reflect-completion, cleanup finish)과 같이 lease가 아니라 provider
// readback과 origin 관측이 보호한다.
func TestRetargetMutationBindsToHolderOnlyWhileTheLeaseIsActive(t *testing.T) {
	holder := issueops.NativeActor{
		Host: "codex", SessionID: "session-1",
		SessionProcess: &issueops.NativeProcessReceipt{PID: 42, StartedAt: "2026-08-28T00:00:00Z", Executable: "/usr/bin/codex"},
	}
	active, root := retargetActorRecord(t, issueops.LeaseStatusActive, &holder)
	exact := IssueOpsActor{Host: "codex", SessionID: "session-1", CWD: root, NativeProcessAncestry: []issueops.NativeProcessReceipt{*holder.SessionProcess}}
	if err := validateRetargetMutation(active, &exact); err != nil {
		t.Fatalf("the active lease holder must be allowed to retarget: %v", err)
	}
	if err := validateRetargetMutation(active, nil); err == nil {
		t.Fatal("a non-holder must not move the base while the lease is active")
	}

	released, _ := retargetActorRecord(t, issueops.LeaseStatusReleased, nil)
	if err := validateRetargetMutation(released, nil); err != nil {
		t.Fatalf("a released cycle must stay retargetable at cleanup time: %v", err)
	}
	if err := validateRetargetMutation(issueops.IssueOpsRecord{}, nil); err != nil {
		t.Fatalf("a cycle without execution must stay retargetable: %v", err)
	}
}
