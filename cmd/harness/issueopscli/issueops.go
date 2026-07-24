package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	"agent-harness/cmd/harness/issueopscli/feedbackcleanup"
	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"agent-harness/internal/adapter/provider"
	"flag"
	"fmt"
	"strings"
)

// issueOpsSubcommands는 `issueops <subcommand>`의 디스패치 레지스트리다.
// 라우팅은 단일 map 조회이므로 subcommand 추가는 분기가 많은 switch를 키우는
// 대신 항목 하나와 핸들러 하나를 더하는 것으로 끝난다.
var issueOpsSubcommands = map[string]func([]string) error{
	"start":           runIssueOpsStart,
	"status":          runIssueOpsStatus,
	"intent":          runIssueOpsIntent,
	"plan-prep":       runIssueOpsPlanPrep,
	"design":          runIssueOpsDesign,
	"compatibility":   runIssueOpsCompatibility,
	"devils-advocate": runIssueOpsDevilsAdvocate,
	"domain-review":   runIssueOpsDomainReview,
	"ai-slop-clean":   runIssueOpsAISlopClean,
	"regress":         runIssueOpsRegress,
	"link-issue":      runIssueOpsLinkIssue,
	"link-plan":       runIssueOpsLinkPlan,
	"link-worktree":   runIssueOpsLinkWorktree,
	"link-child":      runIssueOpsLinkChild,
	"link-related":    runIssueOpsLinkRelated,
	"child":           runIssueOpsChild,
	"branch":          runIssueOpsBranch,
	"phase":           runIssueOpsPhase,
	"record-routing":  runIssueOpsRecordRouting,
	"routing-score":   runIssueOpsRoutingScore,
	"feedback":        runIssueOpsFeedback,
	"cleanup":         runIssueOpsCleanup,
	"benchmark":       func(args []string) error { return benchmarkcmd.Run(args) },
	"remote":          func(args []string) error { return remotecmd.Run(args, issueOpsRemoteDeps()) },
	"remote-score": func(args []string) error {
		return remotecmd.Run(append([]string{"score"}, args...), issueOpsRemoteDeps())
	},
	"prune":        runIssueOpsPrune,
	"pr-readiness": runIssueOpsPRReadiness,
	"decision":     runIssueOpsDecision,
	"execution":    runIssueOpsExecution,
	"reset-legacy": runIssueOpsResetLegacy,
}

func runIssueOps(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		issueOpsUsage()
		return nil
	}
	handler, ok := issueOpsSubcommands[args[0]]
	if !ok {
		return fmt.Errorf("unknown issueops subcommand %q%s", args[0], suggestIssueOpsSubcommand(args[0]))
	}
	return handler(args[1:])
}

// issueOpsConceptHints는 에이전트가 CLI subcommand으로 자주 오인하는 IssueOps
// 도메인 어휘(lifecycle phase 이름, 결정 동사, ledger artifact 이름)를 매핑한다.
// skill 문서는 생생한 명사(grill, split, domain)를 쓰지만 CLI는 일반 동사(phase,
// remote, link-related)를 쓴다. 이 힌트가 그 이름 간극을 메워, 잘못 추측해도 맨
// "unknown subcommand" 대신 실행 가능한 안내를 내놓는다.
var issueOpsConceptHints = map[string]string{
	"grill":     "did you mean `issueops phase --to grill`? (grill is a lifecycle phase, not a subcommand)",
	"problem":   "did you mean `issueops phase --to problem`? (problem is a lifecycle phase, not a subcommand)",
	"implement": "did you mean `issueops phase --to implement`? (implement is a lifecycle phase, not a subcommand)",
	"split":     "did you mean `issueops remote create-child` or `issueops link-related --type splits-from`? (split is a breakdown decision, not a subcommand)",
}

// suggestIssueOpsSubcommand는 알 수 없는 subcommand에 대한 제안 접미사를 돌려준다.
// 알려진 phase/decision 단어에는 concept hint를, 그 외에는 실제 subcommand
// 레지스트리에 대한 prefix 일치를 쓴다. 쓸 만한 제안이 없으면 ""를 돌려준다.
func suggestIssueOpsSubcommand(input string) string {
	if hint, ok := issueOpsConceptHints[input]; ok {
		return "; " + hint
	}
	var matches []string
	for name := range issueOpsSubcommands {
		if strings.HasPrefix(name, input) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 1 {
		return fmt.Sprintf("; did you mean `%s`?", matches[0])
	}
	return ""
}

func issueOpsRemoteDeps() remotecmd.Deps {
	return remotecmd.Deps{
		PrintJSON:   printJSON,
		PrintResult: printIssueOpsResult,
		PrintError:  printIssueOpsErrorJSON,
		VerifyLive:  verifyIssueOpsRemoteArtifactLive,
	}
}

func parseIssueOpsFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func runIssueOpsFeedback(args []string) error {
	if len(args) > 0 && args[0] == "resolve" {
		return runIssueOpsFeedbackResolve(args[1:])
	}
	return feedbackcleanup.RunFeedback(args, issueOpsFeedbackCleanupDeps())
}

func runIssueOpsCleanup(args []string) error {
	return feedbackcleanup.RunCleanup(args, issueOpsFeedbackCleanupDeps())
}

func issueOpsFeedbackCleanupDeps() feedbackcleanup.Deps {
	return feedbackcleanup.Deps{
		ParseFlags:   parseIssueOpsFlags,
		PrintResult:  printIssueOpsResult,
		PrintJSON:    printJSON,
		PrintError:   printIssueOpsErrorJSON,
		VerifyMerged: verifyIssueOpsRemoteArtifactMergedLive,
		Provider:     provider.Resolve,
	}
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return feedbackcleanup.CleanupMerged(id, requested, issueOpsFeedbackCleanupDeps())
}
