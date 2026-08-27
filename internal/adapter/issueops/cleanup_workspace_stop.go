package issueops

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"

	"agent-harness/internal/contract/issueops"
)

const (
	// cleanupStopGracePeriod는 HUP+TERM 뒤 KILL 전까지 점유 해제를 기다리는 상한이다.
	cleanupStopGracePeriod = 5 * time.Second
	// cleanupStopKillPeriod는 KILL 뒤 점유 해제를 기다리는 상한이다.
	cleanupStopKillPeriod   = 2 * time.Second
	cleanupStopPollInterval = 250 * time.Millisecond
)

// errCleanupOccupancyChanged는 preview에 없던 점유자가 apply 시점에 나타났다는
// 뜻이다. preview receipt에 결속되지 않은 프로세스에는 신호를 보내지 않는다.
var errCleanupOccupancyChanged = errors.New("workspace occupancy changed since preview")

func cleanupReceiptMatches(left, right issueops.CleanupWorkspaceProcess) bool {
	return left.PID == right.PID && left.StartedAt == right.StartedAt && left.Executable == right.Executable
}

// cleanupStopTargets는 apply 시점 점유자 가운데 preview receipt에 결속된 것만
// 신호 대상으로 확정한다. 요청자 자신·조상·명시 제외 pid가 점유 중이면 거부다.
func cleanupStopTargets(root string, preview []issueops.CleanupWorkspaceProcess, excluded map[int]bool, deps CleanupProcessDeps) ([]issueops.CleanupWorkspaceProcess, error) {
	occupancy, err := deps.Observe(root)
	if err != nil {
		return nil, err
	}
	selfAncestry, ok := occupancy.Ancestry[deps.SelfPID]
	if !ok {
		return nil, fmt.Errorf("requester process %d ancestry is unobservable", deps.SelfPID)
	}
	protected := map[int]bool{deps.SelfPID: true}
	for _, pid := range selfAncestry {
		protected[pid] = true
	}
	for pid := range excluded {
		protected[pid] = true
	}
	previewByPID := map[int]issueops.CleanupWorkspaceProcess{}
	for _, item := range preview {
		previewByPID[item.PID] = item
	}
	currentByPID := map[int]issueops.CleanupWorkspaceProcess{}
	for _, occupant := range occupancy.Occupants {
		currentByPID[occupant.PID] = occupant
	}
	targets := make([]issueops.CleanupWorkspaceProcess, 0, len(occupancy.Occupants))
	for _, occupant := range occupancy.Occupants {
		if protected[occupant.PID] {
			return nil, fmt.Errorf("requester process %d (%s) occupies the worktree; cleanup refuses to signal its own session", occupant.PID, occupant.Command)
		}
		if !cleanupOccupantBound(occupant, previewByPID, currentByPID, occupancy.Ancestry[occupant.PID]) {
			return nil, fmt.Errorf("%w: pid=%d command=%s started_at=%s", errCleanupOccupancyChanged, occupant.PID, occupant.Command, occupant.StartedAt)
		}
		targets = append(targets, occupant)
	}
	return targets, nil
}

// cleanupOccupantBound는 점유자가 preview receipt와 같거나, preview 점유자의
// 증명된 자손이면서 그 조상이 지금도 같은 receipt로 점유 중일 때 참이다. 자손
// 관계는 stale 허용 근거일 뿐 종료 범위 확장이 아니다 — 점유하지 않는 자손에는
// 신호를 보내지 않는다(brooks 2차 finding 2).
func cleanupOccupantBound(occupant issueops.CleanupWorkspaceProcess, previewByPID, currentByPID map[int]issueops.CleanupWorkspaceProcess, ancestors []int) bool {
	if previewed, ok := previewByPID[occupant.PID]; ok && cleanupReceiptMatches(previewed, occupant) {
		return true
	}
	for _, ancestor := range ancestors {
		previewed, ok := previewByPID[ancestor]
		if !ok {
			continue
		}
		if current, live := currentByPID[ancestor]; live && cleanupReceiptMatches(previewed, current) {
			return true
		}
	}
	return false
}

// stopCleanupWorkspaceProcesses는 apply ①′다: HUP과 TERM(대화형 셸은 TERM을
// 무시하고 HUP에 종료한다 — 2026-08-27 실측) → 점유 해제 폴링 → 생존자 KILL →
// 최종 재관측으로 점유 0을 증명한다. 생존 판정은 kill(pid,0)이 아니라 점유
// 재관측이다(좀비는 점유하지 않는다).
func stopCleanupWorkspaceProcesses(root string, preview []issueops.CleanupWorkspaceProcess, excluded map[int]bool, deps CleanupProcessDeps) ([]issueops.CleanupWorkspaceProcess, error) {
	deps = deps.withDefaults()
	targets, err := cleanupStopTargets(root, preview, excluded, deps)
	if err != nil || len(targets) == 0 {
		return nil, err
	}
	for _, target := range targets {
		cleanupSignal(deps, target.PID, syscall.SIGHUP)
		cleanupSignal(deps, target.PID, syscall.SIGTERM)
	}
	remaining, err := cleanupAwaitRelease(root, targets, cleanupStopGracePeriod, deps)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 {
		for _, target := range remaining {
			cleanupSignal(deps, target.PID, syscall.SIGKILL)
		}
		if remaining, err = cleanupAwaitRelease(root, remaining, cleanupStopKillPeriod, deps); err != nil {
			return nil, err
		}
	}
	final, err := deps.Observe(root)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 || len(final.Occupants) > 0 {
		return nil, fmt.Errorf("workspace processes still occupy %s after HUP/TERM/KILL: %s", root, describeCleanupProcesses(final.Occupants))
	}
	return targets, nil
}

func cleanupSignal(deps CleanupProcessDeps, pid int, sig syscall.Signal) {
	// ESRCH는 이미 사라진 대상이다. 그 밖의 오류도 여기서 판정하지 않는다 —
	// 판정은 재관측이 한다.
	_ = deps.Signal(pid, sig)
}

// cleanupAwaitRelease는 budget 안에서 targets의 점유 해제를 폴링하고 남은
// 대상을 돌려준다. 경과 시간은 폴링 횟수로 계산해 fake Sleep에서도 결정적이다.
func cleanupAwaitRelease(root string, targets []issueops.CleanupWorkspaceProcess, budget time.Duration, deps CleanupProcessDeps) ([]issueops.CleanupWorkspaceProcess, error) {
	for waited := time.Duration(0); ; waited += cleanupStopPollInterval {
		occupancy, err := deps.Observe(root)
		if err != nil {
			return nil, err
		}
		current := map[int]issueops.CleanupWorkspaceProcess{}
		for _, occupant := range occupancy.Occupants {
			current[occupant.PID] = occupant
		}
		remaining := targets[:0:0]
		for _, target := range targets {
			if occupant, ok := current[target.PID]; ok && cleanupReceiptMatches(occupant, target) {
				remaining = append(remaining, target)
			}
		}
		if len(remaining) == 0 || waited >= budget {
			return remaining, nil
		}
		deps.Sleep(cleanupStopPollInterval)
	}
}

func describeCleanupProcesses(processes []issueops.CleanupWorkspaceProcess) string {
	if len(processes) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(processes))
	for _, process := range processes {
		parts = append(parts, fmt.Sprintf("%d:%s", process.PID, process.Command))
	}
	return strings.Join(parts, ", ")
}
