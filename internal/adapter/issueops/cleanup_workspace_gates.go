package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// cleanupWorkspaceObservation은 finish/abandon preview가 워크트리 점유와 Orca
// 터미널을 한 번 관측한 결과다. Receipts와 Terminals는 fingerprint 입력이다.
type cleanupWorkspaceObservation struct {
	Occupants    []issueops.CleanupWorkspaceProcess
	Receipts     []issueops.NativeProcessReceipt
	Terminals    []string
	RuntimeReady bool
	AppPID       int
}

// cleanupWorkspaceGatesForRecord는 finish/abandon이 공유하는 워크트리 게이트다.
// 소스 체크아웃은 관측 전에 거부하고, Orca 바인딩 여부를 레코드에서 읽는다.
func cleanupWorkspaceGatesForRecord(ctx context.Context, record issueops.IssueOpsRecord, root string, processes CleanupProcessDeps, orca port.CleanupOrcaTerminals) (cleanupWorkspaceObservation, []string) {
	sourceCheckout, pathErr := cleanupStrictSamePath(root, record.Repo)
	if pathErr != nil || sourceCheckout {
		return cleanupWorkspaceObservation{}, []string{"worktree_is_source_checkout"}
	}
	bound := record.Execution != nil && record.Execution.Orca != nil && strings.TrimSpace(record.Execution.Orca.WorktreeID) != ""
	return cleanupWorkspaceGates(ctx, root, bound, processes, orca)
}

// cleanupWorkspaceGates는 점유 관측·요청자 게이트·Orca 터미널 인벤토리를
// 평가한다(#477). 점유 자체는 게이트를 막지 않는다 — apply ①′의 종료 대상이다.
//
// 요청자 보호는 제외가 아니라 거부다. lease 경로는 요청자 자손을 quiescence
// 후보에서 *제외*하지만(직접 승계가 성립해야 하므로), cleanup은 워크트리를
// 지우므로 요청자가 그 안에 서 있으면 진행 자체가 잘못이다.
func cleanupWorkspaceGates(ctx context.Context, root string, bound bool, processes CleanupProcessDeps, orca port.CleanupOrcaTerminals) (cleanupWorkspaceObservation, []string) {
	processes = processes.withDefaults()
	occupancy, err := processes.Observe(root)
	if err != nil {
		return cleanupWorkspaceObservation{}, []string{"workspace_processes_observable"}
	}
	missing, ok := cleanupRequesterOccupancyMissing(occupancy, processes.SelfPID)
	if !ok {
		return cleanupWorkspaceObservation{}, []string{"workspace_processes_observable"}
	}
	observation := cleanupWorkspaceObservation{Occupants: occupancy.Occupants, Receipts: cleanupOccupantReceipts(occupancy.Occupants)}
	requester := cleanupRequesterEnv{
		paneKey: strings.TrimSpace(processes.Getenv("ORCA_PANE_KEY")),
		handle:  strings.TrimSpace(processes.Getenv("ORCA_TERMINAL_HANDLE")),
	}
	orcaMissing := cleanupOrcaGates(ctx, root, bound, requester, orca, &observation)
	return observation, append(missing, orcaMissing...)
}

// cleanupRequesterOccupancyMissing은 요청자 계보가 관측됐는지(ok)와 요청자 또는
// 그 조상이 점유 중인지를 판정한다.
func cleanupRequesterOccupancyMissing(occupancy port.CleanupWorkspaceOccupancy, selfPID int) ([]string, bool) {
	selfAncestry, ok := occupancy.Ancestry[selfPID]
	if !ok {
		return nil, false
	}
	protected := map[int]bool{selfPID: true}
	for _, pid := range selfAncestry {
		protected[pid] = true
	}
	for _, occupant := range occupancy.Occupants {
		if protected[occupant.PID] {
			return []string{"requester_occupies_worktree"}, true
		}
	}
	return []string{}, true
}

// cleanupRequesterEnv는 요청자 터미널을 확정하는 env다. 무선택자
// `orca terminal show`는 호출자가 아니라 UI-active 터미널을 돌려주므로(2026-08-27
// 실측) env를 전체 터미널 인벤토리와 join해서만 요청자 행을 찾는다.
type cleanupRequesterEnv struct {
	paneKey string
	handle  string
}

func (env cleanupRequesterEnv) hosted() bool { return env.paneKey != "" || env.handle != "" }

