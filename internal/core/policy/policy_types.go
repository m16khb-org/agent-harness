package policy

import (
	"sort"
	"strings"
)

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

// PolicyTier is a host-neutral synthesis of the scattered capability flags
// (write/network/shell) into one named privilege envelope, derived from
// Qwen Code's tiered-approval model. It classifies what a request is permitted
// to attempt; it does not decide whether a specific command is allowed (that
// stays in DenyReasons). The YOLO/auto-escalation tiers are deliberately
// excluded so a single approval never raises the whole session's safety level.

type PolicyTier struct {
	Name                string   `json:"name"`
	GrantedCapabilities []string `json:"granted_capabilities"`
	Rationale           string   `json:"rationale"`
}

const (
	PolicyTierReadOnly       = "read_only"
	PolicyTierWorkspaceWrite = "workspace_write"
	PolicyTierNetworkAccess  = "network_access"
	PolicyTierShellException = "shell_exception"
)

// resolvePolicyTier names the most-privileged capability the request grants and
// enumerates every granted capability. The classification is a pure function of
// the request flags, so the request→tier table is fixed by golden tests.

func resolvePolicyTier(req CommandPolicyRequest) PolicyTier {
	caps := []string{}
	if req.WriteAllowed {
		caps = append(caps, "write")
	}
	if req.NetworkAllowed {
		caps = append(caps, "network")
	}
	if req.ShellAllowed {
		caps = append(caps, "shell")
	}
	sort.Strings(caps)
	name := PolicyTierReadOnly
	switch {
	case req.ShellAllowed:
		name = PolicyTierShellException
	case req.NetworkAllowed:
		name = PolicyTierNetworkAccess
	case req.WriteAllowed:
		name = PolicyTierWorkspaceWrite
	}
	return PolicyTier{
		Name:                name,
		GrantedCapabilities: caps,
		Rationale:           policyTierRationale(name),
	}
}

func policyTierRationale(name string) string {
	switch name {
	case PolicyTierShellException:
		return "shell interpreter exception granted; requires an explicit shell_reason and is audited"
	case PolicyTierNetworkAccess:
		return "network capability granted; shell interpreters remain denied"
	case PolicyTierWorkspaceWrite:
		return "write capability granted within the workspace; network and shell remain denied"
	default:
		return "no write, network, or shell capability requested; restricted to the read-only allowlist"
	}
}

type CommandPolicyEvaluation struct {
	OK             bool       `json:"ok"`
	Allowed        bool       `json:"allowed"`
	AuditLogID     string     `json:"audit_log_id"`
	WorkspaceRoot  string     `json:"workspace_root"`
	CWD            string     `json:"cwd"`
	Argv           []string   `json:"argv"`
	Timeout        string     `json:"timeout"`
	EnvAllowlist   []string   `json:"env_allowlist"`
	NetworkAllowed bool       `json:"network_allowed"`
	WriteAllowed   bool       `json:"write_allowed"`
	ShellAllowed   bool       `json:"shell_allowed"`
	ShellReason    string     `json:"shell_reason,omitempty"`
	Tier           PolicyTier `json:"tier"`
	DenyReasons    []string   `json:"deny_reasons"`
	Warnings       []string   `json:"warnings"`
	GeneratedAt    string     `json:"generated_at"`
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

type CommandRunResult struct {
	OK         bool                    `json:"ok"`
	Executed   bool                    `json:"executed"`
	ExitCode   int                     `json:"exit_code"`
	Stdout     string                  `json:"stdout,omitempty"`
	Stderr     string                  `json:"stderr,omitempty"`
	StartedAt  string                  `json:"started_at"`
	FinishedAt string                  `json:"finished_at"`
	DurationMS int64                   `json:"duration_ms"`
	TimedOut   bool                    `json:"timed_out"`
	ReadOnly   bool                    `json:"read_only"`
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
