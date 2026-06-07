package mcpcli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/cmd/harness/apidoc"
	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/core"
)

const skillName = "atomic-commit-push"

var Version = "dev"

var HarnessRoot = func() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "skills")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
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

var PrepareIssueOpsWorktreeTools = func(core.IssueOpsRecord) (any, error) {
	return nil, fmt.Errorf("issueops worktree tool preparation dependency is not configured")
}

var VerifyIssueOpsChildIssueBeforeLink = func(string) error {
	return fmt.Errorf("issueops child verification dependency is not configured")
}

var IssueOpsCleanupMerged = func(_ string, requested bool) bool {
	return requested
}

var VerifyIssueOpsRemoteArtifactLive = func(core.IssueOpsRemoteArtifactVerificationRequest) error {
	return fmt.Errorf("issueops remote artifact live verification dependency is not configured")
}

type selfVerifyRunMode struct {
	Full          bool
	Iterations    int
	ContractLabel string
}

func resolveSelfVerifyRunMode(full bool, iterationsFlagSet bool, iterations int) (selfVerifyRunMode, error) {
	if !full {
		if iterationsFlagSet {
			return selfVerifyRunMode{}, fmt.Errorf("--iterations requires --full; default self-verify runs quick one-iteration mode")
		}
		return selfVerifyRunMode{Full: false, Iterations: 1, ContractLabel: "quick one-iteration gate"}, nil
	}
	if iterations < 10 {
		return selfVerifyRunMode{}, fmt.Errorf("full self-verification requires at least 10 iterations; use --full --iterations=10 or higher")
	}
	return selfVerifyRunMode{Full: true, Iterations: iterations, ContractLabel: "full ten-plus-iteration gate"}, nil
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
