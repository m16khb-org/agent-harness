package issueopscli

import (
	"strings"
	"testing"

	cliadapter "issueops/internal/domain/cli"
)

func TestIssueOpsChildUsageMatchesCanonicalCatalog(t *testing.T) {
	counts := map[string]int{
		"child start":  0,
		"child status": 0,
		"child list":   0,
		"child accept": 0,
		"child reject": 0,
		"child drop":   0,
	}
	var lines []string
	for _, line := range cliadapter.IssueOpsUsageLines() {
		key := cliadapter.IssueOpsUsageKey(line)
		if _, selected := counts[key]; !selected {
			continue
		}
		counts[key]++
		lines = append(lines, line)
	}
	for key, count := range counts {
		if count != 1 {
			t.Fatalf("canonical catalog에서 %q 줄을 정확히 하나 기대했지만 %d개다", key, count)
		}
	}
	want := "Usage:\n" + strings.Join(lines, "\n") + "\n\n" +
		cliadapter.IssueOpsActorFlagLegend
	got := strings.TrimSuffix(captureStdoutForContract(t, func() error {
		return runIssueOpsChild([]string{"--help"})
	}), "\n")
	if got != want {
		t.Fatalf("child help가 canonical catalog projection과 다르다\nwant:\n%s\n\ngot:\n%s", want, got)
	}
	for _, line := range lines {
		if !strings.Contains(line, " RECORD_ACTOR_FLAGS ") {
			t.Fatalf("child usage 줄이 RECORD_ACTOR_FLAGS를 숨긴다: %s", line)
		}
	}
	if legendLine(got, "RECORD_ACTOR_FLAGS") == "" {
		t.Fatal("child help가 RECORD_ACTOR_FLAGS 범례를 정의하지 않는다")
	}
	if strings.Contains(got, "\n  issueops link-child ") {
		t.Fatal("child help에 비-child link-child 명령이 포함됐다")
	}
}
