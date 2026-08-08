package implementation

import (
	preflightadapter "agent-harness/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	GitCmd = preflightadapter.GitCmd
	GitCmdRaw = preflightadapter.GitCmdRaw
}
