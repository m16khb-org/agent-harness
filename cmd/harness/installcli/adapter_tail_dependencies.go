package installcli

import (
	"agent-harness/internal/port"
)

// 설치 계획 수립과 프로젝트 문서 관측은 파일시스템에 닿는다. 구현은 composition
// root가 설치한다.
var (
	EnsureSymlinkPlan func(target, path string, dryRun bool) (port.InstallLink, error)
)
