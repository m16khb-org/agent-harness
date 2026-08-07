package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/adapter/issueops/artifactverify"
	"agent-harness/internal/adapter/issueops/implementation"
	"agent-harness/internal/adapter/issueops/remote"
	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/adapter/policy"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", issueops.IssueOpsSchemaVersion)

const (
	externalIntentRemotePR     = "remote_pr_create"
	remoteInvocationUnknown    = "unknown"
	remoteInvocationNotInvoked = "not_invoked_proven"
)

type RemotePullRequestCreateFunc func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
type RemotePullRequestReconcileFunc func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error)
type RemoteArtifactVerifyFunc func(issueops.IssueOpsRemoteArtifactVerificationRequest) error

type RemotePullRequestRequest struct {
	ID                 string               `json:"id"`
	Provider           string               `json:"provider"`
	Title              string               `json:"title"`
	Body               string               `json:"body"`
	Head               string               `json:"head"`
	Base               string               `json:"base"`
	Labels             []string             `json:"labels"`
	Assignees          []string             `json:"assignees"`
	ExpectedGeneration uint64               `json:"expected_generation"`
	Actor              issueops.NativeActor `json:"actor"`
	CWD                string               `json:"cwd"`
	Confirm            bool                 `json:"confirm"`
}

type RemotePullRequestDependencies struct {
	Handler RemotePullRequestCreateHandler
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
	if deps.Handler == nil {
		return port.IssueProviderCreatePullRequestResult{}, ErrRemotePullRequestCreateHandlerUnavailable
	}
	if req.Confirm {
		actor, err := normalizeNativeActor(req.Actor)
		if err != nil {
			return port.IssueProviderCreatePullRequestResult{}, err
		}
		req.Actor = actor
	}
	return deps.Handler(ctx, stateRoot, req)
}

