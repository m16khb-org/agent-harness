package install

import (
	"testing"
)

// TestBuildGenerationStringNamesTheBuild는 진단 표기를 고정한다. revision만
// 같고 dirty가 다른 두 빌드를 구분할 수 있어야 dogfood 빌드에서도 쓸모가 있다.
func TestBuildGenerationStringNamesTheBuild(t *testing.T) {
	for _, tc := range []struct {
		generation NativeBuildGeneration
		want       string
	}{
		{NativeBuildGeneration{}, "unknown"},
		{NativeBuildGeneration{Revision: "0c50f0c08ab822b719bc4951ba4cacbbdb11f81d"}, "0c50f0c08ab8"},
		{NativeBuildGeneration{Revision: "0c50f0c08ab822b719bc4951ba4cacbbdb11f81d", Modified: true}, "0c50f0c08ab8+dirty"},
		{NativeBuildGeneration{Revision: "short"}, "short"},
	} {
		if got := tc.generation.String(); got != tc.want {
			t.Fatalf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestRunningBuildGenerationIsObservable는 이 진단이 실제 빌드에서 동작하는지
// 확인한다. 테스트 바이너리도 VCS 정보를 담으므로 관측 가능해야 한다.
func TestRunningBuildGenerationIsObservable(t *testing.T) {
	running := RunningBuildGeneration()
	if !running.Observed() {
		t.Skip("이 빌드는 VCS 정보를 담지 않는다 (예: -buildvcs=false)")
	}
	if running.String() == "unknown" {
		t.Fatalf("관측된 세대는 표기를 가져야 한다: %+v", running)
	}
}

// TestFileBuildGenerationFailsQuietlyOnUnreadablePaths는 읽지 못한 경로를
// 오류가 아니라 미관측으로 다루는지 고정한다.
func TestFileBuildGenerationFailsQuietlyOnUnreadablePaths(t *testing.T) {
	for _, path := range []string{"", "   ", "/nonexistent/agent-harness", t.TempDir()} {
		if got := FileBuildGeneration(path); got.Observed() {
			t.Fatalf("읽을 수 없는 경로는 미관측이어야 한다: %q -> %+v", path, got)
		}
	}
}
