package issueops

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const resetLegacyDaemonInstanceMaxBytesV1 = 64 << 10

type resetLegacyDaemonInstanceV1 struct {
	PID              int    `json:"pid"`
	ProcessStartTime string `json:"process_start_time"`
	Executable       string `json:"executable"`
	InstanceNonce    string `json:"instance_nonce"`
	BuildSHA         string `json:"build_sha"`
	ProtocolVersion  string `json:"protocol_version"`
	Generation       string `json:"generation"`
}

func liveHarnessProcessesV1(stateRoot string) ([]resetLegacyV1Process, error) {
	output, err := exec.Command("ps", "-ww", "-axo", "pid=,lstart=,command=").Output()
	if err != nil {
		return nil, err
	}
	return liveHarnessProcessesFromSnapshotV1(stateRoot, output, os.Getpid())
}

func liveHarnessProcessesFromSnapshotV1(stateRoot string, output []byte, currentPID int) ([]resetLegacyV1Process, error) {
	inventory := parseResetLegacyProcessTableV1(output, currentPID)
	selected := make([]resetLegacyV1Process, 0)
	for _, process := range inventory {
		kind, source, ok := classifyResetLegacyHarnessProcessV1(process.Command)
		if !ok {
			continue
		}
		process.Kind = kind
		process.Source = source
		selected = append(selected, process)
	}
	registered, err := registeredResetLegacyDaemonProcessesV1(stateRoot, inventory)
	if err != nil {
		return nil, err
	}
	selected = append(selected, registered...)
	return canonicalResetLegacyProcessSetV1(selected), nil
}

func parseResetLegacyProcessTableV1(output []byte, currentPID int) []resetLegacyV1Process {
	processes := make([]resetLegacyV1Process, 0)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 || pid == currentPID {
			continue
		}
		command := strings.Join(fields[6:], " ")
		processes = append(processes, resetLegacyV1Process{
			PID: pid, StartedAt: strings.Join(fields[1:6], " "), Executable: fields[6], Command: command,
		})
	}
	return processes
}

func classifyResetLegacyHarnessProcessV1(command string) (kind, source string, ok bool) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return "", "", false
	}
	if resetLegacyHarnessExecutableV1(args[0], firstResetLegacyArgumentV1(args[1:])) {
		return resetLegacyProcessKindV1(args[1:]), "direct", true
	}
	proxyMarker := -1
	proxyExecutable := false
	for i, arg := range args {
		if arg == "--" {
			proxyMarker = i
			break
		}
		if strings.EqualFold(filepath.Base(strings.Trim(arg, `"'`)), "mcp-proxy") {
			proxyExecutable = true
		}
	}
	if !proxyExecutable || proxyMarker < 0 || proxyMarker+2 >= len(args) {
		return "", "", false
	}
	target := args[proxyMarker+1]
	if resetLegacyHarnessExecutableV1(target, args[proxyMarker+2]) && strings.EqualFold(strings.Trim(args[proxyMarker+2], `"'`), "mcp") {
		return "mcp", "mcp_proxy", true
	}
	return "", "", false
}

