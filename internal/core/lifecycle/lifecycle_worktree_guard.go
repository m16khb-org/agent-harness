package lifecycle

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/lifecycle/worktreeguard"
	"agent-harness/internal/core/searchrouting"
)

func worktreeGuardBlockReason(req HookToolUseLifecycleRequest) string {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	expected := cleanAbsPath(req.ExpectedWorktree)
	if expected == "" {
		return sourceCheckoutWorktreeGuardBlockReason(req)
	}
	return expectedWorktreeGuardBlockReason(req, expected)
}

func sourceCheckoutMirrorEditAskReason(req HookToolUseLifecycleRequest) (string, string) {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return "", ""
	}
	repo := cleanAbsPath(req.Repo)
	if repo == "" {
		return "", ""
	}
	binding, ok := issueOpsSessionBindingForMirrorGuard(repo)
	if !ok {
		return "", ""
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), binding.CycleID)
	if err != nil || !IssueOpsPhaseExpectsWorktree(record.Phase) {
		return "", ""
	}
	worktree := cleanAbsPath(binding.ExpectedWorktree)
	if worktree == "" {
		worktree = cleanAbsPath(record.WorktreePath)
	}
	if worktree == "" {
		return "", ""
	}
	for _, target := range worktreeGuardEditTargets(req) {
		cleanTarget := cleanAbsPath(target)
		if cleanTarget == "" || !pathWithin(cleanTarget, repo) || isInsideWorktreesPath(cleanTarget) {
			continue
		}
		rel, err := filepath.Rel(repo, cleanTarget)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := os.Stat(filepath.Join(worktree, rel)); err != nil {
			continue
		}
		reason := "세션이 IssueOps 워크트리 " + worktree + "에 바인딩되어 있고 소스 체크아웃의 같은 상대 경로가 워크트리에도 있습니다. " +
			"이 편집이 사이클 작업이면 워크트리에서 편집하세요; 무관한 소스 체크아웃 작업이면 승인하세요. " +
			"사이클이 stale이면 `issueops force-release --id " + record.ID + " --reason <why>`로 해제하세요."
		return "ask", reason
	}
	return "", ""
}

func sourceCheckoutWorktreeGuardBlockReason(req HookToolUseLifecycleRequest) string {
	if reason := localIssueOpsBranchCreationBlockReason(req); reason != "" {
		return reason
	}
	currentBranch := gitBranchFromHead(req.Repo)
	rec, ok := ActiveIssueOpsCycleForBranch(req.Repo, currentBranch)
	if issueOpsWorktreePreparationCommand(req.Command) {
		return ""
	}
	targets := worktreeGuardEditTargets(req)
	if len(targets) == 0 {
		return ""
	}
	if !ok {
		if noCycleIssueOpsBranchNeedsWorktree(req, currentBranch, targets) {
			return "IssueOps branch " + currentBranch + " has no active IssueOps cycle; start the IssueOps workflow and use a linked isolated worktree before mutating source files"
		}
		// No active IssueOps cycle on the current branch: allow mutating edits
		// from the source checkout. Other cycles on their own branches enforce
		// their own worktree isolation. This prevents a stuck cycle on another
		// branch from deadlocking all repo-wide edits (e.g. a cycle stuck in
		// PR phase because remote_artifact verification is unavailable).
		return ""
	}
	if !IssueOpsPhaseExpectsWorktree(rec.Phase) {
		return sourceCheckoutLinkedCycleBlockReason(req.Repo, targets)
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
			return "mutating tool target is outside the linked IssueOps worktree for " + rec.ID + "; run issue-based work from " + linked + " or release the stale cycle with `issueops force-release --id " + rec.ID + " --reason <why>`"
		}
	}
	return ""
}

func localIssueOpsBranchCreationBlockReason(req HookToolUseLifecycleRequest) string {
	creation := worktreeguard.LocalIssueOpsBranchCreation(req.Command)
	if creation.Branch == "" {
		return ""
	}
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
	return "IssueOps branch " + creation.Branch + " must be started through IssueOps before checking it out in the source checkout; run issueops start, create the provider-linked branch in an isolated worktree, then link the worktree before implementation"
}

