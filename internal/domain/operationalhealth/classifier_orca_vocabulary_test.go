package operationalhealth

import (
	"testing"
	"time"
)

// dispatch 인벤토리 판정이 dispatched 하나만 유효로 보아 orca의 나머지 네 어휘를
// "unsupported"로 보고했다. 유효한 상태가 unknown으로 오면 두 신호가 함께
// 망가진다 — 진짜 미지 값이 묻히고, 정상 상태가 해석 불가로 보인다.
//
// 어휘는 orca 공개 소스가 확정한다(이슈 #171):
//
//	src/main/runtime/orchestration/types.ts:15
//	export type DispatchStatus = 'pending' | 'dispatched' | 'completed' | 'failed' | 'circuit_broken'
//
// 사이클이 참조하지 않는 dispatch로 검증한다. 사이클이 소유한 dispatch는
// classifier.go의 cycle-dispatch 일치 검사도 함께 타는데, 그 검사는 어휘가
// 아니라 "살아 있는 홀더의 dispatch는 dispatched여야 한다"는 별개 계약이다.
func TestClassifyKnowsEveryOrcaDispatchStatus(t *testing.T) {
	for _, status := range []string{"pending", "dispatched", "completed", "failed", "circuit_broken"} {
		t.Run(status, func(t *testing.T) {
			snapshot := orcaSnapshotWithUnownedDispatch(t, status)
			result := Classify(snapshot, Options{Now: time.Now()})
			if hasFindingID(result, FindingInventoryUnknown, "dispatch", "unowned-dispatch") {
				t.Fatalf("유효 어휘 %q를 unsupported로 분류하면 진짜 미지 값이 묻힌다: %#v", status, result.Findings)
			}
		})
	}
}

// 어휘를 넓히는 것이 fail-closed를 푸는 것이어서는 안 된다. orca가 상태를
// 추가하면 그 값은 다시 보수적으로 처리된다.
func TestClassifyStillFlagsUnknownDispatchStatus(t *testing.T) {
	snapshot := orcaSnapshotWithUnownedDispatch(t, "quantum_superposition")
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFindingID(result, FindingInventoryUnknown, "dispatch", "unowned-dispatch") {
		t.Fatalf("어휘 밖 값은 여전히 unknown이어야 한다: %#v", result.Findings)
	}
}

// 종결된 dispatch는 잔여물이 아니라 이력이다. task 처리가 같은 이유로 settled를
// 건너뛴다(classifier.go의 "A settled task holds no resource" 주석). orca에
// per-dispatch 삭제 명령이 없으므로 owner를 요구하면 끝난 dispatch가 영원히
// residue로 보고된다.
func TestClassifySkipsResidueForSettledDispatch(t *testing.T) {
	for _, status := range []string{"completed", "failed", "circuit_broken"} {
		t.Run(status, func(t *testing.T) {
			snapshot := orcaSnapshotWithUnownedDispatch(t, status)
			result := Classify(snapshot, Options{Now: time.Now()})
			if hasFindingID(result, FindingTaskResidue, "dispatch", "unowned-dispatch") {
				t.Fatalf("종결된 dispatch를 residue로 보면 정리가 수렴하지 않는다: %#v", result.Findings)
			}
		})
	}
}

// 살아 있는 dispatch는 소유자를 요구한다. 종결 판정이 그 검출력까지 삼키면
// 주인 없는 자원을 놓친다.
func TestClassifyStillFlagsLiveDispatchWithoutOwner(t *testing.T) {
	for _, status := range []string{"pending", "dispatched"} {
		t.Run(status, func(t *testing.T) {
			snapshot := orcaSnapshotWithUnownedDispatch(t, status)
			result := Classify(snapshot, Options{Now: time.Now()})
			if !hasFindingID(result, FindingTaskResidue, "dispatch", "unowned-dispatch") {
				t.Fatalf("살아 있는 dispatch에 소유자가 없으면 residue다: %#v", result.Findings)
			}
		})
	}
}

// GateStatus는 셋이다 — types.ts:17
//
//	export type GateStatus = 'pending' | 'resolved' | 'timeout'
func TestClassifyKnowsEveryOrcaGateStatus(t *testing.T) {
	for _, status := range []string{"pending", "resolved", "timeout"} {
		t.Run(status, func(t *testing.T) {
			snapshot := orcaSnapshotWithUnownedDispatch(t, "dispatched")
			snapshot.Gates = []OrcaGate{{RuntimeID: "runtime", ID: "gate-id", Status: status}}
			result := Classify(snapshot, Options{Now: time.Now()})
			if hasFinding(result, FindingInventoryUnknown, "gate") {
				t.Fatalf("유효 gate 어휘 %q를 unsupported로 분류하면 안 된다: %#v", status, result.Findings)
			}
		})
	}
}

