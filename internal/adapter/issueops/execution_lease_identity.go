package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func normalizeNativeActor(actor issueops.NativeActor) (issueops.NativeActor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.SessionProcess != nil {
		receipt := *actor.SessionProcess
		receipt.StartedAt = strings.TrimSpace(receipt.StartedAt)
		receipt.Executable = strings.TrimSpace(receipt.Executable)
		actor.SessionProcess = &receipt
	}
	actor.ProcessAncestry = append([]issueops.NativeProcessReceipt(nil), actor.ProcessAncestry...)
	if err := issueops.ValidateNativeActor(actor); err != nil {
		return actor, err
	}
	locallyObserved := false
	for _, receipt := range actor.ProcessAncestry {
		if actor.SessionProcess != nil && receipt == *actor.SessionProcess {
			locallyObserved = true
			break
		}
	}
	if !locallyObserved {
		return actor, fmt.Errorf("native session process receipt is not in the local process ancestry")
	}
	if err := requireExactLiveNativeProcessReceipt(*actor.SessionProcess); err != nil {
		return actor, err
	}
	return actor, nil
}

// ValidateNativeActorProcess applies the same ancestry and live-process receipt
// checks used by lease transitions before preparation can persist a new holder.
func ValidateNativeActorProcess(actor issueops.NativeActor) error {
	_, err := normalizeNativeActor(actor)
	return err
}

// refuseSelfRevoke는 살아 있는 홀더가 자기 lease를 revoke하는 것을 막는다.
//
// revoke의 존재 이유는 응답 없는 홀더에게서 제3자가 lease를 뺏는 것이다. 그런데
// 홀더 자신이 호출하면 나갈 문이 전부 막힌다: release는 active를, reseed는
// released/claimable을, claim은 claimable을 요구하고, finalize는 이전 홀더가
// dead여야 한다 — 그 홀더가 나 자신이므로 내가 죽어야만 풀린다(이슈 #170).
//
// 홀더가 원한 것은 lease 교체이고 release가 그것을 준다. 그래서 거부만 하지
// 않고 그 명령을 안내한다.
//
// 생존 판정은 finalize가 쓰는 inspectNativeProcessReceipt와 같은 함수다. 두 곳이
// 같은 기준을 봐야 한쪽은 revoke를 막는데 다른 쪽은 finalize를 막는 교착이
// 생기지 않는다. 판정이 실패하거나 live가 아니면 통과시킨다 — 그것이 지금
// 동작이고, 죽은 홀더 뺏기와 제3자 revoke를 막지 않는다.
func refuseSelfRevoke(lifecycleID string, lease issueops.WriteLease, requester issueops.NativeActor) error {
	holder := lease.Holder
	if holder == nil || !sameNativeActorIdentity(holder, &requester) || holder.SessionProcess == nil {
		return nil
	}
	status, _, err := inspectNativeProcessReceipt(*holder.SessionProcess)
	if err != nil || status != NativeProcessStatusLive {
		return nil
	}
	return fmt.Errorf(
		"revoke takes a lease away from an unresponsive holder, but this session is the live holder: "+
			"revoking your own lease leaves no exit because finalize requires the old holder to be dead. "+
			"Run `issueops execution release --id %s --generation %d` instead",
		strings.TrimSpace(lifecycleID), lease.Generation)
}

func sameNativeActor(a, b *issueops.NativeActor) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameNativeActorIdentity(a, b) && sameNativeProcessReceipt(a.SessionProcess, b.SessionProcess)
}

func sameNativeActorIdentity(a, b *issueops.NativeActor) bool {
	return a != nil && b != nil && strings.EqualFold(a.Host, b.Host) && a.SessionID == b.SessionID && a.AgentID == b.AgentID
}

