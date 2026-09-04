package issueops

import (
	"fmt"
	"os"
	"sort"
	"syscall"
	"time"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// CleanupProcessDeps는 cleanup이 워크트리 점유 프로세스를 관측하고 종료하는
// 표면이다. nil 필드는 기본 구현(lsof+ps 스냅샷 join, syscall.Kill, time.Sleep,
// os.Getpid, os.Getenv)을 쓴다.
type CleanupProcessDeps struct {
	Observe func(root string) (port.CleanupWorkspaceOccupancy, error)
	Signal  func(pid int, sig syscall.Signal) error
	Sleep   func(time.Duration)
	SelfPID int
	Getenv  func(string) string
}

func (d CleanupProcessDeps) withDefaults() CleanupProcessDeps {
	if d.SelfPID == 0 {
		d.SelfPID = os.Getpid()
	}
	if d.Observe == nil {
		self := d.SelfPID
		d.Observe = func(root string) (port.CleanupWorkspaceOccupancy, error) {
			return observeCleanupWorkspaceOccupancy(root, self)
		}
	}
	if d.Signal == nil {
		d.Signal = syscall.Kill
	}
	if d.Sleep == nil {
		d.Sleep = time.Sleep
	}
	if d.Getenv == nil {
		d.Getenv = os.Getenv
	}
	return d
}

// cleanupObserveTimeout은 cleanup 점유 관측(lsof 전체 목록)의 상한이다. 전체
// 테스트 배터리 같은 부하에서 3초 상한이 관측 실패를 만들었다(2026-08-27 실측).
const cleanupObserveTimeout = 10 * time.Second

// observeCleanupWorkspaceOccupancy는 lsof 점유자와 하나의 ps 스냅샷을 join한다.
func observeCleanupWorkspaceOccupancy(root string, selfPID int) (port.CleanupWorkspaceOccupancy, error) {
	procs, err := inspectWorkspaceProcessesWithin(root, nil, cleanupObserveTimeout)
	if err != nil {
		return port.CleanupWorkspaceOccupancy{}, err
	}
	snapshot, err := observeNativeProcessSnapshot()
	if err != nil {
		return port.CleanupWorkspaceOccupancy{}, err
	}
	return buildCleanupOccupancy(procs, snapshot, selfPID)
}

// buildCleanupOccupancy는 점유자마다 receipt와 계보를 같은 스냅샷에서 채운다.
// 요청자 계보를 관측하지 못하면 실패다 — 제외 집합을 {self}로 축약하면 cwd가
// 워크트리인 부모 셸에 신호가 가서 하네스 자신이 HUP을 받는다(design-review 1차 finding 7).
func buildCleanupOccupancy(procs []workspaceProcess, snapshot map[int]nativeProcessSnapshotEntry, selfPID int) (port.CleanupWorkspaceOccupancy, error) {
	occupancy := port.CleanupWorkspaceOccupancy{Ancestry: map[int][]int{}}
	selfAncestry, err := cleanupAncestorPIDs(snapshot, selfPID)
	if err != nil {
		return port.CleanupWorkspaceOccupancy{}, fmt.Errorf("requester process ancestry: %w", err)
	}
	occupancy.Ancestry[selfPID] = selfAncestry
	occupantSet := map[int]bool{}
	for _, pid := range cleanupOccupantPIDs(procs) {
		entry, ok := snapshot[pid]
		if !ok {
			// lsof와 ps 사이에 종료한 단명 프로세스는 소멸로 본다. 살아 있는데
			// 스냅샷에 없으면 관측 실패다(design-review 2차 finding 9).
			alive, aliveErr := nativePIDAlive(pid)
			if aliveErr != nil || alive {
				return port.CleanupWorkspaceOccupancy{}, fmt.Errorf("workspace process %d is live but absent from the process snapshot", pid)
			}
			continue
		}
		ancestors, err := cleanupAncestorPIDs(snapshot, pid)
		if err != nil {
			return port.CleanupWorkspaceOccupancy{}, fmt.Errorf("workspace process %d ancestry: %w", pid, err)
		}
		occupancy.Ancestry[pid] = ancestors
		occupantSet[pid] = true
		occupancy.Occupants = append(occupancy.Occupants, issueops.CleanupWorkspaceProcess{
			PID: pid, Command: cleanupOccupantCommand(procs, pid), StartedAt: entry.Receipt.StartedAt, Executable: entry.Receipt.Executable,
		})
	}
	descendants, collateral := cleanupDescendantCounts(snapshot, occupantSet)
	for i := range occupancy.Occupants {
		occupant := &occupancy.Occupants[i]
		occupant.Descendants = descendants[occupant.PID]
		occupant.Collateral = collateral[occupant.PID]
	}
	return occupancy, nil
}

// cleanupOccupantPIDs는 lsof 행(pid마다 fd 여러 개)을 pid 오름차순 집합으로 줄인다.
func cleanupOccupantPIDs(procs []workspaceProcess) []int {
	seen := map[int]bool{}
	pids := []int{}
	for _, proc := range procs {
		if proc.PID > 0 && !seen[proc.PID] {
			seen[proc.PID] = true
			pids = append(pids, proc.PID)
		}
	}
	sort.Ints(pids)
	return pids
}

func cleanupOccupantCommand(procs []workspaceProcess, pid int) string {
	for _, proc := range procs {
		if proc.PID == pid && proc.Command != "" {
			return proc.Command
		}
	}
	return ""
}

// cleanupDescendantCounts는 점유자별 자손 수와 워크트리를 점유하지 않는 자손
// (부수 피해 후보) 수를 센다. 계보가 끊긴 프로세스는 세지 않는다 — 관측 실패로
// 확대하지 않는다.
func cleanupDescendantCounts(snapshot map[int]nativeProcessSnapshotEntry, occupantSet map[int]bool) (map[int]int, map[int]int) {
	descendants := map[int]int{}
	collateral := map[int]int{}
	for pid := range snapshot {
		ancestors, err := cleanupAncestorPIDs(snapshot, pid)
		if err != nil {
			continue
		}
		for _, ancestor := range ancestors {
			if !occupantSet[ancestor] {
				continue
			}
			descendants[ancestor]++
			if !occupantSet[pid] {
				collateral[ancestor]++
			}
		}
	}
	return descendants, collateral
}

// cleanupAncestorPIDs는 pid 자신을 제외한 조상 pid를 가까운 순서로 돌려준다.
func cleanupAncestorPIDs(snapshot map[int]nativeProcessSnapshotEntry, pid int) ([]int, error) {
	ancestry, err := nativeProcessAncestryFromSnapshot(snapshot, pid)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(ancestry))
	for _, receipt := range ancestry[1:] {
		out = append(out, receipt.PID)
	}
	return out, nil
}
