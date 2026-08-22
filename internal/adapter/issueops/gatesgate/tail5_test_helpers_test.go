package gatesgate

import (
	"agent-harness/internal/adapter/issueops/loopgate"
	looprunadapter "agent-harness/internal/adapter/looprun"
)

// production wiring과 같은 구현을 설치한다. loopgate가 의존하는 RepoGateMissing
// 함수 변수를 채운다.
func init() {
	loopgate.RepoGateMissing = looprunadapter.RepoGateMissing
}
