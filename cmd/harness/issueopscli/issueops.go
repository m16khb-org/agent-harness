package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	"agent-harness/cmd/harness/issueopscli/feedbackcleanup"
	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/orphancleanup"
	"agent-harness/internal/adapter/operationalhealth"
	"agent-harness/internal/adapter/orca"
	corehealth "agent-harness/internal/domain/operationalhealth"
	"agent-harness/internal/port"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/domain/commandparse"
)

// issueOpsSubcommands는 `issueops <subcommand>`의 디스패치 레지스트리다.
// 라우팅은 단일 map 조회이므로 subcommand 추가는 분기가 많은 switch를 키우는
// 대신 항목 하나와 핸들러 하나를 더하는 것으로 끝난다.
var issueOpsSubcommands = map[string]func([]string) error{
	"start":                 runIssueOpsStart,
	"status":                runIssueOpsStatus,
	"list":                  runIssueOpsList,
	"intent":                runIssueOpsIntent,
	"plan-prep":             runIssueOpsPlanPrep,
	"design":                runIssueOpsDesign,
	"compatibility":         runIssueOpsCompatibility,
	"devils-advocate":       runIssueOpsDevilsAdvocate,
	"domain-review":         runIssueOpsDomainReview,
	"ai-slop-clean":         runIssueOpsAISlopClean,
	"regress":               runIssueOpsRegress,
	"link-issue":            runIssueOpsLinkIssue,
	"link-plan":             runIssueOpsLinkPlan,
	"link-worktree":         runIssueOpsLinkWorktree,
	"link-child":            runIssueOpsLinkChild,
	"link-related":          runIssueOpsLinkRelated,
	"child":                 runIssueOpsChild,
	"artifact":              runIssueOpsArtifact,
	"implementation-review": runIssueOpsImplementationReview,
	"branch":                runIssueOpsBranch,
	"phase":                 runIssueOpsPhase,
	"record-routing":        runIssueOpsRecordRouting,
	"routing-score":         runIssueOpsRoutingScore,
	"feedback":              runIssueOpsFeedback,
	"cleanup":               runIssueOpsCleanup,
	"benchmark":             func(args []string) error { return benchmarkcmd.Run(args) },
	"remote":                func(args []string) error { return remotecmd.Run(args, issueOpsRemoteDeps()) },
	"remote-score": func(args []string) error {
		return remotecmd.Run(append([]string{"score"}, args...), issueOpsRemoteDeps())
	},
	"prune":        runIssueOpsPrune,
	"pr-readiness": runIssueOpsPRReadiness,
	"decision":     runIssueOpsDecision,
	"execution":    runIssueOpsExecution,
}

func runIssueOps(args []string) error {
	return runIssueOpsWithDependencies(args, Dependencies{})
}

