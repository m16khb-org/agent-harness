package mcpcli

import (
	basicclipkg "issueops/cmd/issueops/basiccli"
	commitsuggestpkg "issueops/internal/adapter/commitsuggest"
	doctorpkg "issueops/internal/adapter/doctor"
	lifecyclepkg "issueops/internal/adapter/lifecycle"
	lintdiagnosepkg "issueops/internal/adapter/lintdiagnose"
	projectbootstrappkg "issueops/internal/adapter/projectbootstrap"
	projectdocspkg "issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
)

// production wiring과 같은 repo path resolver를 설치한다. 이 package의 테스트는
// 다른 package를 거쳐 정규화에 닿으므로 간접 의존까지 함께 채운다. fitness graph는
// test import를 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	basicclipkg.NormalizeRepoRoot = repopath.NormalizeRoot
	doctorpkg.NormalizeRepoRoot = repopath.NormalizeRoot
	lifecyclepkg.NormalizeRepoRoot = repopath.NormalizeRoot
	lintdiagnosepkg.NormalizeRepoRoot = repopath.NormalizeRoot
	commitsuggestpkg.NormalizeRepoRoot = repopath.NormalizeRoot
	projectbootstrappkg.NormalizeRepoRoot = repopath.NormalizeRoot
	projectdocspkg.NormalizeRepoRoot = repopath.NormalizeRoot
}
