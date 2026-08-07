package statuscli

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"agent-harness/internal/adapter/core"
)

type VerifyWorkResult struct {
	OK                bool                         `json:"ok"`
	Kind              string                       `json:"kind"`
	Repo              string                       `json:"repo"`
	GitStatus         string                       `json:"git_status,omitempty"`
	Preflight         core.PreflightResult         `json:"preflight"`
	Guard             core.GuardCheckResult        `json:"guard"`
	Command           *core.CommandRunResult       `json:"command,omitempty"`
	Evidence          []string                     `json:"evidence"`
	EvidenceMatrix    []VerifyWorkEvidenceItem     `json:"evidence_matrix"`
	SuggestedCommands []VerifyWorkSuggestedCommand `json:"suggested_commands"`
	Warnings          []string                     `json:"warnings"`
}

type VerifyWorkEvidenceItem struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Command string `json:"command,omitempty"`
}

type VerifyWorkSuggestedCommand struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Reason  string   `json:"reason"`
}

func runVerifyWork(args []string) error {
	fs := flag.NewFlagSet("verify-work", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	all := fs.Bool("all", false, "guard all relevant files instead of staged files")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := buildVerifyWork(*repo, *all, fs.Args())
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		fmt.Printf("verify-work ok=%v repo=%s\n", result.OK, result.Repo)
		for _, evidence := range result.Evidence {
			fmt.Printf("- %s\n", evidence)
		}
		for _, warning := range result.Warnings {
			fmt.Printf("warning: %s\n", warning)
		}
	}
	if !result.OK {
		return fmt.Errorf("verify-work found incomplete evidence")
	}
	return nil
}

func buildVerifyWork(repo string, all bool, argv []string) VerifyWorkResult {
	root := deps.ResolveTarget(repo)
	warnings := []string{}
	evidence := []string{}
	evidenceMatrix := []VerifyWorkEvidenceItem{}
	statusBytes, err := exec.Command("git", "-C", root, "status", "--short").Output()
	if err != nil {
		warnings = append(warnings, "git status: "+err.Error())
	}
	preflight := core.GitPreflight(root, deps.HarnessRoot())
	if preflight.OK {
		evidence = append(evidence, "git preflight completed")
	} else {
		warnings = append(warnings, "git preflight reported issues")
	}
	evidenceMatrix = append(evidenceMatrix, verifyWorkEvidenceItem("git_preflight", preflight.OK, "git repository preflight completed"))
	guard := core.GuardCheck(core.GuardCheckRequest{RepoRoot: root, Staged: !all, All: all})
	if guard.OK {
		evidence = append(evidence, fmt.Sprintf("guard check passed (%s, %d files)", guard.Mode, len(guard.CheckedFiles)))
	} else {
		warnings = append(warnings, "guard check has blocking findings")
	}
	evidenceMatrix = append(evidenceMatrix, verifyWorkEvidenceItem("guard_check", guard.OK, fmt.Sprintf("guard check completed in %s mode for %d file(s)", guard.Mode, len(guard.CheckedFiles))))
	var command *core.CommandRunResult
	if len(argv) > 0 {
		run := core.RunReadOnlyCommand(core.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: argv, Timeout: "30s"})
		command = &run
		if run.OK {
			evidence = append(evidence, "read-only verification command passed")
		} else {
			warnings = append(warnings, "read-only verification command failed or was denied")
		}
		evidenceMatrix = append(evidenceMatrix, verifyWorkEvidenceItemWithCommand("read_only_command", run.OK, "read-only verification command completed", strings.Join(argv, " ")))
	} else {
		evidenceMatrix = append(evidenceMatrix, VerifyWorkEvidenceItem{Name: "read_only_command", OK: true, Status: "skipped", Summary: "no read-only verification command provided"})
	}
	ok := preflight.OK && guard.OK && len(warnings) == 0
	if command != nil {
		ok = ok && command.OK
	}
	return VerifyWorkResult{OK: ok, Kind: "verify_work", Repo: root, GitStatus: string(statusBytes), Preflight: preflight, Guard: guard, Command: command, Evidence: evidence, EvidenceMatrix: evidenceMatrix, SuggestedCommands: buildVerifyWorkSuggestedCommands(root), Warnings: warnings}
}

func verifyWorkEvidenceItem(name string, ok bool, summary string) VerifyWorkEvidenceItem {
	return verifyWorkEvidenceItemWithCommand(name, ok, summary, "")
}

func verifyWorkEvidenceItemWithCommand(name string, ok bool, summary string, command string) VerifyWorkEvidenceItem {
	status := "failed"
	if ok {
		status = "passed"
	}
	return VerifyWorkEvidenceItem{Name: name, OK: ok, Status: status, Summary: summary, Command: command}
}

func buildVerifyWorkSuggestedCommands(root string) []VerifyWorkSuggestedCommand {
	signals := core.AnalyzeProjectSignals(root)
	out := []VerifyWorkSuggestedCommand{}
	add := func(kind string, commands []core.EvidenceCommand) {
		for i, command := range commands {
			fields := strings.Fields(command.Command)
			if len(fields) == 0 {
				continue
			}
			reason := fmt.Sprintf("%s command inferred from %s (confidence=%s)", kind, strings.Join(command.Evidence, ","), command.Confidence)
			out = append(out, VerifyWorkSuggestedCommand{Name: fmt.Sprintf("%s_%d", kind, i+1), Command: fields, Reason: reason})
		}
	}
	add("test", signals.TestCommands)
	add("build", signals.BuildCommands)
	add("lint", signals.LintCommands)
	return out
}
