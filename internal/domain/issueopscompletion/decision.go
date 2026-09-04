// Package issueopscompletion owns the pure execution completion transition.
package issueopscompletion

import (
	"errors"
	"fmt"
	"strings"
	"time"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

type Actor = completioncontract.Actor
type ProcessReceipt = completioncontract.ProcessReceipt
type Lease = completioncontract.Lease
type Completion = completioncontract.Completion
type LedgerEntry = completioncontract.LedgerEntry
type Command = completioncontract.Command

type Snapshot struct {
	Phase      string
	Lease      Lease
	Completion *Completion
	Ledger     map[string]LedgerEntry
}

type Outcome struct {
	Phase      string
	Lease      Lease
	Completion *Completion
	Ledger     map[string]LedgerEntry
}

type DenyCode string

const (
	DenyPhase     DenyCode = "phase"
	DenyAuthority DenyCode = "authority"
	DenyCWD       DenyCode = "cwd"
)

type Denial struct {
	Code  DenyCode
	Cause error
}

func (d *Denial) Error() string { return d.Cause.Error() }
func (d *Denial) Unwrap() error { return d.Cause }

func deny(code DenyCode, message string) error {
	return &Denial{Code: code, Cause: errors.New(message)}
}

func CodeOf(err error) DenyCode {
	if denial, ok := errors.AsType[*Denial](err); ok {
		return denial.Code
	}
	return ""
}

func ValidateActive(snapshot Snapshot, command Command, canonicalCWD bool) error {
	if snapshot.Phase != "pr" {
		return deny(DenyPhase, "execution completion requires pr phase")
	}
	if snapshot.Lease.Status != "active" || snapshot.Lease.Generation != command.Generation || !sameActor(snapshot.Lease.Holder, &command.Actor) {
		return deny(DenyAuthority, fmt.Sprintf("only the current holder may complete generation %d", command.Generation))
	}
	if !canonicalCWD {
		return deny(DenyCWD, "completion cwd must be the canonical worktree")
	}
	return nil
}

func Apply(snapshot Snapshot, command Command, resolvedReport string, now time.Time) Outcome {
	return ApplyAt(snapshot, command, resolvedReport, now, now)
}

// ApplyAt preserves the established completion contract: the durable completion
// receipt/released lease and the phase-ledger transition observe consecutive
// clock reads rather than sharing a synthesized timestamp.
func ApplyAt(snapshot Snapshot, command Command, resolvedReport string, completedAt, transitionedAt time.Time) Outcome {
	completionTimestamp := completedAt.UTC().Format(time.RFC3339Nano)
	transitionTimestamp := transitionedAt.UTC().Format(time.RFC3339Nano)
	lease := snapshot.Lease
	lease.Status = "released"
	lease.Holder = nil
	lease.ClaimTokenSHA256 = ""
	lease.ReleasedAt = completionTimestamp
	completion := &Completion{
		Generation: command.Generation, FinalHead: strings.ToLower(strings.TrimSpace(command.FinalHead)), VerificationReportPath: resolvedReport,
		Verification: append([]string(nil), command.Verification...), RemoteArtifactURL: strings.TrimSpace(command.RemoteArtifactURL), CompletedAt: completionTimestamp,
	}
	ledger := cloneLedger(snapshot.Ledger)
	previous := ledger["pr"]
	previous.Phase = "pr"
	if previous.EnteredAt == "" {
		previous.EnteredAt = transitionTimestamp
	}
	previous.CompletedAt = transitionTimestamp
	previous.Artifacts = []string{"strict_pr_readiness", "children_complete", "remote_artifact", "target_branch_match"}
	previous.Missing = nil
	previous.Notes = clearStaleNotes(previous.Notes)
	ledger["pr"] = previous
	done := ledger["done"]
	done.Phase = "done"
	if done.EnteredAt == "" {
		done.EnteredAt = transitionTimestamp
	}
	done.Notes = clearStaleNotes(done.Notes)
	ledger["done"] = done
	return Outcome{Phase: "done", Lease: lease, Completion: completion, Ledger: ledger}
}

func sameActor(left, right *Actor) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	if !strings.EqualFold(strings.TrimSpace(left.Host), strings.TrimSpace(right.Host)) || left.SessionID != strings.TrimSpace(right.SessionID) || left.AgentID != strings.TrimSpace(right.AgentID) {
		return false
	}
	if left.Process == nil || right.Process == nil {
		return left.Process == nil && right.Process == nil
	}
	return *left.Process == *right.Process
}

func cloneLedger(source map[string]LedgerEntry) map[string]LedgerEntry {
	result := make(map[string]LedgerEntry, len(source)+2)
	for phase, entry := range source {
		entry.Artifacts = append([]string(nil), entry.Artifacts...)
		entry.Missing = append([]string(nil), entry.Missing...)
		entry.Notes = append([]string(nil), entry.Notes...)
		result[phase] = entry
	}
	return result
}

func clearStaleNotes(notes []string) []string {
	kept := make([]string, 0, len(notes))
	for _, note := range notes {
		if !strings.HasPrefix(note, "stale:") {
			kept = append(kept, note)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}
