package operationalhealth

import (
	"fmt"
	"strings"
)

func validateCanonicalSource(builder *findingBuilder, snapshot Snapshot) {
	repo := clean(snapshot.RepoRoot)
	branch := strings.TrimSpace(snapshot.CanonicalBranch)
	head := strings.TrimSpace(snapshot.SourceHead)
	if repo == "" || branch == "" || head == "" {
		builder.add(FindingInventoryUnknown, "source", "source", "canonical source repository, branch, and HEAD identity must be complete", repo)
		return
	}
	if !snapshot.SourceClean {
		builder.add(FindingInventoryUnknown, "source", repo, "canonical source checkout is not clean", repo)
	}
	canonicalWorktrees := make([]GitWorktree, 0, 1)
	for _, worktree := range snapshot.GitWorktrees {
		if worktree.Canonical || clean(worktree.Path) == repo {
			canonicalWorktrees = append(canonicalWorktrees, worktree)
		}
	}
	if len(canonicalWorktrees) != 1 || clean(canonicalWorktrees[0].Path) != repo || !canonicalWorktrees[0].Canonical || strings.TrimSpace(canonicalWorktrees[0].Branch) != branch || strings.TrimSpace(canonicalWorktrees[0].Head) != head || !canonicalWorktrees[0].Clean {
		builder.add(FindingInventoryUnknown, "source_worktree", repo, "canonical Git worktree must occur once and match the clean source branch and HEAD", repo)
	}
	validateCanonicalRef(builder, snapshot.LocalRefs, "local", branch, head)
	validateCanonicalRef(builder, snapshot.RemoteRefs, "remote", branch, head)
}

func validateCanonicalOrcaSource(builder *findingBuilder, snapshot Snapshot) {
	repo := clean(snapshot.RepoRoot)
	branch := strings.TrimSpace(snapshot.CanonicalBranch)
	head := strings.TrimSpace(snapshot.SourceHead)
	runtimeID := strings.TrimSpace(snapshot.OrcaRuntimeID)
	repoID := strings.TrimSpace(snapshot.OrcaRepoID)
	matches := make([]OrcaWorktree, 0, 1)
	for _, worktree := range snapshot.OrcaWorktrees {
		if clean(worktree.Path) == repo {
			matches = append(matches, worktree)
		}
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0].RuntimeID) != runtimeID || strings.TrimSpace(matches[0].RepoID) != repoID || clean(matches[0].Repo) != repo || strings.TrimSpace(matches[0].Branch) != branch || strings.TrimSpace(matches[0].Head) != head {
		builder.add(FindingInventoryUnknown, "orca_source_worktree", repo, "canonical Orca worktree must occur once and match the source runtime, repository, branch, and HEAD", repo)
	}
}

func validateCanonicalRef(builder *findingBuilder, refs []GitRef, location, branch, head string) {
	matches := make([]GitRef, 0, 1)
	for _, ref := range refs {
		if strings.TrimSpace(ref.Location) == location && strings.TrimSpace(ref.Branch) == branch {
			matches = append(matches, ref)
		}
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0].OID) != head {
		builder.add(FindingInventoryUnknown, "branch", location+":"+branch, fmt.Sprintf("canonical %s ref must occur once at source HEAD %s", location, head), "")
	}
}

func knownPhase(value string) bool {
	switch value {
	case "problem", "grill", "plan", "compatibility-review", "implement", "ai-slop-clean", "feedback", "pr", "done":
		return true
	default:
		return false
	}
}

func knownLeaseStatus(value string) bool {
	switch value {
	case "", "claimable", "active", "revoking", "released":
		return true
	default:
		return false
	}
}

func hasExecutionProjection(cycle Cycle) bool {
	return strings.TrimSpace(cycle.ExecutionMode) != "" || cycle.Generation != 0 || strings.TrimSpace(cycle.WorktreePath) != "" ||
		!holderIdentityEmpty(cycle) || hasOrcaIdentity(cycle)
}

