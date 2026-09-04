package installcli

import (
	installutiladapter "issueops/internal/adapter/installutil"
	installcontract "issueops/internal/contract/install"
	"os"
	"testing"
)

// 프로덕션에서는 issueopsapp이 주입한다. 설치 경로 테스트는 실제 채택 트랜잭션을
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	PrepareManagedCommandPathCandidate = func(target, candidate, path string, adopt, dryRun bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
		transaction, plan, err := installutiladapter.PrepareManagedCommandPathCandidate(target, candidate, path, adopt, dryRun)
		if transaction == nil {
			return nil, plan, err
		}
		return transaction, plan, err
	}
	SemanticSHA256 = installutiladapter.SemanticSHA256
	os.Exit(m.Run())
}
