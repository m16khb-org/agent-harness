// Package loopgate는 IssueOps의 PR readiness에 loop run gate를 합성한다.
//
// issueops는 IssueOps 레코드만 보고 readiness를 판정하고, looprun은 저장소의 loop
// 상태만 안다. 두 판정을 합치는 규칙은 어느 한쪽의 관심사가 아니므로 별도 패키지가
// 소유한다. internal/adapter/core facade가 갖고 있던 조립을 옮겨온 것이며, 이름이
// issueops의 동명 함수와 겹치지 않도록 여기서는 접두사 없는 이름을 쓴다.
package loopgate

import (
	"fmt"
	"sort"
	"strings"

	"agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// StrictPRReadiness는 레코드 기반 strict readiness에 loop gate를 더한다.
func StrictPRReadiness(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	return withLoopGate(issueops.IssueOpsStrictPRReadiness(record), record.Repo)
}

// StrictPRReadinessWithState는 state까지 읽는 strict readiness에 loop gate를 더한다.
func StrictPRReadinessWithState(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	return withLoopGate(issueops.IssueOpsStrictPRReadinessWithState(stateRoot, record), record.Repo)
}

// AdvancePhase는 pr 단계 진입 전에 strict readiness를 강제한다.
func AdvancePhase(stateRoot, id, to string) (issueopscontract.IssueOpsRecord, error) {
	if err := guardPRPhase(stateRoot, id, to); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	return issueops.AdvanceIssueOpsPhase(stateRoot, id, to)
}

// AdvancePhaseWithActor는 AdvancePhase와 같은 gate를 actor 경로에 적용한다.
func AdvancePhaseWithActor(stateRoot, id, to string, actor issueops.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
	if err := guardPRPhase(stateRoot, id, to); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	return issueops.AdvanceIssueOpsPhaseWithActor(stateRoot, id, to, actor)
}

// guardPRPhase는 이미 pr 단계인 레코드는 통과시킨다. 재진입까지 막으면 복구 경로가
// 사라지기 때문이다.
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
	if ready := StrictPRReadiness(record); !ready.Ready {
		return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
	}
	return nil
}

func withLoopGate(ready issueopscontract.IssueOpsReadiness, repo string) issueopscontract.IssueOpsReadiness {
	missing, warnings := RepoGateMissing(repo)
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	ready.Missing = uniqSorted(append(append([]string{}, ready.Missing...), missing...))
	ready.Warnings = append(ready.Warnings, warnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func uniqSorted(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		if v != "" {
			m[v] = true
		}
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
