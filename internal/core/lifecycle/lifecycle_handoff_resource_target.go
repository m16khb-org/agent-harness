package lifecycle

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/searchrouting"
)

type protectedOrcaResourceKind string

const (
	protectedOrcaTerminal protectedOrcaResourceKind = "terminal"
	protectedOrcaTask     protectedOrcaResourceKind = "task"
	protectedOrcaDispatch protectedOrcaResourceKind = "dispatch"
	protectedOrcaWorktree protectedOrcaResourceKind = "worktree"
)

type protectedOrcaResourceTarget struct {
	Kind protectedOrcaResourceKind
	ID   string
}

// protectedOrcaResourceTargets extracts only literal identifiers from a
// mutating Orca control request. Its boolean result means the request is a
// resource control; an empty target list is a literal control of an unrelated
// resource, while false means dynamic or malformed input that must fail closed.
func protectedOrcaResourceTargets(req HookToolUseLifecycleRequest) ([]protectedOrcaResourceTarget, bool, bool) {
	if searchrouting.IsShellTool(req.Tool) {
		return protectedOrcaResourceTargetsFromShell(req.Command)
	}
	return protectedOrcaResourceTargetsFromMCP(req)
}

func protectedOrcaResourceTargetsFromShell(command string) ([]protectedOrcaResourceTarget, bool, bool) {
	if commandparse.HasUnquotedControlOperator(command) || commandparse.HasActiveCommandSubstitution(command) || commandparse.HasActiveOutputRedirect(command) || commandparse.HasActiveParameterOrTildeExpansion(command) || commandparse.HasActivePathnameExpansion(command) || commandparse.HasActiveShellSpecialQuoting(command) || commandparse.HasActiveZshEqualsExpansion(command) {
		return nil, looksLikeOrcaResourceControl(command), false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) < 3 || filepath.Base(tokens[0]) != "orca" {
		return nil, false, true
	}
	switch tokens[1] {
	case "terminal":
		if !mutatingTerminalSubcommand(tokens[2]) {
			return nil, false, true
		}
		return shellOrcaTargets(tokens[3:], map[string]protectedOrcaResourceKind{"--terminal": protectedOrcaTerminal, "--worktree": protectedOrcaWorktree})
	case "orchestration":
		if !mutatingOrchestrationSubcommand(tokens[2]) {
			return nil, false, true
		}
		return shellOrcaTargets(tokens[3:], map[string]protectedOrcaResourceKind{
			"--task": protectedOrcaTask, "--task-id": protectedOrcaTask, "--id": protectedOrcaTask,
			"--dispatch": protectedOrcaDispatch, "--dispatch-id": protectedOrcaDispatch,
		})
	case "worktree":
		if !mutatingWorktreeSubcommand(tokens[2]) {
			return nil, false, true
		}
		return shellOrcaTargets(tokens[3:], map[string]protectedOrcaResourceKind{"--worktree": protectedOrcaWorktree, "--id": protectedOrcaWorktree})
	default:
		return nil, false, true
	}
}

func protectedOrcaResourceTargetsFromMCP(req HookToolUseLifecycleRequest) ([]protectedOrcaResourceTarget, bool, bool) {
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	if !strings.Contains(tool, "__orca__") {
		if strings.HasSuffix(tool, "terminal_send") || strings.HasSuffix(tool, "terminal_stop") || strings.HasSuffix(tool, "terminal_create") || strings.HasSuffix(tool, "terminal_switch") || strings.HasSuffix(tool, "terminal_focus") || strings.HasSuffix(tool, "terminal_close") || strings.HasSuffix(tool, "terminal_rename") || strings.HasSuffix(tool, "terminal_split") || strings.HasSuffix(tool, "terminal_write") || strings.HasSuffix(tool, "terminal_input") || strings.HasSuffix(tool, "terminal_type") || strings.HasSuffix(tool, "terminal_paste") {
			return nil, true, false
		}
		return nil, false, true
	}
	isTerminal := strings.Contains(tool, "__terminal_") && !strings.HasSuffix(tool, "__terminal_list") && !strings.HasSuffix(tool, "__terminal_show") && !strings.HasSuffix(tool, "__terminal_read") && !strings.HasSuffix(tool, "__terminal_wait")
	isOrchestration := strings.Contains(tool, "__orchestration_") && !strings.HasSuffix(tool, "__orchestration_task_list")
	isWorktree := strings.Contains(tool, "__worktree_") && !strings.HasSuffix(tool, "__worktree_list") && !strings.HasSuffix(tool, "__worktree_show")
	if !isTerminal && !isOrchestration && !isWorktree {
		return nil, false, true
	}
	input, ok := flatMCPInput(req.ToolInput)
	if !ok {
		return nil, true, false
	}
	keys := map[string]protectedOrcaResourceKind{}
	if isTerminal {
		keys["terminal"] = protectedOrcaTerminal
		keys["terminal_handle"] = protectedOrcaTerminal
		keys["worktree"] = protectedOrcaWorktree
		keys["worktree_id"] = protectedOrcaWorktree
	} else if isOrchestration {
		keys["task"] = protectedOrcaTask
		keys["task_id"] = protectedOrcaTask
		keys["id"] = protectedOrcaTask
		keys["dispatch"] = protectedOrcaDispatch
		keys["dispatch_id"] = protectedOrcaDispatch
	} else {
		keys["worktree"] = protectedOrcaWorktree
		keys["worktree_id"] = protectedOrcaWorktree
		keys["id"] = protectedOrcaWorktree
	}
	return mcpOrcaTargets(input, keys)
}

