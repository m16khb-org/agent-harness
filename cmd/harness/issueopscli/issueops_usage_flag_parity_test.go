package issueopscli

import (
	"sort"
	"strings"
	"testing"

	cliadapter "agent-harness/internal/adapter/cli"
	"agent-harness/internal/testsupport"
)

// usage 문자열은 운영자와 에이전트가 명령을 만드는 **첫 번째 근거**다. 그것이
// 실제 플래그 등록과 어긋나면 만들어진 명령이 거부되고, 거부 사유는 형태 문제만
// 말하므로 어느 플래그가 잘못인지 알 수 없다(#184). 이 파일은 그 어긋남을
// 기계적으로 막는다.
//
// usage는 두 축약을 쓴다. 잘못된 축약을 쓰면 legend를 따른 운영자가
// `flag provided but not defined`를 만난다.
//
//   - `ACTOR_FLAGS`(7종): execution lease 전이와 generation-fenced 발행. live
//     session process까지 검증하므로 receipt 3종이 더 붙는다.
//   - `RECORD_ACTOR_FLAGS`(4종): durable record mutation.
var actorFlagShorthands = map[string][]string{
	"ACTOR_FLAGS": {
		"host", "session-id", "agent-id",
		"session-pid", "session-started-at", "session-executable", "cwd",
	},
	"RECORD_ACTOR_FLAGS": {"host", "session-id", "agent-id", "cwd"},
}

// 이 두 플래그가 등록돼 있으면 그 명령은 native actor를 요구한다. usage가
// 그것을 밝히지 않으면 운영자는 actor 없이 실행하고 lease 소유자 거부를 만난다.
var actorFlagTell = []string{"host", "session-id"}

// usageActorShorthand는 라인이 쓰는 축약 이름을 돌려준다. `RECORD_ACTOR_FLAGS`가
// `ACTOR_FLAGS`를 부분 문자열로 포함하므로 긴 쪽을 먼저 본다.
func usageActorShorthand(line string) string {
	if strings.Contains(line, "RECORD_ACTOR_FLAGS") {
		return "RECORD_ACTOR_FLAGS"
	}
	if strings.Contains(line, "ACTOR_FLAGS") {
		return "ACTOR_FLAGS"
	}
	return ""
}

type usageFlagLine struct {
	command   string   // "execution prepare" 형태. `agent-harness issueops ` 접두는 제거한다
	args      []string // 핸들러에 넘길 인자
	declared  []string // usage가 언급한 플래그 이름(하이픈 없음)
	shorthand string   // 쓰는 actor 축약 이름. 없으면 빈 문자열
}

func parseUsageFlagLines(t *testing.T, usage string) []usageFlagLine {
	t.Helper()
	const prefix = "agent-harness issueops "
	var lines []usageFlagLine
	for _, raw := range strings.Split(usage, "\n") {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		key := strings.TrimPrefix(usageCommandKey(trimmed), prefix)
		if key == "" {
			continue
		}
		line := usageFlagLine{
			command:   key,
			args:      strings.Fields(key),
			shorthand: usageActorShorthand(trimmed),
		}
		seen := map[string]bool{}
		for _, field := range strings.Fields(trimmed) {
			name := usageFlagName(field)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			line.declared = append(line.declared, name)
		}
		sort.Strings(line.declared)
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		t.Fatal("usage exposes no issueops command lines; parity test inputs are broken")
	}
	return lines
}

// usageFlagName은 usage 토큰에서 플래그 이름만 뽑는다. 대괄호·괄호·파이프·쉼표
// 같은 표기 문자를 벗기므로 문장 구조에 의존하지 않는다.
func usageFlagName(field string) string {
	field = strings.Trim(field, "[]()|,.")
	if !strings.HasPrefix(field, "--") {
		return ""
	}
	name := strings.TrimPrefix(field, "--")
	if index := strings.IndexAny(name, "=|]) "); index >= 0 {
		name = name[:index]
	}
	name = strings.Trim(name, "[]()|,.")
	if name == "" || name == "help" {
		return ""
	}
	return name
}

// registeredFlags는 명령을 --help로 실행해 FlagSet이 인쇄하는 등록 플래그를
// 모은다. flag 등록이 함수 안에 있고 addIssueOpsActorFlags 같은 헬퍼를 거치므로
// 정적 파싱은 놓친다. --help는 실제 FlagSet을 그대로 보여준다.
//
// parseIssueOpsFlags가 flag.ErrHelp를 파싱 직후 반환하므로 --help 호출에
// 부작용이 없다. 그 규율이 깨지면 이 테스트가 먼저 알려준다.
func registeredFlags(t *testing.T, args []string) map[string]bool {
	t.Helper()
	output := testsupport.CaptureStderr(t, func() error {
		_ = runIssueOps(append(append([]string{}, args...), "--help"))
		return nil
	})
	flags := map[string]bool{}
	for _, raw := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "--") {
			continue
		}
		name := strings.TrimPrefix(trimmed, "-")
		if index := strings.IndexAny(name, " \t"); index >= 0 {
			name = name[:index]
		}
		if name == "" || name == "help" {
			continue
		}
		flags[name] = true
	}
	return flags
}

