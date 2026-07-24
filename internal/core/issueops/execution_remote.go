package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", model.IssueOpsSchemaVersion)

const (
	externalIntentRemotePR     = "remote_pr_create"
	remoteInvocationUnknown    = "unknown"
	remoteInvocationNotInvoked = "not_invoked_proven"
)

type RemotePullRequestCreateFunc func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
type RemotePullRequestReconcileFunc func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error)
type RemoteArtifactVerifyFunc func(model.IssueOpsRemoteArtifactVerificationRequest) error

type RemotePullRequestRequest struct {
	ID                 string            `json:"id"`
	Provider           string            `json:"provider"`
	Title              string            `json:"title"`
	Body               string            `json:"body"`
	Head               string            `json:"head"`
	Base               string            `json:"base"`
	Labels             []string          `json:"labels"`
	Assignees          []string          `json:"assignees"`
	ExpectedGeneration uint64            `json:"expected_generation"`
	Actor              model.NativeActor `json:"actor"`
	CWD                string            `json:"cwd"`
	Confirm            bool              `json:"confirm"`
}

type RemotePullRequestDependencies struct {
	Create    RemotePullRequestCreateFunc
	Reconcile RemotePullRequestReconcileFunc
	Verify    RemoteArtifactVerifyFunc
	Now       func() time.Time
}

type externalRemotePRPayload struct {
	SchemaVersion   int                                        `json:"schema_version"`
	OperationID     string                                     `json:"operation_id"`
	Generation      uint64                                     `json:"generation"`
	Provider        string                                     `json:"provider"`
	Kind            string                                     `json:"kind"`
	Request         port.IssueProviderCreatePullRequestRequest `json:"request"`
	InvocationState string                                     `json:"invocation_state"`
	RetryCount      int                                        `json:"retry_count"`
	KnownURL        string                                     `json:"known_url,omitempty"`
}

// CreateRemotePullRequest는 IssueOps v1의 유일한 PR/MR 생성 경로다. provider를
// 호출하기 전에 정확한 operation intent 하나를 영속화하며, 모호한 호출은 절대
// 재시도하지 않는다.
func CreateRemotePullRequest(ctx context.Context, stateRoot string, req RemotePullRequestRequest, deps RemotePullRequestDependencies) (port.IssueProviderCreatePullRequestResult, error) {
	if req.Confirm {
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return port.IssueProviderCreatePullRequestResult{}, err
		}
	}
	if deps.Create == nil {
		return port.IssueProviderCreatePullRequestResult{}, fmt.Errorf("remote pull request provider is unavailable")
	}
	if req.Confirm {
		actor, err := normalizeNativeActor(req.Actor)
		if err != nil {
			return port.IssueProviderCreatePullRequestResult{}, err
		}
		req.Actor = actor
	}
	record, providerReq, kind, err := prepareRemotePullRequest(stateRoot, req)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{}, err
	}
	if !req.Confirm {
		return deps.Create(strings.ToLower(strings.TrimSpace(req.Provider)), providerReq)
	}
	pending, payload, err := beginRemotePullRequestIntent(stateRoot, record, req.Actor, req.CWD, req.ExpectedGeneration, providerReq, req.Provider, kind, deps.Now)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{}, err
	}
	result, callErr := deps.Create(payload.Provider, payload.Request)
	if callErr != nil {
		invocation := remoteInvocationUnknown
		var typed *port.IssueProviderCreateError
		if errors.As(callErr, &typed) && !typed.Invoked {
			invocation = remoteInvocationNotInvoked
		}
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, invocation, payload.RetryCount, result.URL, callErr, deps.Now)
		return result, fmt.Errorf("remote create outcome requires execution reconcile; creation was not retried: %w", callErr)
	}
	if strings.TrimSpace(result.URL) == "" {
		err = fmt.Errorf("provider create returned no canonical URL")
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, "", err, deps.Now)
		return result, err
	}
	if err := verifyRemotePullRequestResult(record, payload, result.URL, deps.Verify); err != nil {
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, result.URL, err, deps.Now)
		return result, fmt.Errorf("provider returned a URL but durable verification requires execution reconcile: %w", err)
	}
	if _, err := finishRemotePullRequestIntent(stateRoot, pending.ID, payload, result.URL, true, deps.Now); err != nil {
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, result.URL, err, deps.Now)
		return result, fmt.Errorf("provider succeeded but durable receipt requires execution reconcile: %w", err)
	}
	return result, nil
}

