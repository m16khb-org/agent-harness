package commandparse

import (
	"strconv"
	"strings"
)

// ExactIssueOpsCommand is a parsed, exact `agent-harness issueops …` command:
// its subcommand path (e.g. "handoff start"), the full token slice, and the
// index where flags begin. Moved out of the lifecycle authority layer so the
// parser and its security filters live in one place; the authority layer is a
// consumer. Behavior is byte-identical to the prior lifecycle-internal parser.
type ExactIssueOpsCommand struct {
	Path   string
	Tokens []string
	Start  int
}

// ParseExactIssueOpsCommand parses a command into an ExactIssueOpsCommand,
// rejecting any command that carries active shell control/expansion (fail
// closed). Only bare `agent-harness`/`./bin/agent-harness issueops …` invocations
// parse; two-word subcommands (handoff/worktree/…) are folded into Path.
func ParseExactIssueOpsCommand(command string) (ExactIssueOpsCommand, bool) {
	command = strings.TrimSpace(command)
	if command == "" || HasUnquotedControlOperator(command) || HasActiveCommandSubstitution(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveZshEqualsExpansion(command) {
		return ExactIssueOpsCommand{}, false
	}
	tokens := SplitCommandTokens(command)
	if len(tokens) < 3 || (tokens[0] != "agent-harness" && tokens[0] != "./bin/agent-harness") || tokens[1] != "issueops" {
		return ExactIssueOpsCommand{}, false
	}
	parts := []string{tokens[2]}
	start := 3
	if len(tokens) > 3 {
		switch tokens[2] {
		case "handoff", "worktree", "compatibility", "execution", "devils-advocate", "feedback", "remote", "cleanup", "ai-slop-clean":
			if strings.HasPrefix(tokens[3], "--") {
				return ExactIssueOpsCommand{}, false
			}
			parts = append(parts, tokens[3])
			start = 4
		}
	}
	return ExactIssueOpsCommand{Path: strings.Join(parts, " "), Tokens: tokens, Start: start}, true
}

// ExactFlags validates and collects the flags of an ExactIssueOpsCommand against
// the value/boolean/repeatable spec, rejecting unknown flags, duplicate
// non-repeatable flags, and missing values (fail closed).
func ExactFlags(command ExactIssueOpsCommand, values, booleans, repeatable map[string]bool) (map[string][]string, bool) {
	parsed := map[string][]string{}
	for i := command.Start; i < len(command.Tokens); i++ {
		token := command.Tokens[i]
		if !strings.HasPrefix(token, "--") {
			return nil, false
		}
		name, value, hasValue := token, "", false
		if at := strings.Index(token, "="); at >= 0 {
			name, value, hasValue = token[:at], token[at+1:], true
		}
		switch {
		case booleans[name]:
			if hasValue || len(parsed[name]) > 0 {
				return nil, false
			}
			parsed[name] = []string{"true"}
		case values[name]:
			if !hasValue {
				if i+1 >= len(command.Tokens) || strings.HasPrefix(command.Tokens[i+1], "--") {
					return nil, false
				}
				i++
				value = command.Tokens[i]
			}
			if !repeatable[name] && len(parsed[name]) > 0 {
				return nil, false
			}
			parsed[name] = append(parsed[name], value)
		default:
			return nil, false
		}
	}
	return parsed, true
}

// IssueOpsCommandSpec returns the (values, booleans, repeatable, ok) flag spec
// for an exact issueops subcommand path. ok is false for unrecognized paths.
func IssueOpsCommandSpec(path string) (map[string]bool, map[string]bool, map[string]bool, bool) {
	v := func(names ...string) map[string]bool {
		out := map[string]bool{}
		for _, name := range names {
			out[name] = true
		}
		return out
	}
	b := func(names ...string) map[string]bool { return v(names...) }
	r := map[string]bool{}
	switch path {
	case "status":
		return v("--id"), b("--json"), r, true
	case "resume":
		return v("--repo", "--id"), b("--bind", "--json"), r, true
	case "link-plan":
		return v("--id", "--plan-path"), b("--json"), r, true
	case "compatibility review":
		values := v("--id", "--backward-compatibility", "--side-effect", "--rollback-plan", "--verification", "--blocker")
		for _, name := range []string{"--backward-compatibility", "--side-effect", "--verification", "--blocker"} {
			r[name] = true
		}
		return values, b("--approved", "--json"), r, true
	case "execution decide":
		values := v("--id", "--auto", "--hook-block", "--human-gate", "--subagent-use", "--subagent-rationale", "--subagent-plan-file")
		for _, name := range []string{"--auto", "--hook-block", "--human-gate"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "devils-advocate review":
		values := v("--id", "--verdict", "--finding", "--waiver-rationale")
		r["--finding"] = true
		return values, b("--waive", "--json"), r, true
	case "phase":
		return v("--id", "--to"), b("--force", "--json"), r, true
	case "worktree prepare":
		return v("--id", "--orchestrator", "--inline-reason", "--agent"), b("--confirm", "--json"), r, true
	case "worktree prepare-tools":
		return v("--id"), b("--json"), r, true
	case "handoff start":
		values := v("--id", "--coordinator-recipient", "--coordinator-host", "--coordinator-session-id", "--coordinator-agent-id", "--source-cwd", "--expected-context-sha256", "--criteria-id", "--required-doc", "--required-skill", "--verification", "--stop-condition", "--worker-scope", "--heartbeat-cadence", "--result-format")
		for _, name := range []string{"--criteria-id", "--required-doc", "--required-skill", "--verification", "--stop-condition"} {
			r[name] = true
		}
		return values, b("--allow-codex-hook-trust-bypass", "--confirm", "--json"), r, true
	case "handoff recover":
		return v("--id", "--action", "--reason", "--cleanup-disposition", "--cleanup-step"), b("--confirm", "--force", "--json"), r, true
	case "handoff publish":
		return v("--id", "--host", "--session-id", "--agent-id", "--source-cwd"), b("--approve-legacy-coordinator-seal", "--confirm", "--json"), r, true
	case "handoff accept":
		return v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--final-head", "--host", "--session-id", "--agent-id", "--source-cwd"), b("--json"), r, true
	case "handoff claim":
		return v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--host", "--session-id", "--agent-id", "--cwd", "--orca-worktree-id"), b("--json"), r, true
	case "handoff finish":
		values := v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--host", "--session-id", "--agent-id", "--outcome", "--final-head", "--changed-file", "--turing-report", "--verification", "--cleanup-receipt", "--evidence-digest", "--task-id", "--dispatch-id")
		for _, name := range []string{"--changed-file", "--verification", "--cleanup-receipt"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "heartbeat":
		return v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--host", "--session-id", "--agent-id"), b("--json"), r, true
	default:
		return nil, nil, nil, false
	}
}

// ExactReadOnlyShellCommand reports whether a non-issueops shell command is an
// exact read-only observation (pwd, safe rg, read-only git, read-only orca
// terminal/orchestration). It rejects any command carrying active shell
// control/expansion. The issueops status/resume read-only carve-out is handled
// by the caller (it needs the record identity).
func ExactReadOnlyShellCommand(command string) bool {
	if HasUnquotedControlOperator(command) || HasActiveCommandSubstitution(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveZshEqualsExpansion(command) {
		return false
	}
	tokens := SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "pwd":
		return len(tokens) == 1
	case "rg":
		return SafeRipgrepArgs(tokens[1:])
	case "git":
		i := CommandAfterDirectoryOption(tokens, 1)
		if i < 0 || i >= len(tokens) {
			return false
		}
		switch tokens[i] {
		case "status", "diff", "log", "show", "rev-parse":
			for _, token := range tokens[i+1:] {
				if token == "-o" || strings.HasPrefix(token, "--output") || token == "--ext-diff" || token == "--textconv" || strings.HasPrefix(token, "--exec") {
					return false
				}
			}
			return true
		}
	case "orca":
		return ExactReadOnlyOrcaTerminalCommand(tokens) ||
			(len(tokens) == 4 && tokens[1] == "orchestration" && tokens[2] == "task-list" && tokens[3] == "--json")
	}
	return false
}

// ExactReadOnlyOrcaTerminalCommand reports whether the tokens are an exact
// read-only `orca terminal list|show|read|wait` invocation with bounded flags.
func ExactReadOnlyOrcaTerminalCommand(tokens []string) bool {
	if len(tokens) < 4 || tokens[0] != "orca" || tokens[1] != "terminal" {
		return false
	}
	values := map[string]bool{}
	booleans := map[string]bool{"--json": true}
	switch tokens[2] {
	case "list":
		values = map[string]bool{"--worktree": true, "--limit": true}
	case "show":
		values = map[string]bool{"--terminal": true}
	case "read":
		values = map[string]bool{"--terminal": true, "--cursor": true, "--limit": true}
	case "wait":
		values = map[string]bool{"--terminal": true, "--for": true, "--timeout-ms": true}
	default:
		return false
	}
	flags, ok := ExactFlags(ExactIssueOpsCommand{Tokens: tokens, Start: 3}, values, booleans, map[string]bool{})
	if !ok || len(flags["--json"]) != 1 {
		return false
	}
	for name, entries := range flags {
		if name == "--json" {
			continue
		}
		if len(entries) != 1 || strings.TrimSpace(entries[0]) == "" {
			return false
		}
	}
	if value, exists := flags["--for"]; exists && value[0] != "exit" && value[0] != "tui-idle" {
		return false
	}
	for _, name := range []string{"--cursor", "--limit", "--timeout-ms"} {
		if value, exists := flags[name]; exists {
			n, err := strconv.Atoi(value[0])
			if err != nil || n < 0 || (name == "--limit" && n == 0) {
				return false
			}
		}
	}
	return tokens[2] != "wait" || len(flags["--for"]) == 1
}

// SafeRipgrepArgs reports whether every rg argument is on the read-only
// value/boolean allowlist (fail closed on any unknown flag).
func SafeRipgrepArgs(tokens []string) bool {
	valueOptions := map[string]bool{
		"-g": true, "--glob": true, "-t": true, "--type": true, "-T": true, "--type-not": true,
		"-m": true, "--max-count": true, "-A": true, "--after-context": true, "-B": true, "--before-context": true,
		"-C": true, "--context": true, "--color": true, "--sort": true, "--sortr": true,
	}
	boolOptions := map[string]bool{
		"-n": true, "--line-number": true, "--files": true, "--hidden": true, "--no-ignore": true,
		"-F": true, "--fixed-strings": true, "--json": true, "-l": true, "--files-with-matches": true,
		"--stats": true, "--pcre2": true, "-U": true, "--multiline": true, "--no-heading": true,
		"--column": true, "--count": true, "--count-matches": true, "--no-messages": true,
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if !strings.HasPrefix(token, "-") || token == "-" {
			continue
		}
		name := token
		if at := strings.Index(token, "="); at >= 0 {
			name = token[:at]
			if !valueOptions[name] {
				return false
			}
			continue
		}
		if boolOptions[name] {
			continue
		}
		if valueOptions[name] {
			if i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "-") {
				return false
			}
			i++
			continue
		}
		return false
	}
	return true
}

// CommandAfterDirectoryOption returns the index of the first non-`-C`
// (directory-option) token starting from start, or -1 when the -C option is
// malformed (missing/empty value).
func CommandAfterDirectoryOption(tokens []string, start int) int {
	for start < len(tokens) {
		token := tokens[start]
		if token == "-C" {
			if start+1 >= len(tokens) || strings.HasPrefix(tokens[start+1], "-") {
				return -1
			}
			start += 2
			continue
		}
		if strings.HasPrefix(token, "-C=") {
			if strings.TrimPrefix(token, "-C=") == "" {
				return -1
			}
			start++
			continue
		}
		return start
	}
	return -1
}

// ContainsASCIITerminalControl reports whether value contains any ASCII C0
// control or DEL character (which could steer a PTY or erase a comment marker).
func ContainsASCIITerminalControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