func sourceCheckoutLinkedCycleBlockReason(repo string, targets []string) string {
	linkedRecs := ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
	if len(linkedRecs) == 0 {
		return ""
	}
	for _, target := range targets {
		if !sourceCheckoutTargetNeedsLinkedWorktree(target, repo) {
			continue
		}
		if targetInsideAnyLinkedIssueOpsWorktree(target, linkedRecs) {
			continue
		}
		return linkedWorktreeCyclesBlockReason(linkedRecs)
	}
	return ""
}

func expectedWorktreeGuardBlockReason(req HookToolUseLifecycleRequest, expected string) string {
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

// linkedWorktreeCyclesBlockReason builds the block message shown when a source-
// checkout edit collides with one or more active IssueOps worktree cycles on
// OTHER branches. The records slice is already deterministically ordered (see
// active.LinkedWorktreeCyclesForRepo) so the message is reproducible. With
// multiple parallel cycles it names every holder rather than an arbitrary first
// one — singling out one cycle's force-release would (a) be destructive to an
// unrelated, possibly-live cycle and (b) not actually unblock the edit while the
// other worktrees remain (the non-working-escape trap from CAUTIONS section 21).
func linkedWorktreeCyclesBlockReason(records []IssueOpsRecord) string {
	if len(records) == 1 {
		r := records[0]
		return "mutating tool target is outside the linked IssueOps worktree for " + r.ID +
			"; run issue-based work from " + cleanAbsPath(r.WorktreePath) +
			" or release the stale cycle with `issueops force-release --id " + r.ID + " --reason <why>`"
	}
	var b strings.Builder
	b.WriteString("mutating tool target is outside the linked IssueOps worktree of an active parallel cycle; ")
	b.WriteString(strconv.Itoa(len(records)))
	b.WriteString(" cycles currently hold worktrees [")
	for i, r := range records {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(r.ID)
		b.WriteString(" -> ")
		b.WriteString(cleanAbsPath(r.WorktreePath))
	}
	b.WriteString("]; run this edit from the matching worktree, or if a specific cycle is abandoned release it with `issueops force-release --id <id> --reason <why>`")
	return b.String()
}

func targetInsideAnyLinkedIssueOpsWorktree(target string, records []IssueOpsRecord) bool {
	for _, record := range records {
		if pathWithin(target, record.WorktreePath) {
			return true
		}
	}
	return false
}

func noCycleIssueOpsBranchNeedsWorktree(req HookToolUseLifecycleRequest, branch string, targets []string) bool {
	if strings.TrimSpace(branch) == "" {
		return false
	}
	if validateIssueOpsIssueBranch(branch) != nil {
		return false
	}
	if rec, err := ReadIssueOps(IssueOpsStateRoot(), newIssueOpsID(req.Repo, branch)); err == nil {
		if rec.Phase == IssueOpsPhaseDone {
			return false
		}
		// A worktree-phase cycle whose isolated worktree was deleted is stale and
		// must not force worktree isolation onto the source checkout — that would
		// deadlock every edit on this branch with no reachable escape. Let it fall
		// through to allow; the user can start fresh or force-release the cycle.
		if issueOpsCycleWorktreeMissing(rec) {
			return false
		}
	}
	if issueOpsWorktreePreparationCommand(req.Command) || issueOpsBootstrapCommand(req.Command) {
		return false
	}
	linkedRecs := ActiveIssueOpsLinkedWorktreeCyclesForRepo(req.Repo)
	for _, target := range targets {
		if sourceCheckoutTargetNeedsLinkedWorktree(target, req.Repo) && !targetInsideAnyLinkedIssueOpsWorktree(target, linkedRecs) {
			return true
		}
	}
	return false
}

func issueOpsBootstrapCommand(command string) bool {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "issueops" || i+1 >= len(tokens) {
			continue
		}
		switch searchrouting.SearchTokenName(tokens[i+1]) {
		case "start", "link-worktree":
			return true
		case "branch":
			if i+2 < len(tokens) && searchrouting.SearchTokenName(tokens[i+2]) == "prepare" {
				return true
			}
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