func prepareRemotePullRequest(stateRoot string, req RemotePullRequestRequest) (issueops.IssueOpsRecord, port.IssueProviderCreatePullRequestRequest, string, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", err
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	} else if provider != "github" {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote provider must be github or gitlab")
	}
	if record.Phase != issueops.IssueOpsPhasePR || record.RemoteArtifact != nil {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires phase pr and no existing remote artifact")
	}
	if req.Confirm {
		if record.Execution == nil {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires IssueOps execution v1")
		}
		if req.ExpectedGeneration == 0 || record.Execution.Lease.Generation != req.ExpectedGeneration {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, req.ExpectedGeneration)
		}
		mutationActor := IssueOpsActor{
			Host: req.Actor.Host, SessionID: req.Actor.SessionID, AgentID: req.Actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: req.Actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(record, &mutationActor); err != nil {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", err
		}
		if record.Execution.Pending != nil {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("external intent is already pending; run execution reconcile")
		}
		// orca 모드(이원 구조 사이클) 한정 하드 게이트: planner급 brooks 리뷰의
		// pass 기록 없이는 publication을 열지 않는다(설계 v5 WS5). direct 모드는
		// 단독 구현 세션의 자기리뷰가 devils-advocate ledger로 기록되므로 제외.
		currentReviewFingerprint := implementation.ChangeFingerprint(record)
		if missing := implementationReviewMissing(record, currentReviewFingerprint); missing != "" {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires a pass implementation review (%s); record it with `agent-harness issueops implementation-review record --id %s ...`", missing, record.ID)
		}
		// ai_slop_clean 선례(strict:59-61)와 동형: 리뷰가 fingerprint를 봉인했는데
		// 현재 값을 계산할 수 없으면 staleness 판정을 조용히 끄는 대신 거부한다.
		if record.Execution.Mode == issueops.ExecutionModeOrca && currentReviewFingerprint == "" &&
			record.ImplementationReview != nil && record.ImplementationReview.ReviewedFingerprint != "" {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create cannot verify implementation review freshness (current_fingerprint unavailable)")
		}
	}
	if record.BranchPrepare == nil {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires branch preparation")
	}
	projectKey := remote.ProjectKey(record.IssueURL, provider, "issue")
	head, base := strings.TrimSpace(req.Head), strings.TrimSpace(req.Base)
	workspaceBranch := strings.TrimSpace(record.Branch)
	if record.Execution != nil {
		workspaceBranch = record.Execution.Workspace.Branch
	}
	if projectKey == "" || provider != strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)) || head != workspaceBranch || base != strings.TrimSpace(record.BranchPrepare.BaseBranch) {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create request does not match execution workspace and linked issue authority")
	}
	title, body := strings.TrimSpace(req.Title), strings.TrimSpace(req.Body)
	if title == "" || len(title) > 1024 || len(body) > 1<<20 {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create title is required and body must not exceed 1048576 bytes")
	}
	if policy.RedactFreeform(title) != title || policy.RedactFreeform(body) != body {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create title or body contains secret-like content")
	}
	labels, assignees := remote.CleanValues(req.Labels), remote.CleanValues(req.Assignees)
	if len(labels) == 0 || len(assignees) == 0 {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires canonical labels and assignees")
	}
	if invalid := remote.InvalidAssignee(assignees); invalid != "" {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create assignee must not be placeholder %q", invalid)
	}
	headSHA := ""
	if req.Confirm {
		headSHA = issueOpsCurrentHead(record)
	}
	if req.Confirm && !validExecutionHead(headSHA) {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires a resolvable canonical worktree HEAD")
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

func beginRemotePullRequestIntent(stateRoot string, expected issueops.IssueOpsRecord, actor issueops.NativeActor, cwd string, expectedGeneration uint64, providerReq port.IssueProviderCreatePullRequestRequest, provider, kind string, now func() time.Time) (issueops.IssueOpsRecord, externalRemotePRPayload, error) {
	operationID, err := newExecutionOperationID()
	if err != nil {
		return issueops.IssueOpsRecord{}, externalRemotePRPayload{}, err
	}
	return beginRemotePullRequestIntentWithOperationID(stateRoot, expected, actor, cwd, expectedGeneration, providerReq, provider, kind, operationID, now)
}

func beginRemotePullRequestIntentWithOperationID(stateRoot string, expected issueops.IssueOpsRecord, actor issueops.NativeActor, cwd string, expectedGeneration uint64, providerReq port.IssueProviderCreatePullRequestRequest, provider, kind, operationID string, now func() time.Time) (issueops.IssueOpsRecord, externalRemotePRPayload, error) {
	if !validRemotePullRequestOperationID(operationID) {
		return issueops.IssueOpsRecord{}, externalRemotePRPayload{}, fmt.Errorf("remote operation ID must be exactly 32 lowercase hexadecimal characters")
	}
	marker := "<!-- agent-harness:issueops-v1 operation=" + operationID + " -->"
	providerReq.Body = strings.TrimSpace(providerReq.Body) + "\n\n" + marker
	payload := externalRemotePRPayload{
		SchemaVersion: issueops.IssueOpsSchemaVersion, OperationID: operationID, Provider: provider, Kind: kind,
		Request: providerReq, InvocationState: remoteInvocationUnknown,
	}
	var persisted issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
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
		current.Execution.Pending = &issueops.ExternalIntent{OperationID: operationID, Kind: externalIntentRemotePR, Marker: marker, StartedAt: executionNow(now)}
		current.Execution.Failure = nil
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []port.RecordMutation{{
			Bucket: externalIntentBucket, ID: operationID, Data: data, RequireAbsent: true,
		}})
		return err
	})
	return persisted, payload, err
}

