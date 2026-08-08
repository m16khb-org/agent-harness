package claude

import (
	"agent-harness/internal/port"
)

// 설치 계획 수립과 프로젝트 문서 관측은 파일시스템에 닿는다. 구현은 composition
// root가 설치한다.
var (
	CaptureNativeActivationEvidence func(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error)
	EnsureSymlinkPlan               func(target, path string, dryRun bool) (port.InstallLink, error)
	HookGroupContainsAgentHarness   func(group any) bool
	HookTargetDriftMessages         func(config map[string]any, host, expected string) []string
	PlanHostSkillLinks              func(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error)
	PreToolUseEnforcementFlags      func() string
	SemanticSHA256                  func(value any) (string, error)
	StopEnforcementFlags            func() string
	VerifyHookActivation            func(path string, expected map[string]any) (string, error)
)
