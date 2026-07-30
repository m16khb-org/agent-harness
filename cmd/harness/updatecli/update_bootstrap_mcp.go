package updatecli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"agent-harness/cmd/harness/daemoncli/daemonpaths"
)

var postInstallMCPProxyRefresh = refreshRunningMCPProxiesAfterInstall
var mcpProxyProcessLister = listMCPProxyProcesses
var mcpProxyTerminator = terminateMCPProxyProcess
var mcpProxyOrphanTerminationSupported = func() bool { return runtime.GOOS == "darwin" }

type mcpProxyProcess struct {
	PID              int
	ParentPID        int
	Command          string
	StartTime        string
	Executable       string
	IdentityVerified bool
}

type MCPCleanupProcess struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Action  string `json:"action"`
}

type MCPCleanupResult struct {
	OK         bool                `json:"ok"`
	DryRun     bool                `json:"dry_run"`
	Matched    int                 `json:"matched"`
	Terminated int                 `json:"terminated"`
	Processes  []MCPCleanupProcess `json:"processes"`
	Message    string              `json:"message,omitempty"`
}

func refreshRunningMCPProxiesAfterInstall() (int, error) {
	// 업데이트는 host가 소유한 stdio 수명을 보존한다. 새 daemon generation은
	// 살아 있는 proxy가 세션 초기화를 재생해 채택하며, 프로세스 정리는 명시적
	// `mcp cleanup` 경계에서만 수행한다.
	return 0, nil
}

func cleanupMCPProxies(dryRun bool) (MCPCleanupResult, error) {
	processes, err := mcpProxyProcessLister()
	if err != nil {
		return MCPCleanupResult{OK: false, DryRun: dryRun}, err
	}
	currentPID := os.Getpid()
	result := MCPCleanupResult{
		OK:        true,
		DryRun:    dryRun,
		Matched:   len(processes),
		Processes: []MCPCleanupProcess{},
	}
	for _, process := range processes {
		cleanupProcess := MCPCleanupProcess{
			PID:     process.PID,
			Command: process.Command,
		}
		switch {
		case process.PID == currentPID:
			cleanupProcess.Action = "skip-current"
		case !process.IdentityVerified || process.StartTime == "" || process.Executable == "":
			cleanupProcess.Action = "skip-unverified"
		case process.Command != process.Executable+" mcp":
			cleanupProcess.Action = "skip-not-exact"
		case process.ParentPID != 1:
			cleanupProcess.Action = "skip-live-parent"
		case !mcpProxyOrphanTerminationSupported():
			cleanupProcess.Action = "skip-unsupported-platform"
		case dryRun:
			cleanupProcess.Action = "would-terminate"
		default:
			freshProcesses, err := mcpProxyProcessLister()
			if err != nil {
				cleanupProcess.Action = "skip-revalidation-error"
				result.OK = false
				result.Processes = append(result.Processes, cleanupProcess)
				return result, err
			}
			fresh, found := findMCPProxyProcess(freshProcesses, process.PID)
			if !found || !sameMCPProxyProcessIdentity(process, fresh) ||
				fresh.ParentPID != 1 || !fresh.IdentityVerified ||
				fresh.Command != fresh.Executable+" mcp" {
				cleanupProcess.Action = "skip-identity-changed"
				break
			}
			if err := mcpProxyTerminator(process.PID); err != nil {
				cleanupProcess.Action = "terminate-error"
				result.OK = false
				result.Processes = append(result.Processes, cleanupProcess)
				return result, err
			}
			cleanupProcess.Action = "terminated"
			result.Terminated++
		}
		result.Processes = append(result.Processes, cleanupProcess)
	}
	if dryRun {
		result.Message = "dry-run: no MCP proxy processes terminated"
	} else {
		result.Message = "MCP proxy cleanup complete"
	}
	return result, nil
}

func listMCPProxyProcesses() ([]mcpProxyProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("MCP proxy inventory failed: %w", err)
	}
	binary := filepath.Join(deps.HarnessRoot(), "bin", "agent-harness")
	canonicalBinary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP proxy binary: %w", err)
	}
	var processes []mcpProxyProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseMCPProxyProcessSnapshot(line, canonicalBinary)
		if !ok {
			continue
		}
		identity, identityErr := daemonpaths.InspectProcess(process.PID)
		if identityErr == nil {
			process.StartTime = identity.StartTime
			process.Executable = identity.Executable
			process.IdentityVerified = identity.Executable == canonicalBinary
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func parseMCPProxyProcess(line, binary string) (mcpProxyProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return mcpProxyProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return mcpProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return mcpProxyProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	if command != binary+" mcp" {
		return mcpProxyProcess{}, false
	}
	return mcpProxyProcess{PID: pid, Command: command}, true
}

func parseMCPProxyProcessSnapshot(line, binary string) (mcpProxyProcess, bool) {
	line = strings.TrimSpace(line)
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return mcpProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return mcpProxyProcess{}, false
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil || parentPID < 0 {
		return mcpProxyProcess{}, false
	}
	command := strings.Join(fields[2:], " ")
	if command != binary+" mcp" {
		return mcpProxyProcess{}, false
	}
	return mcpProxyProcess{PID: pid, ParentPID: parentPID, Command: command}, true
}

func findMCPProxyProcess(processes []mcpProxyProcess, pid int) (mcpProxyProcess, bool) {
	for _, process := range processes {
		if process.PID == pid {
			return process, true
		}
	}
	return mcpProxyProcess{}, false
}

func sameMCPProxyProcessIdentity(left, right mcpProxyProcess) bool {
	return left.PID == right.PID &&
		left.ParentPID == right.ParentPID &&
		left.Command == right.Command &&
		left.StartTime == right.StartTime &&
		left.Executable == right.Executable &&
		left.IdentityVerified == right.IdentityVerified
}

func terminateMCPProxyProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