func sameNativeProcessReceipt(a, b *issueops.NativeProcessReceipt) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func samePath(a, b string) bool {
	left, err := filepath.Abs(strings.TrimSpace(a))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(left); resolveErr == nil {
		left = resolved
	}
	right, err := filepath.Abs(strings.TrimSpace(b))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(right); resolveErr == nil {
		right = resolved
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func executionInventoryFingerprint(ctx context.Context, record issueops.IssueOpsRecord, requester issueops.NativeActor, deps ExecutionReplaceDependencies) (string, port.ExecutionOrcaOwnerInventory, error) {
	snapshot, err := workspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	processSnapshot, _ := observeNativeProcessSnapshot()
	processStatus, orcaInventory, err := executionOwnerInventory(ctx, record, deps, processSnapshot)
	if err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionRuntimeRolloverWithProcess(record, processStatus, orcaInventory); err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Status     issueops.LeaseStatus             `json:"status"`
		Holder     *issueops.NativeActor            `json:"holder,omitempty"`
		Requester  issueops.NativeActor             `json:"requester"`
		Process    string                           `json:"process_status"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, requester, processStatus, orcaInventory, snapshot}
	fingerprint, err := hashJSON(payload)
	return fingerprint, orcaInventory, err
}

func executionQuiescenceFingerprint(ctx context.Context, record issueops.IssueOpsRecord, requester issueops.NativeActor, deps ExecutionReplaceDependencies) (string, error) {
	holder := record.Execution.Lease.Holder
	if holder == nil || holder.SessionProcess == nil {
		return "", fmt.Errorf("revoking lease is missing its old process receipt")
	}
	processSnapshot, _ := observeNativeProcessSnapshot()
	processStatus, _, err := inspectNativeProcessReceiptForRollover(*holder.SessionProcess, processSnapshot)
	if err != nil {
		return "", err
	}
	if processStatus == "live" {
		return "", fmt.Errorf("old holder process is still live: pid=%d executable=%s", holder.SessionProcess.PID, holder.SessionProcess.Executable)
	}
	if processStatus != "dead" {
		return "", fmt.Errorf("old holder process identity is unsafe to finalize: pid=%d status=%s", holder.SessionProcess.PID, processStatus)
	}
	orcaInventory, err := executionOrcaOwnerInventory(ctx, record, deps, processStatus)
	if err != nil {
		return "", err
	}
	if err := validateExecutionRuntimeRolloverWithProcess(record, processStatus, orcaInventory); err != nil {
		return "", err
	}
	if !deadOwnerRuntimeRollover(record, processStatus, orcaInventory) && (orcaInventory.TerminalLive || orcaInventory.TaskLive) {
		return "", fmt.Errorf("Orca owner is not quiescent: terminal_live=%t task_live=%t task_status=%s dispatch_status=%s", orcaInventory.TerminalLive, orcaInventory.TaskLive, orcaInventory.TaskStatus, orcaInventory.DispatchStatus)
	}
	inventoryOwners := map[int]bool{os.Getpid(): true}
	requesterOwners := map[int]bool{}
	if requester.SessionProcess != nil {
		inventoryOwners[requester.SessionProcess.PID] = true
		requesterOwners[requester.SessionProcess.PID] = true
	}
	excluded := map[int]bool{}
	for pid := range inventoryOwners {
		for ancestor := range nativeProcessAncestryPIDsFromSnapshot(processSnapshot, pid) {
			excluded[ancestor] = true
		}
	}
	workspaceProcesses, err := deps.workspaceInspector()(record.Execution.Workspace.Root, excluded)
	if err != nil {
		return "", err
	}
	workspaceProcesses = dropRequesterOwnedProcessesFromSnapshot(
		workspaceProcesses,
		requesterOwners,
		processSnapshot,
	)
	if len(workspaceProcesses) > 0 {
		process := workspaceProcesses[0]
		return "", fmt.Errorf("workspace process is not quiescent: pid=%d command=%s fd=%s access=%s path=%s", process.PID, process.Command, process.FD, process.Access, process.Path)
	}
	snapshot, err := workspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Holder     issueops.NativeActor             `json:"holder"`
		Requester  issueops.NativeActor             `json:"requester"`
		Process    issueops.NativeProcessReceipt    `json:"process"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, *holder, requester, *holder.SessionProcess, orcaInventory, snapshot}
	return hashJSON(payload)
}

func executionOwnerInventory(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	deps ExecutionReplaceDependencies,
	processSnapshot map[int]nativeProcessSnapshotEntry,
) (string, port.ExecutionOrcaOwnerInventory, error) {
	status := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		var err error
		status, _, err = inspectNativeProcessReceiptForRollover(*holder.SessionProcess, processSnapshot)
		if err != nil {
			return "", port.ExecutionOrcaOwnerInventory{}, err
		}
	}
	inventory, err := executionOrcaOwnerInventory(ctx, record, deps, status)
	return status, inventory, err
}

