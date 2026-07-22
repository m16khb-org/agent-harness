package issueops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

type IssueOpsRemotePullRequestCreateFunc func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)

type IssueOpsRemoteCreateReconcileRequest struct {
	ID                   string
	ClaimID              string
	CoordinatorRecipient string
	Confirm              bool
	ApproveZeroClear     bool
	Host                 string
	SessionID            string
	AgentID              string
	SourceCWD            string
}

type IssueOpsRemoteCreateCandidate struct {
	URL              string
	Provider         string
	Kind             string
	ProjectKey       string
	SourceProjectKey string
	Head             string
	Base             string
	FinalHead        string
	Title            string
	BodySHA256       string
	Labels           []string
	Assignees        []string
	Draft            bool
}

type IssueOpsRemoteCreateProbeResult struct {
	Candidates        []IssueOpsRemoteCreateCandidate
	AuthoritativeZero bool
}

type IssueOpsRemoteCreateProbe func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error)

var finalizeIssueOpsRemoteCreateClaimForReconcile = FinalizeIssueOpsRemoteCreateClaim

func ProjectIssueOpsRemoteCreateProbeResult(record IssueOpsRecord, result port.IssueProviderReconcilePullRequestResult) (IssueOpsRemoteCreateProbeResult, error) {
	claim := record.RemoteCreateClaim
	if claim == nil {
		return IssueOpsRemoteCreateProbeResult{}, fmt.Errorf("remote create claim disappeared before provider reconciliation")
	}
	out := IssueOpsRemoteCreateProbeResult{AuthoritativeZero: result.AuthoritativeZero}
	for _, candidate := range result.Candidates {
		out.Candidates = append(out.Candidates, IssueOpsRemoteCreateCandidate{
			URL: candidate.URL, Provider: claim.Provider, Kind: claim.Kind, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
			Head: candidate.HeadBranch, Base: candidate.BaseBranch, FinalHead: candidate.HeadSHA,
			Title: candidate.Title, BodySHA256: candidate.BodySHA256,
			Labels: candidate.Labels, Assignees: candidate.Assignees, Draft: candidate.Draft,
		})
	}
	return out, nil
}

func ProjectIssueOpsRemoteCreateClaimForProviderReconcile(record IssueOpsRecord) (port.IssueProviderReconcilePullRequestRequest, error) {
	claim := record.RemoteCreateClaim
	if claim == nil {
		return port.IssueProviderReconcilePullRequestRequest{}, fmt.Errorf("remote create claim disappeared before provider reconciliation")
	}
	return port.IssueProviderReconcilePullRequestRequest{
		Repo:            record.Repo,
		ProjectKey:      claim.ProjectKey,
		HeadBranch:      claim.Head,
		BaseBranch:      claim.Base,
		ExpectedHeadSHA: claim.FinalHead,
		Title:           claim.Title,
		BodySHA256:      claim.BodySHA256,
		Labels:          append([]string(nil), claim.Labels...),
		Assignees:       append([]string(nil), claim.Assignees...),
		Draft:           claim.Draft,
	}, nil
}

