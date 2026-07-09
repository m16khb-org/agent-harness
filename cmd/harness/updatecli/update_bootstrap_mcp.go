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

type registeredMCPCommand struct {
	Name    string
	Command string
	Args    []string
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
	registered := readCodexRegisteredMCPCommands()
	var processes []mcpProxyProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseMCPProxyProcess(line, binary)
		if !ok {
			process, ok = parseRegisteredMCPProcess(line, registered)
		}
		if !ok {
			continue
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

func parseRegisteredMCPProcess(line string, registered []registeredMCPCommand) (mcpProxyProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return mcpProxyProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return mcpProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return mcpProxyProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	for _, candidate := range registered {
		if matchesRegisteredMCPCommand(command, candidate) {
			return mcpProxyProcess{PID: pid, Command: command}, true
		}
	}
	return mcpProxyProcess{}, false
}

func matchesRegisteredMCPCommand(command string, candidate registeredMCPCommand) bool {
	if !isNPXCommand(candidate.Command) || len(candidate.Args) == 0 {
		return false
	}
	exact := strings.Join(append([]string{candidate.Command}, candidate.Args...), " ")
	if command == exact {
		return true
	}
	pkg, rest, ok := npxPackageAndArgs(candidate.Args)
	if !ok {
		return false
	}
	if command == strings.TrimSpace("npm exec "+strings.Join(append([]string{pkg}, rest...), " ")) {
		return true
	}
	bin := packageBinaryName(pkg)
	if bin == "" || !strings.HasPrefix(command, "node ") || !containsBinCommand(command, bin) {
		return false
	}
	if len(rest) == 0 {
		return true
	}
	return strings.HasSuffix(command, " "+strings.Join(rest, " "))
}

func isNPXCommand(command string) bool {
	return filepath.Base(command) == "npx"
}

func npxPackageAndArgs(args []string) (string, []string, bool) {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-y", "--yes":
			continue
		default:
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) == 0 {
		return "", nil, false
	}
	return filtered[0], filtered[1:], true
}

func packageBinaryName(pkg string) string {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return ""
	}
	if strings.HasPrefix(pkg, "@") {
		parts := strings.Split(pkg, "/")
		if len(parts) != 2 {
			return ""
		}
		return stripPackageVersion(parts[1])
	}
	return stripPackageVersion(pkg)
}

func containsBinCommand(command, bin string) bool {
	needle := "/.bin/" + bin
	i := strings.Index(command, needle)
	if i < 0 {
		return false
	}
	after := i + len(needle)
	return after == len(command) || command[after] == ' '
}

func stripPackageVersion(pkg string) string {
	if i := strings.LastIndex(pkg, "@"); i > 0 {
		return pkg[:i]
	}
	return pkg
}

func readCodexRegisteredMCPCommands() []registeredMCPCommand {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		return nil
	}
	return parseCodexRegisteredMCPCommands(string(b))
}

func parseCodexRegisteredMCPCommands(config string) []registeredMCPCommand {
	var commands []registeredMCPCommand
	var current *registeredMCPCommand
	flush := func() {
		if current != nil && current.Command != "" {
			commands = append(commands, *current)
		}
		current = nil
	}
	for _, raw := range strings.Split(config, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			if strings.HasPrefix(section, "mcp_servers.") && !strings.HasSuffix(section, ".env") {
				current = &registeredMCPCommand{Name: strings.TrimPrefix(section, "mcp_servers.")}
			}
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "command":
			current.Command = parseTOMLStringValue(value)
		case "args":
			current.Args = parseTOMLStringArray(value)
		}
	}
	flush()
	return commands
}

func parseTOMLStringValue(value string) string {
	value = strings.TrimSpace(stripInlineComment(value))
	parsed, err := strconv.Unquote(value)
	if err != nil {
		return ""
	}
	return parsed
}

func parseTOMLStringArray(value string) []string {
	value = strings.TrimSpace(stripInlineComment(value))
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if value == "" {
		return nil
	}
	var args []string
	for len(value) > 0 {
		value = strings.TrimSpace(value)
		if !strings.HasPrefix(value, "\"") {
			return nil
		}
		end := 1
		for end < len(value) {
			if value[end] == '"' && value[end-1] != '\\' {
				break
			}
			end++
		}
		if end >= len(value) {
			return nil
		}
		parsed, err := strconv.Unquote(value[:end+1])
		if err != nil {
			return nil
		}
		args = append(args, parsed)
		value = strings.TrimSpace(value[end+1:])
		if value == "" {
			break
		}
		if !strings.HasPrefix(value, ",") {
			return nil
		}
		value = strings.TrimSpace(strings.TrimPrefix(value, ","))
	}
	return args
}

func stripInlineComment(value string) string {
	inString := false
	escaped := false
	for i, r := range value {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case r == '#' && !inString:
			return value[:i]
		}
	}
	return value
}

func terminateMCPProxyProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