func inspectNativeProcessReceiptForRollover(
	receipt issueops.NativeProcessReceipt,
	processSnapshot map[int]nativeProcessSnapshotEntry,
) (string, issueops.NativeProcessReceipt, error) {
	if processSnapshot != nil {
		return inspectNativeProcessReceiptFromSnapshot(receipt, processSnapshot)
	}
	return inspectNativeProcessReceipt(receipt)
}

func executionOrcaOwnerInventory(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	deps ExecutionReplaceDependencies,
	status string,
) (port.ExecutionOrcaOwnerInventory, error) {
	if record.Execution.Mode != issueops.ExecutionModeOrca {
		return port.ExecutionOrcaOwnerInventory{}, nil
	}
	if record.Execution.Orca == nil || deps.OrcaOwner == nil {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca execution requires exact owner terminal and task inventory")
	}
	binding := record.Execution.Orca
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		AllowRuntimeRollover: allowExecutionRuntimeRollover(record, status),
	})
	return inventory, err
}

func allowExecutionRuntimeRollover(record issueops.IssueOpsRecord, processStatus string) bool {
	lease := record.Execution.Lease
	if lease.Holder == nil {
		return lease.Status == issueops.LeaseStatusReleased || lease.Status == issueops.LeaseStatusClaimable
	}
	// Adapter가 바뀐 runtime을 읽는 권한은 core가 확인한 exact holder process의
	// 종료 영수증에만 묶는다. lease 상태만으로 허용하면 live owner와 경쟁할 수 있다.
	return processStatus == NativeProcessStatusDead &&
		(lease.Status == issueops.LeaseStatusActive || lease.Status == issueops.LeaseStatusRevoking)
}

func deadOwnerRuntimeRollover(record issueops.IssueOpsRecord, processStatus string, inventory port.ExecutionOrcaOwnerInventory) bool {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return false
	}
	lease := record.Execution.Lease
	sealed := strings.TrimSpace(record.Execution.Orca.RuntimeID)
	observed := strings.TrimSpace(inventory.RuntimeID)
	return lease.Holder != nil && lease.Holder.SessionProcess != nil &&
		(lease.Status == issueops.LeaseStatusActive || lease.Status == issueops.LeaseStatusRevoking) &&
		processStatus == NativeProcessStatusDead && observed != "" && observed != sealed &&
		inventory.TerminalID == "" && !inventory.TerminalLive
}

func validateExecutionRuntimeRolloverWithProcess(record issueops.IssueOpsRecord, processStatus string, inventory port.ExecutionOrcaOwnerInventory) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return nil
	}
	sealed := strings.TrimSpace(record.Execution.Orca.RuntimeID)
	observed := strings.TrimSpace(inventory.RuntimeID)
	if observed == "" || observed == sealed {
		return nil
	}
	if deadOwnerRuntimeRollover(record, processStatus, inventory) {
		return nil
	}
	lease := record.Execution.Lease
	holderless := lease.Holder == nil && (lease.Status == issueops.LeaseStatusReleased || lease.Status == issueops.LeaseStatusClaimable)
	taskSettled := inventory.TaskStatus == "completed" || inventory.TaskStatus == "failed"
	dispatchSettled := inventory.DispatchStatus == "completed" || inventory.DispatchStatus == "failed" || inventory.DispatchStatus == "circuit_broken"
	if !holderless || inventory.TerminalID != "" || inventory.TerminalLive || inventory.TaskLive || !taskSettled || !dispatchSettled {
		return fmt.Errorf(
			"Orca runtime rollover owner is not quiescent: terminal_id=%s terminal_live=%t task_live=%t task_status=%s dispatch_status=%s",
			inventory.TerminalID, inventory.TerminalLive, inventory.TaskLive, inventory.TaskStatus, inventory.DispatchStatus,
		)
	}
	return nil
}
func validateExecutionReplacementCWD(record issueops.IssueOpsRecord, cwd string) error {
	workspace := record.Execution.Workspace
	if !samePath(cwd, workspace.SourceRoot) && !samePath(cwd, workspace.Root) {
		return fmt.Errorf("execution replace cwd must be source_root or the canonical worktree")
	}
	return nil
}
