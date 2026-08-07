package projectdoc

import projectdocdomain "agent-harness/internal/domain/projectdoc"

// 프로젝트 문서의 이름·마커·경로 규칙은 I/O가 없는 도메인 규칙이므로
// internal/domain/projectdoc이 소유한다. catalog.go와 util.go만 os를 통해 디스크를
// 읽으므로 이 패키지에 남는다.
//
// 아래 별칭은 그 분리가 호출부를 건드리지 않게 한다. 이 심볼들은
// core/projectdocs, core/doctor, core/hookprompt, core/lifecycle/docupkeep,
// core/projectbootstrap 등 열 곳 넘게서 참조하는데, 그 전부를 새 경로로 바꾸면
// 이번 분리의 위험이 실제 이동보다 import 수정 쪽에 쏠린다. 소비자들이 core를
// 떠날 때 각자 도메인 경로를 직접 import하게 되고, 그 시점에 이 파일은 사라진다.
const (
	ProjectDocsDir             = projectdocdomain.ProjectDocsDir
	AgentsStartMarker          = projectdocdomain.AgentsStartMarker
	AgentsEndMarker            = projectdocdomain.AgentsEndMarker
	BehavioralGuidelines       = projectdocdomain.BehavioralGuidelines
	SolidDesignPatternGuidance = projectdocdomain.SolidDesignPatternGuidance
)

var (
	ProjectDocNames         = projectdocdomain.ProjectDocNames
	OptionalProjectDocNames = projectdocdomain.OptionalProjectDocNames
	AllowedProjectDocNames  = projectdocdomain.AllowedProjectDocNames

	DocMetaDescription    = projectdocdomain.DocMetaDescription
	ParseFrontmatter      = projectdocdomain.ParseFrontmatter
	EnsureMetaFrontmatter = projectdocdomain.EnsureMetaFrontmatter

	PrefixedProjectDocNames = projectdocdomain.PrefixedProjectDocNames
	NormalizeRelPath        = projectdocdomain.NormalizeRelPath
	NonEmptyStrings         = projectdocdomain.NonEmptyStrings
	AppendUnique            = projectdocdomain.AppendUnique
)
