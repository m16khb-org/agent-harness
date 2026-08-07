package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

type fakeCompletionProvider struct {
	updateReq  *port.IssueProviderUpdateIssueBodySectionRequest
	updateRes  port.IssueProviderUpdateIssueBodySectionResult
	closeReq   *port.IssueProviderCloseIssueRequest
	closeRes   port.IssueProviderCloseIssueResult
	updateErr  error
	closeError error
}

func (p *fakeCompletionProvider) Name() string { return "github" }
func (p *fakeCompletionProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (p *fakeCompletionProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}
func (p *fakeCompletionProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (p *fakeCompletionProvider) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (p *fakeCompletionProvider) CloseIssue(req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	p.closeReq = &req
	return p.closeRes, p.closeError
}
func (p *fakeCompletionProvider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	p.updateReq = &req
	return p.updateRes, p.updateErr
}

func completionTestRecord(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "81-completion"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/81"
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/85",
	}
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, e := writeIssueOps(stateRoot, record)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record
}

func TestReflectIssueCompletionGates(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	prov := &fakeCompletionProvider{}

	if _, _, err := ReflectIssueCompletion(stateRoot, record.ID, false, true, prov); err == nil {
		t.Fatal("missing merge evidence must be rejected")
	}
	if prov.updateReq != nil {
		t.Fatal("provider must not be called without merge evidence")
	}

	prov.updateRes = port.IssueProviderUpdateIssueBodySectionResult{OK: true, Preview: "[dry-run]"}
	got, result, err := ReflectIssueCompletion(stateRoot, record.ID, true, false, prov)
	if err != nil || result.Preview == "" {
		t.Fatalf("preview must pass through: %v %+v", err, result)
	}
	if got.RemoteCompletion != nil {
		t.Fatal("preview must not stamp the local completion cache")
	}
	if prov.updateReq.Section != port.IssueBodySectionCompletion || prov.updateReq.Completion == nil {
		t.Fatalf("completion payload must be routed: %+v", prov.updateReq)
	}

	prov.updateRes = port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true, URL: record.IssueURL}
	got, _, err = ReflectIssueCompletion(stateRoot, record.ID, true, true, prov)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCompletion == nil || got.RemoteCompletion.ReflectedAt == "" {
		t.Fatalf("confirmed update must stamp ReflectedAt: %+v", got.RemoteCompletion)
	}
}

// cleanup audit은 audit 라인만 더하는 것이 아니라 completion payload 전체를 원격에
// 쓴다. 그런데 로컬 캐시를 갱신하지 않아, 레코드를 유지하는 cleanup remote-branch
// 직후 issueops list가 원격에 반영된 사이클을 거짓으로 미반영이라 보고했다.
func TestReflectCleanupAuditStampsTheCompletionCache(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	prov := &fakeCompletionProvider{updateRes: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true, URL: record.IssueURL}}

	if err := ReflectCleanupAudit(stateRoot, record, gatherCompletionSection(record), "cleanup 완료: 원격 브랜치 삭제", prov); err != nil {
		t.Fatal(err)
	}
	if prov.updateReq == nil || prov.updateReq.Completion == nil || prov.updateReq.Completion.CleanupAudit == "" {
		t.Fatalf("audit must be routed inside the completion payload: %+v", prov.updateReq)
	}
	got, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCompletion == nil || got.RemoteCompletion.ReflectedAt == "" {
		t.Fatalf("confirmed audit reflection must stamp ReflectedAt: %+v", got.RemoteCompletion)
	}
}

// audit 반영은 best-effort다. 실패가 캐시를 오염시키면 이후 진단이 원격보다
// 낙관적으로 보고한다.
func TestReflectCleanupAuditDoesNotStampOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  port.IssueProviderUpdateIssueBodySectionResult
		err  error
	}{
		{name: "provider error", res: port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err: fmt.Errorf("gh: HTTP 503")},
		{name: "unconfirmed update", res: port.IssueProviderUpdateIssueBodySectionResult{OK: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := completionTestRecord(t)
			prov := &fakeCompletionProvider{updateRes: tc.res, updateErr: tc.err}
			if err := ReflectCleanupAudit(stateRoot, record, gatherCompletionSection(record), "cleanup 완료", prov); err == nil {
				t.Fatal("failed audit reflection must return an error")
			}
			got, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got.RemoteCompletion != nil && got.RemoteCompletion.ReflectedAt != "" {
				t.Fatalf("failed reflection must not stamp the cache: %+v", got.RemoteCompletion)
			}
		})
	}
}

func TestReflectIssueCompletionGathersArtifactsFromDisk(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	artifactDir := filepath.Join(record.Repo, completionArtifactDir)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "plan.md"), []byte("plan 본문"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "turing-loop.md"), []byte("turing 본문"), 0o600); err != nil {
		t.Fatal(err)
	}
	prov := &fakeCompletionProvider{updateRes: port.IssueProviderUpdateIssueBodySectionResult{OK: true}}
	if _, _, err := ReflectIssueCompletion(stateRoot, record.ID, true, false, prov); err != nil {
		t.Fatal(err)
	}
	c := prov.updateReq.Completion
	if c.PlanBody != "plan 본문" || !strings.Contains(c.TuringSummary, "turing 본문") {
		t.Fatalf("artifact bodies must be gathered: %+v", c)
	}
	if len(c.ArtifactManifest) != 2 {
		t.Fatalf("manifest must digest existing artifacts only: %+v", c.ArtifactManifest)
	}
	if c.RemoteArtifactURL != "https://github.com/acme/repo/pull/85" {
		t.Fatalf("remote artifact url must come from the record: %+v", c)
	}
}

func TestCloseIssueOpsRemoteIssueGatesAndStamps(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	prov := &fakeCompletionProvider{}

	if _, _, err := CloseIssueOpsRemoteIssue(stateRoot, record.ID, false, true, prov); err == nil {
		t.Fatal("missing merge evidence must be rejected")
	}

	prov.closeRes = port.IssueProviderCloseIssueResult{OK: true, Preview: "[dry-run]"}
	got, result, err := CloseIssueOpsRemoteIssue(stateRoot, record.ID, true, false, prov)
	if err != nil || result.Preview == "" {
		t.Fatalf("preview must pass through: %v %+v", err, result)
	}
	if got.RemoteCompletion != nil && got.RemoteCompletion.IssueClosedAt != "" {
		t.Fatal("preview must not stamp the close cache")
	}

	prov.closeRes = port.IssueProviderCloseIssueResult{OK: true, Closed: true, IssueURL: record.IssueURL}
	got, _, err = CloseIssueOpsRemoteIssue(stateRoot, record.ID, true, true, prov)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCompletion == nil || got.RemoteCompletion.IssueClosedAt == "" {
		t.Fatalf("verified close must stamp IssueClosedAt: %+v", got.RemoteCompletion)
	}
	if prov.closeReq.IssueURL != record.IssueURL {
		t.Fatalf("close must target the linked issue: %+v", prov.closeReq)
	}
}
