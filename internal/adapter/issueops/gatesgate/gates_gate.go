// Package gatesgate는 IssueOps의 PR readiness에 태스크 게이트 ledger 게이트를
// 합성한다.
//
// loopgate가 loop run 상태를 readiness에 더하는 것과 같은 조립 구조다.
// 게이트 파일(worktree의 .agent-harness/gates/*.md 또는 호환 경로)이 존재하면 미충족 게이트가
// PR 진입을 막고, 파일이 없으면 게이트가 적용되지 않는다 — unlazy와 같은
// opt-in: 게이트를 만드는 순간 완료가 구조적으로 강제된다.
package gatesgate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/loopgate"
	gatescontract "agent-harness/internal/contract/gates"
	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsremote "agent-harness/internal/domain/issueopsremote"
)

// gates ledger 조회·평가는 composition root가 설치한다. gatesgate는 gates
// adapter를 직접 import하지 않는다(크로스 케퍼빌리티 adapter edge 금지,
// loopgate의 RepoGateMissing 패턴과 같다).
var (
	// DiscoverGateFiles는 canonical/compatible 게이트 파일 발견 연산이다.
	DiscoverGateFiles func(root string) ([]string, error)
	// CheckGateLedger는 게이트 ledger 평가 연산이다.
	CheckGateLedger func(req gatescontract.CheckRequest) (gatescontract.CheckResult, error)
)

// StrictPRReadinessWithState는 loop 게이트에 게이트 ledger 게이트를 더한
// strict readiness다.
func StrictPRReadinessWithState(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	ready := loopgate.StrictPRReadinessWithState(stateRoot, record)
	ready = withGatesGate(ready, GatesRootFor(record), linkedIssueNumber(record))
	return withDuplicateIssueArtifactGate(ready, GatesRootFor(record), linkedIssueNumber(record))
}

// linkedIssueNumber는 레코드가 가리키는 provider 이슈 번호다. 없으면 빈 문자열.
func linkedIssueNumber(record issueopscontract.IssueOpsRecord) string {
	if n := issueopsremote.IssueNumber(record.IssueURL); n != "" {
		return n
	}
	if record.BranchPrepare != nil {
		return issueopsremote.IssueNumber(record.BranchPrepare.IssueURL)
	}
	return ""
}

// withDuplicateIssueArtifactGate는 현재 사이클의 이슈 원장이 canonical
// `.agent-harness/issues/<n>/gates.md`와 호환 경로 `.agent-harness/gates/`
// (`issue-<n>*`, `<n>-*`) 양쪽에 있으면 `duplicate_issue_artifact:<n>`으로
// fail-closed한다(#480). 다른 이슈의 중복은 이 사이클을 막지 않으며, 번호를
// 모르면 판정하지 않는다.
func withDuplicateIssueArtifactGate(ready issueopscontract.IssueOpsReadiness, root, issueNumber string) issueopscontract.IssueOpsReadiness {
	root, issueNumber = strings.TrimSpace(root), strings.TrimSpace(issueNumber)
	if root == "" || issueNumber == "" {
		return ready
	}
	canonical := filepath.Join(root, ".agent-harness", "issues", issueNumber, "gates.md")
	if info, err := os.Stat(canonical); err != nil || info.IsDir() {
		return ready
	}
	entries, err := os.ReadDir(filepath.Join(root, ".agent-harness", "gates"))
	if err != nil {
		return ready
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if legacyLedgerIssueNumber(entry.Name()) == issueNumber {
			ready.Missing = uniqSorted(append(append([]string{}, ready.Missing...), "duplicate_issue_artifact:"+issueNumber))
			ready.Ready = false
			return ready
		}
	}
	return ready
}

// scopeLedgers는 발견된 원장을 현재 사이클이 판정할 것(judged)과 다른 이슈의
// 것(skipped)으로 가른다(#483). issueNumber가 비면 전부 판정한다(fail-closed).
// 판정: issues/<issueNumber>/gates.md(폴더명 문자열 완전일치 — `021`·`210`은
// `21`이 아니다), 전부 숫자가 아닌 폴더(소유자 불명), `.agent-harness/gates/`의
// 같은 번호 또는 번호 없는 파일, 그 밖의 호환 원장(root GATES.md, gates/*.md).
// 제외: 다른 숫자 폴더의 gates.md와 다른 번호 접두의 `.agent-harness/gates/` 파일.
func scopeLedgers(root string, files []string, issueNumber string) (judged, skipped []string) {
	issueNumber = strings.TrimSpace(issueNumber)
	if issueNumber == "" {
		return files, nil
	}
	issuesDir := filepath.Join(root, ".agent-harness", "issues") + string(filepath.Separator)
	legacyDir := filepath.Join(root, ".agent-harness", "gates") + string(filepath.Separator)
	for _, file := range files {
		switch {
		case strings.HasPrefix(file, issuesDir):
			folder := strings.SplitN(strings.TrimPrefix(file, issuesDir), string(filepath.Separator), 2)[0]
			if folder != issueNumber && allDigits(folder) {
				skipped = append(skipped, file)
				continue
			}
		case strings.HasPrefix(file, legacyDir):
			if n := legacyLedgerIssueNumber(filepath.Base(file)); n != "" && n != issueNumber {
				skipped = append(skipped, file)
				continue
			}
		}
		judged = append(judged, file)
	}
	return judged, skipped
}

func allDigits(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

const gatesLedgerCompatibilitySchemaVersion = 1

// legacyLedgerIssueNumber는 schema v1 persisted state의 호환 원장 파일명
// `issue-<n>*.md` 또는 `<n>-*.md`에서 번호를 뽑는다. state migration이
// canonical issue folder로 이름을 옮긴 뒤 schema가 올라가면 이 경로는 닫힌다.
func legacyLedgerIssueNumber(name string) string {
	return legacyLedgerIssueNumberForSchema(name, gatesLedgerCompatibilitySchemaVersion)
}

func legacyLedgerIssueNumberForSchema(name string, schemaVersion int) string {
	if schemaVersion != gatesLedgerCompatibilitySchemaVersion {
		return ""
	}
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimPrefix(name, "issue-")
	digits := 0
	for digits < len(name) && name[digits] >= '0' && name[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return ""
	}
	if digits == len(name) || name[digits] == '-' {
		return name[:digits]
	}
	return ""
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

func withGatesGate(ready issueopscontract.IssueOpsReadiness, root, issueNumber string) issueopscontract.IssueOpsReadiness {
	root = strings.TrimSpace(root)
	if root == "" {
		return ready
	}
	files, err := DiscoverGateFiles(root)
	if err != nil || len(files) == 0 {
		return ready
	}
	files, skipped := scopeLedgers(root, files, issueNumber)
	missing := append([]string{}, ready.Missing...)
	warnings := []string{}
	if len(skipped) > 0 {
		rels := make([]string, 0, len(skipped))
		for _, file := range skipped {
			rels = append(rels, relPath(root, file))
		}
		warnings = append(warnings, fmt.Sprintf("gates_skipped:%d (%s)", len(skipped), strings.Join(rels, ", ")))
	}
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
