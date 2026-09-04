package issueopscli

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	cliadapter "issueops/internal/domain/cli"
)

// issueOpsNamespaceHelpText는 `issueops <namespace> --help`가 실제로 출력하는
// 문자열을 돌려준다. 하위 usage는 각 네임스페이스 패키지가 자기 문자열을
// 소유하므로, 카탈로그와의 정합성은 렌더된 출력을 읽어야만 검사할 수 있다.
func issueOpsNamespaceHelpText(t *testing.T, namespace string) string {
	t.Helper()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{namespace, "--help"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops %s --help: %v", namespace, runErr)
	}
	return out
}

var issueOpsUsageFlagPattern = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

// issueOpsActorFlagNames는 RECORD_ACTOR_FLAGS/ACTOR_FLAGS 축약이 대신하는
// 플래그다. 카탈로그는 축약을 쓰고 네임스페이스 usage는 펼쳐 쓰므로, 이
// 차이는 drift가 아니라 의도된 표기 차이다.
var issueOpsActorFlagNames = map[string]bool{
	"--host": true, "--session-id": true, "--agent-id": true, "--cwd": true,
	"--session-pid": true, "--session-started-at": true, "--session-executable": true,
}

func issueOpsUsageFlagSet(line string) map[string]bool {
	flags := map[string]bool{}
	for _, flag := range issueOpsUsageFlagPattern.FindAllString(line, -1) {
		if !issueOpsActorFlagNames[flag] {
			flags[flag] = true
		}
	}
	return flags
}

// TestIssueOpsNamespaceUsageExistsInCatalog는 하위 네임스페이스 명령과 플래그가
// 정본 카탈로그에서 빠지는 구멍을 막는다.
//
// 기존 parity 검사는 `strings.Fields(...)[0]`, 즉 최상위 토큰만 비교한다.
// `remote`가 카탈로그에 있으면 `remote reconcile-issue`와 `remote sync-graph`가
// 통째로 빠져 있어도 통과했다. 그 결과 두 명령은 `issueops remote --help`에만
// 존재하고 `issueops --help`에서는 보이지 않았다. 스킬 문서는 모호한
// create-issue 결과의 복구 수단으로 `remote reconcile-issue`를 지시하면서
// 명령 존재의 근거를 usage 카탈로그로 못박으므로, 이 구멍은 문맥 없는 새
// 세션의 복구 경로를 통째로 지운다(#111이 최상위에서 막은 것과 같은 결함).
//
// 플래그 방향도 같이 본다. `execution prepare`의 preview가 next_command로
// 내보내는 `--expected-readiness-fingerprint`는 confirm에 필수인데 카탈로그에
// 없었다. 축약이 대신하는 actor 플래그는 비교에서 제외한다.
func TestIssueOpsNamespaceUsageExistsInCatalog(t *testing.T) {
	const prefix = "issueops "
	catalog := map[string]string{}
	for _, line := range cliadapter.IssueOpsUsageLines() {
		trimmed := strings.TrimSpace(line)
		catalog[cliadapter.IssueOpsUsageKey(trimmed)] = trimmed
	}
	if len(catalog) == 0 {
		t.Fatal("canonical catalog exposes no issueops lines; parity test inputs are broken")
	}

	var missingCommands, missingFlags []string
	for _, namespace := range []string{"remote", "execution", "cleanup", "child", "branch", "artifact", "implementation-review"} {
		for _, line := range strings.Split(issueOpsNamespaceHelpText(t, namespace), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, prefix) {
				continue
			}
			key := cliadapter.IssueOpsUsageKey(trimmed)
			if key == "" {
				continue
			}
			catalogLine, ok := catalog[key]
			if !ok {
				missingCommands = append(missingCommands, key)
				continue
			}
			catalogFlags := issueOpsUsageFlagSet(catalogLine)
			for flag := range issueOpsUsageFlagSet(trimmed) {
				if !catalogFlags[flag] {
					missingFlags = append(missingFlags, key+" "+flag)
				}
			}
		}
	}
	sort.Strings(missingCommands)
	sort.Strings(missingFlags)
	for _, key := range missingCommands {
		t.Errorf("namespace usage command %q is missing from the canonical catalog", key)
	}
	for _, pair := range missingFlags {
		t.Errorf("namespace usage flag %q is missing from the canonical catalog line", pair)
	}
}
