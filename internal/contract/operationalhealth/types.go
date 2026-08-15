package operationalhealth

import "time"

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
	FindingExecutionFailure     = "operational_execution_failure"
	FindingIssueCreateFailure   = "operational_issue_create_failure"
	FindingCleanupFailure       = "operational_cleanup_failure"
)

const (
	ProcessStatusLive             = "live"
	ProcessStatusDead             = "dead"
	ProcessStatusIdentityMismatch = "identity-mismatch"
	ProcessStatusUnknown          = "unknown"
)

type Cycle struct {
	ID                        string
	Repo                      string
	Branch                    string
	Phase                     string
	ExecutionMode             string
	LeaseStatus               string
	Generation                uint64
	HolderHost                string
	HolderSessionID           string
	HolderAgentID             string
	HolderPID                 int
	HolderStartedAt           string
	HolderExecutable          string
	HolderProcessStatus       string
	CompletionPresent         bool
	ExecutionFailurePresent   bool
	CleanupFailurePresent     bool
	IssueCreateFailurePresent bool
	OrcaRuntimeID             string
	OrcaRepoID                string
	WorktreePath              string
	OrcaWorktreeID            string
	OrcaWorktreeInstanceID    string
	OrcaOwnerHost             string
	TerminalPTYID             string
	RunID                     string
	TaskID                    string
	DispatchID                string
}

type LeaseHolderIndex struct {
	Key         string
	LifecycleID string
	Generation  uint64
	Host        string
	SessionID   string
	AgentID     string
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
	RunID       string
	ID          string
	Status      string
	DispatchID  string
	CompletedAt time.Time
	HasResult   bool
}

type OrcaDispatch struct {
	RuntimeID      string
	RunID          string
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

	Cycles             []Cycle
	LeaseHolderIndexes []LeaseHolderIndex
	GitWorktrees       []GitWorktree
	LocalRefs          []GitRef
	RemoteRefs         []GitRef
	OrcaWorktrees      []OrcaWorktree
	Terminals          []OrcaTerminal
	Tasks              []OrcaTask
	Dispatches         []OrcaDispatch
	Gates              []OrcaGate
	Messages           MessagePresence
	StateArtifacts     []StateArtifact
	InventoryProblems  []InventoryProblem
}

// Profile selects how strictly unowned-but-live resources are classified.
// ProfileSealed (the zero value) is the audit contract: every terminal and
// inbox row must be accounted for by a cycle. ProfileInteractive is for a live
// developer machine, where Orca tabs the user opened directly and orchestration
// message history are normal, not residue.
const (
	ProfileSealed      = ""
	ProfileInteractive = "interactive"
)

type Options struct {
	Now                     time.Time
	Profile                 string
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
