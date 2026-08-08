package hookcli

// 편집된 Go 파일 lint는 외부 도구를 실행한다. 구현은 composition root가 설치한다.
var LintEditedGoFiles func(repo string, paths []string) (failed bool, feedback string)
