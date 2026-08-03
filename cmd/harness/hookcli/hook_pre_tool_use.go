package hookcli

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookadapter "agent-harness/internal/adapter/hook"
	"agent-harness/internal/core"
	coreinstall "agent-harness/internal/core/install"
	issueopscore "agent-harness/internal/core/issueops"
)

func runHookPreToolUse(args []string) error {
	fs := flag.NewFlagSet("hook pre-tool-use", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	host := fs.String("host", "", "hook host (codex or claude); controls host-specific block schema")
	enforceWorktree := fs.Bool("enforce-worktree", false, "block mutating tool targets outside HARNESS_EXPECTED_WORKTREE or --expected-worktree")
	enforceKoreanRemote := fs.Bool("enforce-korean-remote-artifacts", false, "block gh issue/pr create/edit when title/body fail the IssueOps Korean remote artifact gate")
	enforceVCSLinking := fs.Bool("enforce-vcs-issue-linking", false, "block gh/glab remote create without labels and issue create/edit bodies that violate provider-specific IssueOps linking rules")
	enforceGitOpsKubectl := fs.Bool("enforce-gitops-kubectl", false, "block direct mutating kubectl commands so cluster changes go through GitOps")
	enforceStagedChecks := fs.Bool("enforce-staged-checks", false, "ask before broad lint/format checks that should use staged or changed-file scope")
	expectedWorktree := fs.String("expected-worktree", os.Getenv("HARNESS_EXPECTED_WORKTREE"), "expected isolated IssueOps worktree path")
	sourceCheckout := fs.String("source-checkout", os.Getenv("HARNESS_SOURCE_CHECKOUT"), "source checkout path for diagnostics")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
	}
	payloadHost := strings.TrimSpace(hookinput.HostFromHookInput(stdin))
	flagHost := strings.TrimSpace(*host)
	nativeHost := firstNonEmptyHookValue(payloadHost, flagHost)
	if nativeHost == "" {
		nativeHost = string(hookadapter.HostCodex)
	}
	diagnostic, runtimeErr := DiagnoseCurrentNativeRuntime()
	if reason, blocked := coreinstall.NativeRuntimeDiagnosticMessage(diagnostic, runtimeErr); blocked {
		if *jsonOut {
			return printJSON(diagnostic)
		}
		markHookMetricBlocked()
		return printJSON(hookadapter.Resolve(nativeHost).FormatBlock(reason))
	}
	processAncestry, _ := issueopscore.ObserveNativeProcessAncestry(os.Getpid())
	result := core.BuildLifecyclePreToolUseDecision(core.HookToolUseLifecycleRequest{
		Repo:                  parsedRepo,
		CWD:                   hookinput.CWDFromHookInput(stdin),
		Host:                  nativeHost,
		SessionID:             hookinput.SessionIDFromHookInput(stdin),
		AgentID:               hookinput.AgentIDFromHookInput(stdin),
		Tool:                  hookinput.ToolNameFromHookInput(stdin),
		ToolInput:             hookinput.ToolInputFromHookInput(stdin),
		Paths:                 hookinput.PathsFromHookInput(stdin),
		Command:               hookinput.CommandFromHookInput(stdin),
		ProjectPath:           hookinput.ProjectPathFromHookInput(stdin),
		NativeProcessAncestry: processAncestry,
		Source:                "pre-tool-use",
		EnforceWorktree:       *enforceWorktree,
		EnforceKoreanRemote:   *enforceKoreanRemote,
		EnforceVCSLinking:     *enforceVCSLinking,
		EnforceGitOpsKubectl:  *enforceGitOpsKubectl,
		EnforceStagedChecks:   *enforceStagedChecks,
		ExpectedWorktree:      resolveExpectedWorktree(*expectedWorktree, parsedRepo),
		SourceCheckout:        *sourceCheckout,
	})
	if payloadHost != "" && flagHost != "" && !strings.EqualFold(payloadHost, flagHost) {
		result.Decision = "block"
		result.Reason = "explicit payload and CLI hosts conflict"
	}
	if *jsonOut {
		return printJSON(result)
	}
	ho := hookadapter.Resolve(strings.TrimSpace(*host))
	if result.Decision == "block" || result.Decision == "ask" {
		// "ask" 결정은 항상 host 독립적인 hookSpecificOutput 형식을 쓴다.
		// "block" 결정은 host별로 다르다. Claude는 hookSpecificOutput을,
		// Codex는 평평한 decision/reason 객체를 쓴다.
		if result.Decision == "ask" {
			markHookMetricAsked()
			return printJSON(ho.FormatAsk(result.Reason))
		}
		markHookMetricBlocked()
		return printJSON(ho.FormatBlock(hookDenyReason(result)))
	}
	// PreToolUse는 모든 tool 호출 전에 실행되는 critical path다. 공용 harness
	// hook은 기본적으로 저렴하고 non-blocking으로 유지한다.
	return printJSON(ho.FormatNoop())
}

func hookDenyReason(result core.HookPreToolUseDecisionResult) string {
	if result.Deny == nil {
		return result.Reason
	}
	encoded, err := json.Marshal(result.Deny)
	if err != nil {
		return result.Reason
	}
	return string(encoded)
}

func firstNonEmptyHookValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// resolveExpectedWorktree는 명시적으로 제공된 expected worktree(flag 또는
// HARNESS_EXPECTED_WORKTREE)를 반환한다. 영속된 session-binding fallback은
// lifecycle MCP guard에 있으며, 그 guard는 session이 bound branch에 있는지도
// 확인한다. 여기서 branch guard 없이 fallback을 중복하면 한 cycle의 binding이
// 같은 repo의 무관한 작업을 막을 수 있다.
func resolveExpectedWorktree(explicit, repo string) string {
	_ = repo
	return strings.TrimSpace(explicit)
}
