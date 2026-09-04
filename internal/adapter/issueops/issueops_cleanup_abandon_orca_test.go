package issueops

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type fakeOwnerInspector struct {
	inventory port.ExecutionOrcaOwnerInventory
	err       error
	calls     []port.ExecutionOrcaOwnerInventoryRequest
}

func (f *fakeOwnerInspector) InspectOwner(_ context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	f.calls = append(f.calls, req)
	return f.inventory, f.err
}

// abandon은 아무것도 지우지 않는 경로다. orca task가 살아 있으면 레코드를
// 지우는 순간 그 task는 소유자를 잃고 영구 residue가 되므로, 잔여물을 지울 수
// 있는 경로로 보내야 한다(#136).
func TestAbandonRejectsLiveOrcaTask(t *testing.T) {
	stateRoot, record := abandonSettledOrcaRecord(t, "dispatched")
	inspector := &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{
		TaskLive: true, TaskStatus: "dispatched",
	}}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector))
	if err == nil {
		t.Fatal("a live orca task must block abandon")
	}
	if !containsString(result.Missing, "orca_resources_absent") {
		t.Fatalf("the gate must name itself in missing: %v", result.Missing)
	}
	if result.OrcaResidueError == "" {
		t.Fatal("the gate must surface why it blocked")
	}
	if len(inspector.calls) != 1 || inspector.calls[0].TaskID != "task-136" {
		t.Fatalf("the gate must inspect the bound owner inventory: %+v", inspector.calls)
	}
}

// 거부 메시지가 정답 경로를 지시해야 한다. abandon이 막힌 운영자는 어디로
// 가야 하는지 알아야 한다.
func TestAbandonLiveOrcaTaskNamesTheCorrectPath(t *testing.T) {
	stateRoot, record := abandonSettledOrcaRecord(t, "dispatched")
	inspector := &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{TaskLive: true}}

	result, _ := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector))
	for _, want := range []string{"finish", "orphan"} {
		if !strings.Contains(result.OrcaResidueError, want) {
			t.Fatalf("the block reason %q must route to %s", result.OrcaResidueError, want)
		}
	}
}

// 종결된 task는 자원을 점유하지 않는다. dispatch될 수 없으므로 소유자를 잃어도
// residue가 아니다.
func TestAbandonAllowsSettledOrcaTask(t *testing.T) {
	for _, status := range []string{"completed", "failed", "cancelled", "closed"} {
		t.Run(status, func(t *testing.T) {
			stateRoot, record := abandonSettledOrcaRecord(t, status)
			inspector := &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{
				TaskLive: false, TaskStatus: status,
			}}

			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
				abandonOrcaDeps(&fakeAbandonGit{}, inspector))
			if err != nil {
				t.Fatalf("a settled orca task must not block abandon: %v (%v)", err, result.Missing)
			}
		})
	}
}

// 조회할 수 없으면 통과가 아니라 거부다. 어댑터 부재가 게이트를 무력화하면
// #106이 세운 fail-closed 계약이 무너진다.
func TestAbandonRejectsUninspectableOrcaOwner(t *testing.T) {
	t.Run("missing inspector", func(t *testing.T) {
		stateRoot, record := abandonSettledOrcaRecord(t, "dispatched")
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
			CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: (&fakeAbandonGit{}).run})
		if err == nil || !containsString(result.Missing, "orca_resources_absent") {
			t.Fatalf("a missing inspector must block: %v %v", err, result.Missing)
		}
	})
	t.Run("transport failure", func(t *testing.T) {
		stateRoot, record := abandonSettledOrcaRecord(t, "dispatched")
		inspector := &fakeOwnerInspector{err: fmt.Errorf("orca runtime unreachable")}
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
			abandonOrcaDeps(&fakeAbandonGit{}, inspector))
		if err == nil || !containsString(result.Missing, "orca_resources_absent") {
			t.Fatalf("an inspect failure must block: %v %v", err, result.Missing)
		}
	})
}

// direct 모드와 orca 바인딩이 없는 레코드는 조회 자체가 일어나지 않는다.
func TestAbandonSkipsOrcaGateWithoutBinding(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	inspector := &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{TaskLive: true}}

	if _, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector)); err != nil {
		t.Fatalf("a record without an orca binding must abandon as before: %v", err)
	}
	if len(inspector.calls) != 0 {
		t.Fatalf("no binding means no inspection: %+v", inspector.calls)
	}
}

func abandonOrcaDeps(git *fakeAbandonGit, inspector port.ExecutionOrcaOwnerInspector) CleanupAbandonDeps {
	return CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git.run, Orca: authoritativeZeroOrca(), OrcaOwner: inspector}
}

// abandonSettledOrcaRecord는 prepare가 끝난 orca 사이클을 만든다. 워크트리
// 디렉터리는 존재하지 않으므로 게이트 ⑥은 통과하고 orca 자원만 남는다 —
// 이 이슈가 다루는 정확한 조건이다.
func abandonSettledOrcaRecord(t *testing.T, taskStatus string) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot, record := abandonTestRecord(t)
	record.Execution = &issueops.Execution{
		Mode: issueops.ExecutionModeOrca,
		Workspace: issueops.Workspace{
			SourceRoot: record.Repo, Root: record.Repo + ".worktrees/deleted-136",
			Branch: record.Branch, BaseHead: "0000000000000000000000000000000000000000",
			Driver: "orca", LinkedAt: "2026-07-25T00:00:00Z",
		},
		Orca: &issueops.OrcaBinding{
			RuntimeID: "runtime-136", RepoID: "repo-136", WorktreeID: "worktree-136",
			OwnerHost: "claude", OwnerModel: "claude-opus-5", TerminalPTYID: "pty-136",
			TaskID: "task-136", DispatchID: "dispatch-136",
		},
		Lease: issueops.WriteLease{
			Generation: 1, Status: issueops.LeaseStatusReleased, ReleasedAt: "2026-07-25T00:00:01Z",
		},
	}
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}
