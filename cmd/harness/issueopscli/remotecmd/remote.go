package remotecmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

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
		fmt.Println("Usage: agent-harness issueops remote score --input PATH [--judge none|agy] [--json]\n       agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --label LABEL --assignee USER [--json]")
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
