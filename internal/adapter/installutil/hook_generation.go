package installutil

import (
	"fmt"
	"strings"
)

// HookGenerationReader는 한 바이너리의 빌드 세대를 짧은 표기로 돌려준다.
// 관측하지 못하면 빈 문자열이다 — 모르는 것을 불일치로 승격하면 정상 설치가
// 고칠 수 없는 경고로 뒤덮인다.
type HookGenerationReader func(path string) string

// HookTargetGenerationMessages는 설치된 hook이 가리키는 바이너리와 지금 설치를
// 실행 중인 CLI의 빌드 세대가 다른지 보고한다(#328).
//
// 경로 drift(HookTargetDriftMessages)와 다른 축이다. 경로가 **같아도** 파일이
// 교체됐거나 아직 교체되지 않았으면 세대가 갈리고, 그 상태에서 새 typed
// command를 쓰면 이전 세대 hook이 그것을 모른 채 차단해 복구가 교착된다.
//
// 이 함수는 진단으로 끝내지 않고 exact recovery를 낸다. "reinstall hooks"라는
// 서술만으로는 사용자가 무엇을 실행할지 알 수 없고, 두 호스트를 같은 세대로
// 맞춰야 한다는 사실도 드러나지 않는다.
func HookTargetGenerationMessages(config map[string]any, host, expected, runningGeneration string, read HookGenerationReader) []string {
	if read == nil || strings.TrimSpace(runningGeneration) == "" {
		return nil
	}
	var messages []string
	for _, target := range hookTargets(config) {
		// 경로가 다른 것은 drift 축이 이미 보고한다. 두 축이 같은 사실을 두 번
		// 말하면 사용자는 무엇이 문제인지 흐려진다.
		if target != expected {
			continue
		}
		targetGeneration := strings.TrimSpace(read(target))
		if targetGeneration == "" || targetGeneration == runningGeneration {
			continue
		}
		messages = append(messages, fmt.Sprintf(
			"%s native hook target is a different build than this CLI: hook=%s (%s) cli=%s; "+
				"rebuild and reinstall so both hosts load one generation, then restart the %s session: "+
				"%s",
			host, target, targetGeneration, runningGeneration, host,
			HookGenerationRecoveryCommand(target)))
	}
	return messages
}

// HookGenerationRecoveryCommand는 두 호스트를 같은 세대로 맞추는 정확한 명령
// 순서다. 문장이 아니라 실행 가능한 형태로 준다 — 교착 상태의 사용자에게
// 필요한 것은 설명이 아니라 다음 명령이다.
func HookGenerationRecoveryCommand(target string) string {
	return fmt.Sprintf("go build -o %s ./cmd/harness && %s install --json", target, target)
}