// cleanupOrcaGates는 Orca 런타임 상태·요청자 터미널·워크트리 터미널 인벤토리를
// 평가하고 observation의 Terminals/RuntimeReady/AppPID를 채운다.
func cleanupOrcaGates(ctx context.Context, root string, bound bool, requester cleanupRequesterEnv, orca port.CleanupOrcaTerminals, observation *cleanupWorkspaceObservation) []string {
	missing := []string{}
	if orca == nil {
		// Orca 표면이 배선되지 않으면 apply도 exact terminal close를 부르지 않으므로
		// 요청자 터미널 게이트가 막아야 할 터미널 단위 종료가 없다. 요청자 보호는
		// pid-조상 게이트로 남고, Orca 바인딩 사이클은 여전히 런타임을 요구한다.
		if bound {
			missing = append(missing, "orca_runtime_ready")
		}
		return missing
	}
	// 런타임이 ready면 점유·바인딩·호스팅과 무관하게 워크트리 터미널을 나열한다.
	// cwd를 옮긴 셸은 점유자가 아니어도 Orca 레지스트리에는 워크트리 터미널로
	// 남아 있고, apply ①′가 닫아야 한다(AC-01).
	status, err := orca.Status(ctx)
	runtimeReady := err == nil && status.RuntimeReachable && status.RuntimeState == "ready"
	ready := runtimeReady && (!bound || status.GraphState == "ready")
	observation.RuntimeReady = ready
	observation.AppPID = status.AppPID
	if requester.hosted() {
		missing = append(missing, cleanupRequesterTerminalMissing(ctx, root, requester, ready, orca)...)
	}
	switch {
	case ready:
		rows, listErr := orca.ListWorktreeTerminalsByPath(ctx, root)
		if listErr != nil {
			missing = append(missing, "orca_terminals_observable")
		} else {
			handles, handlesErr := cleanupTerminalHandles(root, rows)
			if handlesErr != nil {
				missing = append(missing, "orca_terminals_observable")
			} else {
				observation.Terminals = handles
			}
		}
	case bound:
		// Orca 바인딩 사이클은 런타임 없이 터미널을 죽이면 ②(orca 회수)가 확실히
		// 실패하는 파괴적 부분 apply가 된다(brooks 2차 finding 11).
		missing = append(missing, "orca_runtime_ready")
	}
	return missing
}

func cleanupRequesterTerminalMissing(ctx context.Context, root string, requester cleanupRequesterEnv, ready bool, orca port.CleanupOrcaTerminals) []string {
	if !ready {
		return []string{"requester_terminal_unresolved"}
	}
	rows, err := orca.ListAllTerminals(ctx)
	if err != nil {
		return []string{"requester_terminal_unresolved"}
	}
	row, found := cleanupRequesterTerminal(rows, requester.paneKey, requester.handle)
	worktreePath := strings.TrimSpace(row.WorktreePath)
	switch {
	case !found:
		return []string{"requester_terminal_unresolved"}
	case worktreePath == "":
		return []string{"requester_terminal_unresolved"}
	}
	same, err := cleanupStrictSamePath(worktreePath, root)
	switch {
	case err != nil:
		return []string{"requester_terminal_unresolved"}
	case same:
		return []string{"requester_terminal_outside_worktree"}
	}
	return nil
}

// cleanupRequesterTerminal은 pane key(tabId:leafId)를 우선하고 handle을 보조로
// 써서 요청자 터미널 행을 찾는다. 런타임 롤오버 뒤 spawn-time handle은 stale해질
// 수 있지만 tab/leaf 조합은 현재 pane을 계속 식별한다(cautions §149).
func cleanupRequesterTerminal(rows []port.OrcaTerminal, paneKey, handle string) (port.OrcaTerminal, bool) {
	if paneKey != "" {
		var matched port.OrcaTerminal
		count := 0
		for _, row := range rows {
			if row.TabID != "" && row.LeafID != "" && row.TabID+":"+row.LeafID == paneKey {
				matched = row
				count++
			}
		}
		if count > 0 {
			return matched, count == 1
		}
	}
	if handle != "" {
		var matched port.OrcaTerminal
		count := 0
		for _, row := range rows {
			if row.Handle == handle {
				matched = row
				count++
			}
		}
		if count > 0 {
			return matched, count == 1
		}
	}
	return port.OrcaTerminal{}, false
}

