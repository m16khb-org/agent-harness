package policy

import "sort"

const (
	TierReadOnly       = "read_only"
	TierWorkspaceWrite = "workspace_write"
	TierNetworkAccess  = "network_access"
	TierShellException = "shell_exception"
)

type Request struct {
	WriteAllowed   bool
	NetworkAllowed bool
	ShellAllowed   bool
}

type Tier struct {
	Name                string   `json:"name"`
	GrantedCapabilities []string `json:"granted_capabilities"`
	Rationale           string   `json:"rationale"`
}

func ResolveTier(request Request) Tier {
	capabilities := []string{}
	if request.WriteAllowed {
		capabilities = append(capabilities, "write")
	}
	if request.NetworkAllowed {
		capabilities = append(capabilities, "network")
	}
	if request.ShellAllowed {
		capabilities = append(capabilities, "shell")
	}
	sort.Strings(capabilities)
	name := TierReadOnly
	switch {
	case request.ShellAllowed:
		name = TierShellException
	case request.NetworkAllowed:
		name = TierNetworkAccess
	case request.WriteAllowed:
		name = TierWorkspaceWrite
	}
	return Tier{Name: name, GrantedCapabilities: capabilities, Rationale: Rationale(name)}
}

func Rationale(name string) string {
	switch name {
	case TierShellException:
		return "shell interpreter exception granted; requires an explicit shell_reason and is audited"
	case TierNetworkAccess:
		return "network capability granted; shell interpreters remain denied"
	case TierWorkspaceWrite:
		return "write capability granted within the workspace; network and shell remain denied"
	default:
		return "no write, network, or shell capability requested; restricted to the read-only allowlist"
	}
}
