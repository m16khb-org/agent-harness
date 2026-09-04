package qagate

// native runtime 진단과 skill 목록 조회는 파일시스템을 읽는다. 그 구현은
// composition root가 설치한다.
var (
	ListSkillNames func(root string) ([]string, error)
)
