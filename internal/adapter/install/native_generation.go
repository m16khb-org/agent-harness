package install

import (
	"debug/buildinfo"
	"runtime/debug"
	"strings"
)

// NativeBuildGeneration은 한 바이너리의 빌드 세대를 식별한다. 경로가 같아도
// 파일이 교체됐으면 세대가 달라지고, 실행 중인 프로세스는 교체 이전 세대를
// 계속 쓴다.
type NativeBuildGeneration struct {
	// Revision은 VCS 커밋이다. 빈 문자열이면 관측하지 못한 것이다.
	Revision string
	// Modified는 작업 트리에 커밋되지 않은 변경이 있는 상태로 빌드됐는지다.
	// dogfood 빌드는 대부분 true이므로, revision이 같아도 이 값이 다르면
	// 서로 다른 바이너리다.
	Modified bool
}

// Observed는 세대를 관측했는지 보고한다. 관측하지 못한 세대는 비교 근거가
// 될 수 없다 — 모르는 것을 불일치로 보고하면 정상 설치가 경고로 뒤덮인다.
func (g NativeBuildGeneration) Observed() bool {
	return strings.TrimSpace(g.Revision) != ""
}

// String은 진단에 넣을 짧은 표기다.
func (g NativeBuildGeneration) String() string {
	if !g.Observed() {
		return "unknown"
	}
	revision := g.Revision
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if g.Modified {
		return revision + "+dirty"
	}
	return revision
}

// RunningBuildGeneration은 지금 실행 중인 프로세스의 빌드 세대를 돌려준다.
func RunningBuildGeneration() NativeBuildGeneration {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return NativeBuildGeneration{}
	}
	return buildGenerationFromSettings(info.Settings)
}

// FileBuildGeneration은 디스크에 있는 바이너리의 빌드 세대를 읽는다.
// 읽지 못하면 빈 세대를 돌려주고, 호출부는 그것을 비교 대상에서 제외한다.
func FileBuildGeneration(path string) NativeBuildGeneration {
	info, err := buildinfo.ReadFile(strings.TrimSpace(path))
	if err != nil || info == nil {
		return NativeBuildGeneration{}
	}
	return buildGenerationFromSettings(info.Settings)
}

func buildGenerationFromSettings(settings []debug.BuildSetting) NativeBuildGeneration {
	generation := NativeBuildGeneration{}
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			generation.Revision = strings.TrimSpace(setting.Value)
		case "vcs.modified":
			generation.Modified = strings.TrimSpace(setting.Value) == "true"
		}
	}
	return generation
}

// RunningBuildGenerationString과 FileBuildGenerationString은 설치 경로가 쓰는
// 짧은 표기 어댑터다. 관측하지 못하면 빈 문자열을 돌려준다 — 호출부가
// "unknown"과 미관측을 구별할 필요 없이 조용히 판단을 미루게 하기 위해서다.
func RunningBuildGenerationString() string {
	return observedGenerationString(RunningBuildGeneration())
}

func FileBuildGenerationString(path string) string {
	return observedGenerationString(FileBuildGeneration(path))
}

func observedGenerationString(generation NativeBuildGeneration) string {
	if !generation.Observed() {
		return ""
	}
	return generation.String()
}
