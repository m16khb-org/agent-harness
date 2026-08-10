package codex

import (
	"agent-harness/internal/port"
)

// 설치 계획 수립과 프로젝트 문서 관측은 파일시스템에 닿는다. 구현은 composition
// root가 설치한다.
var (
	CaptureNativeActivationEvidence func(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error)
	HookGroupContainsAgentHarness   func(group any) bool
	HookGroupContainsCommand        func(group any, commandPrefix string) bool
	HookTargetDriftMessages         func(config map[string]any, host, expected string) []string
	HookTargetGenerationMessages    func(config map[string]any, host, expected, running string, read func(string) string) []string
	PlanHostSkillLinks              func(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error)
	FileBuildGenerationString       func(path string) string
	RunningBuildGenerationString    func() string
	SemanticSHA256                  func(value any) (string, error)
	ValidateHookConfigForMerge      func(config map[string]any, knownEvents []string) error
	VerifyHookActivation            func(path string, expected map[string]any) (string, error)
)
