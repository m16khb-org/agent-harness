package installcli

// IssueOps 상태 조회와 native process 관측은 파일시스템·프로세스를 읽는다.
// 그 구현은 composition root가 설치한다.
var (
	IssueOpsStateRoot func() string
)
