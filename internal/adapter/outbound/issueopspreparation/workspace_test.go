package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/port"
)

func TestDirectWorkspaceMapsPrepareAndAccess(t *testing.T) {
	provider := &workspaceProviderFake{}
	adapter := NewDirectWorkspace(provider)
	request := preparationcontract.WorkspaceRequest{LifecycleID: "io-prepare", SourceRoot: "/repo", Root: "/repo.worktrees/199-prepare", Branch: "199-prepare", BaseBranch: "117-parent", BaseHead: "base", ParentWorktree: "/repo.worktrees/117-parent", Confirm: true, CWD: "/repo"}
	access, err := adapter.ProbeAccess(context.Background(), request, "codex")
	if err != nil || !access.Allowed || access.Code != "allowed" {
		t.Fatalf("access=%+v err=%v", access, err)
	}
	receipt, err := adapter.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	want := port.ExecutionWorkspaceRequest{LifecycleID: request.LifecycleID, SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch, BaseBranch: request.BaseBranch, BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Confirm: true}
	if !reflect.DeepEqual(provider.request, want) || receipt.Driver != "git" || receipt.Root != request.Root {
		t.Fatalf("request=%+v receipt=%+v", provider.request, receipt)
	}
}

func TestDirectWorkspaceRequiresAccessProber(t *testing.T) {
	request := preparationcontract.WorkspaceRequest{SourceRoot: "/repo", Root: "/repo.worktrees/199-prepare", CWD: "/repo"}
	adapter := NewDirectWorkspace(workspacePrepareOnlyFake{})
	_, err := adapter.ProbeAccess(context.Background(), request, "codex")
	if err == nil || !strings.Contains(err.Error(), "cannot verify canonical worktree base access") {
		t.Fatalf("err=%v", err)
	}
	adapter = NewDirectWorkspace(nil)
	if _, err := adapter.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "provisioner is unavailable") {
		t.Fatalf("err=%v", err)
	}
}

func TestDirectWorkspaceRejectsNonCanonicalCWD(t *testing.T) {
	provider := &workspaceProviderFake{}
	adapter := NewDirectWorkspace(provider)
	request := preparationcontract.WorkspaceRequest{
		LifecycleID: "io-prepare", SourceRoot: "/repo", Root: "/repo.worktrees/199-prepare",
		Branch: "199-prepare", BaseHead: "base", CWD: "/other",
	}
	if _, err := adapter.ProbeAccess(context.Background(), request, "codex"); err == nil || !strings.Contains(err.Error(), "sealed parent_worktree") {
		t.Fatalf("access err=%v", err)
	}
	if _, err := adapter.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "sealed parent_worktree") {
		t.Fatalf("prepare err=%v", err)
	}
	if provider.request.LifecycleID != "" {
		t.Fatalf("provider called with %+v", provider.request)
	}
}

func TestDirectWorkspaceAllowsSealedDelegatedParentCWD(t *testing.T) {
	provider := &workspaceProviderFake{}
	adapter := NewDirectWorkspace(provider)
	request := preparationcontract.WorkspaceRequest{
		LifecycleID: "io-child", SourceRoot: "/repo", Root: "/repo.worktrees/333-child",
		Branch: "333-child", BaseBranch: "228-parent", BaseHead: "base",
		ParentWorktree: "/repo.worktrees/228-parent", CWD: "/repo.worktrees/228-parent",
	}
	if _, err := adapter.ProbeAccess(context.Background(), request, "codex"); err != nil {
		t.Fatalf("sealed delegated parent cwd must be admitted for access probing: %v", err)
	}
	if _, err := adapter.Prepare(context.Background(), request); err != nil {
		t.Fatalf("sealed delegated parent cwd must be admitted for preparation: %v", err)
	}
}

type workspaceProviderFake struct {
	request port.ExecutionWorkspaceRequest
}

func (fake *workspaceProviderFake) ProbeAccess(_ context.Context, request port.ExecutionWorkspaceRequest, _ string) (port.ExecutionWorkspaceAccessResult, error) {
	fake.request = request
	return port.ExecutionWorkspaceAccessResult{Allowed: true, Code: "allowed"}, nil
}

func (fake *workspaceProviderFake) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	fake.request = request
	return port.ExecutionWorkspaceReceipt{SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch, BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: "git", Exists: true}, nil
}

type workspacePrepareOnlyFake struct{}

func (workspacePrepareOnlyFake) Prepare(context.Context, port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	return port.ExecutionWorkspaceReceipt{}, errors.New("unused")
}
