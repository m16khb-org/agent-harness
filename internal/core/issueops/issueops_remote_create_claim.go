package issueops

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

func ClaimIssueOpsRemoteCreate(stateRoot, id, provider, head, base, finalHead string) (IssueOpsRecord, error) {
	var out IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		r, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if r.Phase != model.IssueOpsPhasePR || r.RemoteArtifact != nil {
			return fmt.Errorf("remote create requires phase pr and no existing artifact")
		}
		if r.RemoteCreateClaim != nil {
			return fmt.Errorf("remote create is already claimed or requires reconciliation")
		}
		h := r.ExecutionHandoff
		if h == nil || h.State != handoff.StateClosed || h.ClosedDisposition != handoff.DispositionAccepted || h.Result == nil || h.Result.FinalHead != finalHead {
			return fmt.Errorf("remote create requires accepted final head authority")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		r.RemoteCreateClaim = &model.IssueOpsRemoteCreateClaim{Provider: strings.ToLower(strings.TrimSpace(provider)), Head: strings.TrimSpace(head), Base: strings.TrimSpace(base), FinalHead: finalHead, State: "pending", ClaimedAt: now}
		r.UpdatedAt = now
		out, err = writeIssueOps(stateRoot, r)
		return err
	})
	return out, err
}

func ClearIssueOpsRemoteCreateClaim(stateRoot string, expected IssueOpsRecord) error {
	return mutateRemoteCreateClaim(stateRoot, expected, func(r *IssueOpsRecord) { r.RemoteCreateClaim = nil })
}
func MarkIssueOpsRemoteCreateUnknown(stateRoot string, expected IssueOpsRecord) error {
	return mutateRemoteCreateClaim(stateRoot, expected, func(r *IssueOpsRecord) { r.RemoteCreateClaim.State = "unknown" })
}

func FinalizeIssueOpsRemoteCreateClaim(stateRoot string, expected IssueOpsRecord, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	var out IssueOpsRecord
	err := withIssueOpsLock(stateRoot, expected.ID, func() error {
		r, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(r.RemoteCreateClaim, expected.RemoteCreateClaim) || r.RemoteCreateClaim == nil || r.RemoteCreateClaim.State != "pending" {
			return fmt.Errorf("remote create claim changed before finalize")
		}
		a, err := artifactverify.Projection(r, req)
		if err != nil {
			return err
		}
		r.RemoteArtifact = &a
		r.RemoteCreateClaim = nil
		r.UpdatedAt = a.VerifiedAt
		out, err = writeIssueOps(stateRoot, r)
		return err
	})
	return out, err
}

func mutateRemoteCreateClaim(stateRoot string, expected IssueOpsRecord, fn func(*IssueOpsRecord)) error {
	return withIssueOpsLock(stateRoot, expected.ID, func() error {
		r, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(r.RemoteCreateClaim, expected.RemoteCreateClaim) || r.RemoteCreateClaim == nil {
			return fmt.Errorf("remote create claim changed")
		}
		fn(&r)
		r.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_, err = writeIssueOps(stateRoot, r)
		return err
	})
}