func CreateIssueOpsRemotePullRequest(ctx context.Context, stateRoot, id, provider string, request port.IssueProviderCreatePullRequestRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, create IssueOpsRemotePullRequestCreateFunc) (port.IssueProviderCreatePullRequestResult, error) {
	if create == nil {
		return port.IssueProviderCreatePullRequestResult{}, fmt.Errorf("remote create provider dependency is unavailable")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{}, err
	}
	if currentIssueOpsHandoff(record) == nil || !request.Confirm {
		return create(request)
	}
	request.Title = strings.TrimSpace(request.Title)
	request.Body = strings.TrimSpace(request.Body)
	if request.Title == "" || len(request.Title) > 1024 {
		return port.IssueProviderCreatePullRequestResult{}, fmt.Errorf("supervised remote create title is required and must not exceed 1024 bytes")
	}
	if len(request.Body) > 1<<20 {
		return port.IssueProviderCreatePullRequestResult{}, fmt.Errorf("supervised remote create rendered body exceeds 1048576 bytes")
	}
	if policy.RedactFreeform(request.Title) != request.Title || policy.RedactFreeform(request.Body) != request.Body {
		return port.IssueProviderCreatePullRequestResult{}, fmt.Errorf("supervised remote create title or rendered body contains secret-like content")
	}
	actor := IssueOpsActor{Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD}
	if err := ValidateIssueOpsHandoffPublicationWithActor(ctx, stateRoot, record, provider, request.HeadBranch, request.BaseBranch, reader, lease, actor); err != nil {
		return port.IssueProviderCreatePullRequestResult{}, err
	}
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	}
	var result port.IssueProviderCreatePullRequestResult
	err = withIssueOpsRemoteCreateLiveLock(ctx, stateRoot, record.ID, func(spanCtx context.Context) error {
		claimed, claimErr := ClaimIssueOpsRemoteCreate(spanCtx, stateRoot, IssueOpsRemoteCreateClaimRequest{
			ID: record.ID, Provider: provider, Kind: kind, Title: request.Title, Body: request.Body, Head: request.HeadBranch, Base: request.BaseBranch,
			Labels: request.Labels, Assignees: request.Assignees, Draft: request.Draft, Actor: actor,
		})
		if claimErr != nil {
			return claimErr
		}
		claim := claimed.RemoteCreateClaim
		request.ProjectKey, request.HeadBranch, request.BaseBranch = claim.ProjectKey, claim.Head, claim.Base
		request.Repo = issueOpsPublicationWorkingRoot(claimed)
		request.Title, request.Body = claim.Title, claim.Body
		request.Labels, request.Assignees, request.Draft = append([]string(nil), claim.Labels...), append([]string(nil), claim.Assignees...), claim.Draft
		request.ExpectedHeadSHA = claim.FinalHead
		if validationErr := ValidateIssueOpsHandoffPublicationWithActor(spanCtx, stateRoot, claimed, claim.Provider, claim.Head, claim.Base, reader, lease, actor); validationErr != nil {
			proof := &port.IssueProviderCreateError{Invoked: false, Err: validationErr}
			clearErr := ClearIssueOpsRemoteCreateClaimPreInvocation(spanCtx, stateRoot, claimed, claim.ClaimID, proof)
			return combineRemoteCreateTransitionError("remote create publication revalidation failed before provider invocation", validationErr, clearErr)
		}
		var createErr error
		result, createErr = create(request)
		if createErr != nil {
			var typed *port.IssueProviderCreateError
			if errors.As(createErr, &typed) && !typed.Invoked {
				clearErr := ClearIssueOpsRemoteCreateClaimPreInvocation(spanCtx, stateRoot, claimed, claim.ClaimID, typed)
				return combineRemoteCreateTransitionError("remote create failed before provider invocation", createErr, clearErr)
			}
			markErr := MarkIssueOpsRemoteCreateUnknown(spanCtx, stateRoot, claimed, result.URL)
			return combineRemoteCreateTransitionError("remote create outcome is ambiguous and requires reconciliation; do not retry", createErr, markErr)
		}
		if validationErr := ValidateIssueOpsHandoffPublicationWithActor(spanCtx, stateRoot, claimed, provider, claim.Head, claim.Base, reader, lease, actor); validationErr != nil {
			markErr := MarkIssueOpsRemoteCreateUnknown(spanCtx, stateRoot, claimed, result.URL)
			return combineRemoteCreateTransitionError("remote ref changed after provider readback; create outcome requires reconciliation and must not be retried", validationErr, markErr)
		}
		_, finalizeErr := FinalizeIssueOpsRemoteCreateClaim(spanCtx, stateRoot, claimed, IssueOpsRemoteArtifactVerificationRequest{
			Provider: provider, Kind: claim.Kind, URL: result.URL, Labels: claim.Labels, Assignees: claim.Assignees, TargetBranch: claim.Base,
		})
		if finalizeErr != nil {
			current, readErr := ReadIssueOps(stateRoot, claimed.ID)
			markErr := readErr
			if readErr == nil {
				if current.RemoteCreateClaim == nil || current.RemoteCreateClaim.ClaimID != claim.ClaimID {
					markErr = fmt.Errorf("remote create claim identity changed before preserving the known URL")
				} else {
					markErr = MarkIssueOpsRemoteCreateUnknown(spanCtx, stateRoot, current, result.URL)
				}
			}
			return combineRemoteCreateTransitionError("provider succeeded but durable finalize failed; known URL requires reconciliation and must not be retried", finalizeErr, markErr)
		}
		return nil
	})
	return result, err
}

