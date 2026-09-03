package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	bodysynccontract "agent-harness/internal/contract/issueopsbodysync"
	bodysync "agent-harness/internal/domain/issueopsbodysync"
	"agent-harness/internal/port"
)

const syncCompletionBlock = "<!-- issueops:completion:start -->\n## 완료 기록\n<!-- issueops:completion:end -->"

// fakeBodySyncProvider implements IssueProvider plus the three body-sync
// capabilities, storing one body so a write is observable by the next read.
type fakeBodySyncProvider struct {
	body      string
	state     string
	reads     int
	writes    int
	lastWrite *port.IssueProviderReplaceArtifactBodyRequest
}

func (f *fakeBodySyncProvider) Name() string { return "github" }
func (f *fakeBodySyncProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (f *fakeBodySyncProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}
func (f *fakeBodySyncProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (f *fakeBodySyncProvider) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (f *fakeBodySyncProvider) CloseIssue(port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	return port.IssueProviderCloseIssueResult{}, nil
}
func (f *fakeBodySyncProvider) UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}

func (f *fakeBodySyncProvider) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	f.reads++
	return port.IssueProviderArtifactBody{
		Provider: "github", Kind: req.Kind, URL: req.URL, Body: f.body, State: f.state,
	}, nil
}

func (f *fakeBodySyncProvider) ReplaceArtifactBody(_ context.Context, req port.IssueProviderReplaceArtifactBodyRequest) (port.IssueProviderReplaceArtifactBodyResult, error) {
	copied := req
	f.lastWrite = &copied
	if !req.Confirm {
		return port.IssueProviderReplaceArtifactBodyResult{OK: true, Preview: "[dry-run] would replace body"}, nil
	}
	f.writes++
	f.body = req.Body
	return port.IssueProviderReplaceArtifactBodyResult{
		OK: true, Updated: true, URL: req.URL, VerifiedBodySHA256: bodysync.SHA256Body(req.Body),
	}, nil
}

// fakeHierarchyProvider adds the child-hierarchy capability. Keeping it on a
// wrapper lets a test model a provider that cannot verify hierarchy at all,
// which Go cannot express by un-embedding a method.
type fakeHierarchyProvider struct {
	*fakeBodySyncProvider
	verified bool
}

func (f fakeHierarchyProvider) VerifyChildHierarchy(context.Context, port.IssueProviderChildHierarchyRequest) (port.IssueProviderChildHierarchyResult, error) {
	return port.IssueProviderChildHierarchyResult{Provider: "github", OK: true, Verified: f.verified}, nil
}

// bodySyncCreateIntent is a complete, record-valid create intent whose digest
// stands in for "the body the harness last wrote".
func bodySyncCreateIntent(url, sha string) *issueops.IssueOpsIssueCreateIntent {
	const operationID = "0123456789abcdef0123456789abcdef"
	return &issueops.IssueOpsIssueCreateIntent{
		OperationID:      operationID,
		Marker:           "<!-- agent-harness:issue-create:" + operationID + " -->",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "[enhancement] 본문 동기화",
		BodySHA256:       sha,
		Status:           issueops.IssueCreateIntentCompleted,
		Attempt:          1,
		CanonicalURL:     url,
		StartedAt:        "2026-07-01T00:00:00Z",
		UpdatedAt:        "2026-07-01T00:00:00Z",
	}
}

func bodySyncFixture(t *testing.T) (stateRoot string, record issueops.IssueOpsRecord, actor IssueOpsActor) {
	t.Helper()
	stateRoot = filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	worktree := filepath.Join(repo+".worktrees", "412-body-sync")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "412-body-sync"})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.IssueURL = "https://github.com/acme/repo/issues/412"
	record.Execution = issueOpsExecutionForTest(repo, worktree, record.Branch)
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, writeErr := writeIssueOps(stateRoot, record)
		return writeErr
	}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, issueOpsActorForTest(worktree)
}

