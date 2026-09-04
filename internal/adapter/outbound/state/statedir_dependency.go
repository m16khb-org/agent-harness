package state

import (
	"os"
	"path/filepath"
)

// 상태 디렉터리 결정은 환경변수와 홈 디렉터리를 읽는 I/O다. composition root가
// 주입하며, 기본값은 어댑터 없이도 동작하는 같은 규칙이다.
var stateDir = defaultStateDir

// ConfigureStateDir는 composition root가 실제 해석기를 꽂는 진입점이다.
func ConfigureStateDir(resolve func() string) {
	if resolve != nil {
		stateDir = resolve
	}
}

func defaultStateDir() string {
	if env := os.Getenv("ISSUEOPS_STATE_DIR"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "issueops-state")
	}
	return filepath.Join(home, ".local", "state", "issueops")
}
