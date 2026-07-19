package operationalhealth

import "time"

const HeartbeatTTL = 15 * time.Minute

const FindingDeadOwner = "operational_dead_owner"

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
	WorktreePath           string
	OrcaWorktreeID         string
	OrcaWorktreeInstanceID string
	TerminalHandle         string
	PTYID                  string
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

type OrcaTask struct {
	ID     string
	Status string
}

type Snapshot struct {
	Cycles   []Cycle
	Bindings []Binding
	Tasks    []OrcaTask
}

type Options struct {
	Now                     time.Time
	PreserveCycleIDs        []string
	PreserveTerminalHandles []string
}

type Finding struct {
	Code         string
	ResourceKind string
	ResourceID   string
	Summary      string
	Path         string
}

type Result struct {
	Healthy  bool
	Findings []Finding
}

type CycleAuthority string

const (
	AuthorityLive      CycleAuthority = "live"
	AuthorityPreserved CycleAuthority = "preserved"
	AuthorityDead      CycleAuthority = "dead"
	AuthorityUnknown   CycleAuthority = "unknown"
)
