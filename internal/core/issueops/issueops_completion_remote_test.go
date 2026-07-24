package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func completionTestRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "81-completion"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/81"
	record.RemoteArtifact = &IssueOpsRemoteArtifactVerification{
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
