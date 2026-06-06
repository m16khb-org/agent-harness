package policy

import (
	"regexp"
	"sort"
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
