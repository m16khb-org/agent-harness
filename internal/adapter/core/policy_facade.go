package core

import (
	"regexp"
	"sort"

	"agent-harness/internal/core/policy"
)

type CommandPolicyRequest = policy.CommandPolicyRequest
type CommandPolicyEvaluation = policy.CommandPolicyEvaluation
type CommandFakeRunResult = policy.CommandFakeRunResult
type CommandRunResult = policy.CommandRunResult
type PolicyDeniedError = policy.PolicyDeniedError

const (
	PolicyTierReadOnly       = policy.PolicyTierReadOnly
	PolicyTierWorkspaceWrite = policy.PolicyTierWorkspaceWrite
	PolicyTierNetworkAccess  = policy.PolicyTierNetworkAccess
	PolicyTierShellException = policy.PolicyTierShellException
)

func EvaluateCommandPolicy(req CommandPolicyRequest) CommandPolicyEvaluation {
	return policy.EvaluateCommandPolicy(req)
}

func FakeRunCommand(req CommandPolicyRequest) CommandFakeRunResult {
	return policy.FakeRunCommand(req)
}

func RunReadOnlyCommand(req CommandPolicyRequest) CommandRunResult {
	return policy.RunReadOnlyCommand(req)
}

func CommandPolicySummary() map[string]any {
	return policy.CommandPolicySummary()
}

func IsPolicyDenied(err error) bool {
	return policy.IsPolicyDenied(err)
}

func cleanEnvAllowlist(items []string) []string {
	return policy.CleanEnvAllowlist(items)
}

func redactArgv(argv []string) []string {
	return policy.RedactArgv(argv)
}

func redactFreeform(s string) string {
	return policy.RedactFreeform(s)
}

var secretPathRe = regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)

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
