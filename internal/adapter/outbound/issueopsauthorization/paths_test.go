package issueopsauthorization

import (
	"path/filepath"
	"testing"
)

// CanonicalPaths는 실행 리스 홀더 검증의 경로 동일성 규칙이다. 절대화,
// 심볼릭 링크 해소, trim을 모두 거친 뒤 비교한다.
func TestCanonicalPathsSame(t *testing.T) {
	paths := CanonicalPaths{}
	dir := t.TempDir()
	if !paths.Same(dir, "  "+dir+"  ") {
		t.Fatal("trimmed identical path must match")
	}
	if paths.Same(dir, filepath.Join(t.TempDir(), "other")) {
		t.Fatal("distinct paths must not match")
	}
	if paths.Same(dir, "") {
		t.Fatal("empty path must not match a real dir")
	}
}