func ReconcileIssueOpsRemoteCreate(ctx context.Context, stateRoot string, req IssueOpsRemoteCreateReconcileRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, probe IssueOpsRemoteCreateProbe) (IssueOpsRecord, error) {
	var result IssueOpsRecord
	err := withIssueOpsRemoteCreateLiveLock(ctx, stateRoot, req.ID, func(spanCtx context.Context) error {
		var reconcileErr error
		result, reconcileErr = reconcileIssueOpsRemoteCreate(spanCtx, stateRoot, req, reader, lease, probe)
		return reconcileErr
	})
	return result, err
}

func reconcileIssueOpsRemoteCreate(ctx context.Context, stateRoot string, req IssueOpsRemoteCreateReconcileRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, probe IssueOpsRemoteCreateProbe) (IssueOpsRecord, error) {
	if !req.Confirm {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile requires explicit confirmation")
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	claim := record.RemoteCreateClaim
	if claim == nil || strings.TrimSpace(req.ClaimID) == "" || req.ClaimID != claim.ClaimID {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile requires the exact durable claim identity")
	}
	coordinator := ""
	if currentIssueOpsHandoff(record) != nil {
		coordinator = strings.TrimSpace(currentIssueOpsHandoff(record).CoordinatorMailboxHandle)
	}
	if coordinator == "" || strings.TrimSpace(req.CoordinatorRecipient) != coordinator {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile is coordinator-only and requires the sealed coordinator recipient")
	}
	if !handoff.CoordinatorIdentityMatches(record, model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.SourceCWD) {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile requires the sealed coordinator native session from the exact source checkout")
	}
	if err := ValidateIssueOpsHandoffPublication(ctx, stateRoot, record, claim.Provider, claim.Head, claim.Base, reader, lease); err != nil {
		markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, record, claim.KnownURL)
		return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile publication validation failed before live probe; claim retained unknown", err, markErr)
	}
	if probe == nil {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile live verification dependency is unavailable")
	}
	result, err := probe(ctx, record)
	if err != nil {
		markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, record, claim.KnownURL)
		return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile transport is ambiguous; claim retained unknown", err, markErr)
	}
	if len(result.Candidates) > 1 {
		primary := fmt.Errorf("remote create reconcile found multiple live candidates")
		markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, record, claim.KnownURL)
		return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile found multiple live candidates; claim retained unknown", primary, markErr)
	}
	if len(result.Candidates) == 0 {
		if !req.ApproveZeroClear || !result.AuthoritativeZero {
			primary := fmt.Errorf("zero-candidate remote create reconcile requires explicit user approval and authoritative live proof")
			markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, record, claim.KnownURL)
			return IssueOpsRecord{}, combineRemoteCreateTransitionError("zero-candidate remote create reconcile requires explicit user approval and authoritative live proof; claim retained unknown", primary, markErr)
		}
		current, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			return IssueOpsRecord{}, readErr
		}
		if err := ValidateIssueOpsHandoffPublication(ctx, stateRoot, current, claim.Provider, claim.Head, claim.Base, reader, lease); err != nil {
			markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, current, claim.KnownURL)
			return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile publication validation failed before authoritative zero clear; claim retained unknown", err, markErr)
		}
		return clearIssueOpsRemoteCreateClaimAfterAuthoritativeZero(ctx, stateRoot, current, claim.ClaimID)
	}
	candidate := result.Candidates[0]
	if err := validateRemoteCreateReconcileCandidate(record, candidate); err != nil {
		markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, record, claim.KnownURL)
		return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile candidate verification failed; claim retained unknown", err, markErr)
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if !reflect.DeepEqual(current.RemoteCreateClaim, claim) {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile claim identity changed after live probe; stale candidate was not adopted")
	}
	if err := ValidateIssueOpsHandoffPublication(ctx, stateRoot, current, claim.Provider, claim.Head, claim.Base, reader, lease); err != nil {
		markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, current, claim.KnownURL)
		return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile publication validation failed before finalize; claim retained unknown", err, markErr)
	}
	current, err = ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if !reflect.DeepEqual(current.RemoteCreateClaim, claim) {
		return IssueOpsRecord{}, fmt.Errorf("remote create reconcile claim identity changed after publication revalidation; stale candidate was not adopted")
	}
	finalized, finalizeErr := finalizeIssueOpsRemoteCreateClaimForReconcile(ctx, stateRoot, current, IssueOpsRemoteArtifactVerificationRequest{
		Provider: claim.Provider, Kind: claim.Kind, URL: candidate.URL, Labels: claim.Labels, Assignees: claim.Assignees, TargetBranch: claim.Base,
	})
	if finalizeErr == nil {
		return finalized, nil
	}
	markErr := MarkIssueOpsRemoteCreateUnknown(ctx, stateRoot, current, candidate.URL)
	return IssueOpsRecord{}, combineRemoteCreateTransitionError("remote create reconcile verified one candidate but finalize failed; claim retained unknown and needs reconciliation; creation was not retried", finalizeErr, markErr)
}

func withIssueOpsRemoteCreateLiveLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error {
	normalized, err := normalizeIssueOpsID(id)
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(filepath.Join(stateRoot, "remote-create-live", normalized))
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}

func combineRemoteCreateTransitionError(message string, primary, transition error) error {
	message = boundedRemoteCreateDiagnostic(message)
	if transition == nil {
		return fmt.Errorf("%s: %s", message, boundedRemoteCreateDiagnostic(primary.Error()))
	}
	return fmt.Errorf("%s: %s; durable state transition also failed: %s", message, boundedRemoteCreateDiagnostic(primary.Error()), boundedRemoteCreateDiagnostic(transition.Error()))
}

func boundedRemoteCreateDiagnostic(value string) string {
	value = strings.TrimSpace(policy.RedactDiagnostic(value))
	if len(value) > 384 {
		value = value[:384] + "...[truncated]"
	}
	return value
}

func validateRemoteCreateReconcileCandidate(record IssueOpsRecord, candidate IssueOpsRemoteCreateCandidate) error {
	claim := record.RemoteCreateClaim
	if claim == nil || candidate.Provider != claim.Provider || candidate.Kind != claim.Kind || candidate.ProjectKey != claim.ProjectKey || candidate.Head != claim.Head || candidate.Base != claim.Base || candidate.FinalHead != claim.FinalHead || candidate.Title != claim.Title || candidate.BodySHA256 != claim.BodySHA256 || candidate.Draft != claim.Draft ||
		!sameCanonicalRemoteCreateSet(candidate.Labels, claim.Labels) || !sameCanonicalRemoteCreateSet(candidate.Assignees, claim.Assignees) {
		return fmt.Errorf("remote create reconcile candidate does not match exact durable claim authority; claim retained unknown")
	}
	if candidate.SourceProjectKey != claim.ProjectKey {
		return fmt.Errorf("remote create reconcile candidate source project differs from published project; claim retained unknown")
	}
	if claim.KnownURL != "" && candidate.URL != claim.KnownURL {
		return fmt.Errorf("remote create reconcile candidate URL differs from durable known URL; claim retained unknown")
	}
	if err := remote.ValidateArtifactURL(candidate.URL, claim.Provider, claim.Kind); err != nil {
		return fmt.Errorf("remote create reconcile candidate URL is not canonical; claim retained unknown")
	}
	if err := remote.ValidateArtifactMatchesIssue(record.IssueURL, candidate.URL, claim.Provider, claim.Kind); err != nil {
		return fmt.Errorf("remote create reconcile candidate project differs from durable authority; claim retained unknown")
	}
	return nil
}

