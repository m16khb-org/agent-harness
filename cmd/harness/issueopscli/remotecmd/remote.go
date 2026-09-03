package remotecmd

import (
	artifacttemplate "agent-harness/internal/domain/artifacttemplate"
	issueopsremote "agent-harness/internal/domain/issueopsremote"
	policydomain "agent-harness/internal/domain/policy"
	port "agent-harness/internal/port"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
)

type Deps struct {
	PrintJSON              func(any) error
	PrintResult            func(issueopscontract.IssueOpsRecord, bool, error) error
	PrintError             func(error) error
	VerifyLive             func(issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error
	VerifyLiveContext      func(context.Context, issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error
	VerifyMerged           func(issueopscontract.IssueOpsRemoteArtifactVerification) error
	ObserveProcessAncestry func(int) ([]issueopscontract.NativeProcessReceipt, error)
	Publication            PublicationHandlers
}

func Run(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage:")
		fmt.Println("  agent-harness issueops remote score --input PATH [--judge none|prompt|file] [--judge-file PATH] [--json]")
		fmt.Println("  agent-harness issueops remote render-template --kind issue|child|pr --template KIND --title TEXT --provider github|gitlab --field key=value... [--score-file PATH] [--json]")
		fmt.Println("  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --target-branch BRANCH --label LABEL --assignee USER [--json]")
		fmt.Println("  agent-harness issueops remote create-issue --id ID --title TEXT [--provider github|gitlab] [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote reconcile-issue --id ID [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote create-child --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... --host codex|claude|omo --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote create-pr --id ID --expected-generation N --title TEXT --head BRANCH --base BRANCH [--body TEXT] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... --host codex|claude|omo --session-id SESSION [--agent-id ID] --session-pid PID --session-started-at RFC3339 --session-executable PATH --cwd WORKER_PATH [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote sync-graph --id ID [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote sync-issue --id ID [--provider github|gitlab] [--url CHILD_URL] [--body TEXT|--body-file PATH] [--expected-body-sha256 SHA] [--accept-remote-edits] [--host codex|claude|omo] [--session-id SESSION] [--agent-id ID] [--cwd WORKER_PATH] [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote sync-pr --id ID --expected-generation N [--provider github|gitlab] [--body TEXT|--body-file PATH] [--expected-body-sha256 SHA] [--accept-remote-edits] --host codex|claude|omo --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote reflect-devils-advocate --id ID [--provider github|gitlab] --host codex|claude|omo --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote reflect-completion --id ID [--provider github|gitlab] [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote close-issue --id ID [--provider github|gitlab] [--confirm] [--json]")
		return nil
	}
	if args[0] == "remote-score" {
		args[0] = "score"
	}
	if len(args) == 0 {
		return fmt.Errorf("unknown issueops remote subcommand")
	}
	switch args[0] {
	case "score":
		fs := flag.NewFlagSet("issueops remote score", flag.ContinueOnError)
		input := fs.String("input", "", "IssueOps remote scoring request JSON file")
		judge := fs.String("judge", "none", "judge backend: none, prompt, or file")
		judgeFile := fs.String("judge-file", "", "host-agent remote score result JSON path for --judge file")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		req, err := readIssueOpsRemoteScoringRequestFile(*input)
		if err != nil {
			if *jsonOut {
				if printErr := deps.printError(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *judge == "prompt" {
			result, promptErr := remoteDeps.RenderIssueOpsRemoteJudgePrompt(issueopsremote.IssueOpsRemoteLLMJudgeRequest{Request: req})
			if promptErr != nil {
				if *jsonOut {
					if printErr := deps.printError(promptErr); printErr != nil {
						return printErr
					}
				}
				return promptErr
			}
			if *jsonOut {
				return deps.printJSON(result)
			}
			fmt.Println(result.Prompt)
			return nil
		}
		var result issueopsremote.IssueOpsRemoteScoringResult
		switch *judge {
		case "file":
			result, err = readIssueOpsRemoteJudgeFile(*judgeFile)
		case "none":
			result, err = remoteDeps.ScoreIssueOpsRemoteCandidates(req)
		default:
			err = fmt.Errorf("unsupported issueops remote score judge %q", *judge)
		}
		if err != nil {
			if *jsonOut {
				if printErr := deps.printError(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *jsonOut {
			return deps.printJSON(result)
		}
		fmt.Printf("provider=%s threshold=%.2f related_issues=%d labels=%d\n", result.Provider, result.Threshold, len(result.SelectedRelatedIssues), len(result.SelectedLabels))
		for _, issue := range result.SelectedRelatedIssues {
			fmt.Printf("- related issue: %s score=%.2f\n", formatIssueOpsRemoteIssueRef(issue), issue.Score)
		}
		for _, label := range result.SelectedLabels {
			fmt.Printf("- label: %s score=%.2f\n", label.Name, label.Score)
		}
		return nil
	case "verify-artifact":
		fs := flag.NewFlagSet("issueops remote verify-artifact", flag.ContinueOnError)
		id := fs.String("id", "", "IssueOps id")
		provider := fs.String("provider", "", "remote provider: github or gitlab")
		kind := fs.String("kind", "", "remote artifact kind: pr or mr")
		url := fs.String("url", "", "remote PR/MR URL")
		targetBranch := fs.String("target-branch", "", "verified PR/MR target branch")
		var labels repeatedFlag
		var assignees repeatedFlag
		fs.Var(&labels, "label", "verified remote label; may be repeated")
		fs.Var(&labels, "labels", "verified remote label; may be repeated")
		fs.Var(&assignees, "assignee", "verified remote assignee; may be repeated")
		fs.Var(&assignees, "assignees", "verified remote assignee; may be repeated")
		host := fs.String("host", "", "native holder host")
		sessionID := fs.String("session-id", "", "native holder session id")
		agentID := fs.String("agent-id", "", "optional native holder agent id")
		cwd := fs.String("cwd", "", "canonical holder worktree cwd")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		req := issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
			Provider:     *provider,
			Kind:         *kind,
			URL:          *url,
			TargetBranch: *targetBranch,
			Labels:       labels,
			Assignees:    assignees,
		}
		_, err := remoteDeps.ValidateIssueOpsRemoteArtifactVerification(remoteDeps.IssueOpsStateRoot(), *id, req)
		var record issueopscontract.IssueOpsRecord
		if err == nil {
			err = deps.verifyLive(context.Background(), req)
		}
		if err == nil {
			var ancestry []issueopscontract.NativeProcessReceipt
			ancestry, err = deps.observeNativeProcessAncestry()
			if err == nil {
				record, err = remoteDeps.VerifyIssueOpsRemoteArtifactWithActor(remoteDeps.IssueOpsStateRoot(), *id, req, issueopscontract.IssueOpsActor{
					Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, NativeProcessAncestry: ancestry,
				})
			}
		}
		return deps.printResult(record, *jsonOut, err)
	case "render-template":
		return runRemoteRenderTemplate(args[1:], deps)
	case "create-issue":
		return runRemoteCreateIssue(context.Background(), args[1:], deps)
	case "reconcile-issue":
		return runRemoteReconcileIssue(context.Background(), args[1:], deps)
	case "create-child":
		return runRemoteCreateChild(args[1:], deps)
	case "create-pr":
		return runRemoteCreatePR(args[1:], deps)
	case "sync-graph":
		return runRemoteSyncGraph(args[1:], deps)
	case "sync-issue":
		return runRemoteSyncIssue(context.Background(), args[1:], deps)
	case "sync-pr":
		return runRemoteSyncPR(context.Background(), args[1:], deps)
	case "reflect-devils-advocate":
		return runRemoteReflectDevilsAdvocate(args[1:], deps)
	case "reflect-completion":
		return runRemoteReflectCompletion(args[1:], deps)
	case "close-issue":
		return runRemoteCloseIssue(args[1:], deps)
	default:
		return fmt.Errorf("unknown issueops remote subcommand %q", args[0])
	}
}

func runRemoteReflectDevilsAdvocate(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reflect-devils-advocate", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	host := fs.String("host", "", "native holder host")
	sessionID := fs.String("session-id", "", "native holder session id")
	agentID := fs.String("agent-id", "", "optional native holder agent id")
	cwd := fs.String("cwd", "", "canonical holder worktree cwd")
	confirm := fs.Bool("confirm", false, "write to the remote issue; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, remoteDeps.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	prov, err := Resolve(providerName)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	ancestry, err := deps.observeNativeProcessAncestry()
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	_, result, err := remoteDeps.ReflectDevilsAdvocateFindingsWithActor(remoteDeps.IssueOpsStateRoot(), *id, *confirm, prov, issueopscontract.IssueOpsActor{
		Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, NativeProcessAncestry: ancestry,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.Updated {
		fmt.Printf("reflected devil's-advocate findings: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

// resolveRemoteCompletionInputs는 completion 계열 명령의 공통 전제(레코드,
// provider, provider readback 머지 검증)를 fail-closed로 해석한다. readback
// 실패는 "판정 불가"이며 강등 없이 에러다(설계 v5 WS3).
func resolveRemoteCompletionInputs(deps Deps, id, providerOverride string) (issueopscontract.IssueOpsRecord, port.IssueProvider, error) {
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, nil, err
	}
	providerName := firstNonEmptyMain(providerOverride, remoteDeps.ResolveRecordProvider(record))
	if providerName == "" {
		return issueopscontract.IssueOpsRecord{}, nil, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	prov, err := Resolve(providerName)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, nil, err
	}
	if record.RemoteArtifact == nil {
		return issueopscontract.IssueOpsRecord{}, nil, fmt.Errorf("cannot verify merge evidence before a verified remote artifact")
	}
	if deps.VerifyMerged == nil {
		return issueopscontract.IssueOpsRecord{}, nil, fmt.Errorf("merge verification is not configured")
	}
	if err := deps.VerifyMerged(*record.RemoteArtifact); err != nil {
		return issueopscontract.IssueOpsRecord{}, nil, fmt.Errorf("merge evidence readback failed (refusing to continue): %w", err)
	}
	return record, prov, nil
}

func runRemoteReflectCompletion(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reflect-completion", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	confirm := fs.Bool("confirm", false, "write to the remote issue; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	_, prov, err := resolveRemoteCompletionInputs(deps, *id, *providerOverride)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	_, result, err := remoteDeps.ReflectIssueCompletion(remoteDeps.IssueOpsStateRoot(), *id, true, *confirm, prov)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.Updated {
		fmt.Printf("reflected completion section: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func runRemoteCloseIssue(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote close-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	confirm := fs.Bool("confirm", false, "close the remote issue; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	_, prov, err := resolveRemoteCompletionInputs(deps, *id, *providerOverride)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	_, result, err := remoteDeps.CloseIssueOpsRemoteIssue(remoteDeps.IssueOpsStateRoot(), *id, true, *confirm, prov)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.Closed {
		fmt.Printf("closed issue: %s\n", result.IssueURL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func (deps Deps) printJSON(v any) error {
	if deps.PrintJSON != nil {
		return deps.PrintJSON(v)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (deps Deps) printError(err error) error {
	if deps.PrintError != nil {
		return deps.PrintError(err)
	}
	return err
}

func (deps Deps) printResult(record issueopscontract.IssueOpsRecord, jsonOut bool, err error) error {
	if deps.PrintResult != nil {
		return deps.PrintResult(record, jsonOut, err)
	}
	return err
}

func (deps Deps) printErrorResult(jsonOut bool, err error) error {
	if err == nil {
		return nil
	}
	if jsonOut {
		if printErr := deps.printError(err); printErr != nil {
			return printErr
		}
	}
	return err
}

func (deps Deps) verifyLive(ctx context.Context, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	if deps.VerifyLiveContext != nil {
		return deps.VerifyLiveContext(ctx, req)
	}
	if deps.VerifyLive != nil {
		return deps.VerifyLive(req)
	}
	return fmt.Errorf("live remote artifact verifier is not configured")
}

func durableIssueCreateFailure(err error) string {
	if err == nil {
		return ""
	}
	const maxBytes = 2048
	diagnostic := policydomain.RedactDiagnostic(strings.TrimSpace(err.Error()))
	if len(diagnostic) > maxBytes {
		diagnostic = diagnostic[:maxBytes]
	}
	return diagnostic
}

func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func normalizeRemoteCreateMetadata(labels, assignees repeatedFlag) (repeatedFlag, repeatedFlag) {
	return repeatedFlag(issueopsremote.CleanValues(labels)), repeatedFlag(issueopsremote.CleanValues(assignees))
}

func formatIssueOpsRemoteIssueRef(issue issueopsremote.IssueOpsRemoteScoredItem) string {
	ref := firstNonEmptyMain(issue.ID, issue.URL)
	title := strings.TrimSpace(issue.Title)
	if title == "" {
		return firstNonEmptyMain(ref, issue.Title)
	}
	if ref == "" {
		return title
	}
	return fmt.Sprintf("%s (%s)", ref, title)
}

func readIssueOpsRemoteScoringRequestFile(path string) (issueopsremote.IssueOpsRemoteScoringRequest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return issueopsremote.IssueOpsRemoteScoringRequest{}, fmt.Errorf("input is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return issueopsremote.IssueOpsRemoteScoringRequest{}, err
	}
	var req issueopsremote.IssueOpsRemoteScoringRequest
	req, err = remoteDeps.DecodeIssueOpsRemoteScoringRequest(b)
	if err != nil {
		return issueopsremote.IssueOpsRemoteScoringRequest{}, fmt.Errorf("parse input file %s: %w", path, err)
	}
	return req, nil
}

func readIssueOpsRemoteJudgeFile(path string) (issueopsremote.IssueOpsRemoteScoringResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return issueopsremote.IssueOpsRemoteScoringResult{}, fmt.Errorf("judge-file is required for --judge file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return issueopsremote.IssueOpsRemoteScoringResult{}, err
	}
	result, err := remoteDeps.DecodeIssueOpsRemoteJudgeJSON(b)
	if err != nil {
		return issueopsremote.IssueOpsRemoteScoringResult{}, fmt.Errorf("parse judge file %s: %w", path, err)
	}
	return result, nil
}

func runRemoteRenderTemplate(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote render-template", flag.ContinueOnError)
	kind := fs.String("kind", "", "artifact kind: issue, child, or pr")
	template := fs.String("template", "", "template kind")
	providerName := fs.String("provider", "", "remote provider: github or gitlab")
	title := fs.String("title", "", "artifact title")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	var fields repeatedFlag
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	fieldMap, err := artifacttemplate.ParseFieldAssignments(fields)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	scoreSummary, err := readScoreSummaryFile(*scoreFile)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	result := artifacttemplate.Render(artifacttemplate.IssueOpsTemplateInput{
		Kind:         artifacttemplate.IssueOpsArtifactKind(*kind),
		Template:     artifacttemplate.IssueOpsTemplateKind(*template),
		Provider:     *providerName,
		Title:        *title,
		Fields:       fieldMap,
		ScoreSummary: scoreSummary,
	})
	if *jsonOut {
		return deps.printJSON(result)
	}
	fmt.Printf("# %s\n\n%s\n", result.Title, result.Body)
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	if len(result.MissingRequiredFields) > 0 {
		fmt.Printf("missing_required_fields: %s\n", strings.Join(result.MissingRequiredFields, ","))
	}
	return nil
}

func firstNonEmptyMain(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
