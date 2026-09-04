package issueopspreparation

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

type DirectWorkspaceAdapter struct {
	provisioner port.ExecutionWorkspaceProvisioner
}

func NewDirectWorkspace(provisioner port.ExecutionWorkspaceProvisioner) *DirectWorkspaceAdapter {
	return &DirectWorkspaceAdapter{provisioner: provisioner}
}

func (adapter *DirectWorkspaceAdapter) ProbeAccess(ctx context.Context, request preparationcontract.WorkspaceRequest, host string) (preparationcontract.AccessResult, error) {
	if err := validateDirectCWD(request); err != nil {
		return preparationcontract.AccessResult{}, err
	}
	if adapter.provisioner == nil {
		return preparationcontract.AccessResult{}, fmt.Errorf("direct Git worktree provisioner is unavailable")
	}
	prober, ok := adapter.provisioner.(port.ExecutionWorkspaceAccessProber)
	if !ok {
		return preparationcontract.AccessResult{}, fmt.Errorf("direct provisioner cannot verify canonical worktree base access")
	}
	result, err := prober.ProbeAccess(ctx, toWorkspaceRequest(request), host)
	return preparationcontract.AccessResult{Allowed: result.Allowed, Code: result.Code, RelaunchCommand: result.RelaunchCommand}, err
}

func (adapter *DirectWorkspaceAdapter) Prepare(ctx context.Context, request preparationcontract.WorkspaceRequest) (preparationcontract.WorkspaceReceipt, error) {
	if err := validateDirectCWD(request); err != nil {
		return preparationcontract.WorkspaceReceipt{}, err
	}
	if adapter.provisioner == nil {
		return preparationcontract.WorkspaceReceipt{}, fmt.Errorf("direct Git worktree provisioner is unavailable")
	}
	receipt, err := adapter.provisioner.Prepare(ctx, toWorkspaceRequest(request))
	return preparationcontract.WorkspaceReceipt{
		SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch,
		BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree,
		Driver: receipt.Driver, Exists: receipt.Exists,
	}, err
}

func validateDirectCWD(request preparationcontract.WorkspaceRequest) error {
	if !sameResolvedPath(request.CWD, request.SourceRoot) && !sameResolvedPath(request.CWD, request.Root) &&
		(strings.TrimSpace(request.ParentWorktree) == "" || !sameResolvedPath(request.CWD, request.ParentWorktree)) {
		return fmt.Errorf("direct prepare cwd must be source_root, the canonical worktree, or the sealed parent_worktree")
	}
	return nil
}

func sameResolvedPath(left, right string) bool {
	a, err := filepath.Abs(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(a); resolveErr == nil {
		a = resolved
	}
	b, err := filepath.Abs(strings.TrimSpace(right))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(b); resolveErr == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func toWorkspaceRequest(request preparationcontract.WorkspaceRequest) port.ExecutionWorkspaceRequest {
	return port.ExecutionWorkspaceRequest{
		LifecycleID: request.LifecycleID, SourceRoot: request.SourceRoot, Root: request.Root,
		Branch: request.Branch, BaseBranch: request.BaseBranch, BaseHead: request.BaseHead,
		ParentWorktree: request.ParentWorktree, Confirm: request.Confirm,
	}
}