func validRemotePullRequestOperationID(operationID string) bool {
	if len(operationID) != 32 {
		return false
	}
	for _, char := range []byte(operationID) {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
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
		record.Execution.Failure = &issueops.ExecutionFailure{OperationID: operationID, Code: "external_operation_ambiguous", Message: message, At: executionNow(now)}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []port.RecordMutation{{Bucket: externalIntentBucket, ID: operationID, Data: data}})
		return err
	})
}

func finishRemotePullRequestIntent(stateRoot, id string, payload externalRemotePRPayload, url string, enforceOriginalGeneration bool, now func() time.Time) (issueops.IssueOpsRecord, error) {
	var persisted issueops.IssueOpsRecord
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
			if payload.Generation == 0 || lease.Generation != payload.Generation || lease.Status != issueops.LeaseStatusActive || holder == nil ||
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
		artifact, err := artifactverify.Projection(current, issueops.IssueOpsRemoteArtifactVerificationRequest{
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
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []port.RecordMutation{{Bucket: externalIntentBucket, ID: payload.OperationID, Delete: true}})
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
	if payload.SchemaVersion != issueops.IssueOpsSchemaVersion || payload.OperationID != operationID || payload.Generation == 0 || payload.Provider == "" || payload.Kind == "" {
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

// remotePullRequestCandidateTitle는 provider가 draft 상태를 제목 접두사로 표현하는
// 것을 되돌린다. GitLab은 draft MR의 제목을 "Draft: <title>"로 저장하고 목록 API도
// 접두사를 포함해 반환하므로, 봉인된 의도의 제목과 그대로 비교하면 자기 자신이
// 만든 draft MR조차 채택할 수 없다.
func remotePullRequestCandidateTitle(candidate port.IssueProviderReconcilePullRequestCandidate) string {
	title := strings.TrimSpace(candidate.Title)
	if !candidate.Draft {
		return title
	}
	for _, prefix := range []string{"Draft:", "WIP:"} {
		if len(title) >= len(prefix) && strings.EqualFold(title[:len(prefix)], prefix) {
			return strings.TrimSpace(title[len(prefix):])
		}
	}
	return title
}

// remotePullRequestCandidateDraftMatches는 draft 의도와 관측된 draft 상태를 비교한다.
// 이미 merged 또는 closed된 아티팩트는 draft일 수 없으므로, draft로 만들어 달라던
// 의도와 "draft가 해제된 뒤 머지된" 관측은 모순이 아니다. 아직 열려 있는 아티팩트는
// 기존대로 정확히 일치해야 한다.
func remotePullRequestCandidateDraftMatches(candidate port.IssueProviderReconcilePullRequestCandidate, expectedDraft bool) bool {
	if candidate.Draft == expectedDraft {
		return true
	}
	if !expectedDraft || candidate.Draft {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(candidate.State)) {
	case "merged", "closed":
		return true
	default:
		return false
	}
}

func validateRemotePullRequestCandidate(record issueops.IssueOpsRecord, payload externalRemotePRPayload, candidate port.IssueProviderReconcilePullRequestCandidate) error {
	expected := remotePullRequestReconcileRequest(payload)
	if strings.TrimSpace(candidate.ProjectKey) != expected.ProjectKey || strings.TrimSpace(candidate.SourceProjectKey) != expected.ProjectKey ||
		strings.TrimSpace(candidate.HeadBranch) != expected.HeadBranch || strings.TrimSpace(candidate.BaseBranch) != expected.BaseBranch ||
		strings.TrimSpace(candidate.HeadSHA) != expected.ExpectedHeadSHA || remotePullRequestCandidateTitle(candidate) != expected.Title ||
		strings.TrimSpace(candidate.BodySHA256) != expected.BodySHA256 ||
		!remotePullRequestCandidateDraftMatches(candidate, expected.Draft) ||
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

func verifyRemotePullRequestResult(record issueops.IssueOpsRecord, payload externalRemotePRPayload, url string, verify RemoteArtifactVerifyFunc) error {
	req := issueops.IssueOpsRemoteArtifactVerificationRequest{
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
