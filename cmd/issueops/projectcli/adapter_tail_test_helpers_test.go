package projectcli

import (
	fingerprintt4d "issueops/internal/adapter/lifecycle/fingerprint"
	projectbootstrapt4d "issueops/internal/adapter/projectbootstrap"
	projectdocsadapter "issueops/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	AppendProjectDocsEntry = projectdocsadapter.AppendProjectDocsEntry
	RouteProjectDocs = projectdocsadapter.RouteProjectDocs
	fingerprintt4d.ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
	projectbootstrapt4d.AnalyzeProjectSignals = projectdocsadapter.AnalyzeProjectSignals
	projectbootstrapt4d.RenderAgentsWithBlock = projectdocsadapter.RenderAgentsWithBlock
	projectbootstrapt4d.RenderProjectDocs = projectdocsadapter.RenderProjectDocs
}
