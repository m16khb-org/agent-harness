package projectbootstrap

import (
	fingerprintt4d "issueops/internal/adapter/lifecycle/fingerprint"
	projectdocsadapter "issueops/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	AnalyzeProjectSignals = projectdocsadapter.AnalyzeProjectSignals
	RenderAgentsWithBlock = projectdocsadapter.RenderAgentsWithBlock
	RenderProjectDocs = projectdocsadapter.RenderProjectDocs
	fingerprintt4d.ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
}
