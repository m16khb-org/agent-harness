package core

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var secretPathRe = regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)
var secretArgRe = regexp.MustCompile(`(?i)(token|password|passwd|secret|api[_-]?key|credential|authorization)=`)

var policyShellInterpreters = stringSet("sh", "bash", "zsh", "fish", "dash", "ksh")

var policyNetworkCommands = stringSet("curl", "wget", "ssh", "scp", "sftp", "rsync", "gh", "brew", "npm", "pnpm", "yarn", "pip", "pip3")
var policyNetworkSubcommands = map[string]map[string]bool{
	"git": stringSet("clone", "fetch", "pull", "push", "ls-remote", "submodule"),
}

var policyWriteCommands = stringSet("touch", "mkdir", "rmdir", "rm", "mv", "cp", "install", "chmod", "chown", "tee", "python", "python3", "node", "ruby", "perl")
var policyWriteSubcommands = map[string]map[string]bool{
	"git": stringSet("add", "commit", "reset", "clean", "checkout", "switch", "merge", "rebase", "cherry-pick", "revert", "push", "pull", "apply", "am", "stash"),
	"go":  stringSet("build", "test", "run", "install", "mod", "work", "generate"),
}

var policyReadOnlyCommands = stringSet("pwd", "ls", "cat", "grep", "rg", "find", "sed", "awk", "head", "tail", "wc", "test", "stat", "true", "false")
var policyReadOnlySubcommands = map[string]map[string]bool{
	"git": stringSet("status", "diff", "log", "show", "rev-parse", "branch", "remote", "ls-files", "grep", "describe", "merge-base", "config"),
	"go":  stringSet("version", "env", "list"),
}

type CommandPolicyRequest struct {
	WorkspaceRoot  string   `json:"workspace_root"`
	CWD            string   `json:"cwd"`
	Argv           []string `json:"argv"`
	Timeout        string   `json:"timeout"`
	EnvAllowlist   []string `json:"env_allowlist"`
	NetworkAllowed bool     `json:"network_allowed"`
	WriteAllowed   bool     `json:"write_allowed"`
	ShellAllowed   bool     `json:"shell_allowed"`
	ShellReason    string   `json:"shell_reason,omitempty"`
	AuditLogID     string   `json:"audit_log_id,omitempty"`
}

type CommandPolicyEvaluation struct {
	OK             bool     `json:"ok"`
	Allowed        bool     `json:"allowed"`
	AuditLogID     string   `json:"audit_log_id"`
	WorkspaceRoot  string   `json:"workspace_root"`
	CWD            string   `json:"cwd"`
	Argv           []string `json:"argv"`
	Timeout        string   `json:"timeout"`
	EnvAllowlist   []string `json:"env_allowlist"`
	NetworkAllowed bool     `json:"network_allowed"`
	WriteAllowed   bool     `json:"write_allowed"`
	ShellAllowed   bool     `json:"shell_allowed"`
	ShellReason    string   `json:"shell_reason,omitempty"`
	DenyReasons    []string `json:"deny_reasons"`
	Warnings       []string `json:"warnings"`
	GeneratedAt    string   `json:"generated_at"`
}

type CommandFakeRunResult struct {
	OK         bool                    `json:"ok"`
	Executed   bool                    `json:"executed"`
	ExitCode   int                     `json:"exit_code"`
	Stdout     string                  `json:"stdout,omitempty"`
	Stderr     string                  `json:"stderr,omitempty"`
	StartedAt  string                  `json:"started_at"`
	FinishedAt string                  `json:"finished_at"`
	DurationMS int64                   `json:"duration_ms"`
	Policy     CommandPolicyEvaluation `json:"policy"`
}

type PolicyDeniedError struct {
	Reasons []string
}

func (e PolicyDeniedError) Error() string {
	if len(e.Reasons) == 0 {
		return "command denied by policy"
	}
	return "command denied by policy: " + strings.Join(e.Reasons, "; ")
}

func IsPolicyDenied(err error) bool {
	_, ok := err.(PolicyDeniedError)
	return ok
}

