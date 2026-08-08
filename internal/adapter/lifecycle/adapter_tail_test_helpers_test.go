package lifecycle

import (
	fingerprintt4d "agent-harness/internal/adapter/lifecycle/fingerprint"
	projectdocsadapter "agent-harness/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	fingerprintt4d.ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
}
