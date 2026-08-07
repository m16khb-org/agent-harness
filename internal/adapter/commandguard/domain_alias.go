package commandguard

import commandguarddomain "agent-harness/internal/domain/commandguard"

// kubectl 판정은 명령 문자열만 보는 순수 규칙이므로
// internal/domain/commandguard가 소유한다. staged check는 repo의 package.json을
// 디스크에서 읽어야 해서 이 패키지에 남는다.
//
// 아래 별칭은 core/lifecycle 등 기존 호출부가 그대로 동작하게 한다. 소비자가
// core를 떠날 때 각자 도메인 경로를 직접 import하게 되고, 그 시점에 이 파일은
// 사라진다.
type (
	KubectlEvaluation     = commandguarddomain.KubectlEvaluation
	KubectlExecScope      = commandguarddomain.KubectlExecScope
	KubectlLiveAccessKind = commandguarddomain.KubectlLiveAccessKind
)

const (
	KubectlLiveAccessNone         = commandguarddomain.KubectlLiveAccessNone
	KubectlLiveAccessPortForward  = commandguarddomain.KubectlLiveAccessPortForward
	KubectlLiveAccessReadOnlyExec = commandguarddomain.KubectlLiveAccessReadOnlyExec
	KubectlLiveAccessUnsafeExec   = commandguarddomain.KubectlLiveAccessUnsafeExec
)

var (
	EvaluateGitOpsKubectl = commandguarddomain.EvaluateGitOpsKubectl
	GitOpsKubectlDecision = commandguarddomain.GitOpsKubectlDecision
	KubectlFlagTakesValue = commandguarddomain.KubectlFlagTakesValue
)
