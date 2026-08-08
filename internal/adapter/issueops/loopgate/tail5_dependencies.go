package loopgate

// 이 연산들은 파일시스템·프로세스에 닿는다. 구현은 composition root가 설치한다.
var (
	RepoGateMissing func(repo string) ([]string, []string)
)