func prepareRemotePullRequest(stateRoot string, req RemotePullRequestRequest) (IssueOpsRecord, port.IssueProviderCreatePullRequestRequest, string, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", err
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	} else if provider != "github" {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote provider must be github or gitlab")
	}
	if record.Phase != model.IssueOpsPhasePR || record.RemoteArtifact != nil {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires phase pr and no existing remote artifact")
	}
	if req.Confirm {
		if record.Execution == nil {
			return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires IssueOps execution v1")
		}
		if req.ExpectedGeneration == 0 || record.Execution.Lease.Generation != req.ExpectedGeneration {
			return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, req.ExpectedGeneration)
		}
		mutationActor := IssueOpsActor{
			Host: req.Actor.Host, SessionID: req.Actor.SessionID, AgentID: req.Actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: req.Actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(record, &mutationActor); err != nil {
			return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", err
		}
		if record.Execution.Pending != nil {
			return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("external intent is already pending; run execution reconcile")
		}
	}
	if record.BranchPrepare == nil {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires branch preparation")
	}
	projectKey := remote.ProjectKey(record.IssueURL, provider, "issue")
	head, base := strings.TrimSpace(req.Head), strings.TrimSpace(req.Base)
	workspaceBranch := strings.TrimSpace(record.Branch)
	if record.Execution != nil {
		workspaceBranch = record.Execution.Workspace.Branch
	}
	if projectKey == "" || provider != strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)) || head != workspaceBranch || base != strings.TrimSpace(record.BranchPrepare.BaseBranch) {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create request does not match execution workspace and linked issue authority")
	}
	title, body := strings.TrimSpace(req.Title), strings.TrimSpace(req.Body)
	if title == "" || len(title) > 1024 || len(body) > 1<<20 {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create title is required and body must not exceed 1048576 bytes")
	}
	if policy.RedactFreeform(title) != title || policy.RedactFreeform(body) != body {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create title or body contains secret-like content")
	}
	labels, assignees := remote.CleanValues(req.Labels), remote.CleanValues(req.Assignees)
	if len(labels) == 0 || len(assignees) == 0 {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires canonical labels and assignees")
	}
	if invalid := remote.InvalidAssignee(assignees); invalid != "" {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create assignee must not be placeholder %q", invalid)
	}
	headSHA := ""
	if req.Confirm {
		headSHA = issueOpsCurrentHead(record)
	}
	if req.Confirm && !validExecutionHead(headSHA) {
		return IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires a resolvable canonical worktree HEAD")
	}
	workingRoot := record.Repo
	if req.Confirm {
		workingRoot = record.Execution.Workspace.Root
	}
	return record, port.IssueProviderCreatePullRequestRequest{
		Repo: workingRoot, ProjectKey: projectKey, Title: title, Body: body,
		HeadBranch: head, BaseBranch: base, Labels: labels, Assignees: assignees,
		Draft: true, ExpectedHeadSHA: headSHA, Confirm: req.Confirm,
		Host: req.Actor.Host, SessionID: req.Actor.SessionID, AgentID: req.Actor.AgentID, CWD: req.CWD,
	}, kind, nil
}

