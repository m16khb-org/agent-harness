package basiccli

import (
	commitsuggestpkg "agent-harness/internal/adapter/commitsuggest"
	doctorpkg "agent-harness/internal/adapter/doctor"
	lifecyclepkg "agent-harness/internal/adapter/lifecycle"
	lintdiagnosepkg "agent-harness/internal/adapter/lintdiagnose"
	projectbootstrappkg "agent-harness/internal/adapter/projectbootstrap"
	projectdocspkg "agent-harness/internal/adapter/projectdocs"
	"agent-harness/internal/adapter/repopath"
)

// production wiring과 같은 repo path resolver를 설치한다. 이 package의 테스트는
// 다른 package를 거쳐 정규화에 닿으므로 간접 의존까지 함께 채운다. fitness graph는
// test import를 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	NormalizeRepoRoot = repopath.NormalizeRoot
	doctorpkg.NormalizeRepoRoot = repopath.NormalizeRoot
	lifecyclepkg.NormalizeRepoRoot = repopath.NormalizeRoot
	lintdiagnosepkg.NormalizeRepoRoot = repopath.NormalizeRoot
	commitsuggestpkg.NormalizeRepoRoot = repopath.NormalizeRoot
	projectbootstrappkg.NormalizeRepoRoot = repopath.NormalizeRoot
	projectdocspkg.NormalizeRepoRoot = repopath.NormalizeRoot
}
