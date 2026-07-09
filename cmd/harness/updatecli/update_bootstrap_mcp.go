package updatecli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var postInstallMCPProxyRefresh = refreshRunningMCPProxiesAfterInstall
var mcpProxyProcessLister = listMCPProxyProcesses
var mcpProxyTerminator = terminateMCPProxyProcess

type mcpProxyProcess struct {
	PID     int
	Command string
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
	processes, err := mcpProxyProcessLister()
	if err != nil {
		return 0, err
	}
	currentPID := os.Getpid()
	terminated := 0
	for _, process := range processes {
		if process.PID == currentPID {
			continue
		}
		if err := mcpProxyTerminator(process.PID); err != nil {
			return terminated, err
		}
		terminated++
	}
	return terminated, nil
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
		case dryRun:
			cleanupProcess.Action = "would-terminate"
		default:
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
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		// ps may be unavailable in sandboxed environments; treat as no matching processes.
		return nil, nil
	}
	binary := filepath.Join(deps.HarnessRoot(), "bin", "agent-harness")
	var processes []mcpProxyProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseMCPProxyProcess(line, binary)
		if ok {
			processes = append(processes, process)
		}
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

func terminateMCPProxyProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
