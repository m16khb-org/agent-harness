// Package issueopslinkedbranch는 issue linked-branch 관측을 분류하는 순수
// 규칙이다. GitHub `createLinkedBranch`는 오류 없이 `ref:null`을 돌려줄 수
// 있고(#306, #304에서 실측), 그때 남는 레코드는 이름이 없어 브랜치 이름으로
// 지목할 수 없다. 무엇을 지울 수 있는지 판정하는 규칙을 I/O에서 분리해 둔다.
package issueopslinkedbranch

import (
	"fmt"
	"strings"
)

// Node는 이슈에 붙어 있는 linked-branch 레코드 하나다.
type Node struct {
	// ID는 GraphQL node id다. ref가 없는 고아를 지목할 수 있는 유일한 좌표다.
	ID string
	// RefName은 mutation이 ref를 만들지 못했으면 빈 문자열이다.
	RefName string
	// RefOID는 ref가 가리키는 커밋이다.
	RefOID string
}

// RefNull은 이 레코드가 브랜치 없이 남았는지 보고한다.
func (n Node) RefNull() bool { return strings.TrimSpace(n.RefName) == "" }

// Observation은 한 시점의 전체 관측이다. 이슈의 linked-branch 목록과, 그
// lifecycle이 요청했던 브랜치의 원격 ref를 **같은 시점에** 읽어야 한다 —
// 서로 다른 시점의 관측을 섞으면 수렴 중인 상태를 고아로 오판한다.
type Observation struct {
	IssueURL        string
	RequestedBranch string
	SealedBase      string
	// TotalCount는 서버가 보고한 총 개수다. 페이지에 담긴 노드 수와 다르면
	// 관측이 잘린 것이고, 잘린 관측은 부재의 증거가 아니다.
	TotalCount int
	Nodes      []Node
	// RemoteOID는 refs/heads/<RequestedBranch>의 관측값이다. 없으면 빈 값이다.
	RemoteOID string
}

// State는 관측의 분류다. 삭제가 허용되는 상태는 StateOrphan 하나뿐이다.
type State string

const (
	// StateAbsent는 linked-branch 레코드가 아예 없는 상태다. 정리할 것이 없다.
	StateAbsent State = "absent"
	// StateHealthy는 요청한 이름과 봉인된 base로 정상 생성된 상태다.
	StateHealthy State = "healthy"
	// StateOrphan은 ref가 null인 레코드만 남은 부분 성공이다.
	StateOrphan State = "orphan_ref_null"
	// StateMismatched는 ref는 있으나 이름이나 OID가 봉인값과 다른 상태다.
	// 정리 대상이 아니다 — 우리가 만든 것이 아닐 수 있다.
	StateMismatched State = "mismatched"
	// StateAmbiguous는 판정 근거가 부족하거나 서로 어긋나는 상태다.
	// fail-closed의 기본값이다.
	StateAmbiguous State = "ambiguous"
)

// Classify는 관측을 분류하고, 고아일 때 지울 대상 노드와 근거를 함께 돌려준다.
//
// 어느 분기에서도 "아마 이것일 것"으로 노드를 고르지 않는다. ref가 없는
// 레코드는 이름이 없으므로 후보가 둘 이상이면 우리 것을 구분할 방법이 없고,
// 그때는 지우지 않는 편이 항상 옳다.
func Classify(observation Observation) (State, Node, string) {
	if strings.TrimSpace(observation.RequestedBranch) == "" || strings.TrimSpace(observation.SealedBase) == "" {
		return StateAmbiguous, Node{}, "the lifecycle record must seal both the requested branch and the base OID before its linked branch can be classified"
	}
	if observation.TotalCount != len(observation.Nodes) {
		return StateAmbiguous, Node{}, fmt.Sprintf(
			"the issue reports %d linked branches but the readback carried %d; a truncated page is not evidence of absence",
			observation.TotalCount, len(observation.Nodes))
	}
	for _, node := range observation.Nodes {
		if node.RefNull() || node.RefName != observation.RequestedBranch {
			continue
		}
		// 이름이 맞는 ref가 있으면 그것이 이 lifecycle의 링크다. 고아 판정은
		// 여기서 끝난다 — 남은 질문은 봉인값과 일치하는지뿐이다.
		if node.RefOID != observation.SealedBase {
			return StateMismatched, node, fmt.Sprintf(
				"linked branch %s points at %s but the lifecycle sealed %s", node.RefName, node.RefOID, observation.SealedBase)
		}
		if observation.RemoteOID != observation.SealedBase {
			return StateMismatched, node, fmt.Sprintf(
				"the linked branch is sealed at %s but refs/heads/%s reads %s", observation.SealedBase, observation.RequestedBranch, remoteDescription(observation.RemoteOID))
		}
		return StateHealthy, node, ""
	}

	orphans := make([]Node, 0, len(observation.Nodes))
	for _, node := range observation.Nodes {
		if node.RefNull() {
			orphans = append(orphans, node)
		}
	}
	if len(orphans) == 0 {
		if len(observation.Nodes) == 0 {
			return StateAbsent, Node{}, ""
		}
		// 이름 있는 링크만 있고 그중 우리 것이 없다. 남의 링크다.
		return StateMismatched, Node{}, fmt.Sprintf(
			"the issue has %d linked branches but none is named %s", len(observation.Nodes), observation.RequestedBranch)
	}
	if len(orphans) > 1 {
		return StateAmbiguous, Node{}, fmt.Sprintf(
			"%d linked branches have a null ref; a null ref carries no name, so none of them can be attributed to this lifecycle", len(orphans))
	}
	if strings.TrimSpace(observation.RemoteOID) != "" {
		// ref-null 레코드와 실제 원격 브랜치가 같이 보인다. 링크가 뒤늦게
		// 수렴하는 중일 수도, 브랜치가 외부에서 만들어진 것일 수도 있다.
		// 어느 쪽이든 지금 지우면 살아 있는 링크를 지울 위험이 있다.
		return StateAmbiguous, Node{}, fmt.Sprintf(
			"a null-ref linked branch coexists with refs/heads/%s at %s; the link may still be converging", observation.RequestedBranch, observation.RemoteOID)
	}
	return StateOrphan, orphans[0], ""
}

// Deletable은 이 상태에서 typed 삭제가 허용되는지 보고한다.
func Deletable(state State) bool { return state == StateOrphan }

func remoteDescription(oid string) string {
	if strings.TrimSpace(oid) == "" {
		return "nothing"
	}
	return oid
}
