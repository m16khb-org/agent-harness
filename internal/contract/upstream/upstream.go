// Package upstream declares the shared DTOs for upstream plugin and skill
// provisioning: what the harness declares as upstream, what it observed on the
// host, and what one sync pass decided and did.
package upstream

// PluginEntry is one upstream host plugin the harness declares. Name and
// Marketplace form the host plugin id; Source is the marketplace origin the
// host needs registered before the plugin resolves.
type PluginEntry struct {
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
	Source      string `json:"source"`
}

// ID is the host-facing plugin identifier, "name@marketplace".
func (e PluginEntry) ID() string { return e.Name + "@" + e.Marketplace }

// SkillEntry is one upstream skill the harness declares. Repo is a git remote,
// Path is the skill directory inside that repo (empty means repo root), and Ref
// is the branch or tag to fetch (empty means the remote default branch).
type SkillEntry struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
	Path string `json:"path,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

// Config is the on-disk upstream declaration (configs/upstream.json).
type Config struct {
	Version int           `json:"version"`
	Plugins []PluginEntry `json:"plugins,omitempty"`
	Skills  []SkillEntry  `json:"skills,omitempty"`
}

// Observed is what the host already has, as read before planning.
type Observed struct {
	Plugins      []string `json:"plugins"`
	Marketplaces []string `json:"marketplaces"`
	Skills       []string `json:"skills"`
}

// Kind separates the two declaration families in plans and reports.
const (
	KindPlugin = "plugin"
	KindSkill  = "skill"
)

// Action is what a sync pass decided for one declared entry.
const (
	ActionInstall = "install"
	ActionSkip    = "skip"
)

// Status is what a sync pass actually did with one declared entry.
const (
	StatusInstalled = "installed"
	StatusSkipped   = "skipped"
	StatusPlanned   = "planned"
	StatusFailed    = "failed"
)

// PlanItem is one planned decision. Exactly one of Plugin or Skill is set.
type PlanItem struct {
	Kind           string       `json:"kind"`
	Name           string       `json:"name"`
	Action         string       `json:"action"`
	Reason         string       `json:"reason,omitempty"`
	AddMarketplace bool         `json:"add_marketplace,omitempty"`
	Plugin         *PluginEntry `json:"plugin,omitempty"`
	Skill          *SkillEntry  `json:"skill,omitempty"`
}

// ItemResult is the outcome for one declared entry after a sync pass.
type ItemResult struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Report is the outcome of one sync pass over the whole declaration.
type Report struct {
	DryRun bool         `json:"dry_run,omitempty"`
	Items  []ItemResult `json:"items"`
}