// ① usage가 언급한 플래그는 모두 등록돼 있어야 한다. 유령 플래그는 운영자를
// `flag provided but not defined`로 보낸다.
//
// 자기 FlagSet 대신 직접 만든 usage 문자열을 인쇄하는 명령(design review,
// regress, feedback resolve 등)은 등록 플래그를 수집할 수 없어 건너뛴다. 그
// 목록을 로그로 남겨 무음 축소가 되지 않게 한다.
func TestUsageDeclaredFlagsAreRegistered(t *testing.T) {
	var skipped []string
	for _, line := range parseUsageFlagLines(t, issueOpsUsageText()) {
		registered := registeredFlags(t, line.args)
		if len(registered) == 0 {
			skipped = append(skipped, line.command)
			continue
		}
		var phantom []string
		for _, name := range line.declared {
			if !registered[name] {
				phantom = append(phantom, "--"+name)
			}
		}
		if len(phantom) > 0 {
			t.Errorf("usage for %q names flags that are not registered: %s", line.command, strings.Join(phantom, ", "))
		}
	}
	if len(skipped) > 0 {
		sort.Strings(skipped)
		t.Logf("flag registration not observable for %d commands (they print a hand-written usage string): %s",
			len(skipped), strings.Join(skipped, ", "))
	}
}

// ② 축약을 쓴 명령은 그 축약이 뜻하는 플래그를 모두 등록해야 한다. 그러지 않으면
// legend를 따른 운영자가 미등록 플래그로 거부된다 — verify-artifact가 7종 축약을
// 쓰면서 4종만 받아 그랬다(#184).
func TestActorFlagShorthandMatchesRegisteredFlags(t *testing.T) {
	checkedByShorthand := map[string]int{}
	for _, line := range parseUsageFlagLines(t, issueOpsUsageText()) {
		if line.shorthand == "" {
			continue
		}
		registered := registeredFlags(t, line.args)
		if len(registered) == 0 {
			continue
		}
		checkedByShorthand[line.shorthand]++
		var absent []string
		for _, name := range actorFlagShorthands[line.shorthand] {
			if !registered[name] {
				absent = append(absent, "--"+name)
			}
		}
		if len(absent) > 0 {
			t.Errorf("usage for %q uses %s but these are not registered: %s\n"+
				"use the shorthand that matches, or list the flags it actually accepts",
				line.command, line.shorthand, strings.Join(absent, ", "))
		}
	}
	for shorthand := range actorFlagShorthands {
		if checkedByShorthand[shorthand] == 0 {
			t.Fatalf("no usage line using %s was checked; parity test inputs are broken", shorthand)
		}
	}
}

// ③ actor를 요구하는 명령은 usage가 그것을 밝혀야 한다. link-plan은 lease
// 소유자 mutation인데 usage가 --id와 --plan-path만 보여줬다.
func TestCommandsRequiringActorDiscloseItInUsage(t *testing.T) {
	checked := 0
	for _, line := range parseUsageFlagLines(t, issueOpsUsageText()) {
		registered := registeredFlags(t, line.args)
		if len(registered) == 0 {
			continue
		}
		requiresActor := true
		for _, name := range actorFlagTell {
			if !registered[name] {
				requiresActor = false
				break
			}
		}
		if !requiresActor {
			continue
		}
		checked++
		if line.shorthand != "" {
			continue
		}
		declared := map[string]bool{}
		for _, name := range line.declared {
			declared[name] = true
		}
		var hidden []string
		for _, name := range actorFlagTell {
			if !declared[name] {
				hidden = append(hidden, "--"+name)
			}
		}
		if len(hidden) > 0 {
			t.Errorf("usage for %q hides actor flags it requires: %s\n"+
				"without them the command fails with a write-lease-holder error and usage gives no hint",
				line.command, strings.Join(hidden, ", "))
		}
	}
	if checked == 0 {
		t.Fatal("no actor-requiring command was checked; parity test inputs are broken")
	}
}

// ④ ACTOR_FLAGS를 쓰는 usage 출력은 같은 출력 안에서 그것을 정의해야 한다.
// legend가 다른 명령의 help에만 있으면 `issueops --help`를 읽는 운영자는 그
// 토큰의 확장을 볼 수 없다.
func TestUsageTextsDefineActorFlagShorthand(t *testing.T) {
	for name, usage := range map[string]string{
		"issueOpsUsageText": issueOpsUsageText(),
		"adapter usage":     cliadapter.Usage("test"),
	} {
		for shorthand, flagNames := range actorFlagShorthands {
			if !strings.Contains(usage, shorthand) {
				continue
			}
			legend := legendLine(usage, shorthand)
			if legend == "" {
				t.Errorf("%s uses %s without defining it in the same output", name, shorthand)
				continue
			}
			for _, flagName := range flagNames {
				if !strings.Contains(legend, "--"+flagName) {
					t.Errorf("%s %s legend omits --%s: %s", name, shorthand, flagName, legend)
				}
			}
		}
	}
}

// legendLine은 `SHORTHAND: ...` 정의 줄을 돌려준다. 정의 줄 안에서 플래그를 확인하지
// 않으면 usage 다른 곳에 우연히 있는 플래그가 legend를 통과시킨다.
func legendLine(usage, shorthand string) string {
	for _, raw := range strings.Split(usage, "\n") {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, shorthand+":") {
			return trimmed
		}
	}
	return ""
}

// ⑤ execution 하위 명령은 sub-subcommand라 dispatch registry 검사가 덮지 않는다.
// switch-mode(#167)가 두 usage 텍스트에서 모두 빠진 채 살아남은 경로다 — #181이
// 문서 열거를 고쳤을 때도 CLI는 남았다.
func TestExecutionSubcommandsAppearInUsageTexts(t *testing.T) {
	want := []string{
		"prepare", "status", "whoami", "claim", "release",
		"replace", "reconcile", "complete", "sync-base", "switch-mode",
	}
	for name, usage := range map[string]string{
		"issueOpsUsageText": issueOpsUsageText(),
		"adapter usage":     cliadapter.Usage("test"),
	} {
		var missing []string
		for _, sub := range want {
			if !strings.Contains(usage, "issueops execution "+sub+" ") {
				missing = append(missing, sub)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s omits execution subcommands: %s", name, strings.Join(missing, ", "))
		}
	}
}
