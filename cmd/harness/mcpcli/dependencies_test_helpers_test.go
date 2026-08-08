package mcpcli

import (
	"agent-harness/internal/adapter/inspect"
	"agent-harness/internal/adapter/preflight"
)

// production wiring과 같은 구현을 설치한다. fitness graph는 test import를
// 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	GitPreflight = preflight.GitPreflight
	ListSkills = inspect.ListSkills
}
