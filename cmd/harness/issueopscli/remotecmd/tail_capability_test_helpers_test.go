package remotecmd

import (
	provideradapter "agent-harness/internal/adapter/provider"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	Resolve = provideradapter.Resolve
}
