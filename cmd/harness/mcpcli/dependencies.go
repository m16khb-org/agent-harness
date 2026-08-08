package mcpcli

import (
	inspectcontract "agent-harness/internal/contract/inspect"
	preflightcontract "agent-harness/internal/contract/preflight"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/cmd/harness/apidoc"
	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/cmd/harness/selfworkflow/runmode"
)

const skillName = "atomic-commit-push"

var Version = "dev"

var HarnessRoot = func() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
		abs, err := filepath.Abs(root)
		if err == nil {
			return abs
		}
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "skills")) {
			abs, err := filepath.Abs(dir)
			if err == nil {
				return abs
			}
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			abs, err := filepath.Abs(cwd)
			if err == nil {
				return abs
			}
			return cwd
		}
	}
}

var ResolveTarget = func(target string) string {
	if target != "" {
		return target
	}
	return HarnessRoot()
}

var ReadHarnessFile = func(parts ...string) (string, error) {
	path := filepath.Join(append([]string{HarnessRoot()}, parts...)...)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var InspectHarness = func(repo string) any {
	return map[string]any{"ok": false, "error": "inspect dependency is not configured", "repo": repo}
}

var RunMCPProxy = func() error {
	return fmt.Errorf("mcp proxy dependency is not configured")
}

var DaemonStatus = func() any {
	return map[string]any{"ok": false, "message": "daemon is not running"}
}

var CompatibilityContract = func() any {
	toolNames := []string{}
	for _, tool := range MCPTools() {
		if name, ok := tool["name"].(string); ok {
			toolNames = append(toolNames, name)
		}
	}
	return map[string]any{"ok": true, "name": "agent_harness_cli_mcp_compatibility", "mcp_tools": toolNames}
}

var SelfVerify = func(int, int64, float64, bool) (selfworkflow.SelfAugmentResult, error) {
	return selfworkflow.SelfAugmentResult{}, fmt.Errorf("self-verify dependency is not configured")
}

var ErrSelfVerificationGateFailed = errors.New("self-verification quality gate failed")

var (
	errAPIDocReviewGateFailed     = apidoc.ErrReviewGateFailed
	errAPIDocStaticGateFailed     = apidoc.ErrStaticGateFailed
	errSelfVerificationGateFailed = ErrSelfVerificationGateFailed
)

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, errAPIDocReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, errAPIDocStaticGateFailed)
}

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, ErrSelfVerificationGateFailed)
}

type selfVerifyRunMode = runmode.Mode

func resolveSelfVerifyRunMode(full bool, iterationsFlagSet bool, iterations int) (selfVerifyRunMode, error) {
	return runmode.Resolve(full, iterationsFlagSet, iterations)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GitPreflight와 ListSkills는 composition root가 설치한다. MCP tool router는
// git 실행이나 skill 디렉터리 탐색을 스스로 하지 않는다.
var GitPreflight func(target, harnessRoot string) preflightcontract.PreflightResult

var ListSkills func(root, skillName string) []inspectcontract.SkillInfo
