package mcpcli

import (
	"errors"
	"fmt"
	channelcontract "issueops/internal/contract/channel"
	gatescontract "issueops/internal/contract/gates"
	inspectcontract "issueops/internal/contract/inspect"
	loopruncontract "issueops/internal/contract/looprun"
	preflightcontract "issueops/internal/contract/preflight"
	projectdocscontract "issueops/internal/contract/projectdocs"
	"os"
	"path/filepath"

	"issueops/cmd/issueops/apidoc"
	"issueops/cmd/issueops/selfworkflow"
)

const skillName = "atomic-commit-push"

var Version = "dev"

var IssueOpsRoot = func() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
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
	return IssueOpsRoot()
}

var ReadHarnessFile = func(parts ...string) (string, error) {
	path := filepath.Join(append([]string{IssueOpsRoot()}, parts...)...)
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
	return map[string]any{"ok": true, "name": "issueops_cli_mcp_compatibility", "mcp_tools": toolNames}
}

var SelfVerify = func(selfworkflow.SelfVerifyRequest) (selfworkflow.SelfAugmentResult, error) {
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loop run 연산은 composition root가 설치한다. MCP tool router는 loop 상태를
// 어디에 저장하는지 알지 않는다.
var (
	LoopStart         func(loopruncontract.StartLoopRequest) (loopruncontract.LoopRun, error)
	LoopRecordAttempt func(loopID string, req loopruncontract.RecordAttemptRequest) (loopruncontract.LoopRun, error)
	LoopStop          func(loopID string, success bool, reason string) (loopruncontract.LoopRun, error)
	LoopStatus        func(loopID string) (loopruncontract.StatusResult, error)
)

// gates ledger 연산도 composition root가 설치한다. policy 게이트 실행
// (gates_check)은 주입된 adapter 함수를 통해서만 일어난다.
var (
	GatesCheck   func(gatescontract.CheckRequest) (gatescontract.CheckResult, error)
	GatesInit    func(gatescontract.InitRequest) (gatescontract.InitResult, error)
	GatesAbandon func(gatescontract.AbandonRequest) (gatescontract.AbandonResult, error)
)

// channel 연산도 composition root가 설치한다.
var (
	ChannelSend func(channelcontract.SendRequest) (channelcontract.SendResult, error)
	ChannelRecv func(channelcontract.RecvRequest) (channelcontract.RecvResult, error)
)

// project docs 연산은 composition root가 설치한다.
var (
	RouteProjectDocs       func(repoRoot, task string) (projectdocscontract.ProjectDocsRouteResult, error)
	ReadProjectDoc         func(repoRoot, relPath string) (projectdocscontract.ProjectDocsReadResult, error)
	ReviseProjectDoc       func(projectdocscontract.ProjectDocsReviseRequest) (projectdocscontract.ProjectDocsReviseResult, error)
	AppendProjectDocsEntry func(projectdocscontract.ProjectDocsAppendRequest) (projectdocscontract.ProjectDocsAppendResult, error)
)

// GitPreflight와 ListSkills는 composition root가 설치한다. MCP tool router는
// git 실행이나 skill 디렉터리 탐색을 스스로 하지 않는다.
var GitPreflight func(target, issueOpsRoot string) preflightcontract.PreflightResult

var ListSkills func(root, skillName string) []inspectcontract.SkillInfo
