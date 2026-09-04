package loopgate

import (
	looprunadapter "issueops/internal/adapter/looprun"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	RepoGateMissing = looprunadapter.RepoGateMissing
}
