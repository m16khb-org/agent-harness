package harnessapp

import (
	"agent-harness/cmd/harness/basiccli"
	"agent-harness/internal/adapter/commitsuggest"
	"agent-harness/internal/adapter/doctor"
	"agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/lintdiagnose"
	"agent-harness/internal/adapter/projectbootstrap"
	"agent-harness/internal/adapter/projectdocs"
	"agent-harness/internal/adapter/repopath"
)

// configureRepoPathResolvers는 repo root 정규화와 파일 경로 확정을 설치한다.
//
// NormalizeRoot는 filepath.Abs로 끝나지 않고 os.Stat으로 디렉터리인지 확인한다.
// 순수 경로 계산이 아니므로 domain으로 내릴 수 없고, 조립 지점을 root로 모은다.
func configureRepoPathResolvers() {
	basiccli.NormalizeRepoRoot = repopath.NormalizeRoot
	doctor.NormalizeRepoRoot = repopath.NormalizeRoot
	lifecycle.NormalizeRepoRoot = repopath.NormalizeRoot
	lintdiagnose.NormalizeRepoRoot = repopath.NormalizeRoot
	commitsuggest.NormalizeRepoRoot = repopath.NormalizeRoot
	projectbootstrap.NormalizeRepoRoot = repopath.NormalizeRoot
	projectdocs.NormalizeRepoRoot = repopath.NormalizeRoot
}
