package issueops

import (
	"strings"

	"agent-harness/internal/core/issueops/implementation"
	"agent-harness/internal/core/issueops/stringlist"
	"agent-harness/internal/core/preflight"
)

func IssueOpsStrictPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsPRReadiness(record)
	ready.Strict = true
	missing := append([]string{}, ready.Missing...)
	warnings := []string{}
	currentHead := ""
	currentFingerprint := ""

	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		missing = append(missing, "repo")
	} else if code, out, _ := preflight.GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		missing = append(missing, "repo_git")
	} else {
		currentHead = issueOpsCurrentHead(record)
		currentFingerprint = implementation.ChangeFingerprint(record)
		branch := strings.TrimSpace(preflight.GitOut(gitRoot, "branch", "--show-current"))
		if strings.TrimSpace(record.Branch) != "" && branch != strings.TrimSpace(record.Branch) {
			missing = append(missing, "branch_match")
			warnings = append(warnings, "current branch "+branch+" does not match IssueOps branch "+strings.TrimSpace(record.Branch))
		}
		if strings.TrimSpace(preflight.GitOut(gitRoot, "status", "--porcelain=v1")) != "" {
			missing = append(missing, "worktree_clean")
		}
		upstream := strings.TrimSpace(preflight.GitOut(gitRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
		if upstream == "" {
			missing = append(missing, "upstream")
		} else {
			if code, _, stderr := preflight.GitCmd(gitRoot, "fetch", "--quiet"); code != 0 {
				missing = append(missing, "upstream_fetch")
				if strings.TrimSpace(stderr) != "" {
					warnings = append(warnings, "failed to fetch upstream: "+strings.TrimSpace(stderr))
				}
			}
			counts := strings.Fields(preflight.GitOut(gitRoot, "rev-list", "--left-right", "--count", "HEAD...@{u}"))
			if len(counts) != 2 || counts[0] != "0" || counts[1] != "0" {
				missing = append(missing, "upstream_synced")
				if len(counts) == 2 {
					warnings = append(warnings, "branch divergence against upstream: ahead="+counts[0]+" behind="+counts[1])
				}
			}
		}
	}
	if strings.TrimSpace(record.AISlopCleanAt) != "" {
		storedFingerprint := strings.TrimSpace(record.AISlopCleanFingerprint)
		if storedFingerprint == "" && currentFingerprint != "" {
			missing = append(missing, "ai_slop_clean_fingerprint")
		} else if storedFingerprint != "" && currentFingerprint == "" {
			missing = append(missing, "current_fingerprint")
		} else if storedFingerprint != "" && storedFingerprint != currentFingerprint {
			missing = append(missing, "ai_slop_clean_stale")
		}
	}

	if path := strings.TrimSpace(record.PlanPath); path != "" && !issueOpsPlanPathExists(gitRoot, path) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	missing = append(missing, issueOpsTargetBranchMatchMissing(record)...)

	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Warnings = warnings
	ready.AISlopCleanHead = record.AISlopCleanHead
	ready.CurrentHead = currentHead
	ready.AISlopCleanFingerprint = record.AISlopCleanFingerprint
	ready.CurrentFingerprint = currentFingerprint
	ready.Ready = len(ready.Missing) == 0
	return ready
}
