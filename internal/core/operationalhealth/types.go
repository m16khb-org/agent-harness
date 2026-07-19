package operationalhealth

import "time"

const HeartbeatTTL = 15 * time.Minute

const (
	FindingInventoryUnknown     = "operational_inventory_unknown"
	FindingDeadOwner            = "operational_dead_owner"
	FindingWorktreeResidue      = "operational_worktree_residue"
	FindingTerminalResidue      = "operational_terminal_residue"
	FindingTaskResidue          = "operational_task_residue"
	FindingGateResidue          = "operational_gate_residue"
	FindingMessageResidue       = "operational_message_residue"
	FindingNonMainBranchResidue = "operational_non_main_branch_residue"
	FindingStateArtifactResidue = "operational_state_artifact_residue"
)

type Cycle struct {
	ID                     string
	Repo                   string
	Branch                 string
	Phase                  string
	HandoffState           string
	Attempt                int
	OwnershipEpoch         string
	ContextSHA256          string
	WorkerSessionID        string
	WorkerAgentID          string
	OrcaRuntimeID          string
	OrcaRepoID             string
	WorktreePath           string
	OrcaWorktreeID         string
	OrcaWorktreeInstanceID string
	TerminalHandle         string
	PTYID                  string
	TerminalTabID          string
	TerminalLeafID         string
	TaskID                 string
	DispatchID             string
	LastHeartbeatAt        time.Time
}

type Binding struct {
	CycleID          string
	Repo             string
	Branch           string
	ExpectedWorktree string
}

type GitWorktree struct {
	Path      string
	Branch    string
	Head      string
	Clean     bool
	Canonical bool
}

type GitRef struct {
	Name     string
	Branch   string
	OID      string
	Location string
}

type OrcaWorktree struct {
	RuntimeID  string
	RepoID     string
	ID         string
	InstanceID string
	Repo       string
	Path       string
	Branch     string
	Head       string
}

type OrcaTerminal struct {
	RuntimeID    string
	Handle       string
	PTYID        string
	TabID        string
	LeafID       string
	WorktreeID   string
	WorktreePath string
	Connected    bool
	Writable     bool
}

type OrcaTask struct {
	RuntimeID   string
	ID          string
	Status      string
	DispatchID  string
	CompletedAt time.Time
	HasResult   bool
}

type OrcaDispatch struct {
	RuntimeID      string
	ID             string
	TaskID         string
	AssigneeHandle string
	Status         string
}

type OrcaGate struct {
	RuntimeID string
	ID        string
	TaskID    string
	Status    string
}

type MessagePresence struct {
	RuntimeID       string
	Count           int
	Empty           bool
	CompleteAbsence bool
}

type InventoryProblem struct {
	Source string
	Code   string
	Detail string
}

type StateArtifact struct {
	Path string
	Code string
}

type Snapshot struct {
	RepoRoot        string
	CanonicalBranch string
	SourceHead      string
	SourceClean     bool
	OrcaObserved    bool
	OrcaRuntimeID   string
	OrcaRepoID      string

	Cycles            []Cycle
	Bindings          []Binding
	GitWorktrees      []GitWorktree
	LocalRefs         []GitRef
	RemoteRefs        []GitRef
	OrcaWorktrees     []OrcaWorktree
	Terminals         []OrcaTerminal
	Tasks             []OrcaTask
	Dispatches        []OrcaDispatch
	Gates             []OrcaGate
	Messages          MessagePresence
	StateArtifacts    []StateArtifact
	InventoryProblems []InventoryProblem
}

type Options struct {
	Now                     time.Time
	PreserveCycleIDs        []string
	PreserveTerminalHandles []string
}

type Finding struct {
	Code         string `json:"code"`
	ResourceKind string `json:"resource_kind"`
	ResourceID   string `json:"resource_id"`
	Summary      string `json:"summary"`
	Path         string `json:"path,omitempty"`
}

type Result struct {
	Healthy  bool      `json:"healthy"`
	Findings []Finding `json:"findings"`
}

type CycleAuthority string

const (
	AuthorityLive      CycleAuthority = "live"
	AuthorityPreserved CycleAuthority = "preserved"
	AuthorityDead      CycleAuthority = "dead"
	AuthorityUnknown   CycleAuthority = "unknown"
)
