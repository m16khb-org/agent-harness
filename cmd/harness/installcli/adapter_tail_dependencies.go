package installcli

import (
	installcontract "agent-harness/internal/contract/install"
	"agent-harness/internal/port"
)

// 관리 대상 명령 파일 트랜잭션이다. installcli는 채택 구현을 모르고 이 세 연산만
// 호출한다 — 소비자가 필요한 만큼만 인터페이스로 선언한다.
type ManagedCommandPathTransaction interface {
	Apply() (installcontract.ManagedCommandPathPlan, error)
	Rollback() (installcontract.ManagedCommandPathPlan, error)
	Finalize() (installcontract.ManagedCommandPathPlan, error)
}

// 설치 계획 수립과 프로젝트 문서 관측은 파일시스템에 닿는다. 구현은 composition
// root가 설치한다.
var (
	EnsureSymlinkPlan func(target, path string, dryRun bool) (port.InstallLink, error)
	// PrepareManagedCommandPathCandidate는 후보가 채택 가능한지 판정하고
	// 되돌릴 수 있는 트랜잭션을 돌려준다.
	PrepareManagedCommandPathCandidate func(target, candidate, path string, adopt, dryRun bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error)
	// SemanticSHA256은 MCP 카탈로그의 정규 다이제스트를 계산한다.
	SemanticSHA256 func(value any) (string, error)
	// VerifyCodexActivation과 VerifyClaudeActivation은 host별 활성화 증적을 읽는다.
	VerifyCodexActivation  func(port.NativeInstallRequest) (any, error)
	VerifyClaudeActivation func(port.NativeInstallRequest) (any, error)
)
