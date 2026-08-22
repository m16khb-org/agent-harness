// Package gatesgate는 IssueOps의 PR readiness에 태스크 게이트 ledger 게이트를
// 합성한다.
//
// loopgate가 loop run 상태를 readiness에 더하는 것과 같은 조립 구조다.
// 게이트 파일(worktree의 GATES.md 또는 gates/*.md)이 존재하면 미충족 게이트가
// PR 진입을 막고, 파일이 없으면 게이트가 적용되지 않는다 — unlazy와 같은
// opt-in: 게이트를 만드는 순간 완료가 구조적으로 강제된다.
package gatesgate

import (
	"fmt"
	"sort"
	"strings"

	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/loopgate"
	gatescontract "agent-harness/internal/contract/gates"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// gates ledger 조회·평가는 composition root가 설치한다. gatesgate는 gates
// adapter를 직접 import하지 않는다(크로스 케퍼빌리티 adapter edge 금지,
// loopgate의 RepoGateMissing 패턴과 같다).
var (
	// DiscoverGateFiles는 게이트 파일(GATES.md + gates/*.md) 발견 연산이다.
	DiscoverGateFiles func(root string) ([]string, error)
	// CheckGateLedger는 게이트 ledger 평가 연산이다.
	CheckGateLedger func(req gatescontract.CheckRequest) (gatescontract.CheckResult, error)
)

// StrictPRReadinessWithState는 loop 게이트에 게이트 ledger 게이트를 더한
// strict readiness다.
func StrictPRReadinessWithState(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	ready := loopgate.StrictPRReadinessWithState(stateRoot, record)
	return withGatesGate(ready, GatesRootFor(record))
}

// AdvancePhaseWithActor는 pr 단계 진입 전에 게이트 ledger까지 포함한 strict
// readiness를 강제한다.
func AdvancePhaseWithActor(stateRoot, id, to string, actor issueops.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
	if err := guardPRPhase(stateRoot, id, to); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	return issueops.AdvanceIssueOpsPhaseWithActor(stateRoot, id, to, actor)
}

// GatesRootFor는 게이트 파일을 찾을 루트다. worktree가 있으면 worktree, 없으면
// 레코드 repo를 쓴다.
func GatesRootFor(record issueopscontract.IssueOpsRecord) string {
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		return path
	}
	return strings.TrimSpace(record.Repo)
}

// guardPRPhase는 이미 pr 단계인 레코드는 통과시킨다(복구 경로 보존).
func guardPRPhase(stateRoot, id, to string) error {
	if issueopscontract.IssueOpsPhase(strings.TrimSpace(to)) != issueopscontract.IssueOpsPhasePR {
		return nil
	}
	record, err := issueops.ReadIssueOps(stateRoot, id)
	if err != nil {
		return err
	}
	if record.Phase == issueopscontract.IssueOpsPhasePR {
		return nil
	}
	if ready := StrictPRReadinessWithState(stateRoot, record); !ready.Ready {
		return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
	}
	return nil
}

func withGatesGate(ready issueopscontract.IssueOpsReadiness, root string) issueopscontract.IssueOpsReadiness {
	root = strings.TrimSpace(root)
	if root == "" {
		return ready
	}
	files, err := DiscoverGateFiles(root)
	if err != nil || len(files) == 0 {
		return ready
	}
	missing := append([]string{}, ready.Missing...)
	warnings := []string{}
	for _, file := range files {
		result, err := CheckGateLedger(gatescontract.CheckRequest{
			WorkspaceRoot: root,
			CWD:           root,
			Files:         []string{file},
			StatusOnly:    true,
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("gates %s: %v", relPath(root, file), err))
			continue
		}
		if result.Complete {
			continue
		}
		missing = append(missing, "gates_incomplete:"+relPath(root, file))
		for _, fileResult := range result.Files {
			for _, gate := range fileResult.Gates {
				if gate.State == "unchecked" || gate.State == "evidence_pending" {
					warnings = append(warnings, fmt.Sprintf("gate %s (%s): %s", gate.ID, gate.State, gate.Title))
				}
			}
		}
	}
	if len(missing) == len(ready.Missing) && len(warnings) == 0 {
		return ready
	}
	ready.Missing = uniqSorted(missing)
	ready.Warnings = append(ready.Warnings, warnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func relPath(root, file string) string {
	rel := strings.TrimPrefix(file, root)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return file
	}
	return rel
}

func uniqSorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
