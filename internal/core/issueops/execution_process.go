package issueops

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"agent-harness/internal/core/issueops/model"
)

const nativeProcessProbeTimeout = 3 * time.Second

const (
	NativeProcessStatusLive             = "live"
	NativeProcessStatusDead             = "dead"
	NativeProcessStatusIdentityMismatch = "identity-mismatch"
	NativeProcessStatusUnknown          = "unknown"
)

type workspaceProcess struct {
	PID     int
	Command string
	FD      string
	Access  string
	Path    string
}

type nativeProcessSnapshotEntry struct {
	ParentPID int
	Receipt   model.NativeProcessReceipt
}

// ObserveNativeProcessReceipt는 PID 재사용을 방지하는 데 쓰는 운영체제
// identity를 읽는다. 호출자는 자체 보고한 시간이 아니라 이 receipt를
// 그대로 영속화한다.
func ObserveNativeProcessReceipt(pid int) (model.NativeProcessReceipt, error) {
	if pid <= 0 {
		return model.NativeProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	alive, err := nativePIDAlive(pid)
	if err != nil {
		return model.NativeProcessReceipt{}, err
	}
	if !alive {
		return model.NativeProcessReceipt{}, fmt.Errorf("native process pid %d is not live", pid)
	}
	startedRaw, err := processField(pid, "lstart=")
	if err != nil {
		return model.NativeProcessReceipt{}, fmt.Errorf("read native process start identity: %w", err)
	}
	started, err := parseNativeProcessStart(startedRaw)
	if err != nil {
		return model.NativeProcessReceipt{}, fmt.Errorf("parse native process start identity: %w", err)
	}
	executable, err := processField(pid, "comm=")
	if err != nil {
		return model.NativeProcessReceipt{}, fmt.Errorf("read native process executable identity: %w", err)
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return model.NativeProcessReceipt{}, fmt.Errorf("native process executable identity is empty")
	}
	return model.NativeProcessReceipt{
		PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
	}, nil
}

// ObserveNativeProcessAncestry는 운영체제 프로세스 테이블 snapshot 하나를
// 캡처해 pid와 그 뒤로 각 부모를 반환한다. 단일 snapshot은 hook이 durable
// lease holder의 자손인지 판단하는 동안 PID/start/executable tuple을 내부적으로
// 일관되게 유지한다.
func ObserveNativeProcessAncestry(pid int) ([]model.NativeProcessReceipt, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("native process pid must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-ww", "-axo", "pid=,ppid=,lstart=,comm=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("read native process ancestry: %w", err)
	}
	snapshot, err := parseNativeProcessSnapshot(string(output))
	if err != nil {
		return nil, err
	}
	return nativeProcessAncestryFromSnapshot(snapshot, pid)
}

func parseNativeProcessSnapshot(output string) (map[int]nativeProcessSnapshotEntry, error) {
	snapshot := make(map[int]nativeProcessSnapshotEntry)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			if len(fields) == 0 {
				continue
			}
			return nil, fmt.Errorf("invalid native process snapshot row %q", scanner.Text())
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid native process snapshot pid %q", fields[0])
		}
		parentPID, err := strconv.Atoi(fields[1])
		if err != nil || parentPID < 0 {
			return nil, fmt.Errorf("invalid native process snapshot parent pid %q", fields[1])
		}
		started, err := parseNativeProcessStart(strings.Join(fields[2:7], " "))
		if err != nil {
			return nil, fmt.Errorf("parse native process snapshot start identity for pid %d: %w", pid, err)
		}
		executable := strings.TrimSpace(strings.Join(fields[7:], " "))
		if executable == "" {
			return nil, fmt.Errorf("native process snapshot executable identity is empty for pid %d", pid)
		}
		snapshot[pid] = nativeProcessSnapshotEntry{
			ParentPID: parentPID,
			Receipt: model.NativeProcessReceipt{
				PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
			},
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func nativeProcessAncestryFromSnapshot(snapshot map[int]nativeProcessSnapshotEntry, pid int) ([]model.NativeProcessReceipt, error) {
	const maxNativeProcessAncestry = 128
	ancestry := make([]model.NativeProcessReceipt, 0, 8)
	seen := make(map[int]bool)
	for len(ancestry) < maxNativeProcessAncestry {
		if seen[pid] {
			return nil, fmt.Errorf("native process ancestry contains a cycle at pid %d", pid)
		}
		seen[pid] = true
		entry, ok := snapshot[pid]
		if !ok {
			return nil, fmt.Errorf("native process ancestry is missing pid %d", pid)
		}
		ancestry = append(ancestry, entry.Receipt)
		if entry.ParentPID == 0 {
			return ancestry, nil
		}
		pid = entry.ParentPID
	}
	return nil, fmt.Errorf("native process ancestry exceeds %d entries", maxNativeProcessAncestry)
}

func parseNativeProcessStart(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if started, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return started, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid process start identity %q", value)
}

func requireExactLiveNativeProcessReceipt(receipt model.NativeProcessReceipt) error {
	status, observed, err := inspectNativeProcessReceipt(receipt)
	if err != nil {
		return err
	}
	if status != "live" {
		return fmt.Errorf("native process identity is not live: pid=%d status=%s", receipt.PID, status)
	}
	if observed.StartedAt != receipt.StartedAt || observed.Executable != receipt.Executable {
		return fmt.Errorf("native process identity does not match live PID %d", receipt.PID)
	}
	return nil
}

// InspectNativeProcessReceipt는 lease replacement이 쓰는 것과 동일한 PID
// 재사용에 안전한 read-only 관측을 운영 inventory 수집기에 노출한다.
func InspectNativeProcessReceipt(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	return inspectNativeProcessReceipt(receipt)
}

func inspectNativeProcessReceipt(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	alive, err := nativePIDAlive(receipt.PID)
	if err != nil {
		return NativeProcessStatusUnknown, model.NativeProcessReceipt{}, fmt.Errorf("inspect native process identity: %w", err)
	}
	if !alive {
		return NativeProcessStatusDead, model.NativeProcessReceipt{}, nil
	}
	observed, err := ObserveNativeProcessReceipt(receipt.PID)
	if err != nil {
		alive, retryErr := nativePIDAlive(receipt.PID)
		if retryErr == nil && !alive {
			return NativeProcessStatusDead, model.NativeProcessReceipt{}, nil
		}
		return NativeProcessStatusUnknown, model.NativeProcessReceipt{}, fmt.Errorf("inspect live native process identity: %w", err)
	}
	if observed.StartedAt != receipt.StartedAt || observed.Executable != receipt.Executable {
		return NativeProcessStatusIdentityMismatch, observed, nil
	}
	return NativeProcessStatusLive, observed, nil
}

func nativePIDAlive(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	err = process.Signal(syscall.Signal(0))
	switch {
	case err == nil, errors.Is(err, os.ErrPermission):
		return true, nil
	case errors.Is(err, os.ErrProcessDone), errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

func processField(pid int, field string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", field)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("ps returned an empty %s field", field)
	}
	return value, nil
}

// inspectWorkspaceProcesses는 워크트리를 점유한 프로세스를 관측한다. lsof에
// "+D root"를 주면 트리 전체를 재귀 stat하므로 node_modules 같은 대형 워크트리에서는
// probe 자체가 타임아웃한다(실측 6~8초). 경로 판정은 어차피
// parseWorkspaceProcesses의 pathWithinResolved가 수행하므로, 전체 open file
// 목록을 받아 Go에서 걸러 같은 결과를 훨씬 싸게 얻는다.
func inspectWorkspaceProcesses(root string, excluded map[int]bool) ([]workspaceProcess, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-nPw", "-Fpcfna")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !(errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "") {
			return nil, fmt.Errorf("lsof workspace inventory: %s", strings.TrimSpace(stderr.String()))
		}
	}
	// probe가 띄운 lsof는 호출자의 cwd를 상속하므로 워크트리 안에서 실행되면
	// 자기 자신을 점유자로 보고한다. 이미 종료한 프로세스라 조상 관측으로는
	// 걸러지지 않으므로 여기서 정확히 제외한다.
	if cmd.Process != nil {
		probeExcluded := make(map[int]bool, len(excluded)+1)
		for pid := range excluded {
			probeExcluded[pid] = true
		}
		probeExcluded[cmd.Process.Pid] = true
		excluded = probeExcluded
	}
	return parseWorkspaceProcesses(stdout.String(), root, excluded)
}

// nativeProcessAncestryPIDs는 pid와 그 조상 PID 집합을 반환한다. quiescence
// 판정에서 요청자 세션을 띄운 터미널 셸은 경쟁 writer가 아니라 요청자 자신의
// 실행 컨텍스트다 — 그 셸의 cwd가 canonical worktree라는 이유로 finalize를
// 막으면 direct 모드 승계가 구조적으로 불가능해진다. 관측에 실패하면 pid
// 하나만 반환해 기존(조상 미제외) 동작으로 안전하게 축약한다.
func nativeProcessAncestryPIDs(pid int) map[int]bool {
	pids := map[int]bool{}
	if pid <= 0 {
		return pids
	}
	pids[pid] = true
	ancestry, err := ObserveNativeProcessAncestry(pid)
	if err != nil {
		return pids
	}
	for _, receipt := range ancestry {
		pids[receipt.PID] = true
	}
	return pids
}

// dropRequesterOwnedProcesses는 요청자 세션이 직접 띄운 자손 프로세스(MCP 서버,
// 테스트 러너, 툴 셸 등)를 quiescence 후보에서 제외한다. 이들은 워크트리를 다투는
// 다른 holder가 아니라 승계를 요청한 세션 자신의 실행 컨텍스트이므로, 이를 근거로
// finalize를 막으면 direct 모드 승계가 성립하지 않는다. 외부 세션의 잔여
// 프로세스는 요청자 조상에 걸리지 않으므로 원래의 fail-closed 계약은 유지된다.
func dropRequesterOwnedProcesses(processes []workspaceProcess, owners map[int]bool) []workspaceProcess {
	if len(processes) == 0 || len(owners) == 0 {
		return processes
	}
	kept := make([]workspaceProcess, 0, len(processes))
	for _, process := range processes {
		if processHasAncestorIn(process.PID, owners) {
			continue
		}
		kept = append(kept, process)
	}
	return kept
}

// processHasAncestorIn은 pid 자신 또는 그 조상이 owners에 속하는지 본다. 관측에
// 실패하면 false를 반환해 판정을 fail-closed로 유지한다.
func processHasAncestorIn(pid int, owners map[int]bool) bool {
	ancestry, err := ObserveNativeProcessAncestry(pid)
	if err != nil {
		return false
	}
	for _, receipt := range ancestry {
		if owners[receipt.PID] {
			return true
		}
	}
	return false
}

func parseWorkspaceProcesses(output, root string, excluded map[int]bool) ([]workspaceProcess, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	var pid int
	var command, fd, access string
	result := []workspaceProcess{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		tag, value := line[0], strings.TrimSpace(line[1:])
		switch tag {
		case 'p':
			pid, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid lsof process id %q", value)
			}
			command, fd, access = "", "", ""
		case 'c':
			command = value
		case 'f':
			fd, access = value, ""
		case 'a':
			access = value
		case 'n':
			path := strings.TrimSuffix(value, " (deleted)")
			// lsof의 name 필드는 파일 경로만이 아니라 소켓/파이프/kqueue 표기
			// ("->0x...", "count=..." 등)도 담는다. 절대 경로가 아닌 값은 워크트리
			// 점유 판정 대상이 아니다 — filepath.Abs가 그런 값을 probe 프로세스의
			// cwd 기준으로 해석하면 무관한 프로세스가 워크트리 점유로 오판된다.
			if !filepath.IsAbs(path) {
				continue
			}
			if pid <= 0 || excluded[pid] || !pathWithinResolved(path, root) {
				continue
			}
			if fd == "cwd" || access == "w" || access == "u" {
				result = append(result, workspaceProcess{PID: pid, Command: command, FD: fd, Access: access, Path: path})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func pathWithinResolved(path, root string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
