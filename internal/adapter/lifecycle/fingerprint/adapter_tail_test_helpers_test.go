package fingerprint

import (
	projectdocsadapter "agent-harness/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
}
