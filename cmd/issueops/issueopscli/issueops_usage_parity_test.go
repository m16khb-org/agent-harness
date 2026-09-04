package issueopscli

import (
	"sort"
	"strings"
	"testing"

	cliadapter "issueops/internal/domain/cli"
)

// usageCommandKey는 usage 라인에서 플래그 직전까지의 명령 경로를 `issueops
// issueops X` 형태로 돌려준다. 파싱 규칙은 카탈로그가 소유하며(#188) 여기서는
// 접두를 붙여 기존 비교 형태를 유지한다 — 규칙을 두 번 적으면 그 둘이 어긋난다.
func usageCommandKey(line string) string {
	key := cliadapter.IssueOpsUsageKey(line)
	if key == "" {
		return ""
	}
	return "issueops " + key
}

// issueops 서브커맨드 usage와 최상위 adapter usage는 같은 명령 표면을
// 서술한다. 최상위 usage는 의도적으로 축약된 카탈로그이므로, 양쪽에 모두
// 존재하는 명령의 라인은 정확히 일치해야 한다. golden은 adapter usage만
// 덮기 때문에 이 테스트가 없으면 issueops 쪽 usage의 drift가 감지되지 않는다.
func TestIssueOpsUsageMatchesAdapterUsage(t *testing.T) {
	adapterByKey := map[string]string{}
	for _, line := range strings.Split(cliadapter.Usage("test"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "issueops ") {
			adapterByKey[usageCommandKey(trimmed)] = trimmed
		}
	}
	if len(adapterByKey) == 0 {
		t.Fatal("adapter usage exposes no issueops lines; parity test inputs are broken")
	}
	shared := 0
	for _, line := range strings.Split(issueOpsUsageText(), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "issueops ") {
			continue
		}
		adapterLine, ok := adapterByKey[usageCommandKey(trimmed)]
		if !ok {
			continue
		}
		shared++
		if adapterLine != trimmed {
			t.Errorf("issueops usage line diverges from adapter usage:\nissueops: %s\nadapter : %s", trimmed, adapterLine)
		}
	}
	if shared == 0 {
		t.Fatal("no shared issueops command lines between the two usage texts")
	}
}

// 위 공존-라인 검사는 한쪽에만 있는 명령을 무음 skip한다. adapter usage가
// 노출하는 issueops 명령이 issueops 서브커맨드 usage에서 통째로 빠지면
// 사용자는 `issueops --help`에서 그 명령을 볼 수 없다. 이 방향 검사가 그
// 구멍을 막는다(#111 — devils-advocate 누락이 #93 전까지 생존한 경로). 반대
// 방향(issueops usage superset)은 축약 카탈로그 계약상 허용이므로 강제하지
// 않는다.
func TestAdapterIssueOpsUsageCommandsExistInIssueOpsUsage(t *testing.T) {
	const prefix = "issueops "
	issueOpsKeys := map[string]bool{}
	for _, line := range strings.Split(issueOpsUsageText(), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		issueOpsKeys[usageCommandKey(trimmed)] = true
	}
	if len(issueOpsKeys) == 0 {
		t.Fatal("issueops usage exposes no issueops lines; parity test inputs are broken")
	}
	var missing []string
	seen := map[string]bool{}
	for _, line := range strings.Split(cliadapter.Usage("test"), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		if cliadapter.IssueOpsUsageKey(trimmed) == "" {
			continue
		}
		key := usageCommandKey(trimmed)
		if issueOpsKeys[key] || seen[key] {
			continue
		}
		seen[key] = true
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("adapter usage exposes issueops commands missing from issueOpsUsageText(): %s", strings.Join(missing, ", "))
	}
}
