package commandparse

import (
	"strings"
	"testing"

	domaincli "agent-harness/internal/domain/cli"
)

// usage 카탈로그(internal/domain/cli)가 광고하는 플래그는 exact-parse 스펙이
// 받아들여야 한다. 카탈로그에만 있고 스펙에 없는 플래그는 hook의 exact 파싱이
// 정상 명령을 거부하는, 사용자에게 바로 보이는 파열이다(#188과 같은 계열의
// 드리프트). 반대 방향(스펙에만 있는 플래그)은 축약 카탈로그가 의도적으로
// 생략할 수 있으므로 여기서 검사하지 않는다.
func TestIssueOpsCommandSpecAcceptsEveryCatalogAdvertisedFlag(t *testing.T) {
	actorRecord := map[string]bool{"--host": true, "--session-id": true, "--agent-id": true, "--cwd": true}
	actorFull := map[string]bool{"--host": true, "--session-id": true, "--agent-id": true, "--session-pid": true, "--session-started-at": true, "--session-executable": true, "--cwd": true}
	for _, line := range domaincli.IssueOpsUsageLines() {
		key := domaincli.IssueOpsUsageKey(line)
		if key == "" {
			continue
		}
		carriesRecordActorFlags := false
		advertised := map[string]bool{}
		for _, field := range strings.Fields(line) {
			switch field {
			case "agent-harness", "issueops":
			case "RECORD_ACTOR_FLAGS", "[RECORD_ACTOR_FLAGS]":
				carriesRecordActorFlags = true
				for name := range actorRecord {
					advertised[name] = true
				}
			case "ACTOR_FLAGS", "[ACTOR_FLAGS]":
				for name := range actorFull {
					advertised[name] = true
				}
			default:
				trimmed := strings.Trim(field, "[]()|,.:")
				trimmed = strings.TrimSuffix(trimmed, "...")
				if !strings.HasPrefix(trimmed, "--") {
					continue
				}
				// 배타 그룹 (--preview|--apply)은 이미 Trim으로 괄호가 벗겨지므로
				// 내부 | 기준으로도 분리한다.
				for _, part := range strings.Split(trimmed, "|") {
					part = strings.Trim(part, "[](),.:")
					if strings.HasPrefix(part, "--") {
						advertised[part] = true
					}
				}
			}
		}
		values, booleans, repeatable, ok := IssueOpsCommandSpec(key)
		if !ok {
			// RECORD_ACTOR_FLAGS를 요구하는 명령은 활성 lease 아래 holder가
			// exact 스펙으로 검증받아야 한다. 스펙 없이 전달되면 그 명령의
			// actor 검증 경로가 없다. 현재 의도된 예외만 여기 나열한다.
			exceptions := map[string]bool{
				// link-issue/record-routing은 실행 prepare 이전 단계 전용
				// planning mutation이라 holder 검증 대상이 아니다.
				"link-issue":     true,
				"record-routing": true,
			}
			if carriesRecordActorFlags && !exceptions[key] {
				t.Errorf("catalog advertises RECORD_ACTOR_FLAGS for %q but IssueOpsCommandSpec has no entry", key)
			}
			continue
		}
		if carriesRecordActorFlags {
			for name := range actorRecord {
				if !values[name] && !booleans[name] && !repeatable[name] {
					t.Errorf("%q: catalog RECORD_ACTOR_FLAGS advertises %s but spec does not accept it", key, name)
				}
			}
		}
		for name := range advertised {
			if !values[name] && !booleans[name] && !repeatable[name] {
				t.Errorf("%q: catalog advertises %s but spec does not accept it", key, name)
			}
		}
	}
}