func shellOrcaTargets(tokens []string, keys map[string]protectedOrcaResourceKind) ([]protectedOrcaResourceTarget, bool, bool) {
	targets := []protectedOrcaResourceTarget{}
	seen := map[protectedOrcaResourceTarget]bool{}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		kind, exact := keys[token]
		value := ""
		if exact {
			if i+1 >= len(tokens) {
				return nil, true, false
			}
			value = tokens[i+1]
			i++
		} else {
			for flag, candidate := range keys {
				if strings.HasPrefix(token, flag+"=") {
					kind, exact, value = candidate, true, strings.TrimPrefix(token, flag+"=")
					break
				}
			}
		}
		if !exact {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || strings.HasPrefix(value, "-") {
			return nil, true, false
		}
		target := protectedOrcaResourceTarget{Kind: kind, ID: normalizeProtectedOrcaResourceID(kind, value)}
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	return targets, true, true
}

func mcpOrcaTargets(input map[string]any, keys map[string]protectedOrcaResourceKind) ([]protectedOrcaResourceTarget, bool, bool) {
	targets := []protectedOrcaResourceTarget{}
	seen := map[protectedOrcaResourceTarget]bool{}
	for key, kind := range keys {
		value, exists := input[key]
		if !exists {
			continue
		}
		id, ok := value.(string)
		id = strings.TrimSpace(id)
		if !ok || id == "" {
			return nil, true, false
		}
		target := protectedOrcaResourceTarget{Kind: kind, ID: normalizeProtectedOrcaResourceID(kind, id)}
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	return targets, true, true
}

func looksLikeOrcaResourceControl(command string) bool {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	return len(tokens) >= 3 && filepath.Base(tokens[0]) == "orca" &&
		((tokens[1] == "terminal" && mutatingTerminalSubcommand(tokens[2])) || (tokens[1] == "orchestration" && mutatingOrchestrationSubcommand(tokens[2])) || (tokens[1] == "worktree" && mutatingWorktreeSubcommand(tokens[2])))
}

func mutatingTerminalSubcommand(name string) bool {
	switch name {
	case "send", "stop", "create", "switch", "focus", "close", "rename", "split", "write", "input", "type", "paste":
		return true
	}
	return false
}

func mutatingOrchestrationSubcommand(name string) bool {
	switch name {
	case "dispatch", "task-update", "send", "task-cancel", "task-delete", "dispatch-cancel":
		return true
	}
	return false
}

func mutatingWorktreeSubcommand(name string) bool {
	switch name {
	case "create", "add", "rm", "remove", "delete", "move", "rename":
		return true
	}
	return false
}

func normalizeProtectedOrcaResourceID(kind protectedOrcaResourceKind, id string) string {
	id = strings.TrimSpace(id)
	if kind == protectedOrcaWorktree {
		return strings.TrimPrefix(id, "id:")
	}
	return id
}

func recordsMatchingProtectedOrcaResource(req HookToolUseLifecycleRequest, records []IssueOpsRecord) ([]IssueOpsRecord, string) {
	targets, control, literal := protectedOrcaResourceTargets(req)
	if !control {
		return nil, ""
	}
	if !literal {
		return nil, "Orca resource control must use literal persisted terminal, task, or dispatch identifiers"
	}
	matched := map[string]IssueOpsRecord{}
	for _, record := range records {
		for _, target := range targets {
			if recordOwnsProtectedOrcaResource(record, target) {
				matched[record.ID] = record
			}
		}
	}
	result := make([]IssueOpsRecord, 0, len(matched))
	for _, record := range matched {
		result = append(result, record)
	}
	return result, ""
}

func recordOwnsProtectedOrcaResource(record IssueOpsRecord, target protectedOrcaResourceTarget) bool {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil {
		return false
	}
	orca := record.ExecutionHandoff.Orca
	switch target.Kind {
	case protectedOrcaTerminal:
		return target.ID != "" && (target.ID == strings.TrimSpace(orca.WorkerTerminalHandle) || target.ID == strings.TrimSpace(orca.WorkerMailboxHandle))
	case protectedOrcaTask:
		return target.ID != "" && target.ID == strings.TrimSpace(orca.TaskID)
	case protectedOrcaDispatch:
		return target.ID != "" && target.ID == strings.TrimSpace(orca.DispatchID)
	case protectedOrcaWorktree:
		return target.ID != "" && target.ID == strings.TrimSpace(orca.WorktreeID)
	}
	return false
}
