package issueopscli

import (
	"sort"
	"strings"
	"testing"
)

// cleanupSubcommandsInUsageText는 usage 문자열에서 `agent-harness issueops
// cleanup <sub>` 라인의 sub 집합을 파생한다. 목록을 테스트에 하드코딩하지
// 않는 것이 핵심이다 — canonical usage가 유일한 출처이고, 하위 usage와 실제
// 디스패치는 거기서 파생해 검증한다(#107, #93 계보의 파생 원칙).
func cleanupSubcommandsInUsageText(usage string) []string {
	const marker = "agent-harness issueops cleanup "
	seen := map[string]struct{}{}
	for _, line := range strings.Split(usage, "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		fields := strings.Fields(line[index+len(marker):])
		if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
			continue
		}
		seen[fields[0]] = struct{}{}
	}
	subs := make([]string, 0, len(seen))
	for sub := range seen {
		subs = append(subs, sub)
	}
	sort.Strings(subs)
	return subs
}

func TestIssueOpsCleanupHelpListsEveryCanonicalCleanupSubcommand(t *testing.T) {
	canonical := cleanupSubcommandsInUsageText(issueOpsUsageText())
	if len(canonical) == 0 {
		t.Fatalf("canonical usage must document issueops cleanup subcommands:\n%s", issueOpsUsageText())
	}
	help := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"cleanup", "--help"})
	})
	local := cleanupSubcommandsInUsageText(help)
	if strings.Join(canonical, ",") != strings.Join(local, ",") {
		t.Fatalf("cleanup usage diverged from canonical usage\ncanonical: %v\ncleanup --help: %v\n--help output:\n%s", canonical, local, help)
	}
}

func TestIssueOpsCleanupDocumentedSubcommandsDispatch(t *testing.T) {
	const unknownSubcommand = "unknown issueops cleanup subcommand"
	for _, sub := range cleanupSubcommandsInUsageText(issueOpsUsageText()) {
		t.Run(sub, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			var runErr error
			_ = captureStdoutForContract(t, func() error {
				runErr = runIssueOps([]string{"cleanup", sub})
				return nil
			})
			// 플래그 부족·record 부재로 인한 오류는 정상이다. 문서화된
			// subcommand가 레지스트리에 없을 때만 나오는 sentinel만 실패다.
			if runErr != nil && strings.Contains(runErr.Error(), unknownSubcommand) {
				t.Fatalf("cleanup %s is documented but not dispatched: %v", sub, runErr)
			}
		})
	}
}
