package fingerprint

import (
	projectdocsadapter "issueops/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
}
