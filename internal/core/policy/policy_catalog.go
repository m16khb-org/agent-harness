package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

type policyCatalog struct {
	shellInterpreters   map[string]bool
	networkCommands     map[string]bool
	networkSubcommands  map[string]map[string]bool
	writeCommands       map[string]bool
	writeSubcommands    map[string]map[string]bool
	readOnlyCommands    map[string]bool
	readOnlySubcommands map[string]map[string]bool
	warnings            []string
}

func builtinPolicyCatalogSnapshot() policyCatalog {
	return policyCatalog{
		shellInterpreters:   copyStringSet(builtinShellInterpreters),
		networkCommands:     copyStringSet(builtinNetworkCommands),
		networkSubcommands:  copySubcommandCatalog(builtinNetworkSubcommands),
		writeCommands:       copyStringSet(builtinWriteCommands),
		writeSubcommands:    copySubcommandCatalog(builtinWriteSubcommands),
		readOnlyCommands:    copyStringSet(builtinReadOnlyCommands),
		readOnlySubcommands: copySubcommandCatalog(builtinReadOnlySubcommands),
		warnings:            []string{},
	}
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

func policyCatalogForWorkspace(repoRoot string) policyCatalog {
	catalog := builtinPolicyCatalogSnapshot()
	overrides, err := readPolicyOverrides(repoRoot)
	if err != nil {
		catalog.warnings = append(catalog.warnings, policyOverrideWarning(err))
		return catalog
	}
	if overrides != nil {
		mergePolicyOverrides(&catalog, overrides)
	}
	return catalog
}

func readPolicyOverrides(repoRoot string) (*PolicyOverrides, error) {
	path := filepath.Join(repoRoot, ".agent-harness", "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("policy_override_read_failed: %w", err)
	}
	var overrides PolicyOverrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("policy_override_parse_failed: %w", err)
	}
	return &overrides, nil
}

func policyOverrideWarning(err error) string {
	msg := err.Error()
	if strings.HasPrefix(msg, "policy_override_parse_failed:") {
		return "policy_override_parse_failed"
	}
	if strings.HasPrefix(msg, "policy_override_read_failed:") {
		return "policy_override_read_failed"
	}
	return "policy_override_failed"
}

func mergePolicyOverrides(catalog *policyCatalog, o *PolicyOverrides) {
	for _, v := range o.AdditionalShellInterpreters {
		catalog.shellInterpreters[v] = true
	}
	for _, v := range o.AdditionalNetworkCommands {
		catalog.networkCommands[v] = true
	}
	for cmd, subs := range o.AdditionalNetworkSubcommands {
		if catalog.networkSubcommands[cmd] == nil {
			catalog.networkSubcommands[cmd] = map[string]bool{}
		}
		for _, s := range subs {
			catalog.networkSubcommands[cmd][s] = true
		}
	}
	for _, v := range o.AdditionalWriteCommands {
		catalog.writeCommands[v] = true
	}
	for cmd, subs := range o.AdditionalWriteSubcommands {
		if catalog.writeSubcommands[cmd] == nil {
			catalog.writeSubcommands[cmd] = map[string]bool{}
		}
		for _, s := range subs {
			catalog.writeSubcommands[cmd][s] = true
		}
	}
	for _, v := range o.AdditionalReadOnlyCommands {
		catalog.readOnlyCommands[v] = true
	}
	for cmd, subs := range o.AdditionalReadOnlySubcommands {
		if catalog.readOnlySubcommands[cmd] == nil {
			catalog.readOnlySubcommands[cmd] = map[string]bool{}
		}
		for _, s := range subs {
			catalog.readOnlySubcommands[cmd][s] = true
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
	catalog := builtinPolicyCatalogSnapshot()
	return map[string]any{
		"shell_interpreters":     sortedKeys(catalog.shellInterpreters),
		"network_commands":       sortedKeys(catalog.networkCommands),
		"network_subcommands":    sortedSubcommandCatalog(catalog.networkSubcommands),
		"write_commands":         sortedKeys(catalog.writeCommands),
		"write_subcommands":      sortedSubcommandCatalog(catalog.writeSubcommands),
		"read_only_commands":     sortedKeys(catalog.readOnlyCommands),
		"read_only_subcommands":  sortedSubcommandCatalog(catalog.readOnlySubcommands),
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
