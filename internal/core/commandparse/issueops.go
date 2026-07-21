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
// closed). Only bare `agent-harness`, `bin/agent-harness`, or
// `./bin/agent-harness issueops …` invocations parse; two-word subcommands
// (handoff/worktree/…) are folded into Path.
func ParseExactIssueOpsCommand(command string) (ExactIssueOpsCommand, bool) {
	command = strings.TrimSpace(command)
	if command == "" || HasUnquotedControlOperator(command) || HasActiveCommandSubstitution(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveZshEqualsExpansion(command) {
		return ExactIssueOpsCommand{}, false
	}
	tokens := SplitCommandTokens(command)
	if len(tokens) < 3 || (tokens[0] != "agent-harness" && tokens[0] != "bin/agent-harness" && tokens[0] != "./bin/agent-harness") || tokens[1] != "issueops" {
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
		return v("--id", "--plan-path", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "compatibility review":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--backward-compatibility", "--side-effect", "--rollback-plan", "--verification", "--blocker")
		for _, name := range []string{"--backward-compatibility", "--side-effect", "--verification", "--blocker"} {
			r[name] = true
		}
		return values, b("--approved", "--json"), r, true
	case "execution decide":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--auto", "--hook-block", "--human-gate", "--subagent-use", "--subagent-rationale", "--subagent-plan-file")
		for _, name := range []string{"--auto", "--hook-block", "--human-gate"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "devils-advocate review":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--verdict", "--finding", "--waiver-rationale")
		r["--finding"] = true
		return values, b("--waive", "--json"), r, true
	case "phase":
		return v("--id", "--to", "--host", "--session-id", "--agent-id", "--cwd"), b("--force", "--json"), r, true
	case "ai-slop-clean record":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--category", "--verification")
		r["--category"] = true
		r["--verification"] = true
		return values, b("--json"), r, true
	case "feedback mark-issue-updated":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "feedback resolve":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--index", "--resolution"), b("--json"), r, true
	case "worktree prepare":
		return v("--id", "--orchestrator", "--inline-reason", "--agent", "--host", "--session-id", "--agent-id", "--source-cwd"), b("--confirm", "--json"), r, true
	case "worktree prepare-tools":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "worktree reconcile":
		return v("--id", "--workspace-epoch", "--host", "--session-id", "--agent-id", "--source-cwd"), b("--json"), r, true
	case "handoff start":
		values := v("--id", "--coordinator-recipient", "--coordinator-host", "--coordinator-session-id", "--coordinator-agent-id", "--source-cwd", "--workspace-epoch", "--expected-context-sha256", "--criteria-id", "--required-doc", "--required-skill", "--verification", "--stop-condition", "--worker-scope", "--heartbeat-cadence", "--result-format")
		for _, name := range []string{"--criteria-id", "--required-doc", "--required-skill", "--verification", "--stop-condition"} {
			r[name] = true
		}
		return values, b("--allow-codex-hook-trust-bypass", "--confirm", "--json"), r, true
	case "handoff recover":
		return v("--id", "--action", "--reason", "--cleanup-disposition", "--cleanup-step"), b("--confirm", "--force", "--json"), r, true
	case "handoff publish":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--confirm", "--json"), r, true
	case "remote create-pr":
		values := v("--id", "--title", "--body", "--body-file", "--template", "--provider", "--score-file", "--head", "--base", "--host", "--session-id", "--agent-id", "--cwd", "--label", "--assignee", "--field")
		for _, name := range []string{"--label", "--assignee", "--field"} {
			r[name] = true
		}
		return values, b("--confirm", "--json"), r, true
	case "remote verify-artifact":
		values := v("--id", "--provider", "--kind", "--url", "--target-branch", "--label", "--labels", "--assignee", "--assignees")
		for _, name := range []string{"--label", "--labels", "--assignee", "--assignees"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "handoff claim":
		return v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--host", "--session-id", "--agent-id", "--cwd", "--orca-worktree-id"), b("--json"), r, true
	case "handoff acknowledge-context":
		return v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--host", "--session-id", "--agent-id", "--cwd", "--issue-url", "--plan-sha256", "--understanding", "--scope-confirmation"), b("--json"), r, true
	case "handoff complete":
		values := v("--id", "--attempt", "--ownership-epoch", "--context-sha256", "--host", "--session-id", "--agent-id", "--cwd", "--final-head", "--changed-file", "--turing-report", "--verification")
		for _, name := range []string{"--changed-file", "--verification"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "handoff cleanup-preview":
		return v("--id", "--host", "--session-id", "--agent-id", "--source-cwd"), b("--json"), r, true
	case "handoff cleanup-approve":
		return v("--id", "--host", "--session-id", "--agent-id", "--source-cwd", "--inventory-fingerprint", "--disposition", "--reason"), b("--confirm", "--json"), r, true
	case "handoff cleanup-record":
		return v("--id", "--host", "--session-id", "--agent-id", "--source-cwd", "--step"), b("--json"), r, true
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
	if HasUnquotedControlOperator(command) || HasActiveCommandSubstitution(command) || HasActiveInputRedirect(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveZshEqualsExpansion(command) {
		return false
	}
	tokens := SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "pwd":
		return len(tokens) == 1
	case "wc":
		return exactReadOnlyWCCommand(tokens[1:])
	case "sed":
		return exactReadOnlySedCommand(tokens[1:])
	case "codegraph":
		return len(tokens) == 3 && tokens[1] == "explore" && strings.TrimSpace(tokens[2]) != "" && !strings.HasPrefix(tokens[2], "-")
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
		case "ls-remote":
			return exactReadOnlyGitLSRemote(tokens[i+1:])
		}
	case "gh":
		return exactReadOnlyGHCommand(tokens)
	case "orca":
		return ExactReadOnlyOrcaTerminalCommand(tokens) ||
			(len(tokens) == 4 && tokens[1] == "orchestration" && tokens[2] == "task-list" && tokens[3] == "--json")
	}
	return false
}

func exactReadOnlyGitLSRemote(tokens []string) bool {
	if len(tokens) < 2 {
		return false
	}
	allowedOptions := map[string]bool{
		"--heads": true, "--tags": true, "--refs": true, "--quiet": true,
		"-q": true, "--exit-code": true, "--symref": true,
	}
	i := 0
	for i < len(tokens) && strings.HasPrefix(tokens[i], "-") {
		if !allowedOptions[tokens[i]] {
			return false
		}
		i++
	}
	if i >= len(tokens) || tokens[i] != "origin" {
		return false
	}
	i++
	if i >= len(tokens) {
		return false
	}
	for _, ref := range tokens[i:] {
		if (!strings.HasPrefix(ref, "refs/heads/") && !strings.HasPrefix(ref, "refs/tags/")) || strings.ContainsAny(ref, "*?[]") {
			return false
		}
	}
	return true
}

func exactReadOnlyGHCommand(tokens []string) bool {
	if len(tokens) < 4 || tokens[0] != "gh" {
		return false
	}
	switch tokens[1] {
	case "pr":
		return exactReadOnlyGHPRCommand(tokens)
	case "run":
		return exactReadOnlyGHRunCommand(tokens)
	default:
		return false
	}
}

func exactReadOnlyGHPRCommand(tokens []string) bool {
	number, err := strconv.Atoi(tokens[3])
	if err != nil || number <= 0 {
		return false
	}
	values := map[string]bool{"--json": true, "--repo": true}
	booleans := map[string]bool{}
	switch tokens[2] {
	case "view":
		booleans["--comments"] = true
	case "checks":
		booleans["--required"] = true
	default:
		return false
	}
	flags, ok := ExactFlags(ExactIssueOpsCommand{Tokens: tokens, Start: 4}, values, booleans, map[string]bool{})
	if !ok {
		return false
	}
	if fields, exists := flags["--json"]; exists && !safeGHJSONFields(fields[0]) {
		return false
	}
	if repo, exists := flags["--repo"]; exists && !safeGHRepository(repo[0]) {
		return false
	}
	return true
}

func exactReadOnlyGHRunCommand(tokens []string) bool {
	if tokens[2] != "view" {
		return false
	}
	runID, err := strconv.ParseUint(tokens[3], 10, 64)
	if err != nil || runID == 0 {
		return false
	}
	flags, ok := ExactFlags(
		ExactIssueOpsCommand{Tokens: tokens, Start: 4},
		map[string]bool{"--json": true, "--job": true, "--attempt": true, "--repo": true},
		map[string]bool{"--log": true, "--log-failed": true, "--verbose": true},
		map[string]bool{},
	)
	if !ok {
		return false
	}
	if fields, exists := flags["--json"]; exists && !safeGHJSONFields(fields[0]) {
		return false
	}
	if repo, exists := flags["--repo"]; exists && !safeGHRepository(repo[0]) {
		return false
	}
	for _, name := range []string{"--job", "--attempt"} {
		if values, exists := flags[name]; exists {
			value, parseErr := strconv.ParseUint(values[0], 10, 64)
			if parseErr != nil || value == 0 {
				return false
			}
		}
	}
	return true
}

func safeGHRepository(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 100 {
			return false
		}
		for _, r := range part {
			if r != '-' && r != '_' && r != '.' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
		}
	}
	return true
}

func safeGHJSONFields(value string) bool {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r != '_' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
		}
	}
	return true
}

func exactReadOnlyWCCommand(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	longOptions := map[string]bool{
		"--bytes": true, "--chars": true, "--lines": true,
		"--max-line-length": true, "--words": true,
	}
	operands, optionsDone := 0, false
	for _, token := range tokens {
		if token == "" {
			return false
		}
		if !optionsDone && token == "--" {
			optionsDone = true
			continue
		}
		if !optionsDone && strings.HasPrefix(token, "--") {
			if !longOptions[token] {
				return false
			}
			continue
		}
		if !optionsDone && strings.HasPrefix(token, "-") {
			if token == "-" || len(token) == 1 {
				return false
			}
			for _, flag := range token[1:] {
				if !strings.ContainsRune("cmlwL", flag) {
					return false
				}
			}
			continue
		}
		if token == "-" {
			return false
		}
		operands++
	}
	return operands > 0
}

func exactReadOnlySedCommand(tokens []string) bool {
	if len(tokens) < 3 || tokens[0] != "-n" || !numericSedPrintRange(tokens[1]) {
		return false
	}
	for _, operand := range tokens[2:] {
		if operand == "" || operand == "-" || strings.HasPrefix(operand, "-") {
			return false
		}
	}
	return true
}

func numericSedPrintRange(script string) bool {
	if !strings.HasSuffix(script, "p") {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(script, "p"), ",")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	lines := make([]int, len(parts))
	for i, part := range parts {
		line, err := strconv.Atoi(part)
		if err != nil || line <= 0 {
			return false
		}
		lines[i] = line
	}
	return len(lines) == 1 || lines[0] <= lines[1]
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