func EvaluateCommandPolicy(req CommandPolicyRequest) CommandPolicyEvaluation {
	root := absOrOriginal(req.WorkspaceRoot)
	cwd := absOrOriginal(req.CWD)
	canonicalRoot := canonicalPotentialPath(root)
	canonicalCWD := canonicalPotentialPath(cwd)
	argv := append([]string{}, req.Argv...)
	timeout, timeoutErr := time.ParseDuration(req.Timeout)
	if req.Timeout == "" {
		timeout = 30 * time.Second
	}
	auditID := req.AuditLogID
	if auditID == "" {
		auditID = makeAuditLogID(req)
	}
	result := CommandPolicyEvaluation{
		OK:             true,
		AuditLogID:     auditID,
		WorkspaceRoot:  root,
		CWD:            cwd,
		Argv:           redactArgv(argv),
		Timeout:        timeout.String(),
		EnvAllowlist:   cleanEnvAllowlist(req.EnvAllowlist),
		NetworkAllowed: req.NetworkAllowed,
		WriteAllowed:   req.WriteAllowed,
		ShellAllowed:   req.ShellAllowed,
		ShellReason:    redactFreeform(req.ShellReason),
		DenyReasons:    []string{},
		Warnings:       []string{},
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	addDeny := func(reason string) {
		result.DenyReasons = append(result.DenyReasons, reason)
	}
	addWarn := func(warning string) {
		result.Warnings = append(result.Warnings, warning)
	}

	if req.WorkspaceRoot == "" {
		addDeny("workspace_root_required")
	} else if info, err := os.Stat(root); err != nil || !info.IsDir() {
		addDeny("workspace_root_not_directory")
	}
	if req.CWD == "" {
		addDeny("cwd_required")
	} else if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		addDeny("cwd_not_directory")
	}
	if root != "" && cwd != "" && !sameOrWithin(canonicalRoot, canonicalCWD) {
		addDeny("cwd_outside_workspace")
	}
	if len(argv) == 0 {
		addDeny("argv_required")
	}
	if timeoutErr != nil || timeout <= 0 {
		addDeny("invalid_timeout")
	} else if timeout > 15*time.Minute {
		addDeny("timeout_exceeds_15m")
	}
	for _, envName := range req.EnvAllowlist {
		if !validEnvName(envName) {
			addDeny("invalid_env_allowlist_name")
			break
		}
	}
	for _, arg := range argv {
		if secretLikeArg(arg) {
			addDeny("secret_like_argument")
			break
		}
	}
	if len(argv) > 0 {
		if commandReferencesOutsideWorkspace(canonicalRoot, canonicalCWD, argv) {
			addDeny("path_outside_workspace")
		}
		if isShellCommand(argv[0]) {
			if !req.ShellAllowed {
				addDeny("shell_interpreter_not_allowed")
			} else if strings.TrimSpace(req.ShellReason) == "" {
				addDeny("shell_reason_required")
			} else {
				addWarn("shell_interpreter_exception")
			}
		}
		if commandUsesNetwork(argv) && !req.NetworkAllowed {
			addDeny("network_not_allowed")
		}
		if commandWrites(argv) && !req.WriteAllowed {
			addDeny("write_not_allowed")
		}
		if !req.WriteAllowed && !readOnlyAllowed(argv) {
			addDeny("command_not_in_read_only_allowlist")
		}
	}
	result.DenyReasons = uniqSorted(result.DenyReasons)
	result.Warnings = uniqSorted(result.Warnings)
	result.Allowed = len(result.DenyReasons) == 0
	return result
}

func FakeRunCommand(req CommandPolicyRequest) CommandFakeRunResult {
	started := time.Now()
	policy := EvaluateCommandPolicy(req)
	finished := time.Now()
	result := CommandFakeRunResult{
		OK:         policy.Allowed,
		Executed:   false,
		ExitCode:   0,
		StartedAt:  started.UTC().Format(time.RFC3339Nano),
		FinishedAt: finished.UTC().Format(time.RFC3339Nano),
		DurationMS: finished.Sub(started).Milliseconds(),
		Policy:     policy,
	}
	if !policy.Allowed {
		result.ExitCode = 3
		result.Stderr = "fake-run denied by policy: " + strings.Join(policy.DenyReasons, "; ") + "\n"
		return result
	}
	result.Stdout = fmt.Sprintf("fake-run accepted by policy; command was not executed\nargv: %s\naudit_log_id: %s\n", strings.Join(policy.Argv, " "), policy.AuditLogID)
	return result
}

func CommandPolicySummary() map[string]any {
	return map[string]any{
		"ok":                    true,
		"mode":                  "policy_check_plus_fake_runner",
		"executes_commands":     false,
		"default_timeout":       "30s",
		"max_timeout":           "15m",
		"required_fields":       []string{"workspace_root", "cwd", "argv", "timeout", "env_allowlist", "network_allowed", "write_allowed", "audit_log_id"},
		"default_denials":       []string{"cwd_outside_workspace", "path_outside_workspace", "shell_interpreter_not_allowed", "network_not_allowed", "write_not_allowed", "command_not_in_read_only_allowlist", "secret_like_argument"},
		"read_only_examples":    [][]string{{"git", "status", "--short"}, {"git", "diff", "--stat"}, {"rg", "pattern", "."}},
		"catalog":               commandPolicyCatalog(),
		"write_requires_flag":   true,
		"network_requires_flag": true,
	}
}

func makeAuditLogID(req CommandPolicyRequest) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(req.WorkspaceRoot))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(req.CWD))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(req.Argv, "\x00")))
	return fmt.Sprintf("audit-%s-%08x", time.Now().UTC().Format("20060102T150405Z"), h.Sum32())
}

