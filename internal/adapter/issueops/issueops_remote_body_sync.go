package issueops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/port"
)

// SyncRemoteArtifactBody refreshes the body of an artifact this cycle already
// published, for the case the cycle moved on and the remote text did not.
//
// The write is fail-closed in three independent ways: the caller has to name
// the exact live body its proposal was built on, an artifact edited outside the
// harness needs a separate acknowledgement, and every managed block the harness
// maintains is spliced back in rather than replaced.
func SyncRemoteArtifactBody(
	ctx context.Context,
	stateRoot, id string,
	cmd bodysynccontract.Command,
	prov port.IssueProvider,
	actor IssueOpsActor,
) (issueops.IssueOpsRecord, bodysynccontract.Result, error) {
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("no issue provider configured")
	}
	reader, ok := prov.(port.IssueProviderArtifactBodyReader)
	if !ok {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("provider %q cannot read artifact bodies", prov.Name())
	}
	replacer, ok := prov.(port.IssueProviderArtifactBodyReplacer)
	if !ok {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("provider %q cannot replace artifact bodies", prov.Name())
	}
	// 쓸 수 없는 본문은 원격을 읽기 전에 거부한다. provider 왕복은 공짜가 아니다.
	if err := bodysync.ValidateProposal(cmd.ProposedBody); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, bodysynccontract.Result{}, err
	}
	if err := validateExecutionMutation(record, &actor); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	kind, url, err := resolveBodySyncTarget(record, cmd)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if isPublicationKind(kind) {
		if err := validateBodySyncGeneration(record, cmd.ExpectedGeneration); err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
	}
	if kind == bodysynccontract.KindChild {
		if err := verifyBodySyncChildHierarchy(ctx, prov, record, url); err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
	}
	live, err := reader.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{
		Repo: record.Repo, Kind: kind, URL: url,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if err := rejectClosedPublication(kind, live.State); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	baselineSHA, baselineAt := bodySyncBaseline(record, kind, url)
	plan, err := bodysync.BuildPlan(baselineSHA, live.Body, cmd.ProposedBody)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	result := bodysynccontract.Result{
		OK: true, ID: record.ID, Provider: prov.Name(), Kind: kind, URL: url,
		Confirm:            cmd.Confirm,
		Drift:              plan.Drift,
		RecordedBodySHA256: baselineSHA,
		RemoteBodySHA256:   plan.RemoteBodySHA256,
		MergedBodySHA256:   plan.MergedBodySHA256,
		ExpectedBodySHA256: plan.RemoteBodySHA256,
		PreservedSections:  plan.PreservedSections,
		RecordedAt:         baselineAt,
		AgeDays:            bodySyncAgeDays(baselineAt),
		AcceptRemoteEdits:  cmd.AcceptRemoteEdits,
	}
	if !cmd.Confirm {
		preview, err := replacer.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
			Repo: record.Repo, Kind: kind, URL: url, Body: plan.MergedBody,
		})
		if err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
		result.Preview = preview.Preview
		return record, result, nil
	}
	if err := bodysync.ValidateWrite(plan, cmd.ExpectedBodySHA256, cmd.AcceptRemoteEdits); err != nil {
		if errors.Is(err, bodysync.ErrAlreadyInSync) {
			// 이미 같은 본문이면 provider를 건드리지 않는다. 성공이지 실패가 아니다.
			return record, result, nil
		}
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	written, err := replacer.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
		Repo: record.Repo, Kind: kind, URL: url, Body: plan.MergedBody, Confirm: true,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if !written.Updated {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("provider did not report the body replacement as applied")
	}
	if written.VerifiedBodySHA256 != plan.MergedBodySHA256 {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf(
			"remote body readback does not match what was written (readback %s, intended %s)",
			written.VerifiedBodySHA256, plan.MergedBodySHA256)
	}
	result.Updated = true
	result.RemoteBodySHA256 = written.VerifiedBodySHA256
	// 두 필드 모두 "지금 원격 본문"을 가리켜야 다음 confirm이 그대로 재사용한다.
	result.ExpectedBodySHA256 = written.VerifiedBodySHA256
	result.Drift = bodysynccontract.DriftInSync

	entry := issueops.IssueOpsRemoteBodySync{
		Kind: kind, URL: url,
		FromSHA256: plan.RemoteBodySHA256,
		ToSHA256:   written.VerifiedBodySHA256,
		SyncedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if isPublicationKind(kind) {
		entry.Generation = cmd.ExpectedGeneration
	}
	stamped, err := recordBodySync(ctx, stateRoot, id, entry, &actor)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	return stamped, result, nil
}

// resolveBodySyncTarget decides which artifact a sync addresses. The issue side
// accepts a child URL so a provider-native child can be refreshed too; the
// publication side accepts only the artifact this cycle actually verified, so a
// stray URL cannot rewrite an unrelated PR.
func resolveBodySyncTarget(record issueops.IssueOpsRecord, cmd bodysynccontract.Command) (kind, url string, err error) {
	requested := strings.TrimSpace(cmd.URL)
	switch cmd.Kind {
	case bodysynccontract.KindIssue:
		parent := strings.TrimSpace(record.IssueURL)
		if parent == "" {
			return "", "", fmt.Errorf("cannot sync an issue body before the cycle has a linked issue")
		}
		if requested == "" || sameIssueOpsArtifactURL(requested, parent) {
			return bodysynccontract.KindIssue, parent, nil
		}
		return bodysynccontract.KindChild, requested, nil
	case bodysynccontract.KindPR:
		artifact := record.RemoteArtifact
		if artifact == nil || strings.TrimSpace(artifact.URL) == "" {
			return "", "", fmt.Errorf("cannot sync a PR/MR body before the cycle has a verified remote artifact")
		}
		if requested != "" && !sameIssueOpsArtifactURL(requested, artifact.URL) {
			return "", "", fmt.Errorf("--url %s is not this cycle's verified artifact (%s)", requested, artifact.URL)
		}
		resolved := strings.TrimSpace(artifact.Kind)
		if resolved != bodysynccontract.KindPR && resolved != bodysynccontract.KindMR {
			return "", "", fmt.Errorf("verified remote artifact kind %q is not a PR or MR", artifact.Kind)
		}
		return resolved, strings.TrimSpace(artifact.URL), nil
	}
	return "", "", fmt.Errorf("unsupported body sync kind %q (want %s|%s)", cmd.Kind, bodysynccontract.KindIssue, bodysynccontract.KindPR)
}

func isPublicationKind(kind string) bool {
	return kind == bodysynccontract.KindPR || kind == bodysynccontract.KindMR
}

// sameIssueOpsArtifactURL compares artifact URLs ignoring the trailing slash and
// the GitLab work-item alias, which serves the same issue under a second path.
func sameIssueOpsArtifactURL(left, right string) bool {
	normalize := func(raw string) string {
		trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
		return strings.Replace(trimmed, "/-/work_items/", "/-/issues/", 1)
	}
	return normalize(left) == normalize(right)
}

func validateBodySyncGeneration(record issueops.IssueOpsRecord, expected uint64) error {
	if record.Execution == nil {
		return fmt.Errorf("cannot sync a PR/MR body without an execution lease")
	}
	current := record.Execution.Lease.Generation
	if expected == 0 || current != expected {
		return fmt.Errorf("stale lease generation: current=%d expected=%d", current, expected)
	}
	return nil
}

func rejectClosedPublication(kind, state string) error {
	if !isPublicationKind(kind) {
		return nil
	}
	switch normalized := strings.ToLower(strings.TrimSpace(state)); normalized {
	case "open", "opened", "locked":
		return nil
	case "":
		// 상태를 읽지 못했다면 머지 여부를 증명할 수 없다. 열려 있다고 가정하지 않는다.
		return fmt.Errorf("refusing to rewrite a %s body without an observed artifact state", kind)
	default:
		return fmt.Errorf("refusing to rewrite the body of a %s artifact (state=%s)", kind, normalized)
	}
}

func verifyBodySyncChildHierarchy(ctx context.Context, prov port.IssueProvider, record issueops.IssueOpsRecord, childURL string) error {
	verifier, ok := prov.(port.IssueProviderChildHierarchyVerifier)
	if !ok {
		return fmt.Errorf("provider %q cannot verify child hierarchy, so a child body cannot be synced", prov.Name())
	}
	result, err := verifier.VerifyChildHierarchy(ctx, port.IssueProviderChildHierarchyRequest{
		Repo: record.Repo, ParentIssueURL: record.IssueURL, ChildURL: childURL,
	})
	if err != nil {
		return err
	}
	if !result.Verified {
		return fmt.Errorf("%s is not a provider-native child of %s; sync it from the cycle that owns it", childURL, record.IssueURL)
	}
	return nil
}

// bodySyncBaseline is the body the harness last put on the artifact. A prior
// sync is the most recent truth; before any sync, the create intent's digest is
// the only thing the harness ever wrote.
func bodySyncBaseline(record issueops.IssueOpsRecord, kind, url string) (sha, at string) {
	for _, entry := range record.BodySyncs {
		if sameIssueOpsArtifactURL(entry.URL, url) {
			return entry.ToSHA256, entry.SyncedAt
		}
	}
	if kind == bodysynccontract.KindIssue && record.IssueCreateIntent != nil &&
		sameIssueOpsArtifactURL(record.IssueCreateIntent.CanonicalURL, url) {
		return record.IssueCreateIntent.BodySHA256, record.IssueCreateIntent.UpdatedAt
	}
	if isPublicationKind(kind) && record.RemoteArtifact != nil {
		// 생성 시 본문 digest는 기록되지 않으므로 기준선 없이 검증 시각만 보고한다.
		return "", record.RemoteArtifact.VerifiedAt
	}
	return "", ""
}

func bodySyncAgeDays(at string) int {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(at))
	if err != nil {
		return 0
	}
	days := int(time.Since(parsed).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// recordBodySync stores the new baseline under the record lock, keeping one
// entry per artifact so the list cannot grow without bound.
func recordBodySync(ctx context.Context, stateRoot, id string, entry issueops.IssueOpsRemoteBodySync, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var stamped issueops.IssueOpsRecord
	err := withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		rec, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if err := validateExecutionMutation(rec, actor); err != nil {
			return err
		}
		kept := make([]issueops.IssueOpsRemoteBodySync, 0, len(rec.BodySyncs)+1)
		for _, existing := range rec.BodySyncs {
			if !sameIssueOpsArtifactURL(existing.URL, entry.URL) {
				kept = append(kept, existing)
			}
		}
		kept = append(kept, entry)
		if len(kept) > issueops.MaxIssueOpsBodySyncs {
			kept = kept[len(kept)-issueops.MaxIssueOpsBodySyncs:]
		}
		rec.BodySyncs = kept
		rec.UpdatedAt = entry.SyncedAt
		var writeErr error
		stamped, writeErr = writeIssueOps(stateRoot, rec)
		return writeErr
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return stamped, nil
}
