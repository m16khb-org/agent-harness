package validationcli

import (
	nativeintegrationt4d "agent-harness/cmd/harness/validationcli/nativeintegration"
	installutiladapter "agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	nativeintegrationt4d.SkillNamesForHost = installutiladapter.SkillNamesForHost
}
