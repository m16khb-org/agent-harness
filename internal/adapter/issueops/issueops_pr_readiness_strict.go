package issueops

import (
	"fmt"
	"strings"

	"agent-harness/internal/adapter/issueops/implementation"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/stringlist"
)

func IssueOpsStrictPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	ready := IssueOpsPRReadiness(record)
	ready.Strict = true
	missing := append([]string{}, ready.Missing...)
	warnings := []string{}
	currentHead := ""
	currentFingerprint := ""

	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		missing = append(missing, "repo")
	} else if code, out, _ := GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		missing = append(missing, "repo_git")
	} else {
		currentHead = issueOpsCurrentHead(record)
		currentFingerprint = implementation.ChangeFingerprint(record)
		branch := strings.TrimSpace(GitOut(gitRoot, "branch", "--show-current"))
		if strings.TrimSpace(record.Branch) != "" && branch != strings.TrimSpace(record.Branch) {
			missing = append(missing, "branch_match")
			warnings = append(warnings, "current branch "+branch+" does not match IssueOps branch "+strings.TrimSpace(record.Branch))
		}
		if strings.TrimSpace(GitOut(gitRoot, "status", "--porcelain=v1")) != "" {
			missing = append(missing, "worktree_clean")
		}
		upstream := strings.TrimSpace(GitOut(gitRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
		if upstream == "" {
			missing = append(missing, "upstream")
		} else {
			if code, _, stderr := GitCmd(gitRoot, "fetch", "--quiet"); code != 0 {
				missing = append(missing, "upstream_fetch")
				if strings.TrimSpace(stderr) != "" {
					warnings = append(warnings, "failed to fetch upstream: "+strings.TrimSpace(stderr))
				}
			}
			counts := strings.Fields(GitOut(gitRoot, "rev-list", "--left-right", "--count", "HEAD...@{u}"))
			if len(counts) != 2 || counts[0] != "0" || counts[1] != "0" {
				missing = append(missing, "upstream_synced")
				if len(counts) == 2 {
					warnings = append(warnings, "branch divergence against upstream: ahead="+counts[0]+" behind="+counts[1])
				}
			}
		}
	}
	if record.SourceMisdirectWarnings > 0 {
		warnings = append(warnings, fmt.Sprintf("source_misdirect_warnings:%d", record.SourceMisdirectWarnings))
	}
	// strict는 :12에서 IssueOpsPRReadiness를 포함하므로 미기록/비-pass 미싱은
	// 이미 들어 있다 — 여기서는 fingerprint가 필요한 stale 판정만 추가한다.
	if reviewMissing := implementationReviewMissing(record, currentFingerprint); strings.HasSuffix(reviewMissing, "_stale") {
		missing = append(missing, reviewMissing)
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

func issueOpsStrictPRReadinessWithState(stateRoot string, record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	ready := IssueOpsStrictPRReadiness(record)
	childMissing, childWarnings := issueOpsChildPRGateMissing(stateRoot, record)
	if len(childMissing) == 0 && len(childWarnings) == 0 {
		return ready
	}
	ready.Missing = stringlist.UniqueSorted(append(append([]string{}, ready.Missing...), childMissing...))
	ready.Warnings = append(ready.Warnings, childWarnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func IssueOpsStrictPRReadinessWithState(stateRoot string, record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithState(stateRoot, record)
}