func dispatchIssueOps(args []string) error {
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

func runIssueOpsWithDependencies(args []string, deps Dependencies) error {
	clean, generated, err := prepareGeneratedCommandInvocation(args, deps)
	if err == nil && generated {
		err = requireGeneratedOwnerProcessCWD(clean)
	}
	if err != nil {
		if issueOpsJSONRequested(args) {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	args = clean
	if len(args) > 0 {
		switch args[0] {
		case "execution":
			return runIssueOpsExecutionWithDependencies(args[1:], deps)
		case "feedback":
			return runIssueOpsFeedbackWithDependencies(args[1:], deps)
		case "cleanup":
			return runIssueOpsCleanupWithDependencies(args[1:], deps)
		case "remote":
			return remotecmd.Run(args[1:], issueOpsRemoteDepsWithPublication(deps.Publication))
		case "remote-score":
			return remotecmd.Run(append([]string{"score"}, args[1:]...), issueOpsRemoteDepsWithPublication(deps.Publication))
		}
	}
	return dispatchIssueOps(args)
}

func requireGeneratedOwnerProcessCWD(args []string) error {
	command, ok := commandparse.ParseExactIssueOpsArgs(args)
	if !ok {
		return nil
	}
	flags, ok := commandparse.ExactIssueOpsOwnerMutation(command)
	if !ok {
		return nil
	}
	values := flags["--cwd"]
	if len(values) != 1 {
		return fmt.Errorf("generated owner mutation requires one exact --cwd")
	}
	processCWD, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("observe generated owner mutation actual process cwd: %w", err)
	}
	if !sameExistingIssueOpsPath(processCWD, values[0]) {
		return fmt.Errorf("generated owner mutation actual process cwd must match --cwd before mutation")
	}
	return nil
}

func sameExistingIssueOpsPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(strings.TrimSpace(left))
	rightAbs, rightErr := filepath.Abs(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil || strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	leftResolved, leftErr := filepath.EvalSymlinks(leftAbs)
	rightResolved, rightErr := filepath.EvalSymlinks(rightAbs)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftResolved) == filepath.Clean(rightResolved)
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
	return issueOpsRemoteDepsWithPublication(issueops.RemotePublicationHandlers{})
}

func issueOpsRemoteDepsWithPublication(publication issueops.RemotePublicationHandlers) remotecmd.Deps {
	return remotecmd.Deps{
		PrintJSON:    printJSON,
		PrintResult:  printIssueOpsResult,
		PrintError:   printIssueOpsErrorJSON,
		VerifyLive:   verifyIssueOpsRemoteArtifactLive,
		VerifyMerged: verifyIssueOpsRemoteArtifactMergedLive,
		Publication:  publication,
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
	return runIssueOpsFeedbackWithDependencies(args, Dependencies{})
}

func runIssueOpsFeedbackWithDependencies(args []string, deps Dependencies) error {
	if len(args) > 0 && args[0] == "resolve" {
		return runIssueOpsFeedbackResolve(args[1:])
	}
	return feedbackcleanup.RunFeedback(args, issueOpsFeedbackCleanupDeps(deps.Provenance))
}

func runIssueOpsCleanup(args []string) error {
	return runIssueOpsCleanupWithDependencies(args, Dependencies{})
}

func runIssueOpsCleanupWithDependencies(args []string, deps Dependencies) error {
	return feedbackcleanup.RunCleanup(args, issueOpsFeedbackCleanupDeps(deps.Provenance))
}

// normalizeOrcaRemoveWorktreeErr는 orca 워크트리 회수 오류를 멱등 계약으로
// 정규화한다: "이미 없음"(typed not_found 계열)은 제거 목표가 이미 달성된
// 상태이므로 성공이다(#97 — cleanup finish 재실행 수렴의 전제).
func normalizeOrcaRemoveWorktreeErr(err error) error {
	if err == nil {
		return nil
	}
	var orcaErr *port.OrcaError
	if errors.As(err, &orcaErr) && strings.Contains(strings.ToLower(orcaErr.Code), "not_found") {
		return nil
	}
	// 폴백: orca CLI 산문 메시지 매칭. 문구/로캘 변경에 취약하므로
	// 타입드 코드가 항상 우선이다(C2-F5).
	if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "unknown worktree") {
		return nil
	}
	return err
}

func issueOpsFeedbackCleanupDeps(provenance provenanceport.Observer) feedbackcleanup.Deps {
	orphanDeps := issueOpsOrphanCleanupDeps()
	orcaClient := orca.New()
	return feedbackcleanup.Deps{
		// cleanup finish ② 단계: orca 회수. "이미 없음"은 멱등 계약상 성공.
		RemoveOrcaWorktree: func(ctx context.Context, worktreeID string) error {
			return normalizeOrcaRemoveWorktreeErr(orcaClient.RemoveWorktree(ctx, worktreeID, false))
		},
		// cleanup abandon pending_intent_safe 게이트: sealed marker로 orca
		// 인벤토리를 실조회한다. 조회 전용이며 mutation은 부르지 않는다.
		OrcaIntent: orca.NewExecution(),
		// cleanup abandon orca_resources_absent 게이트: orca 자원 잔여를
		// 실조회한다. 같은 provisioner가 owner 인벤토리도 제공한다(#136).
		OrcaOwner:    orca.NewExecution(),
		Provenance:   provenance,
		ParseFlags:   parseIssueOpsFlags,
		PrintResult:  printIssueOpsResult,
		PrintJSON:    printJSON,
		PrintError:   printIssueOpsErrorJSON,
		VerifyMerged: verifyIssueOpsRemoteArtifactMergedLive,
		// cleanup remote-branch 게이트 ⑧·⑨·⑩의 단일 readback 표면.
		VerifyMergedHead: verifyIssueOpsRemoteArtifactMergedHeadLive,
		// cleanup abandon의 artifact 게이트는 미병합을 요구하므로 조회 실패와
		// 미병합을 구분하는 별도 관측 표면을 쓴다(#342).
		ObserveArtifactMerged: observeIssueOpsRemoteArtifactMergedLive,
		Provider:              Resolve,
		OrphanPreview: func(ctx context.Context, request orphancleanup.Request) (orphancleanup.Result, error) {
			return orphancleanup.Preview(ctx, request, orphanDeps)
		},
		OrphanApply: func(ctx context.Context, request orphancleanup.Request, apply orphancleanup.ApplyRequest) (orphancleanup.Result, error) {
			return orphancleanup.Apply(ctx, request, apply, orphanDeps)
		},
	}
}

func issueOpsOrphanCleanupDeps() orphancleanup.Dependencies {
	collector := operationalhealth.Collector{Git: operationalhealth.ExecGitRunner{}, Orca: orca.New()}
	return orphancleanup.Dependencies{
		Collect: func(ctx context.Context, repo string) (corehealth.Snapshot, error) {
			return collector.Collect(ctx, repo), nil
		},
		VerifyMerged: verifyIssueOpsRemoteArtifactMergedLive,
	}
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return feedbackcleanup.CleanupMerged(id, requested, issueOpsFeedbackCleanupDeps(nil))
}
