package issueops

import (
	"fmt"
	"strings"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/stringlist"
)

// IssueOpsLocalPRReadiness는 네트워크를 건드리지 않는 판정이다. strict에서
// `git fetch`와 그 결과에 기대는 upstream 동기화 판정만 뺐다. 단계 분류처럼
// 자주 부르는 읽기 전용 표면이 원격을 때리지 않게 하려는 분리다.
func IssueOpsLocalPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsObservedPRReadiness(record, false)
}

func IssueOpsStrictPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsObservedPRReadiness(record, true)
}

// issueOpsObservedPRReadiness는 두 표면의 유일한 본체다. syncUpstream이 false면
// fetch와 동기화 판정을 건너뛴다. 나머지 관측은 한 번만 수행한다 — local을
// 부른 뒤 strict가 같은 git 명령을 다시 실행하지 않게 하려는 것이다.
func issueOpsObservedPRReadiness(record issueops.IssueOpsRecord, syncUpstream bool) issueops.IssueOpsReadiness {
	ready := IssueOpsPRReadiness(record)
	ready.Strict = syncUpstream
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
		} else if syncUpstream {
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
	if docsMissing := projectDocsReviewMissing(record, currentFingerprint); strings.HasSuffix(docsMissing, "_stale") {
		missing = append(missing, docsMissing)
	}
	// schema_evidence는 non-strict 표면이 판정하지 않으므로 여기서 전체를 본다.
	if schemaMissing := schemaEvidenceMissing(record, currentFingerprint); schemaMissing != "" {
		missing = append(missing, schemaMissing)
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
