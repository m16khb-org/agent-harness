package remotecmd

import (
	artifacttemplate "agent-harness/internal/domain/artifacttemplate"
	port "agent-harness/internal/port"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/provider"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/issueopsremote"
)

type Deps struct {
	PrintJSON              func(any) error
	PrintResult            func(issueopscontract.IssueOpsRecord, bool, error) error
	PrintError             func(error) error
	VerifyLive             func(issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error
	VerifyMerged           func(issueopscontract.IssueOpsRemoteArtifactVerification) error
	ObserveProcessAncestry func(int) ([]issueopscontract.NativeProcessReceipt, error)
	Publication            issueopscore.RemotePublicationHandlers
}

func Run(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage:")
		fmt.Println("  agent-harness issueops remote score --input PATH [--judge none|prompt|file] [--judge-file PATH] [--json]")
		fmt.Println("  agent-harness issueops remote render-template --kind issue|child|pr --template KIND --title TEXT --provider github|gitlab --field key=value... [--score-file PATH] [--json]")
		fmt.Println("  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --target-branch BRANCH --label LABEL --assignee USER [--json]")
		fmt.Println("  agent-harness issueops remote create-issue --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote create-child --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... --host codex|claude --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote create-pr --id ID --expected-generation N --title TEXT --head BRANCH --base BRANCH [--body TEXT] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... --host codex|claude --session-id SESSION [--agent-id ID] --session-pid PID --session-started-at RFC3339 --session-executable PATH --cwd WORKER_PATH [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote sync-graph --id ID [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote reflect-devils-advocate --id ID [--provider github|gitlab] --host codex|claude --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]")
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
			result, promptErr := issueopscore.RenderIssueOpsRemoteJudgePrompt(issueopscore.IssueOpsRemoteLLMJudgeRequest{Request: req})
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
		var result issueopscore.IssueOpsRemoteScoringResult
		switch *judge {
		case "file":
			result, err = readIssueOpsRemoteJudgeFile(*judgeFile)
		case "none":
			result, err = issueopscore.ScoreIssueOpsRemoteCandidates(req)
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
		_, err := issueopscore.ValidateIssueOpsRemoteArtifactVerification(issueopscore.IssueOpsStateRoot(), *id, req)
		var record issueopscontract.IssueOpsRecord
		if err == nil {
			err = deps.verifyLive(req)
		}
		if err == nil {
			var ancestry []issueopscontract.NativeProcessReceipt
			ancestry, err = deps.observeNativeProcessAncestry()
			if err == nil {
				record, err = issueopscore.VerifyIssueOpsRemoteArtifactWithActor(issueopscore.IssueOpsStateRoot(), *id, req, issueopscore.IssueOpsActor{
					Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, NativeProcessAncestry: ancestry,
				})
			}
		}
		return deps.printResult(record, *jsonOut, err)
	case "render-template":
		return runRemoteRenderTemplate(args[1:], deps)
	case "create-issue":
		return runRemoteCreateIssue(args[1:], deps)
	case "create-child":
		return runRemoteCreateChild(args[1:], deps)
	case "create-pr":
		return runRemoteCreatePR(args[1:], deps)
	case "sync-graph":
		return runRemoteSyncGraph(args[1:], deps)
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
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, issueopscore.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	ancestry, err := deps.observeNativeProcessAncestry()
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	_, result, err := issueopscore.ReflectDevilsAdvocateFindingsWithActor(issueopscore.IssueOpsStateRoot(), *id, *confirm, prov, issueopscore.IssueOpsActor{
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
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, nil, err
	}
	providerName := firstNonEmptyMain(providerOverride, issueopscore.ResolveRecordProvider(record))
	if providerName == "" {
		return issueopscontract.IssueOpsRecord{}, nil, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	prov, err := provider.Resolve(providerName)
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
	_, result, err := issueopscore.ReflectIssueCompletion(issueopscore.IssueOpsStateRoot(), *id, true, *confirm, prov)
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
	_, result, err := issueopscore.CloseIssueOpsRemoteIssue(issueopscore.IssueOpsStateRoot(), *id, true, *confirm, prov)
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

func (deps Deps) verifyLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	if deps.VerifyLive != nil {
		return deps.VerifyLive(req)
	}
	return nil
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

func formatIssueOpsRemoteIssueRef(issue issueopscore.IssueOpsRemoteScoredItem) string {
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

func readIssueOpsRemoteScoringRequestFile(path string) (issueopscore.IssueOpsRemoteScoringRequest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return issueopscore.IssueOpsRemoteScoringRequest{}, fmt.Errorf("input is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return issueopscore.IssueOpsRemoteScoringRequest{}, err
	}
	var req issueopscore.IssueOpsRemoteScoringRequest
	req, err = issueopscore.DecodeIssueOpsRemoteScoringRequest(b)
	if err != nil {
		return issueopscore.IssueOpsRemoteScoringRequest{}, fmt.Errorf("parse input file %s: %w", path, err)
	}
	return req, nil
}

func readIssueOpsRemoteJudgeFile(path string) (issueopscore.IssueOpsRemoteScoringResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return issueopscore.IssueOpsRemoteScoringResult{}, fmt.Errorf("judge-file is required for --judge file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return issueopscore.IssueOpsRemoteScoringResult{}, err
	}
	result, err := issueopscore.DecodeIssueOpsRemoteJudgeJSON(b)
	if err != nil {
		return issueopscore.IssueOpsRemoteScoringResult{}, fmt.Errorf("parse judge file %s: %w", path, err)
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

func runRemoteCreateIssue(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "issue title")
	body := fs.String("body", "", "issue body (markdown)")
	bodyFile := fs.String("body-file", "", "issue body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	var fields repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, issueopscore.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	finalBody, err := resolveTemplateBody(resolveTemplateBodyRequest{
		Kind:      artifacttemplate.IssueOpsArtifactIssue,
		Template:  *template,
		Provider:  providerName,
		Title:     *title,
		Body:      *body,
		BodyFile:  *bodyFile,
		Fields:    fields,
		ScoreFile: *scoreFile,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err := validateConfirmRemoteCreate(*confirm, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	result, err := issueopscore.CreateRemoteIssue(port.IssueProviderCreateIssueRequest{
		Repo:      record.Repo,
		Title:     *title,
		Body:      finalBody,
		Labels:    labels,
		Assignees: assignees,
		Confirm:   *confirm,
	}, prov)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	// Mirror create-child's verification gate: once an issue is really created,
	// confirm the live issue carries the requested labels/assignees before the
	// command reports success. Without --confirm this is a dry-run preview only.
	if *confirm && strings.TrimSpace(result.URL) != "" {
		if err := deps.verifyLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
			Provider:  providerName,
			Kind:      "issue",
			URL:       result.URL,
			Labels:    labels,
			Assignees: assignees,
		}); err != nil {
			return deps.printErrorResult(*jsonOut, err)
		}
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.URL != "" {
		fmt.Printf("created: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func runRemoteCreateChild(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-child", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "child title")
	body := fs.String("body", "", "child body (markdown)")
	bodyFile := fs.String("body-file", "", "child body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	host := fs.String("host", "", "native holder host")
	sessionID := fs.String("session-id", "", "native holder session id")
	agentID := fs.String("agent-id", "", "optional native holder agent id")
	cwd := fs.String("cwd", "", "canonical holder worktree cwd")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	var fields repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		err := fmt.Errorf("cannot create child before linked parent issue")
		return deps.printErrorResult(*jsonOut, err)
	}
	// 우산 브랜치 게이트는 provider 호출 이전에 선다. 자식이 만들어진 뒤에는
	// 위상을 되돌릴 수 없고, 부모에 원격 artifact가 없는 채로 자식만 생기면
	// 정리 경로가 순환 차단된다(#129).
	if reason := issueopscore.UmbrellaBranchGateReason(record); reason != "" {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("%s", reason))
	}
	if err := validateCreateChildInputs(*title, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, issueopscore.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	finalBody, err := resolveTemplateBody(resolveTemplateBodyRequest{
		Kind:      artifacttemplate.IssueOpsArtifactChild,
		Template:  *template,
		Provider:  providerName,
		Title:     *title,
		Body:      *body,
		BodyFile:  *bodyFile,
		Fields:    fields,
		ScoreFile: *scoreFile,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	var actor issueopscore.IssueOpsActor
	if *confirm && record.Execution != nil {
		ancestry, observeErr := deps.observeNativeProcessAncestry()
		if observeErr != nil {
			return deps.printErrorResult(*jsonOut, observeErr)
		}
		actor = issueopscore.IssueOpsActor{
			Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, NativeProcessAncestry: ancestry,
		}
		if validateErr := issueopscore.ValidateIssueOpsMutationActor(issueopscore.IssueOpsStateRoot(), record.ID, actor); validateErr != nil {
			return deps.printErrorResult(*jsonOut, validateErr)
		}
	}
	result, err := issueopscore.CreateRemoteChild(port.IssueProviderCreateChildRequest{
		Repo:           record.Repo,
		ParentIssueURL: record.IssueURL,
		Title:          *title,
		Body:           finalBody,
		Labels:         labels,
		Assignees:      assignees,
		Confirm:        *confirm,
	}, prov)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *confirm {
		if !result.HierarchyVerified || strings.TrimSpace(result.ChildURL) == "" {
			err := fmt.Errorf("provider did not verify child hierarchy")
			return deps.printErrorResult(*jsonOut, err)
		}
		if _, err := issueopscore.LinkIssueOpsChildWithActor(issueopscore.IssueOpsStateRoot(), record.ID, result.ChildURL, *title, actor); err != nil {
			return deps.printErrorResult(*jsonOut, err)
		}
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.ChildURL != "" {
		fmt.Printf("created child: %s\n", result.ChildURL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func runRemoteCreatePR(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-pr", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "PR title")
	body := fs.String("body", "", "PR body (markdown)")
	bodyFile := fs.String("body-file", "", "PR body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	head := fs.String("head", "", "source branch")
	base := fs.String("base", "", "target branch")
	expectedGeneration := fs.Uint64("expected-generation", 0, "current execution lease generation")
	host := fs.String("host", "", "native owner host")
	sessionID := fs.String("session-id", "", "native owner session id")
	agentID := fs.String("agent-id", "", "native owner agent id")
	sessionPID := fs.Int("session-pid", 0, "native owner process id")
	sessionStartedAt := fs.String("session-started-at", "", "native owner process start identity")
	sessionExecutable := fs.String("session-executable", "", "native owner executable identity")
	cwd := fs.String("cwd", "", "canonical owner worker cwd")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	var fields repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, issueopscore.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	headBranch := firstNonEmptyMain(*head, record.Branch)
	baseBranch := *base
	if baseBranch == "" && record.BranchPrepare != nil {
		baseBranch = record.BranchPrepare.BaseBranch
	}
	finalBody, err := resolveTemplateBody(resolveTemplateBodyRequest{
		Kind:      artifacttemplate.IssueOpsArtifactPR,
		Template:  *template,
		Provider:  providerName,
		Title:     *title,
		Body:      *body,
		BodyFile:  *bodyFile,
		Fields:    fields,
		ScoreFile: *scoreFile,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err := validateConfirmRemoteCreate(*confirm, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	actor, err := deps.remoteNativeActor(*host, *sessionID, *agentID, *sessionPID, *sessionStartedAt, *sessionExecutable, *confirm)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	result, err := issueopscore.CreateRemotePullRequestWithHandler(context.Background(), issueopscore.IssueOpsStateRoot(), issueopscore.RemotePullRequestRequest{
		ID: record.ID, Provider: providerName, Title: *title, Body: finalBody,
		Head: headBranch, Base: baseBranch, Labels: labels, Assignees: assignees,
		ExpectedGeneration: *expectedGeneration,
		Actor:              actor,
		CWD:                *cwd, Confirm: *confirm,
	}, deps.Publication.Create)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.URL != "" {
		fmt.Printf("created: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func (deps Deps) remoteNativeActor(host, sessionID, agentID string, sessionPID int, sessionStartedAt, sessionExecutable string, observe bool) (issueopscontract.NativeActor, error) {
	actor := issueopscontract.NativeActor{
		Host: host, SessionID: sessionID, AgentID: agentID,
		SessionProcess: &issueopscontract.NativeProcessReceipt{PID: sessionPID, StartedAt: sessionStartedAt, Executable: sessionExecutable},
	}
	if !observe {
		return actor, nil
	}
	ancestry, err := deps.observeNativeProcessAncestry()
	if err != nil {
		return issueopscontract.NativeActor{}, err
	}
	actor.ProcessAncestry = ancestry
	return actor, nil
}

func (deps Deps) observeNativeProcessAncestry() ([]issueopscontract.NativeProcessReceipt, error) {
	observe := deps.ObserveProcessAncestry
	if observe == nil {
		observe = issueopscore.ObserveNativeProcessAncestry
	}
	ancestry, err := observe(os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("observe native process ancestry: %w", err)
	}
	if len(ancestry) == 0 {
		return nil, fmt.Errorf("observe native process ancestry: no process receipts returned")
	}
	return ancestry, nil
}

func validateCreateChildInputs(title string, labels, assignees []string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("child title is required")
	}
	if len(labels) == 0 {
		return fmt.Errorf("at least one child label is required")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one child assignee is required")
	}
	return nil
}

type resolveTemplateBodyRequest struct {
	Kind      artifacttemplate.IssueOpsArtifactKind
	Template  string
	Provider  string
	Title     string
	Body      string
	BodyFile  string
	Fields    []string
	ScoreFile string
}

func resolveTemplateBody(req resolveTemplateBodyRequest) (string, error) {
	body := strings.TrimSpace(req.Body)
	bodyFile := strings.TrimSpace(req.BodyFile)
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("body and body-file are mutually exclusive")
	}
	if bodyFile != "" {
		b, err := os.ReadFile(bodyFile)
		if err != nil {
			return "", err
		}
		body = strings.TrimSpace(string(b))
	}
	template := strings.TrimSpace(req.Template)
	if template == "" {
		return body, nil
	}
	fields, err := artifacttemplate.ParseFieldAssignments(req.Fields)
	if err != nil {
		return "", err
	}
	scoreSummary, err := readScoreSummaryFile(req.ScoreFile)
	if err != nil {
		return "", err
	}
	input := artifacttemplate.IssueOpsTemplateInput{
		Kind:         req.Kind,
		Template:     artifacttemplate.IssueOpsTemplateKind(template),
		Provider:     req.Provider,
		Title:        req.Title,
		Body:         body,
		Fields:       fields,
		ScoreSummary: scoreSummary,
	}
	result := artifacttemplate.Render(input)
	if len(result.Validation.Critical) > 0 {
		return "", fmt.Errorf("template validation failed: %s", strings.Join(result.Validation.Critical, ","))
	}
	return result.Body, nil
}

func validateConfirmRemoteCreate(confirm bool, labels, assignees []string) error {
	if !confirm {
		return nil
	}
	labels = remote.CleanValues(labels)
	assignees = remote.CleanValues(assignees)
	if len(labels) == 0 {
		return fmt.Errorf("at least one label is required with --confirm")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one assignee is required with --confirm")
	}
	return nil
}

func readScoreSummaryFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var result issueopscore.IssueOpsRemoteScoringResult
	if err := json.Unmarshal(b, &result); err != nil {
		return strings.TrimSpace(string(b)), nil
	}
	parts := []string{fmt.Sprintf("threshold %.2f", result.Threshold)}
	if len(result.SelectedRelatedIssues) > 0 {
		parts = append(parts, "선택 관련 이슈: "+joinScoredItems(result.SelectedRelatedIssues))
	}
	if len(result.RejectedRelatedIssues) > 0 {
		parts = append(parts, "거절 관련 이슈: "+joinScoredItems(result.RejectedRelatedIssues))
	}
	if len(result.SelectedLabels) > 0 {
		parts = append(parts, "선택 라벨: "+joinScoredItems(result.SelectedLabels))
	}
	if len(result.RejectedLabels) > 0 {
		parts = append(parts, "거절 라벨: "+joinScoredItems(result.RejectedLabels))
	}
	return strings.Join(parts, "\n"), nil
}

func joinScoredItems(items []issueopscore.IssueOpsRemoteScoredItem) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := firstNonEmptyMain(item.Name, item.ID, item.Title, item.URL)
		if name == "" {
			name = "unknown"
		}
		out = append(out, fmt.Sprintf("%s(%.2f)", name, item.Score))
	}
	return strings.Join(out, ", ")
}

func runRemoteSyncGraph(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote sync-graph", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	confirm := fs.Bool("confirm", false, "execute sync; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if !*confirm {
		links := len(record.IssueLinks)
		if *jsonOut {
			return deps.printJSON(map[string]any{
				"ok":         true,
				"synced":     false,
				"dry_run":    true,
				"link_count": links,
				"message":    fmt.Sprintf("[dry-run] would sync %d issue graph links to remote issue %s", links, record.IssueURL),
			})
		}
		if links == 0 {
			fmt.Println("no issue graph links to sync")
			return nil
		}
		fmt.Printf("[dry-run] would sync %d issue graph links to remote issue %s\n", links, record.IssueURL)
		return nil
	}
	result, err := issueopscore.SyncRemoteIssueGraph(record)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	fmt.Printf("synced: %v links\n", result["link_count"])
	return nil
}
