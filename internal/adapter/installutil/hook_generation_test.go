package installutil

import (
	"strings"
	"testing"
)

const hookBin = "/repo/bin/issueops"

func generationConfig(targets ...string) map[string]any {
	groups := make([]any, 0, len(targets))
	for _, target := range targets {
		groups = append(groups, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": target + " hook pre-tool-use"}},
		})
	}
	return map[string]any{"hooks": map[string]any{"PreToolUse": groups}}
}

// TestHookTargetGenerationMessagesNamesBothBuildsAndAnExactRecovery는 #328
// 완료 기준 2를 고정한다. 경로가 같아 drift 축이 조용한 상태에서도 세대가
// 갈릴 수 있고, 그때 사용자에게 필요한 것은 설명이 아니라 실행할 명령이다.
func TestHookTargetGenerationMessagesNamesBothBuildsAndAnExactRecovery(t *testing.T) {
	messages := HookTargetGenerationMessages(generationConfig(hookBin), "claude", hookBin, "bbbbbbbbbbbb+dirty",
		func(string) string { return "aaaaaaaaaaaa" })
	if len(messages) != 1 {
		t.Fatalf("세대 불일치는 정확히 한 번 보고돼야 한다: %v", messages)
	}
	for _, needle := range []string{
		"aaaaaaaaaaaa", "bbbbbbbbbbbb+dirty", hookBin, "claude",
		"go build -o " + hookBin + " ./cmd/issueops", hookBin + " install --json",
	} {
		if !strings.Contains(messages[0], needle) {
			t.Fatalf("메시지에 %q가 있어야 한다: %s", needle, messages[0])
		}
	}
}

// TestHookTargetGenerationStaysQuietWhenItCannotJudge는 fail-open 경계를
// 고정한다. 관측하지 못한 세대를 불일치로 승격하면 정상 설치가 고칠 수 없는
// 경고로 뒤덮인다.
func TestHookTargetGenerationStaysQuietWhenItCannotJudge(t *testing.T) {
	for _, tc := range []struct {
		name    string
		running string
		read    HookGenerationReader
	}{
		{"reader 없음", "aaaaaaaaaaaa", nil},
		{"실행 세대 미관측", "", func(string) string { return "bbbbbbbbbbbb" }},
		{"대상 세대 미관측", "aaaaaaaaaaaa", func(string) string { return "" }},
		{"같은 세대", "aaaaaaaaaaaa", func(string) string { return "aaaaaaaaaaaa" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if messages := HookTargetGenerationMessages(generationConfig(hookBin), "claude", hookBin, tc.running, tc.read); len(messages) != 0 {
				t.Fatalf("판단할 수 없거나 일치하면 조용해야 한다: %v", messages)
			}
		})
	}
}

// TestHookTargetGenerationLeavesPathDriftToItsOwnAxis는 두 축이 같은 사실을
// 두 번 말하지 않음을 고정한다. 경로가 다르면 drift 메시지가 이미 그것을
// 보고하고, 여기서 다시 말하면 사용자는 무엇이 문제인지 흐려진다.
func TestHookTargetGenerationLeavesPathDriftToItsOwnAxis(t *testing.T) {
	messages := HookTargetGenerationMessages(generationConfig("/old/bin/issueops"), "codex", hookBin, "bbbbbbbbbbbb",
		func(string) string { return "aaaaaaaaaaaa" })
	if len(messages) != 0 {
		t.Fatalf("경로 drift는 drift 축의 일이다: %v", messages)
	}
	// 그 경로 불일치는 여전히 보고돼야 한다.
	if drift := HookTargetDriftMessages(generationConfig("/old/bin/issueops"), "codex", hookBin); len(drift) != 1 {
		t.Fatalf("drift 축은 계속 보고해야 한다: %v", drift)
	}
}
