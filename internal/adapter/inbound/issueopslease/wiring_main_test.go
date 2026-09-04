package issueopslease

import (
	issueopscore "issueops/internal/adapter/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 composition root가 주입한다. lease 계약 테스트는 실제 렌더링을
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureNextCommands(NextCommandDeps{
		ReseedNextCommand: issueopscore.ExecutionReseedNextCommand,
		ResumeNextCommand: issueopscore.ExecutionResumeNextCommand,
	})
	os.Exit(m.Run())
}
