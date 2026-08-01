package issueopspreparation

import (
	"encoding/json"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

const (
	ModeAuto   = "auto"
	ModeDirect = "direct"
	ModeOrca   = "orca"
)

type Command struct {
	ID          string              `json:"id"`
	Mode        string              `json:"mode"`
	Actor       leasecontract.Actor `json:"actor"`
	CWD         string              `json:"cwd"`
	OwnerHost   string              `json:"owner_host,omitempty"`
	OwnerModel  string              `json:"owner_model,omitempty"`
	OwnerEffort string              `json:"owner_effort,omitempty"`
	Confirm     bool                `json:"confirm,omitempty"`
}

func (command Command) Clone() Command {
	cloned := command
	cloned.Actor = cloneActor(command.Actor)
	return cloned
}

type Result struct {
	OK                  bool                     `json:"ok"`
	ID                  string                   `json:"id"`
	Preview             bool                     `json:"preview,omitempty"`
	RequestedMode       string                   `json:"requested_mode"`
	ResolvedMode        string                   `json:"resolved_mode"`
	FallbackCode        string                   `json:"fallback_code,omitempty"`
	Workspace           leasecontract.Workspace  `json:"workspace"`
	Execution           *leasecontract.Execution `json:"execution,omitempty"`
	ClaimTokenPath      string                   `json:"claim_token_path,omitempty"`
	IssueBodySHA256     string                   `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string                   `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string                   `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string                   `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string                   `json:"owner_prompt_sha256,omitempty"`
	IssueSnapshotSource string                   `json:"issue_snapshot_source,omitempty"`
	NextCommand         string                   `json:"next_command,omitempty"`
}

func (result Result) Clone() Result {
	cloned := result
	cloned.Execution = cloneExecutionPointer(result.Execution)
	return cloned
}

type RootClaim struct {
	LifecycleID string
	Branch      string
	Root        string
}

type Snapshot struct {
	Record        leasecontract.Record
	RecordRaw     []byte
	CanonicalRoot string
	RootConflict  *RootClaim
}

func (snapshot Snapshot) Clone() Snapshot {
	cloned := snapshot
	cloned.Record = cloneRecord(snapshot.Record)
	cloned.RecordRaw = cloneBytes(snapshot.RecordRaw)
	if snapshot.RootConflict != nil {
		claim := *snapshot.RootConflict
		cloned.RootConflict = &claim
	}
	return cloned
}

func cloneRecord(record leasecontract.Record) leasecontract.Record {
	cloned := record
	cloned.Intent = cloneRaw(record.Intent)
	cloned.DesignReview = cloneRaw(record.DesignReview)
	cloned.DomainReview = cloneRaw(record.DomainReview)
	cloned.IssueLinks = cloneRaw(record.IssueLinks)
	cloned.BranchPrepare = cloneRaw(record.BranchPrepare)
	cloned.RemoteArtifact = cloneRaw(record.RemoteArtifact)
	cloned.Decisions = cloneRaw(record.Decisions)
	cloned.PlanPrep = cloneRaw(record.PlanPrep)
	cloned.CompatibilityReview = cloneRaw(record.CompatibilityReview)
	cloned.DevilsAdvocateReview = cloneRaw(record.DevilsAdvocateReview)
	cloned.Feedback = cloneRaw(record.Feedback)
	cloned.RegressEvents = cloneRaw(record.RegressEvents)
	cloned.Delegation = cloneRaw(record.Delegation)
	cloned.ChildCycles = cloneRaw(record.ChildCycles)
	cloned.RemoteCompletion = cloneRaw(record.RemoteCompletion)
	cloned.CleanupFinishFailure = cloneRaw(record.CleanupFinishFailure)
	cloned.ImplementationReview = cloneRaw(record.ImplementationReview)
	cloned.RoutingTrace = cloneRaw(record.RoutingTrace)
	cloned.AISlopCleanCategories = cloneRaw(record.AISlopCleanCategories)
	cloned.AISlopCleanVerification = cloneRaw(record.AISlopCleanVerification)
	cloned.PhaseLedger = cloneRaw(record.PhaseLedger)
	cloned.Execution = cloneExecutionPointer(record.Execution)
	return cloned
}

func cloneExecutionPointer(execution *leasecontract.Execution) *leasecontract.Execution {
	if execution == nil {
		return nil
	}
	cloned := *execution
	cloned.Lease.Holder = cloneActorPointer(execution.Lease.Holder)
	if execution.Orca != nil {
		binding := *execution.Orca
		cloned.Orca = &binding
	}
	if execution.Pending != nil {
		pending := *execution.Pending
		cloned.Pending = &pending
	}
	if execution.Completion != nil {
		completion := *execution.Completion
		completion.Verification = cloneStrings(execution.Completion.Verification)
		cloned.Completion = &completion
	}
	if execution.Failure != nil {
		failure := *execution.Failure
		cloned.Failure = &failure
	}
	if execution.SyncBaseEvents != nil {
		cloned.SyncBaseEvents = append([]leasecontract.SyncBaseEvent{}, execution.SyncBaseEvents...)
	}
	return &cloned
}

func cloneActor(actor leasecontract.Actor) leasecontract.Actor {
	cloned := actor
	if actor.SessionProcess != nil {
		process := *actor.SessionProcess
		cloned.SessionProcess = &process
	}
	return cloned
}

func cloneActorPointer(actor *leasecontract.Actor) *leasecontract.Actor {
	if actor == nil {
		return nil
	}
	cloned := cloneActor(*actor)
	return &cloned
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage{}, value...)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
