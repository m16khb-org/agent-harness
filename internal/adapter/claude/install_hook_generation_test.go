package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

// TestClaudeSettingsReportsHookGenerationSkew는 #328 완료 기준 1의 호스트
// 배선을 고정한다. installutil이 세대 축을 계산해도 호스트 설치 경로가 그것을
// 부르지 않으면 사용자는 install 출력에서 아무것도 보지 못한다.
//
// 경로는 일부러 **일치**시킨다 — 경로 drift 축이 조용한 상태에서도 세대
// 불일치가 보고돼야 한다는 것이 이 결함의 핵심이기 때문이다.
func TestClaudeSettingsReportsHookGenerationSkew(t *testing.T) {
	previousRunning, previousFile := RunningBuildGenerationString, FileBuildGenerationString
	t.Cleanup(func() { RunningBuildGenerationString, FileBuildGenerationString = previousRunning, previousFile })
	RunningBuildGenerationString = func() string { return "bbbbbbbbbbbb" }
	FileBuildGenerationString = func(string) string { return "aaaaaaaaaaaa" }

	home := t.TempDir()
	binPath := filepath.Join(home, "bin", "issueops")
	settings := filepath.Join(home, "settings.json")
	seeded, err := json.Marshal(claudeSettingsConfig(binPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, seeded, 0o644); err != nil {
		t.Fatal(err)
	}

	_, messages, err := writeClaudeSettings(settings, port.NativeInstallRequest{BinPath: binPath, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(messages, "\n")
	for _, needle := range []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb", "go build -o " + binPath} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("install 출력이 %q를 담아야 한다: %s", needle, joined)
		}
	}

	// 같은 세대면 조용해야 한다 — 정상 설치가 경고로 뒤덮이면 안 된다.
	FileBuildGenerationString = func(string) string { return "bbbbbbbbbbbb" }
	_, quiet, err := writeClaudeSettings(settings, port.NativeInstallRequest{BinPath: binPath, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(quiet) != 0 {
		t.Fatalf("세대가 같으면 조용해야 한다: %v", quiet)
	}
}
