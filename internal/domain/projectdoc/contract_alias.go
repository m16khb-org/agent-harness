package projectdoc

import projectdoccontract "issueops/internal/contract/projectdoc"

// 신호와 계획 파일은 계약 DTO다. domain은 같은 이름으로 재노출만 한다.
type (
	ProjectSignals         = projectdoccontract.ProjectSignals
	EvidenceCommand        = projectdoccontract.EvidenceCommand
	ProjectDocsPlannedFile = projectdoccontract.ProjectDocsPlannedFile
)