func absOrOriginal(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func canonicalPotentialPath(path string) string {
	if path == "" {
		return ""
	}
	abs := absOrOriginal(path)
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		return eval
	}
	originalAbs := abs
	missing := []string{}
	for {
		parent := filepath.Dir(abs)
		if parent == abs {
			return originalAbs
		}
		missing = append([]string{filepath.Base(abs)}, missing...)
		if eval, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{eval}, missing...)
			return filepath.Join(parts...)
		}
		abs = parent
	}
}

func sameOrWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func commandReferencesOutsideWorkspace(root, cwd string, argv []string) bool {
	if root == "" || cwd == "" || len(argv) < 2 {
		return false
	}
	for _, arg := range argv[1:] {
		for _, candidate := range policyPathCandidates(arg) {
			resolved := resolvePolicyPathCandidate(cwd, candidate)
			if resolved == "" {
				continue
			}
			if !sameOrWithin(root, canonicalPotentialPath(resolved)) {
				return true
			}
		}
	}
	return false
}

func policyPathCandidates(arg string) []string {
	arg = strings.TrimSpace(arg)
	if arg == "" || looksLikeRemoteOrURL(arg) {
		return nil
	}
	if strings.HasPrefix(arg, "-") {
		if key, value, ok := strings.Cut(arg, "="); ok && strings.TrimSpace(key) != "" && policyArgLooksPathLike(value) {
			return []string{value}
		}
		return nil
	}
	if !policyArgLooksPathLike(arg) {
		return nil
	}
	return []string{arg}
}

func policyArgLooksPathLike(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" || looksLikeRemoteOrURL(arg) {
		return false
	}
	if arg == "~" || strings.HasPrefix(arg, "~/") || strings.HasPrefix(arg, "~"+string(os.PathSeparator)) {
		return true
	}
	if filepath.IsAbs(arg) || arg == "." || arg == ".." {
		return true
	}
	slashArg := filepath.ToSlash(arg)
	return strings.HasPrefix(slashArg, "./") || strings.HasPrefix(slashArg, "../") || strings.Contains(slashArg, "/")
}

func looksLikeRemoteOrURL(arg string) bool {
	lower := strings.ToLower(arg)
	if strings.Contains(lower, "://") {
		return true
	}
	return strings.Contains(arg, "@") && strings.Contains(arg, ":") && !strings.Contains(arg, string(os.PathSeparator))
}

func resolvePolicyPathCandidate(cwd, candidate string) string {
	if strings.TrimSpace(candidate) == "" || strings.HasPrefix(candidate, "~") {
		return candidate
	}
	if filepath.IsAbs(candidate) {
		return candidate
	}
	return filepath.Join(cwd, candidate)
}

func cleanEnvAllowlist(items []string) []string {
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func redactArgv(argv []string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = redactFreeform(arg)
	}
	return out
}

func redactFreeform(s string) string {
	if secretLikeArg(s) {
		return "<redacted>"
	}
	return s
}

func secretLikeArg(arg string) bool {
	return secretArgRe.MatchString(arg) || secretPathRe.MatchString(filepath.ToSlash(arg))
}

func commandBase(command string) string {
	return strings.ToLower(filepath.Base(command))
}

func isShellCommand(command string) bool {
	return policyShellInterpreters[commandBase(command)]
}

func commandUsesNetwork(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if policyNetworkCommands[base] {
		return true
	}
	return subcommandAllowed(policyNetworkSubcommands, base, argv)
}

func commandWrites(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if policyWriteCommands[base] {
		return true
	}
	return subcommandAllowed(policyWriteSubcommands, base, argv)
}

func readOnlyAllowed(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if policyReadOnlyCommands[base] {
		return true
	}
	return subcommandAllowed(policyReadOnlySubcommands, base, argv)
}

func subcommandAllowed(catalog map[string]map[string]bool, base string, argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	allowed, ok := catalog[base]
	return ok && allowed[strings.ToLower(argv[1])]
}

func commandPolicyCatalog() map[string]any {
	return map[string]any{
		"shell_interpreters":     sortedKeys(policyShellInterpreters),
		"network_commands":       sortedKeys(policyNetworkCommands),
		"network_subcommands":    sortedSubcommandCatalog(policyNetworkSubcommands),
		"write_commands":         sortedKeys(policyWriteCommands),
		"write_subcommands":      sortedSubcommandCatalog(policyWriteSubcommands),
		"read_only_commands":     sortedKeys(policyReadOnlyCommands),
		"read_only_subcommands":  sortedSubcommandCatalog(policyReadOnlySubcommands),
		"secret_path_patterns":   []string{"env files", "private keys", "credentials", "secret-like paths"},
		"secret_arg_assignments": []string{"token=", "password=", "secret=", "api_key=", "credential=", "authorization="},
	}
}

func stringSet(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSubcommandCatalog(catalog map[string]map[string]bool) map[string][]string {
	out := make(map[string][]string, len(catalog))
	for command, subcommands := range catalog {
		out[command] = sortedKeys(subcommands)
	}
	return out
}

func uniqSorted(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		if v != "" {
			m[v] = true
		}
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
