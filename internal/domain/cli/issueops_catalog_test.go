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

func TestTopLevelUsageRendersExactlyTheLifecycleCatalog(t *testing.T) {
	rendered := map[string]bool{}
	for _, raw := range strings.Split(Usage("test"), "\n") {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, "issueops ") {
			continue
		}
		if key := IssueOpsUsageKey("  " + trimmed); key != "" {
			rendered[key] = true
		}
	}
	want := map[string]bool{}
	for _, line := range IssueOpsUsageLines() {
		want[IssueOpsUsageKey(line)] = true
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