func cleanupStrictSamePath(left, right string) (bool, error) {
	canonical := func(path string) (string, error) {
		path = strings.TrimSpace(path)
		if path == "" {
			return "", fmt.Errorf("path is empty")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", err
		}
		return filepath.Clean(resolved), nil
	}
	leftPath, err := canonical(left)
	if err != nil {
		return false, err
	}
	rightPath, err := canonical(right)
	if err != nil {
		return false, err
	}
	return leftPath == rightPath, nil
}

func cleanupOccupantReceipts(occupants []issueops.CleanupWorkspaceProcess) []issueops.NativeProcessReceipt {
	receipts := make([]issueops.NativeProcessReceipt, 0, len(occupants))
	for _, occupant := range occupants {
		receipts = append(receipts, issueops.NativeProcessReceipt{PID: occupant.PID, StartedAt: occupant.StartedAt, Executable: occupant.Executable})
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].PID < receipts[j].PID })
	return receipts
}

func cleanupTerminalHandles(root string, rows []port.OrcaTerminal) ([]string, error) {
	handles := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		handle := strings.TrimSpace(row.Handle)
		worktreePath := strings.TrimSpace(row.WorktreePath)
		sameWorktree, pathErr := cleanupStrictSamePath(worktreePath, root)
		if handle == "" || pathErr != nil || !sameWorktree || seen[handle] {
			return nil, fmt.Errorf("malformed Orca terminal inventory")
		}
		seen[handle] = true
		handles = append(handles, handle)
	}
	sort.Strings(handles)
	return handles, nil
}

// cleanupStopWorkspace는 apply ①′다: preview가 나열한 Orca 터미널이 있으면
// exact handle별 `orca terminal close`를 먼저 부르고, 남은 점유자를 receipt 결속
// 아래 HUP/TERM/KILL로 종료한다. 터미널 stop 실패는 fail-closed다 — 시그널
// 경로로 넘어가면 터미널은 죽고 Orca 회수는 실패하는 파괴적 부분 apply가 된다.
// Orca 상태(ready, app pid)는 apply 직전 게이트 재평가가 fingerprint로 고정했으므로
// 여기서 다시 묻지 않는다; 런타임이 사라졌다면 fingerprint가 이미 어긋난다.
func cleanupStopWorkspace(ctx context.Context, root string, occupants []issueops.CleanupWorkspaceProcess, terminals []string, orcaRuntimeReady bool, appPID int, processes CleanupProcessDeps, orca port.CleanupOrcaTerminals) ([]issueops.CleanupWorkspaceProcess, int, error) {
	excluded := map[int]bool{}
	if appPID > 0 {
		excluded[appPID] = true
	}
	terminalsStopped := 0
	if len(terminals) > 0 {
		if orca == nil {
			return nil, 0, fmt.Errorf("orca terminals %v were previewed but no orca surface is wired", terminals)
		}
		for _, handle := range terminals {
			if err := orca.CloseTerminal(ctx, handle); err != nil {
				return nil, terminalsStopped, fmt.Errorf("close Orca terminal %s for %s: %w", handle, root, err)
			}
			terminalsStopped++
		}
		if err := cleanupRequireNoOrcaTerminals(ctx, root, orca); err != nil {
			return nil, terminalsStopped, err
		}
	}
	stopped, err := stopCleanupWorkspaceProcesses(root, occupants, excluded, processes)
	if err != nil {
		return stopped, terminalsStopped, err
	}
	if orcaRuntimeReady {
		if orca == nil {
			return stopped, terminalsStopped, fmt.Errorf("Orca runtime was previewed ready but no Orca surface is wired")
		}
		if err := cleanupRequireNoOrcaTerminals(ctx, root, orca); err != nil {
			return stopped, terminalsStopped, err
		}
	}
	return stopped, terminalsStopped, nil
}

func cleanupRequireNoOrcaTerminals(ctx context.Context, root string, orca port.CleanupOrcaTerminals) error {
	remaining, err := orca.ListWorktreeTerminalsByPath(ctx, root)
	if err != nil {
		return fmt.Errorf("observe Orca terminals before deletion for %s: %w", root, err)
	}
	handles, err := cleanupTerminalHandles(root, remaining)
	if err != nil {
		return fmt.Errorf("observe Orca terminals before deletion for %s: %w", root, err)
	}
	if len(handles) > 0 {
		return fmt.Errorf("Orca terminals remain before deletion for %s: %v", root, handles)
	}
	return nil
}