func firstResetLegacyArgumentV1(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func resetLegacyHarnessExecutableV1(raw, firstArg string) bool {
	base := strings.ToLower(filepath.Base(strings.Trim(raw, `"'`)))
	if base == "agent-harness" {
		return true
	}
	if base != "harness" {
		return false
	}
	switch strings.ToLower(strings.Trim(firstArg, `"'`)) {
	case "daemon", "mcp", "worker", "workpool", "hook", "issueops":
		return true
	default:
		return false
	}
}

func resetLegacyProcessKindV1(args []string) string {
	if len(args) == 0 {
		return "harness"
	}
	switch strings.ToLower(strings.Trim(args[0], `"'`)) {
	case "daemon":
		return "daemon"
	case "mcp":
		return "mcp"
	case "worker", "workpool":
		return "worker"
	default:
		return "harness"
	}
}

func registeredResetLegacyDaemonProcessesV1(stateRoot string, inventory []resetLegacyV1Process) ([]resetLegacyV1Process, error) {
	paths, err := resetLegacyDaemonInstancePathsV1(stateRoot)
	if err != nil {
		return nil, err
	}
	result := make([]resetLegacyV1Process, 0, len(paths))
	for _, path := range paths {
		record, legacy, exists, err := readResetLegacyDaemonInstanceV1(path)
		if err != nil {
			return nil, fmt.Errorf("read registered daemon identity %s: %w", path, err)
		}
		if !exists {
			continue
		}
		process, live := resetLegacyProcessByPIDV1(inventory, record.PID)
		if !live {
			continue
		}
		if !legacy && (process.StartedAt != record.ProcessStartTime || !sameResetLegacyExecutableV1(process.Executable, record.Executable)) {
			continue
		}
		process.Kind = "daemon"
		if legacy {
			process.Source = "legacy_daemon_record"
		} else {
			process.StartedAt = record.ProcessStartTime
			process.Executable = record.Executable
			process.Source = "daemon_record"
		}
		result = append(result, process)
	}
	return result, nil
}

func resetLegacyDaemonInstancePathsV1(stateRoot string) ([]string, error) {
	stateRoot, err := filepath.Abs(stateRoot)
	if err != nil {
		return nil, err
	}
	dirs := []string{filepath.Join(filepath.Clean(stateRoot), "daemon")}
	if override := strings.TrimSpace(os.Getenv("HARNESS_DAEMON_DIR")); override != "" {
		override, err = filepath.Abs(override)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, filepath.Clean(override))
	}
	seen := make(map[string]struct{}, len(dirs))
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		path := filepath.Join(dir, "agent-harness.pid")
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func readResetLegacyDaemonInstanceV1(path string) (resetLegacyDaemonInstanceV1, bool, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return resetLegacyDaemonInstanceV1{}, false, false, nil
	}
	if err != nil {
		return resetLegacyDaemonInstanceV1{}, false, false, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > resetLegacyDaemonInstanceMaxBytesV1 {
		return resetLegacyDaemonInstanceV1{}, false, true, fmt.Errorf("daemon identity record is not a bounded regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return resetLegacyDaemonInstanceV1{}, false, true, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if pid, parseErr := strconv.Atoi(trimmed); parseErr == nil {
		if pid <= 0 {
			return resetLegacyDaemonInstanceV1{}, true, true, fmt.Errorf("legacy daemon PID must be positive")
		}
		return resetLegacyDaemonInstanceV1{PID: pid}, true, true, nil
	}
	var record resetLegacyDaemonInstanceV1
	if err := json.Unmarshal(raw, &record); err != nil {
		return resetLegacyDaemonInstanceV1{}, false, true, fmt.Errorf("decode daemon identity record: %w", err)
	}
	if err := validateResetLegacyDaemonInstanceV1(record); err != nil {
		return resetLegacyDaemonInstanceV1{}, false, true, err
	}
	return record, false, true, nil
}

func validateResetLegacyDaemonInstanceV1(record resetLegacyDaemonInstanceV1) error {
	if record.PID <= 0 {
		return fmt.Errorf("daemon PID must be positive")
	}
	for name, value := range map[string]string{
		"process_start_time": record.ProcessStartTime,
		"executable":         record.Executable,
		"instance_nonce":     record.InstanceNonce,
		"build_sha":          record.BuildSHA,
		"protocol_version":   record.ProtocolVersion,
		"generation":         record.Generation,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("daemon %s is required", name)
		}
	}
	if !filepath.IsAbs(record.Executable) {
		return fmt.Errorf("daemon executable must be absolute")
	}
	return nil
}

func resetLegacyProcessByPIDV1(processes []resetLegacyV1Process, pid int) (resetLegacyV1Process, bool) {
	for _, process := range processes {
		if process.PID == pid {
			return process, true
		}
	}
	return resetLegacyV1Process{}, false
}

func sameResetLegacyExecutableV1(left, right string) bool {
	left = filepath.Clean(strings.Trim(strings.TrimSpace(left), `"'`))
	right = filepath.Clean(strings.Trim(strings.TrimSpace(right), `"'`))
	if left == right {
		return true
	}
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftResolved) == filepath.Clean(rightResolved)
}

func canonicalResetLegacyProcessSetV1(processes []resetLegacyV1Process) []resetLegacyV1Process {
	sort.Slice(processes, func(i, j int) bool {
		if processes[i].PID != processes[j].PID {
			return processes[i].PID < processes[j].PID
		}
		return processes[i].Source < processes[j].Source
	})
	result := make([]resetLegacyV1Process, 0, len(processes))
	for _, process := range processes {
		if len(result) != 0 && result[len(result)-1].PID == process.PID && result[len(result)-1].StartedAt == process.StartedAt {
			if process.Source == "daemon_record" {
				result[len(result)-1] = process
			}
			continue
		}
		result = append(result, process)
	}
	return result
}
