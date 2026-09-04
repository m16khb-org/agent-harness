package issueops

import (
	"context"
	"fmt"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// rolloverAwareInspector는 실제 Orca 어댑터의 계약을 재현한다. 봉인된 runtime과
// 다른 runtime을 관측했을 때, 호출자가 bounded rollover 권한을 주지 않으면
// 어댑터는 인벤토리를 돌려주지 않고 거부한다.
type rolloverAwareInspector struct {
	sealedRuntimeID string
	inventory       port.ExecutionOrcaOwnerInventory
	calls           []port.ExecutionOrcaOwnerInventoryRequest
}

func (f *rolloverAwareInspector) InspectOwner(_ context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	f.calls = append(f.calls, req)
	if f.inventory.RuntimeID != f.sealedRuntimeID && !req.AllowRuntimeRollover {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca inventory runtime identity changed")
	}
	return f.inventory, nil
}

// Orca 런타임이 롤오버되면 봉인된 runtime ID로는 아무것도 조회할 수 없다. 그
// 상태에서 lease가 holderless라면 이전 런타임의 자원은 이미 사라진 것이므로,
// abandon이 고아 자원을 남긴다는 전제가 성립하지 않는다. 그런데도 게이트가
// 조회 실패를 ambiguous로 취급하면 이 레코드는 어떤 경로로도 은퇴하지 못한다.
func TestAbandonAllowsHolderlessRuntimeRollover(t *testing.T) {
	stateRoot, record := abandonSettledOrcaRecord(t, "completed")
	inspector := &rolloverAwareInspector{
		sealedRuntimeID: "runtime-136",
		inventory: port.ExecutionOrcaOwnerInventory{
			RuntimeID: "runtime-current", TaskLive: false, TerminalLive: false, TaskStatus: "completed",
		},
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector))
	if err != nil {
		t.Fatalf("a holderless runtime rollover must not block abandon: %v (%v) %s", err, result.Missing, result.OrcaResidueError)
	}
	if len(inspector.calls) != 1 || !inspector.calls[0].AllowRuntimeRollover {
		t.Fatalf("the gate must request bounded rollover inventory for a holderless lease: %+v", inspector.calls)
	}
}

// 권한은 holderless에서만 열린다. 살아 있는 writer가 있으면 rollover 조회는
// 이전 런타임의 자원 부재를 증명하지 못하므로 계속 fail-closed다.
func TestAbandonWithheldRolloverAuthorityForActiveHolder(t *testing.T) {
	stateRoot, record := abandonSettledOrcaRecord(t, "completed")
	record.Execution.Lease = issueops.WriteLease{
		Generation: 1, Status: issueops.LeaseStatusActive,
		Holder: &issueops.NativeActor{
			Host: "codex", SessionID: "live-owner",
			SessionProcess: &issueops.NativeProcessReceipt{
				PID: 4242, StartedAt: "2026-07-25T00:00:00Z", Executable: "/usr/bin/codex",
			},
		},
		ClaimedAt: "2026-07-25T00:00:00Z",
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	inspector := &rolloverAwareInspector{
		sealedRuntimeID: "runtime-136",
		inventory:       port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current"},
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector))
	if err == nil {
		t.Fatal("an active holder must keep abandon fail-closed")
	}
	if !containsString(result.Missing, "lease_terminal") {
		t.Fatalf("the lease gate must name itself: %v", result.Missing)
	}
	for _, call := range inspector.calls {
		if call.AllowRuntimeRollover {
			t.Fatalf("rollover authority must stay closed while a writer holds the lease: %+v", inspector.calls)
		}
	}
}

// rollover 권한은 자원 생존 판정을 면제하지 않는다. 새 런타임에서 task가 여전히
// 살아 있으면 거부한다.
func TestAbandonRejectsLiveTaskUnderRuntimeRollover(t *testing.T) {
	stateRoot, record := abandonSettledOrcaRecord(t, "dispatched")
	inspector := &rolloverAwareInspector{
		sealedRuntimeID: "runtime-136",
		inventory: port.ExecutionOrcaOwnerInventory{
			RuntimeID: "runtime-current", TaskLive: true, TaskStatus: "dispatched",
		},
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector))
	if err == nil {
		t.Fatal("a live task under a rolled-over runtime must still block abandon")
	}
	if !containsString(result.Missing, "orca_resources_absent") {
		t.Fatalf("the orca gate must name itself: %v", result.Missing)
	}
}
