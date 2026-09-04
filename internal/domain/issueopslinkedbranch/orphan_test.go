package issueopslinkedbranch

import (
	"strings"
	"testing"
)

const (
	sealedBase  = "5480568a4178d5ea46d5486b97d0ff5223f1c24c"
	wantedName  = "304-completion-reseed-stale-receipt"
	orphanNode  = "LB_kwDPAAAAAS0v3kvOAOdU6g"
	healthyNode = "LB_kwDPAAAAAS0v3kvOAOdU6h"
)

func base(nodes ...Node) Observation {
	return Observation{
		IssueURL:        "https://github.com/m16khb/issueops/issues/304",
		RequestedBranch: wantedName, SealedBase: sealedBase,
		TotalCount: len(nodes), Nodes: nodes,
	}
}

// TestClassifyNamesTheOrphanFromTheNullRef는 #306의 실측 상태를 고정한다.
//
// #304에서 createLinkedBranch가 오류 없이 linkedBranch=null을 돌려줬고,
// 이슈에는 ref=null인 레코드 하나만 남았으며 refs/heads/<branch>는 없었다.
// 이것이 유일하게 지울 수 있는 상태다.
func TestClassifyNamesTheOrphanFromTheNullRef(t *testing.T) {
	state, node, reason := Classify(base(Node{ID: orphanNode}))
	if state != StateOrphan || node.ID != orphanNode || reason != "" {
		t.Fatalf("state=%q node=%q reason=%q", state, node.ID, reason)
	}
	if !Deletable(state) {
		t.Fatal("ref-null 고아만이 typed 삭제 대상이다")
	}
}

// TestClassifyRefusesToGuessAmongSeveralNullRefs는 이 규칙의 핵심 안전
// 근거를 고정한다. ref가 없는 레코드는 **이름이 없다**. 후보가 둘이면 어느
// 것이 이 lifecycle의 것인지 구분할 방법이 원리적으로 없으므로 지우지 않는다.
func TestClassifyRefusesToGuessAmongSeveralNullRefs(t *testing.T) {
	state, node, reason := Classify(base(Node{ID: orphanNode}, Node{ID: "LB_second"}))
	if state != StateAmbiguous || node.ID != "" {
		t.Fatalf("state=%q node=%q", state, node.ID)
	}
	if !strings.Contains(reason, "carries no name") {
		t.Fatalf("왜 지목할 수 없는지 밝혀야 한다: %q", reason)
	}
	if Deletable(state) {
		t.Fatal("모호한 상태는 삭제 대상이 아니다")
	}
}

// TestClassifyTreatsATruncatedReadbackAsAmbiguous는 잘린 관측을 부재로
// 읽지 않음을 고정한다. `first:10`을 넘는 이슈에서 조용히 오판하면
// 보이지 않는 링크를 남긴 채 "정리 완료"를 보고하게 된다.
func TestClassifyTreatsATruncatedReadbackAsAmbiguous(t *testing.T) {
	observation := base(Node{ID: orphanNode})
	observation.TotalCount = 11
	state, _, reason := Classify(observation)
	if state != StateAmbiguous || !strings.Contains(reason, "truncated") {
		t.Fatalf("state=%q reason=%q", state, reason)
	}
}

// TestClassifyRefusesWhenARemoteRefCoexists는 수렴 중인 링크를 고아로
// 오판하지 않음을 고정한다. ref-null 레코드와 실제 브랜치가 같이 보이면
// 지금 지우는 것은 살아 있는 링크를 지울 위험이 있다.
func TestClassifyRefusesWhenARemoteRefCoexists(t *testing.T) {
	observation := base(Node{ID: orphanNode})
	observation.RemoteOID = sealedBase
	state, _, reason := Classify(observation)
	if state != StateAmbiguous || !strings.Contains(reason, "still be converging") {
		t.Fatalf("state=%q reason=%q", state, reason)
	}
}

// TestClassifyAcceptsOnlyAFullyMatchingLink는 정상 상태의 정의를 고정한다.
// 이름·링크 OID·원격 OID가 모두 봉인값과 같아야 한다.
func TestClassifyAcceptsOnlyAFullyMatchingLink(t *testing.T) {
	observation := base(Node{ID: healthyNode, RefName: wantedName, RefOID: sealedBase})
	observation.RemoteOID = sealedBase
	if state, node, reason := Classify(observation); state != StateHealthy || node.ID != healthyNode || reason != "" {
		t.Fatalf("state=%q node=%q reason=%q", state, node.ID, reason)
	}
}

// TestClassifySeparatesEveryNonDeletableShape는 AC-03이 요구하는 구분을
// 고정한다. 이들이 한 가지 오류로 뭉개지면 사용자는 지울 수 없는 이유를
// 알 수 없다.
func TestClassifySeparatesEveryNonDeletableShape(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*Observation)
		wantState  State
		wantReason string
	}{
		{"레코드 없음", func(o *Observation) { o.Nodes, o.TotalCount = nil, 0 }, StateAbsent, ""},
		{"링크 OID가 봉인값과 다름", func(o *Observation) {
			o.Nodes = []Node{{ID: healthyNode, RefName: wantedName, RefOID: "deadbeef"}}
			o.RemoteOID = sealedBase
		}, StateMismatched, "sealed " + sealedBase},
		{"원격 tip이 전진함", func(o *Observation) {
			o.Nodes = []Node{{ID: healthyNode, RefName: wantedName, RefOID: sealedBase}}
			o.RemoteOID = "cafebabe"
		}, StateMismatched, "reads cafebabe"},
		{"원격 ref 부재", func(o *Observation) {
			o.Nodes = []Node{{ID: healthyNode, RefName: wantedName, RefOID: sealedBase}}
		}, StateMismatched, "reads nothing"},
		{"남의 링크만 있음", func(o *Observation) {
			o.Nodes = []Node{{ID: healthyNode, RefName: "999-someone-else", RefOID: sealedBase}}
		}, StateMismatched, "none is named " + wantedName},
		{"봉인 base 없음", func(o *Observation) { o.SealedBase = "" }, StateAmbiguous, "must seal"},
		{"요청 브랜치 없음", func(o *Observation) { o.RequestedBranch = "" }, StateAmbiguous, "must seal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observation := base(Node{ID: orphanNode})
			tc.mutate(&observation)
			observation.TotalCount = len(observation.Nodes)
			state, _, reason := Classify(observation)
			if state != tc.wantState {
				t.Fatalf("state = %q, want %q (reason %q)", state, tc.wantState, reason)
			}
			if !strings.Contains(reason, tc.wantReason) {
				t.Fatalf("reason %q에 %q가 있어야 한다", reason, tc.wantReason)
			}
			if Deletable(state) {
				t.Fatalf("%q는 삭제 대상이 아니다", state)
			}
		})
	}
}
