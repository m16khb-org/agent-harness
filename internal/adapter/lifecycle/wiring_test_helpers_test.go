package lifecycle

import (
	fingerprintdeps "issueops/internal/adapter/lifecycle/fingerprint"
	statestore "issueops/internal/adapter/outbound/state"
	projectdocsadapter "issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
)

// production wiring과 같은 state store, repo path resolver, git origin reader를
// 설치한다. 이 package가 실제로 의존하는 대상만 채운다.
func init() {
	StateDir = statestore.StateDir
	WithKeyLock = statestore.WithKeyLock
	NormalizeRepoRoot = repopath.NormalizeRoot
	fingerprintdeps.ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
}
