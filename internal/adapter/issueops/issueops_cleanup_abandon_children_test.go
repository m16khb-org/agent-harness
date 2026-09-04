package issueops

import (
	"testing"

	issueops "issueops/internal/contract/issueops"
)

func childRefs(ids ...string) []issueops.IssueOpsChildCycleRef {
	refs := make([]issueops.IssueOpsChildCycleRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, issueops.IssueOpsChildCycleRef{
			CycleID: id, Branch: id + "-work",
			ChildIssueURL: "https://github.com/o/r/issues/1", CreatedAt: "2026-08-01T00:00:00Z",
		})
	}
	return refs
}

// TestAbandonChildGateCountsOnlyUnresolvedChildren는 #437을 고정한다.
//
// 게이트의 목적은 "자식 고아 방지"다. 자식이 모두 해소됐으면 부모를 폐기해도
// 고아가 생기지 않는다. 그런데 ChildCycles 경로는 해소 여부를 보지 않고
// **기록이 있으면** 무조건 차단했다 — 같은 함수의 IssueLinks 경로는
// CloseVerifiedAt을 보는데도.
//
// 그 결과 epic을 완주할수록 그 기록이 영구히 남았다. 일을 끝낼수록 정리가
// 어려워지면 계약이 뒤집힌 것이다.
//
// 실측: io-c26802f00c2b(#228)는 자식 23개가 **전부 CLOSED**인데도
// no_children으로 막혔고, finish는 epic 자신의 artifact가 없어 도달 불가였다.
func TestAbandonChildGateCountsOnlyUnresolvedChildren(t *testing.T) {
	resolved := func(ids ...string) map[string]bool {
		set := map[string]bool{}
		for _, id := range ids {
			set[id] = true
		}
		return set
	}

	t.Run("모두 해소되면 통과", func(t *testing.T) {
		record := issueops.IssueOpsRecord{ChildCycles: childRefs("io-a", "io-b")}
		if cleanupAbandonUnresolvedChildren(record, resolved("io-a", "io-b")) != nil {
			t.Fatal("해소된 자식만 있으면 차단하지 않는다")
		}
	})

	t.Run("하나라도 남으면 차단하고 지목한다", func(t *testing.T) {
		record := issueops.IssueOpsRecord{ChildCycles: childRefs("io-a", "io-b", "io-c")}
		unresolved := cleanupAbandonUnresolvedChildren(record, resolved("io-a"))
		if len(unresolved) != 2 {
			t.Fatalf("해소되지 않은 자식만 세야 한다: %v", unresolved)
		}
		// 어느 자식인지 알 수 없으면 사용자는 무엇을 먼저 끝내야 할지 모른다.
		for _, want := range []string{"io-b", "io-c"} {
			if !containsString(unresolved, want) {
				t.Fatalf("%s가 진단에 있어야 한다: %v", want, unresolved)
			}
		}
	})

	t.Run("IssueLinks 경로는 기존 기준을 유지한다", func(t *testing.T) {
		record := issueops.IssueOpsRecord{
			IssueURL: "https://github.com/o/r/issues/9",
			IssueLinks: []issueops.IssueOpsIssueLink{
				{Type: "child", URL: "https://github.com/o/r/issues/10", CloseVerifiedAt: "2026-08-01T00:00:00Z"},
				{Type: "child", URL: "https://github.com/o/r/issues/11"},
			},
		}
		unresolved := cleanupAbandonUnresolvedChildren(record, nil)
		if len(unresolved) != 1 || !containsString(unresolved, "https://github.com/o/r/issues/11") {
			t.Fatalf("닫히지 않은 링크만 세야 한다: %v", unresolved)
		}
	})

	t.Run("자기 자신을 가리키는 링크는 자식이 아니다", func(t *testing.T) {
		self := "https://github.com/o/r/issues/9"
		record := issueops.IssueOpsRecord{
			IssueURL:   self,
			IssueLinks: []issueops.IssueOpsIssueLink{{Type: "child", URL: self}},
		}
		if unresolved := cleanupAbandonUnresolvedChildren(record, nil); len(unresolved) != 0 {
			t.Fatalf("self link는 자식이 아니다: %v", unresolved)
		}
	})
}

// TestAbandonResolvedChildrenRefusesToInferFromAbsence는 fail-open 유혹을
// 고정한다.
//
// 처음에는 자식 record가 사라졌으면 해소로 세려 했다. 그러면 #228 같은 epic이
// 바로 풀리기 때문이다. 하지만 부재는 근거가 아니다 — 정리돼서 사라진 것인지
// 유실된 것인지 구분할 수 없고, 모르는 것을 끝난 것으로 넘기면 이 게이트가
// 막으려던 고아가 그대로 생긴다.
//
// 부재한 자식을 정리하려면 그 사실이 부모 record에 남아야 한다.
// IssueLinks의 CloseVerifiedAt이 그 자리다.
func TestAbandonResolvedChildrenRefusesToInferFromAbsence(t *testing.T) {
	stateRoot := t.TempDir()
	record := issueops.IssueOpsRecord{ChildCycles: childRefs("io-gone1", "io-gone2")}

	if resolved := cleanupAbandonResolvedChildren(stateRoot, record); len(resolved) != 0 {
		t.Fatalf("부재는 해소의 근거가 아니다: %v", resolved)
	}
	if unresolved := cleanupAbandonUnresolvedChildren(record, nil); len(unresolved) != 2 {
		t.Fatalf("근거 없는 자식은 계속 차단한다: %v", unresolved)
	}
}

// TestAbandonResolvedChildrenRequiresDoneForLiveRecords는 완화가 "부재"와
// "done"에만 적용됨을 고정한다. 살아 있는 미완 자식은 계속 차단해야 한다.
func TestAbandonResolvedChildrenRequiresDoneForLiveRecords(t *testing.T) {
	stateRoot := t.TempDir()
	for id, phase := range map[string]issueops.IssueOpsPhase{
		"io-live1": IssueOpsPhaseImplement,
		"io-live2": IssueOpsPhaseDone,
	} {
		child := issueops.IssueOpsRecord{
			OK: true, SchemaVersion: 1, ID: id, Repo: t.TempDir(), Phase: phase,
			CreatedAt: "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T00:00:00Z",
		}
		if _, err := WriteIssueOps(stateRoot, child); err != nil {
			t.Fatal(err)
		}
	}
	record := issueops.IssueOpsRecord{ChildCycles: childRefs("io-live1", "io-live2")}
	resolved := cleanupAbandonResolvedChildren(stateRoot, record)
	if resolved["io-live1"] {
		t.Fatal("implement 단계의 자식은 해소가 아니다")
	}
	if !resolved["io-live2"] {
		t.Fatal("done 자식은 해소다")
	}
	unresolved := cleanupAbandonUnresolvedChildren(record, resolved)
	if len(unresolved) != 1 || unresolved[0] != "io-live1" {
		t.Fatalf("미완 자식만 지목해야 한다: %v", unresolved)
	}
}
