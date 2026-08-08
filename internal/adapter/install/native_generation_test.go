package install

import (
	"strings"
	"testing"

	installcontract "agent-harness/internal/contract/install"
)

// TestSameGenerationTreatsUnobservedAsCompatible는 #328의 fail-open 경계를
// 고정한다. 관측하지 못한 세대를 불일치로 승격하면 사용자는 고칠 수 없는
// 경고를 계속 본다 — 세대 비교는 양쪽을 관측했을 때만 판단한다.
func TestSameGenerationTreatsUnobservedAsCompatible(t *testing.T) {
	observed := NativeBuildGeneration{Revision: "abc123", Modified: false}
	for _, tc := range []struct {
		name        string
		left, right NativeBuildGeneration
		want        bool
	}{
		{"둘 다 관측 못 함", NativeBuildGeneration{}, NativeBuildGeneration{}, true},
		{"왼쪽만 관측", observed, NativeBuildGeneration{}, true},
		{"오른쪽만 관측", NativeBuildGeneration{}, observed, true},
		{"같은 revision", observed, observed, true},
		{"다른 revision", observed, NativeBuildGeneration{Revision: "def456"}, false},
		{"같은 revision 다른 dirty", observed, NativeBuildGeneration{Revision: "abc123", Modified: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameGeneration(tc.left, tc.right); got != tc.want {
				t.Fatalf("SameGeneration(%v, %v) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
		})
	}
}

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

// TestGenerationSkewMessageNamesBothBuildsAndTheRecovery는 #328 AC-02를
// 고정한다. 경로가 같으므로 재설치는 이미 끝났을 수 있고, 남은 복구는 세션
// 재시작이다. 그 사실이 메시지에 있어야 사용자가 다음 행동을 안다.
func TestGenerationSkewMessageNamesBothBuildsAndTheRecovery(t *testing.T) {
	message, ok := NativeRuntimeDiagnosticMessage(installcontract.NativeRuntimeDiagnostic{
		Observed: "/repo/bin/agent-harness", Expected: "/repo/bin/agent-harness",
		GenerationSkew: true, RestartRequired: true,
		ObservedGeneration: "aaaaaaaaaaaa", ExpectedGeneration: "bbbbbbbbbbbb+dirty",
	}, nil)
	if !ok {
		t.Fatal("세대 불일치는 진단으로 보고돼야 한다")
	}
	for _, needle := range []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb+dirty", "/repo/bin/agent-harness", "restart"} {
		if !strings.Contains(message, needle) {
			t.Fatalf("진단에 %q가 있어야 한다: %s", needle, message)
		}
	}
}

// TestStalePathStillReportsTheReinstallRecovery는 기존 계약이 그대로임을
// 고정한다. 경로가 어긋난 경우의 복구는 재설치이고 세대 진단이 그것을
// 가리지 않아야 한다.
func TestStalePathStillReportsTheReinstallRecovery(t *testing.T) {
	message, ok := NativeRuntimeDiagnosticMessage(installcontract.NativeRuntimeDiagnostic{
		Observed: "/old/bin/agent-harness", Expected: "/repo/bin/agent-harness",
		Stale: true, RestartRequired: true, GenerationSkew: true,
	}, nil)
	if !ok || !strings.Contains(message, "reinstall hooks") {
		t.Fatalf("경로 drift는 재설치 안내를 유지해야 한다: %s", message)
	}
}

// TestCleanRuntimeStaysQuiet는 정상 상태가 조용한지 고정한다.
func TestCleanRuntimeStaysQuiet(t *testing.T) {
	if message, ok := NativeRuntimeDiagnosticMessage(installcontract.NativeRuntimeDiagnostic{
		Observed: "/repo/bin/agent-harness", Expected: "/repo/bin/agent-harness",
		ObservedGeneration: "aaaaaaaaaaaa", ExpectedGeneration: "aaaaaaaaaaaa",
	}, nil); ok {
		t.Fatalf("정상 상태는 진단을 남기지 않아야 한다: %s", message)
	}
}