func sameCanonicalRemoteCreateSet(left, right []string) bool {
	left = remote.CleanValues(left)
	right = remote.CleanValues(right)
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	if len(seen) != len(right) {
		return false
	}
	for _, value := range right {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}

func clearIssueOpsRemoteCreateClaimAfterAuthoritativeZero(ctx context.Context, stateRoot string, expected IssueOpsRecord, claimID string) (IssueOpsRecord, error) {
	err := mutateRemoteCreateClaim(ctx, stateRoot, expected, func(r *IssueOpsRecord) error {
		if r.RemoteCreateClaim.ClaimID != claimID {
			return fmt.Errorf("remote create claim changed before authoritative zero clear")
		}
		r.RemoteCreateClaim = nil
		return nil
	})
	if err != nil {
		return IssueOpsRecord{}, err
	}
	return ReadIssueOps(stateRoot, expected.ID)
}

type IssueOpsRemoteCreateClaimRequest struct {
	ID        string
	Provider  string
	Kind      string
	Title     string
	Body      string
	Head      string
	Base      string
	Labels    []string
	Assignees []string
	Draft     bool
	Actor     IssueOpsActor
}

func ClaimIssueOpsRemoteCreate(ctx context.Context, stateRoot string, req IssueOpsRemoteCreateClaimRequest) (IssueOpsRecord, error) {
	var out IssueOpsRecord
	err := withIssueOpsLock(ctx, stateRoot, req.ID, func(context.Context) error {
		r, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if r.Phase != model.IssueOpsPhasePR || r.RemoteArtifact != nil {
			return fmt.Errorf("remote create requires phase pr and no existing artifact")
		}
		if r.RemoteCreateClaim != nil {
			return fmt.Errorf("remote create is already claimed or requires reconciliation")
		}
		h := currentIssueOpsHandoff(r)
		if h == nil || h.PublishReceipt == nil || r.BranchPrepare == nil {
			return fmt.Errorf("remote create requires published final head authority")
		}
		if err := validatePostTransferMutation(r, &req.Actor); err != nil {
			return err
		}
		provider := strings.ToLower(strings.TrimSpace(req.Provider))
		kind := strings.ToLower(strings.TrimSpace(req.Kind))
		if provider == "github" && kind != "pr" || provider == "gitlab" && kind != "mr" {
			return fmt.Errorf("remote create provider and artifact kind are inconsistent")
		}
		projectKey := remote.ProjectKey(r.IssueURL, provider, "issue")
		receipt := h.PublishReceipt
		finalHead := receipt.FinalHead
		if projectKey == "" || provider != strings.ToLower(strings.TrimSpace(r.BranchPrepare.Provider)) || strings.TrimSpace(req.Head) != r.Branch || strings.TrimSpace(req.Base) != r.BranchPrepare.BaseBranch ||
			receipt.Provider != provider || receipt.ProjectKey != projectKey || receipt.Branch != r.Branch || receipt.Base != r.BranchPrepare.BaseBranch || receipt.FinalHead != finalHead {
			return fmt.Errorf("remote create request does not match exact durable publication authority")
		}
		labels := remote.CleanValues(req.Labels)
		assignees := remote.CleanValues(req.Assignees)
		if len(labels) == 0 || len(assignees) == 0 {
			return fmt.Errorf("supervised remote create requires canonical labels and assignees")
		}
		if invalid := remote.InvalidAssignee(assignees); invalid != "" {
			return fmt.Errorf("supervised remote create assignee must not be placeholder %q", invalid)
		}
		if !req.Draft {
			return fmt.Errorf("supervised remote create must be draft")
		}
		title := strings.TrimSpace(req.Title)
		if title == "" || len(title) > 1024 || len(req.Body) > 1<<20 {
			return fmt.Errorf("remote create title or rendered body is outside canonical bounds")
		}
		if policy.RedactFreeform(title) != title || policy.RedactFreeform(req.Body) != req.Body {
			return fmt.Errorf("remote create title or rendered body contains secret-like content")
		}
		bodySum := sha256.Sum256([]byte(req.Body))
		claimID, err := newRemoteCreateClaimID()
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		r.RemoteCreateClaim = &model.IssueOpsRemoteCreateClaim{
			ClaimID: claimID, Provider: provider, Kind: kind, ProjectKey: projectKey,
			Remote: receipt.Remote, RemoteRef: receipt.RemoteRef, PushTargetSHA256: receipt.PushTargetSHA256, Head: r.Branch, Base: r.BranchPrepare.BaseBranch,
			FinalHead: finalHead, Labels: labels, Assignees: assignees, Draft: true,
			Title: title, Body: req.Body, BodySHA256: hex.EncodeToString(bodySum[:]),
			State: "pending", InvocationState: "reserved", ClaimedAt: now,
		}
		r.UpdatedAt = now
		out, err = writeIssueOps(stateRoot, r)
		return err
	})
	return out, err
}

func ClearIssueOpsRemoteCreateClaimPreInvocation(ctx context.Context, stateRoot string, expected IssueOpsRecord, liveClaimID string, proof *port.IssueProviderCreateError) error {
	if proof == nil || proof.Invoked {
		return fmt.Errorf("remote create claim clear requires typed pre-invocation failure proof")
	}
	return mutateRemoteCreateClaim(ctx, stateRoot, expected, func(r *IssueOpsRecord) error {
		if liveClaimID == "" || liveClaimID != r.RemoteCreateClaim.ClaimID || r.RemoteCreateClaim.State != "pending" || r.RemoteCreateClaim.InvocationState != "reserved" {
			return fmt.Errorf("invoked or unknown remote create claim cannot be cleared")
		}
		r.RemoteCreateClaim = nil
		return nil
	})
}

func MarkIssueOpsRemoteCreateUnknown(ctx context.Context, stateRoot string, expected IssueOpsRecord, knownURL string) error {
	knownURL = strings.TrimSpace(knownURL)
	var knownURLErr error
	err := mutateRemoteCreateClaim(ctx, stateRoot, expected, func(r *IssueOpsRecord) error {
		claim := r.RemoteCreateClaim
		if knownURL != "" {
			if claim.KnownURL != "" && knownURL != claim.KnownURL {
				knownURLErr = fmt.Errorf("known remote create URL differs from durable known URL")
			} else if err := remote.ValidateArtifactURL(knownURL, claim.Provider, claim.Kind); err != nil {
				knownURLErr = fmt.Errorf("known remote create URL is not canonical: %w", err)
			} else if err := remote.ValidateArtifactMatchesIssue(r.IssueURL, knownURL, claim.Provider, claim.Kind); err != nil {
				knownURLErr = err
			} else {
				claim.KnownURL = knownURL
			}
		}
		claim.State = "unknown"
		claim.InvocationState = "unknown"
		return nil
	})
	return errors.Join(knownURLErr, err)
}

func FinalizeIssueOpsRemoteCreateClaim(ctx context.Context, stateRoot string, expected IssueOpsRecord, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	var out IssueOpsRecord
	err := withIssueOpsLock(ctx, stateRoot, expected.ID, func(context.Context) error {
		r, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(r.RemoteCreateClaim, expected.RemoteCreateClaim) || r.RemoteCreateClaim == nil || (r.RemoteCreateClaim.State != "pending" && r.RemoteCreateClaim.State != "unknown") || (r.RemoteCreateClaim.InvocationState != "reserved" && r.RemoteCreateClaim.InvocationState != "unknown") {
			return fmt.Errorf("remote create claim changed before finalize")
		}
		claim := r.RemoteCreateClaim
		if strings.ToLower(strings.TrimSpace(req.Provider)) != claim.Provider || normalizedRemoteArtifactKind(req.Kind) != claim.Kind || strings.TrimSpace(req.TargetBranch) != claim.Base ||
			!reflect.DeepEqual(remote.CleanValues(req.Labels), claim.Labels) || !reflect.DeepEqual(remote.CleanValues(req.Assignees), claim.Assignees) {
			return fmt.Errorf("remote artifact finalize request does not match exact create claim authority")
		}
		if claim.KnownURL != "" && strings.TrimSpace(req.URL) != claim.KnownURL {
			return fmt.Errorf("remote artifact finalize URL differs from durable known URL")
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

func mutateRemoteCreateClaim(ctx context.Context, stateRoot string, expected IssueOpsRecord, fn func(*IssueOpsRecord) error) error {
	return withIssueOpsLock(ctx, stateRoot, expected.ID, func(context.Context) error {
		r, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(r.RemoteCreateClaim, expected.RemoteCreateClaim) || r.RemoteCreateClaim == nil {
			return fmt.Errorf("remote create claim changed")
		}
		if err := fn(&r); err != nil {
			return err
		}
		r.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_, err = writeIssueOps(stateRoot, r)
		return err
	})
}

func normalizedRemoteArtifactKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pull_request":
		return "pr"
	case "merge_request":
		return "mr"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func newRemoteCreateClaimID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate remote create claim identity: %w", err)
	}
	return "claim_" + hex.EncodeToString(value[:]), nil
}
