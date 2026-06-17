package remotecmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
)

type Deps struct {
	PrintJSON   func(any) error
	PrintResult func(core.IssueOpsRecord, bool, error) error
	PrintError  func(error) error
	VerifyLive  func(core.IssueOpsRemoteArtifactVerificationRequest) error
}

func Run(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage:")
		fmt.Println("  agent-harness issueops remote score --input PATH [--judge none|agy] [--json]")
		fmt.Println("  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --label LABEL --assignee USER [--json]")
		fmt.Println("  agent-harness issueops remote create-issue --id ID --title TEXT [--body TEXT] [--label LABEL]... [--assignee USER]... [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote create-child --id ID --title TEXT [--body TEXT] [--label LABEL]... [--assignee USER]... [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote create-pr --id ID --title TEXT --head BRANCH --base BRANCH [--body TEXT] [--label LABEL]... [--assignee USER]... [--confirm] [--json]")
		fmt.Println("  agent-harness issueops remote sync-graph --id ID [--confirm] [--json]")
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
		judge := fs.String("judge", "agy", "judge backend: agy or none")
		agyCommand := fs.String("agy-command", "agy", "agy command path")
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
		var result core.IssueOpsRemoteScoringResult
		switch *judge {
		case "agy":
			result, err = core.RunIssueOpsRemoteAgyJudge(core.IssueOpsRemoteAgyJudgeRequest{
				RepoRoot:   ".",
				AgyCommand: *agyCommand,
				Request:    req,
			})
		case "none":
			result, err = core.ScoreIssueOpsRemoteCandidates(req)
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
		var labels repeatedFlag
		var assignees repeatedFlag
		fs.Var(&labels, "label", "verified remote label; may be repeated")
		fs.Var(&labels, "labels", "verified remote label; may be repeated")
		fs.Var(&assignees, "assignee", "verified remote assignee; may be repeated")
		fs.Var(&assignees, "assignees", "verified remote assignee; may be repeated")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		req := core.IssueOpsRemoteArtifactVerificationRequest{
			Provider:  *provider,
			Kind:      *kind,
			URL:       *url,
			Labels:    labels,
			Assignees: assignees,
		}
		_, err := core.ValidateIssueOpsRemoteArtifactVerification(core.IssueOpsStateRoot(), *id, req)
		var record core.IssueOpsRecord
		if err == nil {
			err = deps.verifyLive(req)
		}
		if err == nil {
			record, err = core.VerifyIssueOpsRemoteArtifact(core.IssueOpsStateRoot(), *id, req)
		}
		return deps.printResult(record, *jsonOut, err)
	case "create-issue":
		return runRemoteCreateIssue(args[1:], deps)
	case "create-child":
		return runRemoteCreateChild(args[1:], deps)
	case "create-pr":
		return runRemoteCreatePR(args[1:], deps)
	case "sync-graph":
		return runRemoteSyncGraph(args[1:], deps)
	default:
		return fmt.Errorf("unknown issueops remote subcommand %q", args[0])
	}
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

func (deps Deps) printResult(record core.IssueOpsRecord, jsonOut bool, err error) error {
	if deps.PrintResult != nil {
		return deps.PrintResult(record, jsonOut, err)
	}
	return err
}

func (deps Deps) verifyLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
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

func formatIssueOpsRemoteIssueRef(issue core.IssueOpsRemoteScoredItem) string {
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

func readIssueOpsRemoteScoringRequestFile(path string) (core.IssueOpsRemoteScoringRequest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return core.IssueOpsRemoteScoringRequest{}, fmt.Errorf("input is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return core.IssueOpsRemoteScoringRequest{}, err
	}
	var req core.IssueOpsRemoteScoringRequest
	req, err = core.DecodeIssueOpsRemoteScoringRequest(b)
	if err != nil {
		return core.IssueOpsRemoteScoringRequest{}, fmt.Errorf("parse input file %s: %w", path, err)
	}
	return req, nil
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
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	providerName := resolveRecordProvider(record)
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	result, err := core.CreateRemoteIssue(core.IssueProviderCreateIssueRequest{
		Repo:      record.Repo,
		Title:     *title,
		Body:      *body,
		Labels:    labels,
		Assignees: assignees,
		Confirm:   *confirm,
	}, prov)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
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
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		err := fmt.Errorf("cannot create child before linked parent issue")
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	if err := validateCreateChildInputs(*title, labels, assignees); err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	providerName := resolveRecordProvider(record)
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	result, err := core.CreateRemoteChild(core.IssueProviderCreateChildRequest{
		Repo:           record.Repo,
		ParentIssueURL: record.IssueURL,
		Title:          *title,
		Body:           *body,
		Labels:         labels,
		Assignees:      assignees,
		Confirm:        *confirm,
	}, prov)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	if *confirm {
		if !result.HierarchyVerified || strings.TrimSpace(result.ChildURL) == "" {
			err := fmt.Errorf("provider did not verify child hierarchy")
			if *jsonOut {
				_ = deps.printError(err)
			}
			return err
		}
		if _, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), record.ID, result.ChildURL, *title); err != nil {
			if *jsonOut {
				_ = deps.printError(err)
			}
			return err
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
	head := fs.String("head", "", "source branch")
	base := fs.String("base", "", "target branch")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	providerName := resolveRecordProvider(record)
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	headBranch := firstNonEmptyMain(*head, record.Branch)
	baseBranch := *base
	if baseBranch == "" && record.BranchPrepare != nil {
		baseBranch = record.BranchPrepare.BaseBranch
	}
	result, err := core.CreateRemotePullRequest(core.IssueProviderCreatePullRequestRequest{
		Repo:       record.Repo,
		Title:      *title,
		Body:       *body,
		HeadBranch: headBranch,
		BaseBranch: baseBranch,
		Labels:     labels,
		Assignees:  assignees,
		Confirm:    *confirm,
	}, prov)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
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

func runRemoteSyncGraph(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote sync-graph", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	confirm := fs.Bool("confirm", false, "execute sync; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
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
	result, err := core.SyncRemoteIssueGraph(record)
	if err != nil {
		if *jsonOut {
			_ = deps.printError(err)
		}
		return err
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	fmt.Printf("synced: %v links\n", result["link_count"])
	return nil
}

func resolveRecordProvider(record core.IssueOpsRecord) string {
	if record.BranchPrepare != nil && record.BranchPrepare.Provider != "" {
		return record.BranchPrepare.Provider
	}
	if record.RemoteArtifact != nil && record.RemoteArtifact.Provider != "" {
		return record.RemoteArtifact.Provider
	}
	// Fall back to inferring from the issue URL.
	if record.IssueURL != "" {
		if strings.Contains(record.IssueURL, "github.com") {
			return "github"
		}
		if strings.Contains(record.IssueURL, "gitlab") {
			return "gitlab"
		}
	}
	return ""
}
