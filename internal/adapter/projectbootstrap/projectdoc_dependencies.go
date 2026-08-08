package projectbootstrap

import ()

// 프로젝트 문서 탐색과 파일 상태 판정은 디스크를 읽는다. 그 구현은 composition
// root가 설치한다.
var (
	PlannedFileAction func(path, content string) string
)