func beginRemotePullRequestIntent(stateRoot string, expected IssueOpsRecord, actor model.NativeActor, cwd string, expectedGeneration uint64, providerReq port.IssueProviderCreatePullRequestRequest, provider, kind string, now func() time.Time) (IssueOpsRecord, externalRemotePRPayload, error) {
	operationID, err := newExecutionOperationID()
	if err != nil {
		return IssueOpsRecord{}, externalRemotePRPayload{}, err
	}
	marker := "<!-- agent-harness:issueops-v1 operation=" + operationID + " -->"
	providerReq.Body = strings.TrimSpace(providerReq.Body) + "\n\n" + marker
	payload := externalRemotePRPayload{
		SchemaVersion: model.IssueOpsSchemaVersion, OperationID: operationID, Provider: provider, Kind: kind,
		Request: providerReq, InvocationState: remoteInvocationUnknown,
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		mutationActor := IssueOpsActor{
			Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: cwd,
			NativeProcessAncestry: actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(current, &mutationActor); err != nil {
			return err
		}
		if current.Execution == nil || current.Execution.Pending != nil || current.RemoteArtifact != nil {
			return fmt.Errorf("remote create authority changed before intent CAS")
		}
		if expectedGeneration == 0 || current.Execution.Lease.Generation != expectedGeneration {
			return fmt.Errorf("stale lease generation before remote intent CAS")
		}
		payload.Generation = current.Execution.Lease.Generation
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		current.Execution.Pending = &model.ExternalIntent{OperationID: operationID, Kind: externalIntentRemotePR, Marker: marker, StartedAt: executionNow(now)}
		current.Execution.Failure = nil
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{
			Bucket: externalIntentBucket, ID: operationID, Data: data, RequireAbsent: true,
		}})
		return err
	})
	return persisted, payload, err
}

func recordRemotePullRequestFailure(stateRoot, id, operationID, invocation string, retryCount int, knownURL string, cause error, now func() time.Time) error {
	return withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != operationID {
			return fmt.Errorf("external intent changed before failure receipt")
		}
		payload, err := readExternalRemotePRPayload(stateRoot, operationID)
		if err != nil {
			return err
		}
		payload.InvocationState, payload.RetryCount = invocation, retryCount
		if strings.TrimSpace(knownURL) != "" {
			payload.KnownURL = strings.TrimSpace(knownURL)
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		message := boundedExecutionRemoteDiagnostic(cause)
		record.Execution.Failure = &model.ExecutionFailure{OperationID: operationID, Code: "external_operation_ambiguous", Message: message, At: executionNow(now)}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: operationID, Data: data}})
		return err
	})
}

func finishRemotePullRequestIntent(stateRoot, id string, payload externalRemotePRPayload, url string, enforceOriginalGeneration bool, now func() time.Time) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if current.Execution == nil || current.Execution.Pending == nil || current.Execution.Pending.OperationID != payload.OperationID {
			return fmt.Errorf("external intent changed before remote receipt CAS")
		}
		if enforceOriginalGeneration {
			lease := current.Execution.Lease
			holder := lease.Holder
			if payload.Generation == 0 || lease.Generation != payload.Generation || lease.Status != model.LeaseStatusActive || holder == nil ||
				!strings.EqualFold(holder.Host, payload.Request.Host) || holder.SessionID != payload.Request.SessionID || holder.AgentID != payload.Request.AgentID ||
				!samePath(payload.Request.CWD, current.Execution.Workspace.Root) {
				return fmt.Errorf("remote receipt belongs to a stale execution generation; execution reconcile is required")
			}
		}
		stored, err := readExternalRemotePRPayload(stateRoot, payload.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, payload) {
			return fmt.Errorf("external intent payload changed before remote receipt CAS")
		}
		artifact, err := artifactverify.Projection(current, model.IssueOpsRemoteArtifactVerificationRequest{
			Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url),
			Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch,
		})
		if err != nil {
			return err
		}
		artifact.VerifiedAt = executionNow(now)
		current.RemoteArtifact = &artifact
		current.Execution.Pending = nil
		current.Execution.Failure = nil
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: payload.OperationID, Delete: true}})
		return err
	})
	return persisted, err
}

