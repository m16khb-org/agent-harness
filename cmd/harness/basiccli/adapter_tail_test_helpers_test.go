package basiccli

import (
	fingerprintt4d "agent-harness/internal/adapter/lifecycle/fingerprint"
	projectbootstrapt4d "agent-harness/internal/adapter/projectbootstrap"
	projectdocsadapter "agent-harness/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	fingerprintt4d.ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
	projectbootstrapt4d.AnalyzeProjectSignals = projectdocsadapter.AnalyzeProjectSignals
	projectbootstrapt4d.RenderAgentsWithBlock = projectdocsadapter.RenderAgentsWithBlock
	projectbootstrapt4d.RenderProjectDocs = projectdocsadapter.RenderProjectDocs
}
