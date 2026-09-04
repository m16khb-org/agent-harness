package doctor

import (
	lifecyclepkg "issueops/internal/adapter/lifecycle"
	projectbootstrappkg "issueops/internal/adapter/projectbootstrap"
	projectdocspkg "issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
)

// production wiring과 같은 repo path resolver를 설치한다. 이 package가 import
// 방향을 따라 실제로 거쳐 가는 대상만 채운다 — 역방향으로 채우면 순환이 된다.
func init() {
	NormalizeRepoRoot = repopath.NormalizeRoot
	lifecyclepkg.NormalizeRepoRoot = repopath.NormalizeRoot
	projectbootstrappkg.NormalizeRepoRoot = repopath.NormalizeRoot
	projectdocspkg.NormalizeRepoRoot = repopath.NormalizeRoot
}