func readExternalRemotePRPayload(stateRoot, operationID string) (externalRemotePRPayload, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return externalRemotePRPayload{}, err
	}
	data, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return externalRemotePRPayload{}, err
	}
	if !ok {
		return externalRemotePRPayload{}, fmt.Errorf("external intent payload is missing")
	}
	var payload externalRemotePRPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return externalRemotePRPayload{}, fmt.Errorf("decode external intent payload: %w", err)
	}
	if payload.SchemaVersion != model.IssueOpsSchemaVersion || payload.OperationID != operationID || payload.Generation == 0 || payload.Provider == "" || payload.Kind == "" {
		return externalRemotePRPayload{}, fmt.Errorf("external intent payload is invalid")
	}
	return payload, nil
}

func remotePullRequestReconcileRequest(payload externalRemotePRPayload) port.IssueProviderReconcilePullRequestRequest {
	req := payload.Request
	sum := sha256.Sum256([]byte(req.Body))
	return port.IssueProviderReconcilePullRequestRequest{
		Repo: req.Repo, ProjectKey: req.ProjectKey, HeadBranch: req.HeadBranch, BaseBranch: req.BaseBranch,
		ExpectedHeadSHA: req.ExpectedHeadSHA, Title: req.Title, BodySHA256: hex.EncodeToString(sum[:]),
		Labels: append([]string(nil), req.Labels...), Assignees: append([]string(nil), req.Assignees...), Draft: req.Draft,
	}
}

func validateRemotePullRequestCandidate(record IssueOpsRecord, payload externalRemotePRPayload, candidate port.IssueProviderReconcilePullRequestCandidate) error {
	expected := remotePullRequestReconcileRequest(payload)
	if strings.TrimSpace(candidate.ProjectKey) != expected.ProjectKey || strings.TrimSpace(candidate.SourceProjectKey) != expected.ProjectKey ||
		strings.TrimSpace(candidate.HeadBranch) != expected.HeadBranch || strings.TrimSpace(candidate.BaseBranch) != expected.BaseBranch ||
		strings.TrimSpace(candidate.HeadSHA) != expected.ExpectedHeadSHA || strings.TrimSpace(candidate.Title) != expected.Title ||
		strings.TrimSpace(candidate.BodySHA256) != expected.BodySHA256 || candidate.Draft != expected.Draft ||
		!sameCanonicalRemoteSet(candidate.Labels, expected.Labels) || !sameCanonicalRemoteSet(candidate.Assignees, expected.Assignees) {
		return fmt.Errorf("remote reconcile candidate does not match the exact durable intent")
	}
	if payload.KnownURL != "" && strings.TrimSpace(candidate.URL) != payload.KnownURL {
		return fmt.Errorf("remote reconcile candidate URL differs from the durable known URL")
	}
	if err := remote.ValidateArtifactURL(candidate.URL, payload.Provider, payload.Kind); err != nil {
		return err
	}
	return remote.ValidateArtifactMatchesIssue(record.IssueURL, candidate.URL, payload.Provider, payload.Kind)
}

func verifyRemotePullRequestResult(record IssueOpsRecord, payload externalRemotePRPayload, url string, verify RemoteArtifactVerifyFunc) error {
	req := model.IssueOpsRemoteArtifactVerificationRequest{
		Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url),
		Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch,
	}
	if _, err := artifactverify.Projection(record, req); err != nil {
		return err
	}
	if verify != nil {
		return verify(req)
	}
	return nil
}

func sameCanonicalRemoteSet(left, right []string) bool {
	left, right = remote.CleanValues(left), remote.CleanValues(right)
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

func validExecutionHead(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func boundedExecutionRemoteDiagnostic(err error) string {
	if err == nil {
		return "external operation failed"
	}
	message := strings.TrimSpace(policy.RedactDiagnostic(err.Error()))
	if len(message) > 4096 {
		message = message[:4096]
	}
	return message
}
