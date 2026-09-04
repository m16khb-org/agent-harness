package preflightfuzz

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	preflightadapter "issueops/internal/adapter/preflight"
)

// Validate는 self-verify의 preflight fuzz 게이트가 쓰는 실제 경로다.
// 빌드된 바이너리로 실 git fixture를 만들어 end-to-end 계약을 잠근다.
// mcpsmoke 라이브 테스트가 실제 데이터 레이스를 잡은 것과 같은 원리:
// seam 목킹만으로는 발견되지 않는 구성 결함을 잡는다.
func TestValidatePreflightFuzzRealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("live preflight fuzz skipped in short mode")
	}
	binary, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "bin", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("prebuilt binary unavailable: %v", err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// 프로덕션 wiring(composition root)과 동일한 GitCmd를 설치한다.
	// GitCmd는 process-spawning observer로 composition root의 결정이다.
	if GitCmd == nil {
		GitCmd = preflightadapter.GitCmd
		t.Cleanup(func() { GitCmd = nil })
	}
	for _, seed := range []int64{7, 100} {
		step := Validate(binary, root, seed)
		if !step.OK {
			t.Fatalf("seed %d live fuzz failed: %s stdout=%s", seed, step.Error, step.Stdout)
		}
		if !strings.Contains(step.Stdout, "secrets") && !strings.Contains(step.Stdout, "blocked") {
			// preflight JSON은 검증 통과 형태여도 출력이 있어야 한다.
			if strings.TrimSpace(step.Stdout) == "" {
				t.Fatalf("seed %d must return preflight JSON", seed)
			}
		}
	}
}
