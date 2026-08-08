package nativeintegration

// 설치 계획 수립과 프로젝트 문서 관측은 파일시스템에 닿는다. 구현은 composition
// root가 설치한다.
var (
	SkillNamesForHost func(root string, skillNames []string, host string) (enabled []string, skipped []string)
)
