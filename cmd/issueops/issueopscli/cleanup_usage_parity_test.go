package issueopscli

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"issueops/internal/domain/commandparse"
)

// cleanupSubcommandsInUsageText는 usage 문자열에서 `issueops
// cleanup <sub>` 라인의 sub 집합을 파생한다. 목록을 테스트에 하드코딩하지
// 않는 것이 핵심이다 — canonical usage가 유일한 출처이고, 하위 usage와 실제
// 디스패치는 거기서 파생해 검증한다(#107, #93 계보의 파생 원칙).
func cleanupSubcommandsInUsageText(usage string) []string {
	const marker = "issueops cleanup "
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

// #116 파생 원칙 확장: canonical usage에 문서화된 cleanup 서브커맨드는 전부
// commandparse spec에 등록되어 있어야 한다. spec 등록 누락은 어떤 기존 테스트도
// 실패시키지 않고 CI green으로 통과하던 사각지대였다.
func TestIssueOpsCleanupDocumentedSubcommandsHaveCommandParseSpec(t *testing.T) {
	subs := cleanupSubcommandsInUsageText(issueOpsUsageText())
	if len(subs) == 0 {
		t.Fatalf("canonical usage must document issueops cleanup subcommands:\n%s", issueOpsUsageText())
	}
	for _, sub := range subs {
		path := "cleanup " + sub
		if _, _, _, ok := commandparse.IssueOpsCommandSpec(path); !ok {
			t.Fatalf("documented subcommand %q has no commandparse spec", path)
		}
	}
}

func TestIssueOpsCleanupUsageDocumentsRemoteBranchDeletion(t *testing.T) {
	if !slices.Contains(cleanupSubcommandsInUsageText(issueOpsUsageText()), "remote-branch") {
		t.Fatalf("canonical usage must document cleanup remote-branch:\n%s", issueOpsUsageText())
	}
}

func TestIssueOpsCleanupDocumentedSubcommandsDispatch(t *testing.T) {
	const unknownSubcommand = "unknown issueops cleanup subcommand"
	for _, sub := range cleanupSubcommandsInUsageText(issueOpsUsageText()) {
		t.Run(sub, func(t *testing.T) {
			t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
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