func saveBodySyncRecord(t *testing.T, stateRoot string, record issueops.IssueOpsRecord) {
	t.Helper()
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, err := writeIssueOps(stateRoot, record)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncIssueBodyPreviewReportsDriftAndDoesNotWrite(t *testing.T) {
	stateRoot, record, actor := bodySyncFixture(t)
	live := "## 문제\n옛 본문\n\n" + syncCompletionBlock
	record.IssueCreateIntent = bodySyncCreateIntent(record.IssueURL, bodysync.SHA256Body(live))
	saveBodySyncRecord(t, stateRoot, record)
	prov := &fakeBodySyncProvider{body: live, state: "OPEN"}

	_, result, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: "## 문제\n새 본문\n",
	}, prov, actor)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Drift != bodysynccontract.DriftStale {
		t.Fatalf("drift = %q, want stale", result.Drift)
	}
	if result.ExpectedBodySHA256 != bodysync.SHA256Body(live) {
		t.Fatalf("preview must hand back the live digest to pass to confirm")
	}
	if len(result.PreservedSections) != 1 || result.PreservedSections[0] != bodysync.RegionCompletion {
		t.Fatalf("preserved = %v", result.PreservedSections)
	}
	if result.Updated || prov.writes != 0 {
		t.Fatalf("a preview must not write: updated=%v writes=%d", result.Updated, prov.writes)
	}
	if result.AgeDays <= 0 {
		t.Fatalf("age_days must be reported for a dated baseline, got %d", result.AgeDays)
	}
}

func TestSyncIssueBodyConfirmIsFailClosed(t *testing.T) {
	live := "## 문제\n옛 본문\n"
	tests := []struct {
		name        string
		recordedSHA string
		expected    func(live string) string
		accept      bool
		wantErr     string
	}{
		{
			name: "missing expectation", recordedSHA: bodysync.SHA256Body(live),
			expected: func(string) string { return "" }, wantErr: "expected-body-sha256",
		},
		{
			name: "stale expectation", recordedSHA: bodysync.SHA256Body(live),
			expected: func(string) string { return strings.Repeat("a", 64) }, wantErr: "changed",
		},
		{
			name: "unacknowledged outside edit", recordedSHA: strings.Repeat("b", 64),
			expected: bodysync.SHA256Body, wantErr: "accept-remote-edits",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, actor := bodySyncFixture(t)
			record.IssueCreateIntent = bodySyncCreateIntent(record.IssueURL, tt.recordedSHA)
			saveBodySyncRecord(t, stateRoot, record)
			prov := &fakeBodySyncProvider{body: live, state: "OPEN"}

			_, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
				Kind: bodysynccontract.KindIssue, ProposedBody: "## 문제\n새 본문\n",
				ExpectedBodySHA256: tt.expected(live), AcceptRemoteEdits: tt.accept, Confirm: true,
			}, prov, actor)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want mention of %q", err, tt.wantErr)
			}
			if prov.writes != 0 {
				t.Fatalf("a refused confirm must not reach the provider")
			}
		})
	}
}

func TestSyncIssueBodyConfirmWritesPreservesAndRecordsBaseline(t *testing.T) {
	stateRoot, record, actor := bodySyncFixture(t)
	live := "## 문제\n옛 본문\n\n" + syncCompletionBlock
	record.IssueCreateIntent = bodySyncCreateIntent(record.IssueURL, bodysync.SHA256Body(live))
	saveBodySyncRecord(t, stateRoot, record)
	prov := &fakeBodySyncProvider{body: live, state: "OPEN"}

	updated, result, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: "## 문제\n새 본문\n",
		ExpectedBodySHA256: bodysync.SHA256Body(live), Confirm: true,
	}, prov, actor)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !result.Updated || prov.writes != 1 {
		t.Fatalf("confirm must write exactly once: %+v writes=%d", result, prov.writes)
	}
	if !strings.Contains(prov.body, "새 본문") || !strings.Contains(prov.body, port.IssueBodyCompletionStartMarker) {
		t.Fatalf("the written body must carry the proposal and keep the completion block:\n%s", prov.body)
	}
	if len(updated.BodySyncs) != 1 ||
		updated.BodySyncs[0].ToSHA256 != bodysync.SHA256Body(prov.body) ||
		updated.BodySyncs[0].URL != record.IssueURL {
		t.Fatalf("the record must store the new baseline: %+v", updated.BodySyncs)
	}

	// 같은 본문을 다시 동기화하면 provider를 건드리지 않는다.
	_, second, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: "## 문제\n새 본문\n",
		ExpectedBodySHA256: bodysync.SHA256Body(prov.body), Confirm: true,
	}, prov, actor)
	if err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if second.Drift != bodysynccontract.DriftInSync || second.Updated || prov.writes != 1 {
		t.Fatalf("an unchanged artifact must be a no-op: %+v writes=%d", second, prov.writes)
	}
}

