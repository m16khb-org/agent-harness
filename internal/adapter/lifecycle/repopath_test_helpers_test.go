package lifecycle

import (
	"agent-harness/internal/adapter/repopath"
)

// production wiring과 같은 repo path resolver를 설치한다. 다른 adapter까지 채우면
// import 순환이 되므로 자기 것만 설치한다.
func init() {
	NormalizeRepoRoot = repopath.NormalizeRoot
}
