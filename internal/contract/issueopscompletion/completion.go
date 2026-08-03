// Package issueopscompletion defines the transport-neutral execution
// completion contract shared by domain, application, and adapters.
package issueopscompletion

import leasecontract "agent-harness/internal/contract/issueopslease"

type Execution = leasecontract.Execution

var ErrExecutionNotPrepared = leasecontract.ErrExecutionNotPrepared

type ProcessReceipt struct {
	PID        int
	StartedAt  string
	Executable string
}

type Actor struct {
	Host      string
	SessionID string
	AgentID   string
	Process   *ProcessReceipt
}

type Lease struct {
	Generation        uint64
	Status            string
	Holder            *Actor
	ClaimTokenSHA256  string
	ClaimedAt         string
	ReleasedAt        string
	ReplacedAt        string
	ReplacementReason string
}

type Completion struct {
	Generation        uint64
	FinalHead         string
	TuringReportPath  string
	Verification      []string
	RemoteArtifactURL string
	CompletedAt       string
}

type LedgerEntry struct {
	Phase       string
	EnteredAt   string
	CompletedAt string
	Artifacts   []string
	Missing     []string
	Notes       []string
}

type Command struct {
	Generation        uint64
	Actor             Actor
	FinalHead         string
	TuringReportPath  string
	Verification      []string
	RemoteArtifactURL string
}

type RemoteArtifact struct {
	Provider     string
	Kind         string
	URL          string
	Labels       []string
	Assignees    []string
	VerifiedAt   string
	TargetBranch string
}

type OrcaBinding struct {
	RunID  string
	TaskID string
}

// RecordSnapshot is the minimum persisted projection required to decide and
// apply one completion transition. Persistence-specific raw sidecars stay in
// the outbound repository.
type RecordSnapshot struct {
	ID            string
	Prepared      bool
	Phase         string
	IssueURL      string
	CanonicalRoot string
	Mode          string
	Lease         Lease
	Completion    *Completion
	Artifact      *RemoteArtifact
	BaseBranch    string
	Ledger        map[string]LedgerEntry
	Orca          *OrcaBinding
}

func (r RecordSnapshot) Clone() RecordSnapshot {
	result := r
	if r.Lease.Holder != nil {
		holder := *r.Lease.Holder
		if holder.Process != nil {
			process := *holder.Process
			holder.Process = &process
		}
		result.Lease.Holder = &holder
	}
	if r.Completion != nil {
		completion := *r.Completion
		completion.Verification = append([]string(nil), completion.Verification...)
		result.Completion = &completion
	}
	if r.Artifact != nil {
		artifact := *r.Artifact
		artifact.Labels = append([]string(nil), artifact.Labels...)
		artifact.Assignees = append([]string(nil), artifact.Assignees...)
		result.Artifact = &artifact
	}
	if r.Orca != nil {
		orca := *r.Orca
		result.Orca = &orca
	}
	result.Ledger = make(map[string]LedgerEntry, len(r.Ledger))
	for phase, entry := range r.Ledger {
		entry.Artifacts = append([]string(nil), entry.Artifacts...)
		entry.Missing = append([]string(nil), entry.Missing...)
		entry.Notes = append([]string(nil), entry.Notes...)
		result.Ledger[phase] = entry
	}
	return result
}
