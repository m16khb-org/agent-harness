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
	processAncestry, _ := issueopscore.ObserveNativeProcessAncestryV1(os.Getpid())
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
		// "ask" decisions always use hookSpecificOutput format (host-independent).
		// "block" decisions differ by host: Claude uses hookSpecificOutput, Codex
		// uses a flat decision/reason object.
		if result.Decision == "ask" {
			markHookMetricAsked()
			return printJSON(ho.FormatAsk(result.Reason))
		}
		markHookMetricBlocked()
		return printJSON(ho.FormatBlock(hookDenyReason(result)))
	}
	// PreToolUse is on the critical path before every tool call. Keep the shared
	// harness hook cheap and non-blocking by default.
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

// resolveExpectedWorktree returns the explicitly provided expected worktree
// (flag or HARNESS_EXPECTED_WORKTREE). The persisted session-binding fallback
// lives in the lifecycle MCP guard, which also checks that the session is on
// the bound branch — duplicating the fallback here without that branch guard
// would let one cycle's binding block unrelated work in the same repo.
func resolveExpectedWorktree(explicit, repo string) string {
	_ = repo
	return strings.TrimSpace(explicit)
}
