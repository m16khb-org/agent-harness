package issueopscli

import (
	"strings"
	"testing"

	cliadapter "agent-harness/internal/adapter/cli"
)

// issueops 서브커맨드 usage와 최상위 adapter usage는 같은 명령 표면을
// 서술한다. 최상위 usage는 의도적으로 축약된 카탈로그이므로, 양쪽에 모두
// 존재하는 명령의 라인은 정확히 일치해야 한다. golden은 adapter usage만
// 덮기 때문에 이 테스트가 없으면 issueops 쪽 usage의 drift가 감지되지 않는다.
func TestIssueOpsUsageMatchesAdapterUsage(t *testing.T) {
	commandKey := func(line string) string {
		fields := strings.Fields(line)
		limit := min(len(fields), 4)
		for i, field := range fields[:limit] {
			if strings.HasPrefix(field, "-") {
				limit = i
				break
			}
		}
		return strings.Join(fields[:limit], " ")
	}
	adapterByKey := map[string]string{}
	for _, line := range strings.Split(cliadapter.Usage("test"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "agent-harness issueops ") {
			adapterByKey[commandKey(trimmed)] = trimmed
		}
	}
	if len(adapterByKey) == 0 {
		t.Fatal("adapter usage exposes no issueops lines; parity test inputs are broken")
	}
	shared := 0
	for _, line := range strings.Split(issueOpsUsageText(), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "agent-harness issueops ") {
			continue
		}
		adapterLine, ok := adapterByKey[commandKey(trimmed)]
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
