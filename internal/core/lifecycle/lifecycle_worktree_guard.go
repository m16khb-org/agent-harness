package lifecycle

import (
	"strings"

	"agent-harness/internal/core/lifecycle/worktreeguard"
)

func worktreeGuardBlockReason(req HookToolUseLifecycleRequest) string {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	expected := cleanAbsPath(req.ExpectedWorktree)
	if expected == "" {
		if creation := worktreeguard.LocalIssueOpsBranchCreation(req.Command); creation.Branch != "" {
			if worktreeguard.ShellTokenLooksDynamic(creation.Branch) {
				return ""
			}
			if err := validateIssueOpsIssueBranch(creation.Branch); err != nil {
				return err.Error()
			}
			if strings.TrimSpace(creation.SourceRef) == "" {
				return worktreeguard.IssueOpsBranchCreationSourceReason(creation.Branch)
			}
			if rec, ok := ActiveIssueOpsCycleForBranch(req.Repo, creation.Branch); ok && rec.WorktreePath != "" {
				return "IssueOps branch " + creation.Branch + " must not be checked out in the source checkout; create or use the linked isolated worktree " + cleanAbsPath(rec.WorktreePath)
			}
			if _, ok := ActiveIssueOpsCycleForBranch(req.Repo, creation.Branch); ok {
				return "IssueOps branch " + creation.Branch + " must not be checked out in the source checkout; create the provider-linked branch, add the sibling worktree, then run issueops link-worktree before implementation"
			}
		}
		rec, ok := ActiveIssueOpsCycleForBranch(req.Repo, gitBranchFromHead(req.Repo))
		targets := worktreeGuardEditTargets(req)
		if len(targets) == 0 {
			return ""
		}
		if !ok {
			// No active IssueOps cycle on the current branch: allow mutating edits
			// from the source checkout. Other cycles on their own branches enforce
			// their own worktree isolation. This prevents a stuck cycle on another
			// branch from deadlocking all repo-wide edits (e.g. a cycle stuck in
			// PR phase because remote_artifact verification is unavailable).
			return ""
		}
		if !IssueOpsPhaseExpectsWorktree(rec.Phase) {
			if linkedRecs := ActiveIssueOpsLinkedWorktreeCyclesForRepo(req.Repo); len(linkedRecs) > 0 {
				linked := cleanAbsPath(linkedRecs[0].WorktreePath)
				for _, target := range targets {
					if !sourceCheckoutTargetNeedsLinkedWorktree(target, req.Repo) {
						continue
					}
					if targetInsideAnyLinkedIssueOpsWorktree(target, linkedRecs) {
						continue
					}
					return "mutating tool target is outside the linked IssueOps worktree for " + linkedRecs[0].ID + "; run issue-based work from " + linked + " or mark the stale cycle done"
				}
			}
			return ""
		}
		linked := cleanAbsPath(rec.WorktreePath)
		if linked == "" {
			if issueOpsWorktreePreparationCommand(req.Command) {
				return ""
			}
			for _, target := range targets {
				if sourceCheckoutTargetNeedsLinkedWorktree(target, req.Repo) {
					return "IssueOps " + string(rec.Phase) + " phase requires a linked isolated worktree before mutating source files; create the sibling worktree and run issueops link-worktree for " + rec.ID
				}
			}
			return ""
		}
		for _, target := range targets {
			if !pathWithin(target, linked) {
				return "mutating tool target is outside the linked IssueOps worktree for " + rec.ID + "; run issue-based work from " + linked + " or mark the stale cycle done"
			}
		}
		return ""
	}
	if issueOpsWorktreePreparationCommand(req.Command) {
		return ""
	}
	targets := worktreeGuardEditTargets(req)
	if len(targets) == 0 {
		return ""
	}
	for _, target := range targets {
		if !pathWithin(target, expected) {
			return "mutating tool target is outside expected IssueOps worktree; set cwd/target path to the isolated worktree before editing"
		}
	}
	return ""
}

func targetInsideAnyLinkedIssueOpsWorktree(target string, records []IssueOpsRecord) bool {
	for _, record := range records {
		if pathWithin(target, record.WorktreePath) {
			return true
		}
	}
	return false
}

func sourceCheckoutTargetNeedsLinkedWorktree(target, repo string) bool {
	t := cleanAbsPath(target)
	r := cleanAbsPath(repo)
	if t == "" || r == "" {
		return false
	}
	if pathWithin(t, r) {
		return true
	}
	return isInsideWorktreesPath(t)
}
