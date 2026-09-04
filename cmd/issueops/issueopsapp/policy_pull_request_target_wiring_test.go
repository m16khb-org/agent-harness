package issueopsapp

import (
	"testing"

	policyadapter "issueops/internal/adapter/policy"
)

// TestPolicyPullRequestTargetLookupIsWired는 PR/MR target 가드가 composition
// root에 실제로 설치되는지 본다.
//
// 이 테스트가 없으면 배선 한 줄이 사라져도 policy 패키지의 단위 테스트와
// 통합 테스트는 모두 통과한다. 그 테스트들은 lookup을 직접 주입하기
// 때문이다. 2026-08-27에 legacy hook 표면을 지우면서 배선 파일
// remoteartifact_wiring.go가 함께 사라졌고, 가드 구현과 그 테스트는 그대로
// 남아 CI가 매번 초록을 보고했다. 다음 날 자식 Task MR이 부모 작업 브랜치가
// 아니라 release/stg를 타겟해 열렸다.
//
// 여기서 확인하는 것은 "설치되었는가" 하나다. 설치된 뒤의 판정은
// internal/adapter/policy의 EvaluateCommandPolicy 통합 테스트가 맡는다.
func TestPolicyPullRequestTargetLookupIsWired(t *testing.T) {
	original := policyadapter.PreparedBaseBranchLookup
	policyadapter.PreparedBaseBranchLookup = nil
	t.Cleanup(func() { policyadapter.PreparedBaseBranchLookup = original })

	configurePolicyAndGitObservers()

	if policyadapter.PreparedBaseBranchLookup == nil {
		t.Fatal("composition root가 policy PR/MR target lookup을 설치하지 않았다; " +
			"설치가 빠지면 잘못된 타겟의 PR/MR이 사전 거부 없이 열린다")
	}
}