func executionIdentityComplete(cycle Cycle) bool {
	if strings.TrimSpace(cycle.Branch) == "" || strings.TrimSpace(cycle.WorktreePath) == "" || cycle.Generation == 0 {
		return false
	}
	switch strings.TrimSpace(cycle.ExecutionMode) {
	case "direct":
		return !hasOrcaIdentity(cycle)
	case "orca":
		return strings.TrimSpace(cycle.OrcaRuntimeID) != "" && strings.TrimSpace(cycle.OrcaRepoID) != "" &&
			strings.TrimSpace(cycle.OrcaWorktreeID) != "" && validOrcaOwnerHost(cycle.OrcaOwnerHost) &&
			strings.TrimSpace(cycle.TaskID) != "" && strings.TrimSpace(cycle.DispatchID) != ""
	default:
		return false
	}
}

func activeHolderIdentityComplete(cycle Cycle) bool {
	if !holderIdentityComplete(cycle) {
		return false
	}
	return strings.TrimSpace(cycle.ExecutionMode) != "orca" || strings.TrimSpace(cycle.OrcaOwnerHost) == strings.TrimSpace(cycle.HolderHost)
}

func retainedLeaseIdentityComplete(cycle Cycle) bool {
	switch strings.TrimSpace(cycle.LeaseStatus) {
	case "claimable", "released":
		return holderIdentityEmpty(cycle)
	case "revoking":
		return holderIdentityComplete(cycle)
	default:
		return false
	}

}

func holderIdentityComplete(cycle Cycle) bool {
	host := strings.TrimSpace(cycle.HolderHost)
	return validNativeHost(host) && strings.TrimSpace(cycle.HolderSessionID) != "" &&
		cycle.HolderPID > 0 && strings.TrimSpace(cycle.HolderStartedAt) != "" && strings.TrimSpace(cycle.HolderExecutable) != ""
}

func validNativeHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "codex" || host == "claude" || host == "omo"
}

func validOrcaOwnerHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "codex" || host == "claude" || host == "omo"
}

func holderIdentityEmpty(cycle Cycle) bool {
	return strings.TrimSpace(cycle.HolderHost) == "" && strings.TrimSpace(cycle.HolderSessionID) == "" &&
		strings.TrimSpace(cycle.HolderAgentID) == "" && cycle.HolderPID == 0 && strings.TrimSpace(cycle.HolderStartedAt) == "" &&
		strings.TrimSpace(cycle.HolderExecutable) == ""
}

func hasOrcaIdentity(cycle Cycle) bool {
	return strings.TrimSpace(cycle.OrcaRuntimeID) != "" || strings.TrimSpace(cycle.OrcaRepoID) != "" ||
		strings.TrimSpace(cycle.OrcaWorktreeID) != "" || strings.TrimSpace(cycle.OrcaWorktreeInstanceID) != "" ||
		strings.TrimSpace(cycle.OrcaOwnerHost) != "" || strings.TrimSpace(cycle.TerminalPTYID) != "" ||
		strings.TrimSpace(cycle.TaskID) != "" || strings.TrimSpace(cycle.DispatchID) != ""
}

// knownTaskStatus reports whether a status belongs to Orca's task vocabulary.
//
// The source is the Orca CLI itself, not another Go definition:
//
//	$ orca orchestration task-update --help
//	Notes:
//	  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
//
// #136 matched this list against the adapter's defensive set instead, and both
// definitions ended up wrong together (#145). The adapter's mirror of this
// vocabulary lives in executionTerminalTaskStatus; the two must agree, and the
// CLI decides which values they agree on.
func knownTaskStatus(value string) bool {
	switch value {
	case "pending", "ready", "dispatched", "blocked":
		return true
	default:
		return settledTaskStatus(value)
	}
}