// 종결된 gate는 task/dispatch와 같은 orchestration 이력이다. Orca에는
// per-gate 삭제 명령이 없으므로 resolved/timeout까지 residue로 분류하면
// 정상 정리 후에도 doctor가 영원히 healthy로 수렴하지 않는다.
func TestClassifySkipsResidueForSettledGate(t *testing.T) {
	for _, status := range []string{"resolved", "timeout"} {
		t.Run(status, func(t *testing.T) {
			snapshot := orcaSnapshotWithUnownedDispatch(t, "dispatched")
			snapshot.Gates = []OrcaGate{{RuntimeID: "runtime", ID: "gate-id", Status: status}}
			result := Classify(snapshot, Options{Now: time.Now()})
			if hasFindingID(result, FindingGateResidue, "gate", "gate-id") {
				t.Fatalf("종결된 gate를 residue로 보면 정리가 수렴하지 않는다: %#v", result.Findings)
			}
		})
	}
}

func TestClassifyStillFlagsPendingGateResidue(t *testing.T) {
	snapshot := orcaSnapshotWithUnownedDispatch(t, "dispatched")
	snapshot.Gates = []OrcaGate{{RuntimeID: "runtime", ID: "gate-id", Status: "pending"}}
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFindingID(result, FindingGateResidue, "gate", "gate-id") {
		t.Fatalf("미결 gate는 residue여야 한다: %#v", result.Findings)
	}
}

func TestClassifyStillFlagsUnknownGateStatus(t *testing.T) {
	snapshot := orcaSnapshotWithUnownedDispatch(t, "dispatched")
	snapshot.Gates = []OrcaGate{{RuntimeID: "runtime", ID: "gate-id", Status: "escalated"}}
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFinding(result, FindingInventoryUnknown, "gate") {
		t.Fatalf("어휘 밖 gate 값은 여전히 unknown이어야 한다: %#v", result.Findings)
	}
}

// orcaSnapshotWithUnownedDispatch는 정상 orca 사이클 하나와, 그 사이클이
// 참조하지 않는 dispatch 하나를 관측한 상태를 만든다. 후자의 상태 값만
// 호출자가 정하므로 실패 원인이 그 값 하나로 좁혀진다.
func orcaSnapshotWithUnownedDispatch(t *testing.T, dispatchStatus string) Snapshot {
	t.Helper()
	snapshot := healthyDirectSnapshot()
	cycle := &snapshot.Cycles[0]
	cycle.ExecutionMode = "orca"
	cycle.OrcaRuntimeID = "runtime"
	cycle.OrcaRepoID = "repo-id"
	cycle.OrcaWorktreeID = "worktree-id"
	cycle.OrcaOwnerHost = "codex"
	cycle.TaskID = "task-id"
	cycle.DispatchID = "dispatch-id"
	snapshot.OrcaObserved = true
	snapshot.OrcaRuntimeID = "runtime"
	snapshot.OrcaRepoID = "repo-id"
	snapshot.OrcaWorktrees = []OrcaWorktree{
		{RuntimeID: "runtime", RepoID: "repo-id", ID: "main-id", InstanceID: "main-instance", Repo: "/repo", Path: "/repo", Branch: "main", Head: snapshot.SourceHead},
		{RuntimeID: "runtime", RepoID: "repo-id", ID: "worktree-id", InstanceID: "observed-instance", Repo: "/repo", Path: cycle.WorktreePath, Branch: cycle.Branch, Head: snapshot.SourceHead},
	}
	snapshot.Terminals = []OrcaTerminal{{RuntimeID: "runtime", Handle: "terminal", PTYID: "observed-pty", WorktreeID: "worktree-id", WorktreePath: cycle.WorktreePath, Connected: true, Writable: true}}
	snapshot.Tasks = []OrcaTask{{RuntimeID: "runtime", ID: "task-id", Status: "dispatched", DispatchID: "dispatch-id"}}
	snapshot.Dispatches = []OrcaDispatch{
		{RuntimeID: "runtime", ID: "dispatch-id", TaskID: "task-id", AssigneeHandle: "terminal", Status: "dispatched"},
		{RuntimeID: "runtime", ID: "unowned-dispatch", TaskID: "task-id", AssigneeHandle: "terminal", Status: dispatchStatus},
	}
	snapshot.Messages = MessagePresence{RuntimeID: "runtime", Empty: true, CompleteAbsence: true}
	return snapshot
}

func hasFindingID(result Result, code, kind, id string) bool {
	for _, finding := range result.Findings {
		if finding.Code == code && finding.ResourceKind == kind && finding.ResourceID == id {
			return true
		}
	}
	return false
}
