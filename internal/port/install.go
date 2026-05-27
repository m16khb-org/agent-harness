package port

// NativeInstallRequest describes one host-neutral installation run.
// Core code owns orchestration; host adapters own concrete files for each host.
type NativeInstallRequest struct {
	Root                string   `json:"root"`
	Home                string   `json:"home"`
	CodexHome           string   `json:"codex_home"`
	BinPath             string   `json:"bin_path"`
	LLMWikiRoot         string   `json:"llm_wiki_root"`
	PortableLLMWikiRoot string   `json:"portable_llm_wiki_root"`
	SkillNames          []string `json:"skill_names"`
	ProjectLocal        bool     `json:"project_local"`
	ClaudeUserHook      bool     `json:"claude_user_hook"`
}

// NativeInstallResult is the aggregate result of all host installers.
type NativeInstallResult struct {
	OK                  bool                `json:"ok"`
	Root                string              `json:"root"`
	Home                string              `json:"home"`
	CodexHome           string              `json:"codex_home"`
	BinPath             string              `json:"bin_path"`
	LLMWikiRoot         string              `json:"llm_wiki_root"`
	PortableLLMWikiRoot string              `json:"portable_llm_wiki_root"`
	SkillNames          []string            `json:"skill_names"`
	Hosts               []HostInstallResult `json:"hosts"`
	Files               []InstallFile       `json:"files"`
	Links               []InstallLink       `json:"links"`
	Messages            []string            `json:"messages,omitempty"`
	ProjectLocal        bool                `json:"project_local"`
	ClaudeUserHook      bool                `json:"claude_user_hook"`
}

// HostInstallResult reports one concrete host adapter installation.
type HostInstallResult struct {
	Host     string        `json:"host"`
	OK       bool          `json:"ok"`
	Files    []InstallFile `json:"files,omitempty"`
	Links    []InstallLink `json:"links,omitempty"`
	Messages []string      `json:"messages,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// InstallFile reports a file the installer manages.
type InstallFile struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Written bool   `json:"written"`
}

// InstallLink reports a symlink the installer manages.
type InstallLink struct {
	Path    string `json:"path"`
	Target  string `json:"target"`
	Created bool   `json:"created"`
}

// HostInstaller is implemented by host-specific adapters such as Codex and Claude Code.
type HostInstaller interface {
	Name() string
	Install(NativeInstallRequest) (HostInstallResult, error)
}