// settledTaskStatus reports whether a task reached a terminal state and can no
// longer be dispatched or hold a worker.
//
// Only these two of Orca's six statuses qualify. pending waits to be dispatched,
// blocked waits on a dependency, ready can be dispatched, and dispatched is
// running — all four can still hold or acquire a worker, so a task left in one
// of them without an owner is genuine residue.
//
// #136 widened this to six by mirroring the adapter's defensive set, which
// included values Orca rejects outright. #121's original pair was right (#145).
func settledTaskStatus(value string) bool {
	return value == "completed" || value == "failed"
}

// knownDispatchStatus는 Orca가 dispatch_contexts.status에 담을 수 있는 값을
// 판정한다. 어휘의 출처는 Orca 소스이지 우리 추론이 아니다:
//
//	src/main/runtime/orchestration/types.ts
//	export type DispatchStatus = 'pending' | 'dispatched' | 'completed' | 'failed' | 'circuit_broken'
//
//	src/main/runtime/orchestration/db.ts (dispatch_contexts 스키마)
//	CHECK(status IN ('pending', 'dispatched', 'completed', 'failed', 'circuit_broken'))
//
// CHECK 제약이 있으므로 그 밖의 값은 스키마 마이그레이션 없이는 저장될 수 없다.
// 그래도 미지 값을 unknown으로 떨어뜨리는 것은 유지한다 — Orca가 어휘를 넓히면
// 우리가 먼저 알아야 하고, 조용히 통과시키면 그 시점을 놓친다.
//
// dispatched 하나만 유효로 보던 동안 유효 어휘 5개 중 4개가 "unsupported"로
// 보고됐다. 유효 상태가 unknown으로 오면 진짜 미지 값이 그 사이에 묻힌다(#171).
func knownDispatchStatus(value string) bool {
	switch value {
	case "pending", "dispatched":
		return true
	default:
		return settledDispatchStatus(value)
	}
}

// settledDispatchStatus는 dispatch가 더 이상 워커를 잡거나 획득할 수 없는
// 상태인지 판정한다.
//
// failed가 여기 있는 이유는 db.ts의 failDispatch다:
//
//	const newStatus: DispatchStatus = newFailureCount >= 3 ? 'circuit_broken' : 'failed'
//	const taskStatus: TaskStatus = newStatus === 'circuit_broken' ? 'failed' : 'ready'
//
// dispatch가 failed면 그 **시도**가 끝난 것이고, 작업은 task가 ready로 돌아가
// 재dispatch를 기다린다 — 그 대기는 task 축(settledTaskStatus)이 본다. 재dispatch는
// 새 dispatch ID를 만들므로 이 dispatch로 돌아오지 않는다.
//
// circuit_broken은 coordinator.ts가 재dispatch하지 않고 failedTasks에 넣는
// 종착점이다. 워커를 다시 붙일 경로가 없다.
func settledDispatchStatus(value string) bool {
	return value == "completed" || value == "failed" || value == "circuit_broken"
}

// knownGateStatus의 어휘도 Orca 소스가 정한다:
//
//	src/main/runtime/orchestration/types.ts
//	export type GateStatus = 'pending' | 'resolved' | 'timeout'
//
// timeout이 빠져 있던 동안 시간 초과된 gate가 "unsupported"로 보고됐다(#171).
func knownGateStatus(value string) bool {
	switch value {
	case "pending":
		return true
	default:
		return settledGateStatus(value)
	}
}

func settledGateStatus(value string) bool {
	return value == "resolved" || value == "timeout"
}

func preservableIdentityComplete(cycle Cycle) bool {
	if strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" || strings.TrimSpace(cycle.Branch) == "" {
		return false
	}
	status := strings.TrimSpace(cycle.LeaseStatus)
	if status == "" {
		return !hasExecutionProjection(cycle)
	}
	if !knownLeaseStatus(status) || !executionIdentityComplete(cycle) {
		return false
	}
	if status == "active" {
		return activeHolderIdentityComplete(cycle) && strings.TrimSpace(cycle.HolderProcessStatus) == ProcessStatusLive
	}
	return retainedLeaseIdentityComplete(cycle)
}
