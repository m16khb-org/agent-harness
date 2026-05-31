package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"agent-harness/internal/core"
)

type HarnessStatus struct {
	OK         bool                     `json:"ok"`
	Kind       string                   `json:"kind"`
	Version    string                   `json:"version"`
	Repo       string                   `json:"repo"`
	Inspect    core.InspectInfo         `json:"inspect"`
	Doctor     core.HarnessDoctorResult `json:"doctor"`
	Daemon     daemonStatus             `json:"daemon"`
	State      core.StateListResult     `json:"state"`
	Workers    core.WorkerListResult    `json:"workers"`
	SelfVerify SelfVerifyStatus         `json:"self_verify"`
	Warnings   []string                 `json:"warnings"`
}

type SelfVerifyStatus struct {
	LatestKey string `json:"latest_key,omitempty"`
	Found     bool   `json:"found"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	status := buildHarnessStatus(*repo)
	if *jsonOut {
		return printJSON(status)
	}
	fmt.Printf("agent-harness status: ok=%v repo=%s\n", status.OK, status.Repo)
	fmt.Printf("doctor healthy: %v\n", status.Doctor.Healthy)
	fmt.Printf("daemon running: %v (%s)\n", status.Daemon.Running, status.Daemon.Message)
	fmt.Printf("state records: %d\n", len(status.State.Records))
	fmt.Printf("worker jobs: %d\n", len(status.Workers.Jobs))
	for _, warning := range status.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}

func buildHarnessStatus(repo string) HarnessStatus {
	home, _ := os.UserHomeDir()
	inspect := inspectHarness(repo)
	doctor, doctorErr := core.HarnessDoctor(core.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: harnessRoot(), Home: home, Version: version})
	state, stateErr := core.StateList()
	workers, workerErr := core.ListWorkerJobs()
	warnings := []string{}
	if doctorErr != nil {
		warnings = append(warnings, "doctor: "+doctorErr.Error())
	}
	if stateErr != nil {
		warnings = append(warnings, "state: "+stateErr.Error())
	}
	if workerErr != nil {
		warnings = append(warnings, "workers: "+workerErr.Error())
	}
	selfVerify := SelfVerifyStatus{}
	for _, record := range state.Records {
		if record.Key == "self-verify-latest" || strings.HasPrefix(record.Key, "self-verify") {
			selfVerify = SelfVerifyStatus{LatestKey: record.Key, Found: true, UpdatedAt: record.UpdatedAt, Bytes: record.Bytes}
			break
		}
	}
	ok := len(warnings) == 0 && doctor.OK && state.OK && workers.OK
	return HarnessStatus{
		OK:         ok,
		Kind:       "harness_status",
		Version:    version,
		Repo:       resolveTarget(repo),
		Inspect:    inspect,
		Doctor:     doctor,
		Daemon:     checkDaemonStatus(),
		State:      state,
		Workers:    workers,
		SelfVerify: selfVerify,
		Warnings:   warnings,
	}
}

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
	root := resolveTarget(repo)
	warnings := []string{}
	evidence := []string{}
	evidenceMatrix := []VerifyWorkEvidenceItem{}
	statusBytes, err := exec.Command("git", "-C", root, "status", "--short").Output()
	if err != nil {
		warnings = append(warnings, "git status: "+err.Error())
	}
	preflight := core.GitPreflight(root, harnessRoot())
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
