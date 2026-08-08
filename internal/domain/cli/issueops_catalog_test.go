package cli

import (
	"sort"
	"strings"
	"testing"
)

// `issueops` 명령 목록은 저장소에 **한 번만** 존재해야 한다. 두 곳에 손으로
// 유지하면 한쪽 누락은 parity 테스트가 잡지만 **양쪽에 아예 없으면** 검사할 대상이
// 없다. `execution switch-mode`(#167)가 그 구멍으로 살아남았고 `#184`가 손으로 찾아
// 넣을 때까지 두 텍스트 모두에 없었다(#188).
//
// 카탈로그가 하나가 되면 "한쪽에만 있다"는 불가능해진다. 대신 새 위험이 생긴다 —
// 축약 키가 어느 줄과도 맞지 않으면 그 명령이 최상위 help에서 조용히 사라진다.
// 아래 두 테스트가 그 위험을 막는다.

// ① 축약 키는 카탈로그 줄과 정확히 하나씩 대응해야 한다. 맞지 않는 키는 오타이고,
// 그 명령은 최상위 help에서 사라진다.
func TestAbridgedIssueOpsKeysMatchExactlyOneCatalogLine(t *testing.T) {
	catalog := IssueOpsUsageLines()
	if len(catalog) == 0 {
		t.Fatal("issueops usage catalog is empty; single-source wiring is broken")
	}
	keys := abridgedIssueOpsUsageKeys()
	if len(keys) == 0 {
		t.Fatal("abridged issueops key set is empty; top-level help would list nothing")
	}
	var unmatched, ambiguous []string
	for _, key := range keys {
		matches := 0
		for _, line := range catalog {
			if IssueOpsUsageKey(line) == key {
				matches++
			}
		}
		switch {
		case matches == 0:
			unmatched = append(unmatched, key)
		case matches > 1:
			ambiguous = append(ambiguous, key)
		}
	}
	if len(unmatched) > 0 {
		sort.Strings(unmatched)
		t.Errorf("abridged keys match no catalog line (the command silently disappears from top-level help): %s",
			strings.Join(unmatched, ", "))
	}
	if len(ambiguous) > 0 {
		sort.Strings(ambiguous)
		t.Errorf("abridged keys match more than one catalog line: %s", strings.Join(ambiguous, ", "))
	}
}

// ② 최상위 usage가 실제로 그 키들만, 그리고 그 키들을 모두 렌더해야 한다. 키 집합과
// 렌더 결과가 어긋나면 필터가 깨진 것이다.
func TestTopLevelUsageRendersExactlyTheAbridgedIssueOpsKeys(t *testing.T) {
	rendered := map[string]bool{}
	for _, raw := range strings.Split(Usage("test"), "\n") {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, "agent-harness issueops ") {
			continue
		}
		rendered[IssueOpsUsageKey("  "+trimmed)] = true
	}
	want := map[string]bool{}
	for _, key := range abridgedIssueOpsUsageKeys() {
		want[key] = true
	}
	var extra, absent []string
	for key := range rendered {
		if !want[key] {
			extra = append(extra, key)
		}
	}
	for key := range want {
		if !rendered[key] {
			absent = append(absent, key)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Errorf("top-level usage renders issueops commands outside the abridged key set: %s", strings.Join(extra, ", "))
	}
	if len(absent) > 0 {
		sort.Strings(absent)
		t.Errorf("top-level usage drops abridged issueops commands: %s", strings.Join(absent, ", "))
	}
}

// ③ 축약은 전체의 **부분집합**이다. 카탈로그에 없는 명령이 최상위에만 있으면 그것이
// 새로운 중복이다.
func TestAbridgedIssueOpsKeysAreASubsetOfTheCatalog(t *testing.T) {
	catalogKeys := map[string]bool{}
	for _, line := range IssueOpsUsageLines() {
		catalogKeys[IssueOpsUsageKey(line)] = true
	}
	for _, key := range abridgedIssueOpsUsageKeys() {
		if !catalogKeys[key] {
			t.Errorf("abridged key %q is not in the catalog", key)
		}
	}
}
