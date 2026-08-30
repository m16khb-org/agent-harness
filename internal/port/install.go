package port

// NativeInstallRequest describes one host-neutral installation run.
// Core code owns orchestration; host adapters own concrete files for each host.
type NativeInstallRequest struct {
	Root             string   `json:"root"`
	Home             string   `json:"home"`
	CodexHome        string   `json:"codex_home"`
	BinPath          string   `json:"bin_path"`
	SkillNames       []string `json:"skill_names"`
	ProjectLocal     bool     `json:"project_local"`
	DryRun           bool     `json:"dry_run,omitempty"`
	AdoptCommandFile bool     `json:"adopt_command_file,omitempty"`
}

// NativeActivationEvidence is a host adapter's strict semantic readback of
// one installed MCP or hook surface. IssueOps seals the referenced file bytes
// and identity only after every first-party host returns all required surfaces.
type NativeActivationEvidence struct {
	Host           string `json:"host"`
	Surface        string `json:"surface"`
	Path           string `json:"path"`
	SemanticSHA256 string `json:"semantic_sha256"`
	SHA256         string `json:"sha256"`
	Mode           uint32 `json:"mode"`
	Size           int64  `json:"size"`
	Device         uint64 `json:"device"`
	Inode          uint64 `json:"inode"`
}

// NativeInstallResult is the aggregate result of all first-party host installers.
type NativeInstallResult struct {
	OK            bool                      `json:"ok"`
	Root          string                    `json:"root"`
	Home          string                    `json:"home"`
	CodexHome     string                    `json:"codex_home"`
	BinPath       string                    `json:"bin_path"`
	SkillNames    []string                  `json:"skill_names"`
	Hosts         []HostInstallResult       `json:"hosts"`
	Files         []InstallFile             `json:"files"`
	Links         []InstallLink             `json:"links"`
	Messages      []string                  `json:"messages,omitempty"`
	ProjectLocal  bool                      `json:"project_local"`
	DryRun        bool                      `json:"dry_run,omitempty"`
	CommandPath   *ManagedCommandPathResult `json:"command_path,omitempty"`
	TransitionID  string                    `json:"transition_id,omitempty"`
	Committed     bool                      `json:"committed,omitempty"`
	AbortRequired bool                      `json:"abort_required,omitempty"`
}

type ManagedCommandPathResult struct {
	Path              string `json:"path"`
	Target            string `json:"target"`
	BackupPath        string `json:"backup_path,omitempty"`
	AdoptionApproved  bool   `json:"adoption_approved"`
	WouldAdopt        bool   `json:"would_adopt,omitempty"`
	Adopted           bool   `json:"adopted,omitempty"`
	Committed         bool   `json:"committed,omitempty"`
	RolledBack        bool   `json:"rolled_back,omitempty"`
	RollbackAvailable bool   `json:"rollback_available,omitempty"`
	BackupRetained    bool   `json:"backup_retained,omitempty"`
	AbortRequired     bool   `json:"abort_required,omitempty"`
}

// HostInstallResult reports one concrete host adapter installation.
type HostInstallResult struct {
	Host     string        `json:"host"`
	OK       bool          `json:"ok"`
	DryRun   bool          `json:"dry_run,omitempty"`
	Files    []InstallFile `json:"files,omitempty"`
	Links    []InstallLink `json:"links,omitempty"`
	Messages []string      `json:"messages,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// InstallFile reports a file the installer manages.
type InstallFile struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Written    bool   `json:"written"`
	WouldWrite bool   `json:"would_write,omitempty"`
}

// InstallLink reports a symlink the installer manages.
type InstallLink struct {
	Path        string `json:"path"`
	Target      string `json:"target"`
	Created     bool   `json:"created"`
	WouldCreate bool   `json:"would_create,omitempty"`
	// Removed/WouldRemove report a stale harness-owned link (target under this
	// checkout's skills/ no longer exists) that install/update pruned or would prune.
	Removed     bool `json:"removed,omitempty"`
	WouldRemove bool `json:"would_remove,omitempty"`
}

// InstallPlan is the shared accumulation contract used by host installers.
// Concrete adapters depend on this port instead of redeclaring the same method
// set per host.
type InstallPlan interface {
	Err(error)
	Errs([]error)
	File(InstallFile, error)
	Files([]InstallFile)
	Link(InstallLink, error)
	Links([]InstallLink)
	Message(string)
	Messages([]string)
	Finish() (HostInstallResult, error)
}

// HostInstaller is implemented by host-specific adapters such as Codex, Claude Code, and Omo.
type HostInstaller interface {
	Name() string
	Install(NativeInstallRequest) (HostInstallResult, error)
}
