package mcpsmoke

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ValidateMCP는 self-verify의 MCP 게이트가 쓰는 실제 통합 경로다.
// 빌드된 바이너리로 이 레포에서 1회 실행해 end-to-end 계약(성공 + 결과
// 형식)을 잠근다. 느리므로 short 모드에서는 건너뛴다.
func TestValidateMCPRealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("live MCP smoke skipped in short mode")
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
	step := ValidateMCP(binary, root)
	if !step.OK {
		t.Fatalf("live MCP smoke failed: %s", step.Error)
	}
	if strings.TrimSpace(step.Stdout) == "" {
		t.Fatal("live smoke must return the SDK transcript")
	}
}
