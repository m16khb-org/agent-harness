package issueopsinventory

import (
	"fmt"
	"strings"

	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

func NormalizeID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
}

func ProjectEntry(record issueopsinventorycontract.Record) issueopsinventorycontract.ListEntry {
	entry := issueopsinventorycontract.ListEntry{
		ID:        record.ID,
		Repo:      record.Repo,
		Branch:    record.Branch,
		Phase:     record.Phase,
		UpdatedAt: record.UpdatedAt,
		Invalid:   record.Invalid,
	}
	if record.RemoteArtifact != nil {
		entry.RemoteArtifactURL = record.RemoteArtifact.URL
		entry.CompletionUnreflected = record.RemoteCompletion == nil ||
			record.RemoteCompletion.ReflectedAt == ""
	}
	if record.Execution != nil {
		entry.Mode = string(record.Execution.Mode)
		entry.LeaseStatus = string(record.Execution.Lease.Status)
		entry.WorkspaceRoot = record.Execution.Workspace.Root
		if record.Execution.Lease.Holder != nil {
			entry.HolderHost = record.Execution.Lease.Holder.Host
			entry.HolderSession = record.Execution.Lease.Holder.SessionID
		}
		if record.Execution.Orca != nil {
			entry.OwnerModel = record.Execution.Orca.OwnerModel
		}
		entry.Claimable = record.Execution.Lease.Status == issueopsinventorycontract.LeaseStatusClaimable
	}
	entry.CleanupCandidate = record.Phase == issueopsinventorycontract.PhaseDone
	return entry
}