func TestSyncChildBodyRequiresVerifiedHierarchy(t *testing.T) {
	stateRoot, record, actor := bodySyncFixture(t)
	saveBodySyncRecord(t, stateRoot, record)
	child := "https://github.com/acme/repo/issues/500"

	inner := &fakeBodySyncProvider{body: "## scope\n옛 본문\n", state: "OPEN"}
	prov := fakeHierarchyProvider{fakeBodySyncProvider: inner}
	if _, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, URL: child, ProposedBody: "## scope\n새 본문\n",
	}, prov, actor); err == nil || !strings.Contains(err.Error(), "not a provider-native child") {
		t.Fatalf("an unverified child must be refused, got %v", err)
	}
	if inner.reads != 0 {
		t.Fatalf("hierarchy must be verified before the body is read")
	}

	prov.verified = true
	_, result, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, URL: child, ProposedBody: "## scope\n새 본문\n",
	}, prov, actor)
	if err != nil {
		t.Fatalf("verified child preview: %v", err)
	}
	if result.Kind != bodysynccontract.KindChild || result.URL != child {
		t.Fatalf("result = %+v", result)
	}
}

func TestSyncChildBodyRefusedWhenProviderCannotVerifyHierarchy(t *testing.T) {
	stateRoot, record, actor := bodySyncFixture(t)
	saveBodySyncRecord(t, stateRoot, record)
	prov := &fakeBodySyncProvider{body: "본문", state: "OPEN"}

	_, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, URL: "https://github.com/acme/repo/issues/500", ProposedBody: "새 본문",
	}, prov, actor)
	if err == nil || !strings.Contains(err.Error(), "cannot verify child hierarchy") {
		t.Fatalf("a provider without the capability must refuse, got %v", err)
	}
}

func TestSyncPullRequestBodyFencesGenerationAndLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		generation uint64
		wantErr    string
	}{
		{"stale generation", "OPEN", 7, "stale lease generation"},
		{"merged artifact", "MERGED", 1, "refusing to rewrite"},
		{"closed artifact", "CLOSED", 1, "refusing to rewrite"},
		{"unobserved state", "", 1, "without an observed artifact state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, actor := bodySyncFixture(t)
			record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
				Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/9",
				VerifiedAt: "2026-07-01T00:00:00Z",
			}
			saveBodySyncRecord(t, stateRoot, record)
			prov := &fakeBodySyncProvider{body: "## 의도\n옛 본문\n", state: tt.state}

			_, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
				Kind: bodysynccontract.KindPR, ProposedBody: "## 의도\n새 본문\n", ExpectedGeneration: tt.generation,
			}, prov, actor)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want mention of %q", err, tt.wantErr)
			}
			if prov.writes != 0 {
				t.Fatalf("a refused sync must not write")
			}
		})
	}
}

func TestSyncPullRequestBodyRejectsForeignURL(t *testing.T) {
	stateRoot, record, actor := bodySyncFixture(t)
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/9", VerifiedAt: "2026-07-01T00:00:00Z",
	}
	saveBodySyncRecord(t, stateRoot, record)
	prov := &fakeBodySyncProvider{body: "본문", state: "OPEN"}

	_, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindPR, URL: "https://github.com/acme/repo/pull/999",
		ProposedBody: "새 본문", ExpectedGeneration: 1,
	}, prov, actor)
	if err == nil || !strings.Contains(err.Error(), "not this cycle's verified artifact") {
		t.Fatalf("a foreign artifact URL must be refused, got %v", err)
	}
}

func TestSyncBodyRequiresCurrentLeaseHolder(t *testing.T) {
	stateRoot, record, _ := bodySyncFixture(t)
	saveBodySyncRecord(t, stateRoot, record)
	prov := &fakeBodySyncProvider{body: "본문", state: "OPEN"}
	foreign := issueOpsActorForTest(record.WorktreePath)
	foreign.SessionID = "other-session"

	_, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: "새 본문",
	}, prov, foreign)
	if err == nil || !strings.Contains(err.Error(), "current write lease holder") {
		t.Fatalf("a non-holder must be refused, got %v", err)
	}
	if prov.reads != 0 {
		t.Fatalf("identity must be checked before the provider is called")
	}
}

func TestSyncBodyRejectsManagedMarkersBeforeAnyProviderCall(t *testing.T) {
	stateRoot, record, actor := bodySyncFixture(t)
	saveBodySyncRecord(t, stateRoot, record)
	prov := &fakeBodySyncProvider{body: "## 문제\n옛 본문\n", state: "OPEN"}

	_, _, err := SyncRemoteArtifactBody(context.Background(), stateRoot, record.ID, bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: "## 문제\n본문\n\n" + syncCompletionBlock,
	}, prov, actor)
	if err == nil || !strings.Contains(err.Error(), "managed section marker") {
		t.Fatalf("a hand-authored managed block must be refused, got %v", err)
	}
	if prov.reads != 0 {
		t.Fatalf("an unusable proposal must not cost a provider round trip")
	}
}
