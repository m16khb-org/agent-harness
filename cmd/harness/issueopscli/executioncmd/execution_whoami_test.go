package executioncmd

import (
	"os"
	"testing"
)

// 이슈 #90 발견 3: owner가 claim identity(pid/started-at/executable)를 shell
// 확장($$, $(date), $SHELL) 없이 채울 수 있도록, 호출 프로세스의 native
// ancestry receipt를 그대로 노출하는 read-only 표면이 필요하다.
func TestExecutionWhoamiExposesCallerAncestryReceipts(t *testing.T) {
	var captured any
	deps := Deps{PrintJSON: func(value any) error { captured = value; return nil }}
	if err := Run([]string{"whoami", "--json"}, deps); err != nil {
		t.Fatalf("whoami must not require state or provisioners: %v", err)
	}
	result, ok := captured.(ExecutionWhoamiResult)
	if !ok || !result.OK || len(result.Ancestry) == 0 {
		t.Fatalf("whoami must expose a non-empty ancestry: %#v", captured)
	}
	self := result.Ancestry[0]
	if self.PID != os.Getpid() || self.StartedAt == "" || self.Executable == "" {
		t.Fatalf("first receipt must be the calling process with full identity: %+v", self)
	}
	if result.ClaimActorFlags[0] == "" {
		t.Fatalf("whoami must render copy-pasteable claim actor flags: %+v", result.ClaimActorFlags)
	}
}
