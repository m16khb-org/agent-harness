package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
)

var secretPathRe = regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)

var secretArgRe = regexp.MustCompile(`(?i)(token|password|passwd|secret|api[_-]?key|credential|authorization)=`)

// Built-in policy sets. These are the default allow/deny lists.
var builtinShellInterpreters = stringSet("sh", "bash", "zsh", "fish", "dash", "ksh")

var builtinNetworkCommands = stringSet("curl", "wget", "ssh", "scp", "sftp", "rsync", "gh", "brew", "npm", "pnpm", "yarn", "pip", "pip3")

var builtinNetworkSubcommands = map[string]map[string]bool{
	"git": stringSet("clone", "fetch", "pull", "push", "ls-remote", "submodule"),
}

var builtinWriteCommands = stringSet("touch", "mkdir", "rmdir", "rm", "mv", "cp", "install", "chmod", "chown", "tee", "python", "python3", "node", "ruby", "perl")

var builtinWriteSubcommands = map[string]map[string]bool{
	"git": stringSet("add", "commit", "reset", "clean", "checkout", "switch", "merge", "rebase", "cherry-pick", "revert", "push", "pull", "apply", "am", "stash"),
	"go":  stringSet("build", "test", "run", "install", "mod", "work", "generate"),
}

var builtinReadOnlyCommands = stringSet("pwd", "ls", "cat", "grep", "rg", "find", "sed", "awk", "head", "tail", "wc", "test", "stat", "true", "false")

var builtinReadOnlySubcommands = map[string]map[string]bool{
	"git": stringSet("status", "diff", "log", "show", "rev-parse", "branch", "remote", "ls-files", "grep", "describe", "merge-base", "config"),
	"go":  stringSet("version", "env", "list"),
}

// Active policy sets, initialized to built-in defaults and optionally
// merged with overrides from .agent-harness/policy.json.
var (
	policyShellInterpreters   map[string]bool
	policyNetworkCommands     map[string]bool
	policyNetworkSubcommands  map[string]map[string]bool
	policyWriteCommands       map[string]bool
	policyWriteSubcommands    map[string]map[string]bool
	policyReadOnlyCommands    map[string]bool
	policyReadOnlySubcommands map[string]map[string]bool

	policyOverridesOnce sync.Once
)

func init() {
	initPolicySets()
}

func initPolicySets() {
	policyShellInterpreters = copyStringSet(builtinShellInterpreters)
	policyNetworkCommands = copyStringSet(builtinNetworkCommands)
	policyNetworkSubcommands = copySubcommandCatalog(builtinNetworkSubcommands)
	policyWriteCommands = copyStringSet(builtinWriteCommands)
	policyWriteSubcommands = copySubcommandCatalog(builtinWriteSubcommands)
	policyReadOnlyCommands = copyStringSet(builtinReadOnlyCommands)
	policyReadOnlySubcommands = copySubcommandCatalog(builtinReadOnlySubcommands)
}

// PolicyOverrides describes additional allow/deny entries loaded from
// .agent-harness/policy.json. Every field is additive; overrides cannot
// remove entries from the built-in catalog.
type PolicyOverrides struct {
	AdditionalShellInterpreters   []string            `json:"additional_shell_interpreters,omitempty"`
	AdditionalNetworkCommands     []string            `json:"additional_network_commands,omitempty"`
	AdditionalNetworkSubcommands  map[string][]string `json:"additional_network_subcommands,omitempty"`
	AdditionalWriteCommands       []string            `json:"additional_write_commands,omitempty"`
	AdditionalWriteSubcommands    map[string][]string `json:"additional_write_subcommands,omitempty"`
	AdditionalReadOnlyCommands    []string            `json:"additional_read_only_commands,omitempty"`
	AdditionalReadOnlySubcommands map[string][]string `json:"additional_read_only_subcommands,omitempty"`
}

// LoadPolicyOverrides reads .agent-harness/policy.json from repoRoot if it
// exists and merges additional allow/deny entries into the active policy
// sets. Overrides are additive and cannot remove built-in entries.
// If the file does not exist, the built-in catalog is used unchanged.
// This function is safe to call multiple times; the file is read at most
// once (first call wins).
func LoadPolicyOverrides(repoRoot string) {
	policyOverridesOnce.Do(func() {
		initPolicySets()
		overrides, err := readPolicyOverrides(repoRoot)
		if err != nil || overrides == nil {
			return
		}
		mergePolicyOverrides(overrides)
	})
}

// ResetPolicyOverrides resets the policy sets to built-in defaults.
// Exposed for tests.
func ResetPolicyOverrides() {
	policyOverridesOnce = sync.Once{}
	initPolicySets()
}

func readPolicyOverrides(repoRoot string) (*PolicyOverrides, error) {
	path := filepath.Join(repoRoot, ".agent-harness", "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read policy overrides: %w", err)
	}
	var overrides PolicyOverrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parse policy overrides: %w", err)
	}
	return &overrides, nil
}

func mergePolicyOverrides(o *PolicyOverrides) {
	for _, v := range o.AdditionalShellInterpreters {
		policyShellInterpreters[v] = true
	}
	for _, v := range o.AdditionalNetworkCommands {
		policyNetworkCommands[v] = true
	}
	for cmd, subs := range o.AdditionalNetworkSubcommands {
		if policyNetworkSubcommands[cmd] == nil {
			policyNetworkSubcommands[cmd] = map[string]bool{}
		}
		for _, s := range subs {
			policyNetworkSubcommands[cmd][s] = true
		}
	}
	for _, v := range o.AdditionalWriteCommands {
		policyWriteCommands[v] = true
	}
	for cmd, subs := range o.AdditionalWriteSubcommands {
		if policyWriteSubcommands[cmd] == nil {
			policyWriteSubcommands[cmd] = map[string]bool{}
		}
		for _, s := range subs {
			policyWriteSubcommands[cmd][s] = true
		}
	}
	for _, v := range o.AdditionalReadOnlyCommands {
		policyReadOnlyCommands[v] = true
	}
	for cmd, subs := range o.AdditionalReadOnlySubcommands {
		if policyReadOnlySubcommands[cmd] == nil {
			policyReadOnlySubcommands[cmd] = map[string]bool{}
		}
		for _, s := range subs {
			policyReadOnlySubcommands[cmd][s] = true
		}
	}
}

func copyStringSet(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k := range src {
		out[k] = true
	}
	return out
}

func copySubcommandCatalog(src map[string]map[string]bool) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(src))
	for cmd, subs := range src {
		out[cmd] = copyStringSet(subs)
	}
	return out
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
