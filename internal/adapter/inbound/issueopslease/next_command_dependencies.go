package issueopslease

// next-command 렌더링은 실행 어댑터가 소유한 문자열 조합이다. inbound 어댑터는
// 그 구현을 모르고 composition root가 주입한 함수만 호출한다.
var (
	executionReseedNextCommand = func(id string, generation uint64, mode, claimTokenPath string) string { return "" }
	executionResumeNextCommand = func(id string, generation uint64, claimTokenPath, issueBodySHA256, contextPacketSHA256 string) string {
		return ""
	}
)

// NextCommandDeps는 composition root가 실제 구현을 꽂는 진입점이다.
type NextCommandDeps struct {
	ReseedNextCommand func(id string, generation uint64, mode, claimTokenPath string) string
	ResumeNextCommand func(id string, generation uint64, claimTokenPath, issueBodySHA256, contextPacketSHA256 string) string
}

func ConfigureNextCommands(deps NextCommandDeps) {
	if deps.ReseedNextCommand != nil {
		executionReseedNextCommand = deps.ReseedNextCommand
	}
	if deps.ResumeNextCommand != nil {
		executionResumeNextCommand = deps.ResumeNextCommand
	}
}
