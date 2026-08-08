package policy

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
