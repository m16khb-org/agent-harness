package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core"
)

const version = "0.1.0"
const skillName = "atomic-commit-push"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println("harness", version)
	case "inspect":
		if err := runInspect(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "inspect:", err)
			os.Exit(1)
		}
	case "preflight":
		if err := runPreflight(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "preflight:", err)
			os.Exit(1)
		}
	case "docs":
		if err := runDocs(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "docs:", err)
			os.Exit(1)
		}
	case "policy":
		if err := runPolicy(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "policy:", err)
			if core.IsPolicyDenied(err) {
				os.Exit(3)
			}
			os.Exit(1)
		}
	case "self-augment":
		if err := runSelfAugment(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-augment:", err)
			os.Exit(1)
		}
	case "state":
		if err := runState(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "state:", err)
			os.Exit(1)
		}
	case "mcp":
		if err := runMCP(); err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `harness %s

Usage:
  harness inspect [--json] [--repo PATH]
  harness preflight [--json] [PATH]
  harness docs [index] [--json]
  harness policy check [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  harness policy fake-run [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  harness state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  harness state read --key KEY [--json]
  harness state list [--json]
  harness state prune --max-age DURATION [--confirm] [--json]
  harness state doctor [--json]
  harness state migrate [--confirm] [--json]
  harness self-augment [--iterations=10] [--seed=N] [--save-state] [--state-key KEY] [--json]
  harness self-augment history [--prefix PREFIX] [--limit N] [--json]
  harness self-augment compare --baseline-key KEY --candidate-key KEY [--max-elapsed-regression-pct N] [--fail-on-regression] [--json]
  harness self-augment promote --from-key KEY --baseline-key KEY [--confirm] [--json]
  harness mcp
  harness version
`, version)
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	repo := fs.String("repo", "", "target repo/workspace")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" && fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	info := inspectHarness(*repo)
	if *jsonOut {
		return printJSON(info)
	}
	fmt.Printf("harness harness root: %s\n", info.HarnessRoot)
	fmt.Printf("target repo: %s\n", info.TargetRepo)
	fmt.Printf("skills: %d\n", len(info.Skills))
	for _, s := range info.Skills {
		fmt.Printf("- %s (%s)\n", s.Name, s.Path)
	}
	fmt.Printf("codex skill installed: %v\n", info.Integration.CodexSkillInstalled)
	fmt.Printf("claude skill installed: %v\n", info.Integration.ClaudeSkillInstalled)
	fmt.Printf("project Claude MCP config: %v\n", info.Integration.ProjectClaudeMCPConfig)
	return nil
}

func runPreflight(args []string) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	jsonOut := fs.Bool("json", true, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	result := core.GitPreflight(resolveTarget(target), harnessRoot())
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}

func runDocs(args []string) error {
	if len(args) > 0 && args[0] == "index" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := core.DocsIndex(harnessRoot(), version)
	if *jsonOut {
		return printJSON(result)
	}
	for _, doc := range result.Docs {
		if doc.Title == "" {
			fmt.Println(doc.RelPath)
			continue
		}
		fmt.Printf("%s — %s\n", doc.RelPath, doc.Title)
	}
	return nil
}

func runPolicy(args []string) error {
	if len(args) == 0 {
		policyUsage()
		return fmt.Errorf("missing policy subcommand")
	}
	switch args[0] {
	case "check":
		return runPolicyCheck(args[1:])
	case "fake-run":
		return runPolicyFakeRun(args[1:])
	default:
		policyUsage()
		return fmt.Errorf("unknown policy subcommand %q", args[0])
	}
}

func policyUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  harness policy check [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
  harness policy fake-run [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
`)
}

func runPolicyCheck(args []string) error {
	req, jsonOut, err := parseCommandPolicyFlags("policy check", args)
	if err != nil {
		return err
	}
	result := core.EvaluateCommandPolicy(req)
	if jsonOut {
		return printJSON(result)
	}
	printPolicyEvaluation(result)
	return nil
}

func runPolicyFakeRun(args []string) error {
	req, jsonOut, err := parseCommandPolicyFlags("policy fake-run", args)
	if err != nil {
		return err
	}
	result := core.FakeRunCommand(req)
	if jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		printPolicyEvaluation(result.Policy)
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
	}
	if !result.Policy.Allowed {
		return core.PolicyDeniedError{Reasons: result.Policy.DenyReasons}
	}
	return nil
}

func parseCommandPolicyFlags(name string, args []string) (core.CommandPolicyRequest, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	workspaceRoot := fs.String("workspace-root", "", "workspace root boundary")
	cwd := fs.String("cwd", "", "command working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum runtime")
	envAllowlist := fs.String("env", "", "comma-separated environment variable allowlist")
	writeAllowed := fs.Bool("write", false, "allow workspace writes")
	networkAllowed := fs.Bool("network", false, "allow network access")
	shellAllowed := fs.Bool("shell", false, "allow shell interpreter argv[0] with --shell-reason")
	shellReason := fs.String("shell-reason", "", "reason for shell interpreter exception")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return core.CommandPolicyRequest{}, false, err
	}
	root := *workspaceRoot
	if root == "" {
		root = resolveTarget("")
	}
	workDir := *cwd
	if workDir == "" {
		workDir = root
	}
	req := core.CommandPolicyRequest{
		WorkspaceRoot:  root,
		CWD:            workDir,
		Argv:           fs.Args(),
		Timeout:        timeout.String(),
		EnvAllowlist:   splitCSV(*envAllowlist),
		NetworkAllowed: *networkAllowed,
		WriteAllowed:   *writeAllowed,
		ShellAllowed:   *shellAllowed,
		ShellReason:    *shellReason,
	}
	return req, *jsonOut, nil
}

func printPolicyEvaluation(result core.CommandPolicyEvaluation) {
	if result.Allowed {
		fmt.Printf("policy allowed: %s\n", result.AuditLogID)
		return
	}
	fmt.Printf("policy denied: %s\n", result.AuditLogID)
	for _, reason := range result.DenyReasons {
		fmt.Printf("- %s\n", reason)
	}
}

func runSelfAugment(args []string) error {
	if len(args) > 0 && args[0] == "history" {
		return runSelfAugmentHistory(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return runSelfAugmentCompare(args[1:])
	}
	if len(args) > 0 && args[0] == "promote" {
		return runSelfAugmentPromote(args[1:])
	}
	fs := flag.NewFlagSet("self-augment", flag.ContinueOnError)
	iterations := fs.Int("iterations", 10, "number of validation loop iterations; must be at least 10")
	seed := fs.Int64("seed", time.Now().Unix(), "base seed for randomized checks")
	saveState := fs.Bool("save-state", false, "save compact self-augment summary to harness state")
	stateKey := fs.String("state-key", "self-augment-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := selfAugment(*iterations, *seed, !*jsonOut)
	saveErr := error(nil)
	if *saveState {
		saveErr = saveSelfAugmentSummary(&result, *stateKey)
	}
	if *jsonOut {
		_ = printJSON(result)
	}
	if err == nil && saveErr != nil {
		return saveErr
	}
	return err
}

func runSelfAugmentCompare(args []string) error {
	fs := flag.NewFlagSet("self-augment compare", flag.ContinueOnError)
	baselineKey := fs.String("baseline-key", "", "state key containing the baseline self-augment summary snapshot")
	candidateKey := fs.String("candidate-key", "", "state key containing the candidate self-augment summary snapshot")
	maxElapsedRegressionPct := fs.Float64("max-elapsed-regression-pct", 20, "allowed elapsed_ms increase percentage before regression")
	failOnRegression := fs.Bool("fail-on-regression", false, "return non-zero when comparison reports a regression")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := compareSelfAugmentSummaries(*baselineKey, *candidateKey, *maxElapsedRegressionPct)
	if err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		status := "ok"
		if result.Regressed {
			status = "regressed"
		}
		fmt.Printf("self-augment compare %s: elapsed_delta=%dms failed_steps_delta=%d\n", status, result.ElapsedDeltaMS, result.FailedStepsDelta)
		for _, regression := range result.Regressions {
			fmt.Println("- " + regression)
		}
	}
	if *failOnRegression && result.Regressed {
		return fmt.Errorf("self-augment summary regression detected")
	}
	return nil
}

func runSelfAugmentHistory(args []string) error {
	fs := flag.NewFlagSet("self-augment history", flag.ContinueOnError)
	prefix := fs.String("prefix", "self-augment", "state key prefix to scan; empty string scans all keys")
	limit := fs.Int("limit", 20, "maximum entries to return; 0 returns all")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := selfAugmentHistory(*prefix, *limit)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("self-augment history: %d/%d entries from %s (prefix=%q)\n", result.Returned, result.TotalMatches, result.StateDir, result.Prefix)
	for _, entry := range result.Entries {
		status := "fail"
		if entry.OK {
			status = "ok"
		}
		fmt.Printf("- %s %s iterations=%d elapsed=%dms generated_at=%s\n", entry.Key, status, entry.Iterations, entry.ElapsedMS, entry.GeneratedAt)
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("skipped %d non-summary records\n", len(result.Skipped))
	}
	return nil
}

func runSelfAugmentPromote(args []string) error {
	fs := flag.NewFlagSet("self-augment promote", flag.ContinueOnError)
	fromKey := fs.String("from-key", "", "state key containing the candidate self-augment summary snapshot")
	baselineKey := fs.String("baseline-key", "", "state key to write as the promoted baseline")
	confirm := fs.Bool("confirm", false, "write baseline-key; omitted means dry-run")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := promoteSelfAugmentBaseline(*fromKey, *baselineKey, *confirm)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	action := "would promote"
	if result.Confirm {
		action = "promoted"
	}
	fmt.Printf("%s self-augment summary %q to baseline %q\n", action, result.FromKey, result.BaselineKey)
	return nil
}

func runState(args []string) error {
	if len(args) == 0 {
		stateUsage()
		return fmt.Errorf("missing state subcommand")
	}
	switch args[0] {
	case "write":
		return runStateWrite(args[1:])
	case "read":
		return runStateRead(args[1:])
	case "list":
		return runStateList(args[1:])
	case "prune":
		return runStatePrune(args[1:])
	case "doctor":
		return runStateDoctor(args[1:])
	case "migrate":
		return runStateMigrate(args[1:])
	default:
		stateUsage()
		return fmt.Errorf("unknown state subcommand %q", args[0])
	}
}

func stateUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  harness state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  harness state read --key KEY [--json]
  harness state list [--json]
  harness state prune --max-age DURATION [--confirm] [--json]
  harness state doctor [--json]
  harness state migrate [--confirm] [--json]
`)
}

func runStateWrite(args []string) error {
	fs := flag.NewFlagSet("state write", flag.ContinueOnError)
	key := fs.String("key", "", "state key; [A-Za-z0-9._-], max 128 chars")
	value := fs.String("value", "", "state content")
	input := fs.String("input", "", "read state content from file")
	stdin := fs.Bool("stdin", false, "read state content from stdin")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *key == "" && fs.NArg() > 0 {
		*key = fs.Arg(0)
	}
	valueProvided := flagProvided(args, "value")
	sourceCount := 0
	if valueProvided {
		sourceCount++
	}
	if *input != "" {
		sourceCount++
	}
	if *stdin {
		sourceCount++
	}
	if sourceCount != 1 {
		return fmt.Errorf("provide exactly one content source: --value, --input, or --stdin")
	}
	var content string
	switch {
	case valueProvided:
		content = *value
	case *input != "":
		b, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		content = string(b)
	case *stdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		content = string(b)
	}
	result, err := core.StateWrite(*key, content)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("state %q written (%d bytes) to %s\n", result.Record.Key, result.Record.Bytes, result.StateDir)
	return nil
}

func flagProvided(args []string, name string) bool {
	long := "--" + name
	for i, arg := range args {
		if arg == long || strings.HasPrefix(arg, long+"=") {
			return true
		}
		if arg == "-"+name && i < len(args) {
			return true
		}
	}
	return false
}

func runStateRead(args []string) error {
	fs := flag.NewFlagSet("state read", flag.ContinueOnError)
	key := fs.String("key", "", "state key")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *key == "" && fs.NArg() > 0 {
		*key = fs.Arg(0)
	}
	result, err := core.StateRead(*key)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Print(result.Record.Content)
	return nil
}

func runStateList(args []string) error {
	fs := flag.NewFlagSet("state list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StateList()
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	for _, key := range result.Keys {
		fmt.Println(key)
	}
	return nil
}

func runStatePrune(args []string) error {
	fs := flag.NewFlagSet("state prune", flag.ContinueOnError)
	maxAge := fs.Duration("max-age", 0, "prune records older than this duration, e.g. 720h")
	confirm := fs.Bool("confirm", false, "delete matching records; omitted means dry-run")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StatePrune(*maxAge, *confirm)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	action := "would prune"
	if result.Confirm {
		action = "pruned"
	}
	fmt.Printf("%s %d state records older than %s\n", action, len(result.DeletedKeys), result.MaxAge)
	for _, key := range result.DeletedKeys {
		fmt.Println(key)
	}
	return nil
}

func runStateDoctor(args []string) error {
	fs := flag.NewFlagSet("state doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StateDoctor()
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Healthy {
		fmt.Printf("state doctor healthy: %d valid records in %s\n", len(result.ValidKeys), result.StateDir)
		return nil
	}
	fmt.Printf("state doctor found %d issues in %s\n", len(result.Issues), result.StateDir)
	for _, issue := range result.Issues {
		fmt.Printf("%s %s %s\n", issue.Severity, issue.Code, issue.Path)
	}
	return nil
}

func runStateMigrate(args []string) error {
	fs := flag.NewFlagSet("state migrate", flag.ContinueOnError)
	confirm := fs.Bool("confirm", false, "rewrite legacy records; omitted means dry-run")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StateMigrate(*confirm)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Confirm {
		fmt.Printf("migrated %d state records from schema %d to %d\n", len(result.MigratedKeys), result.FromSchema, result.ToSchema)
		for _, key := range result.MigratedKeys {
			fmt.Println(key)
		}
		return nil
	}
	fmt.Printf("would migrate %d state records from schema %d to %d\n", len(result.CandidateKeys), result.FromSchema, result.ToSchema)
	for _, key := range result.CandidateKeys {
		fmt.Println(key)
	}
	return nil
}

func inspectHarness(repoArg string) core.InspectInfo {
	root := harnessRoot()
	target := resolveTarget(repoArg)
	home, _ := os.UserHomeDir()
	return core.InspectHarness(root, target, home, version, skillName)
}

type SelfAugmentResult struct {
	OK              bool                        `json:"ok"`
	Iterations      int                         `json:"iterations"`
	BaseSeed        int64                       `json:"base_seed"`
	ElapsedMS       int64                       `json:"elapsed_ms"`
	HarnessRoot     string                      `json:"harness_root"`
	InspiredBy      string                      `json:"inspired_by"`
	LoopContract    []string                    `json:"loop_contract"`
	Summary         SelfAugmentSummary          `json:"summary"`
	StateCheckpoint *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	Runs            []SelfAugmentIteration      `json:"runs"`
}

type SelfAugmentSummary struct {
	TotalRuns       int                   `json:"total_runs"`
	TotalSteps      int                   `json:"total_steps"`
	PassedSteps     int                   `json:"passed_steps"`
	FailedSteps     int                   `json:"failed_steps"`
	FailedIteration int                   `json:"failed_iteration,omitempty"`
	FailedSeed      int64                 `json:"failed_seed,omitempty"`
	FailedStep      string                `json:"failed_step,omitempty"`
	StepLabels      []string              `json:"step_labels"`
	SlowestSteps    []SelfAugmentSlowStep `json:"slowest_steps"`
}

type SelfAugmentSlowStep struct {
	Iteration  int    `json:"iteration"`
	Seed       int64  `json:"seed"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
}

type SelfAugmentStateCheckpoint struct {
	OK       bool   `json:"ok"`
	Key      string `json:"key"`
	StateDir string `json:"state_dir,omitempty"`
	Path     string `json:"path,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Error    string `json:"error,omitempty"`
}

type SelfAugmentStateSnapshot struct {
	SchemaVersion int                `json:"schema_version"`
	Kind          string             `json:"kind"`
	OK            bool               `json:"ok"`
	Iterations    int                `json:"iterations"`
	BaseSeed      int64              `json:"base_seed"`
	ElapsedMS     int64              `json:"elapsed_ms"`
	HarnessRoot   string             `json:"harness_root"`
	GeneratedAt   string             `json:"generated_at"`
	Summary       SelfAugmentSummary `json:"summary"`
}

type SelfAugmentCompareResult struct {
	OK                           bool                  `json:"ok"`
	StateDir                     string                `json:"state_dir"`
	BaselineKey                  string                `json:"baseline_key"`
	CandidateKey                 string                `json:"candidate_key"`
	MaxElapsedRegressionPct      float64               `json:"max_elapsed_regression_pct"`
	Regressed                    bool                  `json:"regressed"`
	ElapsedDeltaMS               int64                 `json:"elapsed_delta_ms"`
	ElapsedDeltaPct              float64               `json:"elapsed_delta_pct"`
	FailedStepsDelta             int                   `json:"failed_steps_delta"`
	TotalStepsDelta              int                   `json:"total_steps_delta"`
	MissingStepLabels            []string              `json:"missing_step_labels"`
	AddedStepLabels              []string              `json:"added_step_labels"`
	Regressions                  []string              `json:"regressions"`
	Warnings                     []string              `json:"warnings"`
	BaselineSummary              SelfAugmentSummary    `json:"baseline_summary"`
	CandidateSummary             SelfAugmentSummary    `json:"candidate_summary"`
	BaselineSnapshotGeneratedAt  string                `json:"baseline_snapshot_generated_at"`
	CandidateSnapshotGeneratedAt string                `json:"candidate_snapshot_generated_at"`
	BaselineSlowestSteps         []SelfAugmentSlowStep `json:"baseline_slowest_steps"`
	CandidateSlowestSteps        []SelfAugmentSlowStep `json:"candidate_slowest_steps"`
}

type SelfAugmentPromoteResult struct {
	OK                  bool               `json:"ok"`
	StateDir            string             `json:"state_dir"`
	FromKey             string             `json:"from_key"`
	BaselineKey         string             `json:"baseline_key"`
	Confirm             bool               `json:"confirm"`
	DryRun              bool               `json:"dry_run"`
	Promoted            bool               `json:"promoted"`
	Path                string             `json:"path,omitempty"`
	Bytes               int                `json:"bytes,omitempty"`
	SnapshotGeneratedAt string             `json:"snapshot_generated_at"`
	Summary             SelfAugmentSummary `json:"summary"`
}

type SelfAugmentHistoryResult struct {
	OK           bool                        `json:"ok"`
	StateDir     string                      `json:"state_dir"`
	Prefix       string                      `json:"prefix"`
	Limit        int                         `json:"limit"`
	TotalMatches int                         `json:"total_matches"`
	Returned     int                         `json:"returned"`
	Entries      []SelfAugmentHistoryEntry   `json:"entries"`
	Skipped      []SelfAugmentHistorySkipped `json:"skipped"`
	Warnings     []string                    `json:"warnings"`
}

type SelfAugmentHistoryEntry struct {
	Key          string                `json:"key"`
	UpdatedAt    string                `json:"updated_at"`
	Bytes        int                   `json:"bytes"`
	GeneratedAt  string                `json:"generated_at"`
	OK           bool                  `json:"ok"`
	Iterations   int                   `json:"iterations"`
	BaseSeed     int64                 `json:"base_seed"`
	ElapsedMS    int64                 `json:"elapsed_ms"`
	TotalRuns    int                   `json:"total_runs"`
	TotalSteps   int                   `json:"total_steps"`
	FailedSteps  int                   `json:"failed_steps"`
	StepLabels   []string              `json:"step_labels"`
	SlowestSteps []SelfAugmentSlowStep `json:"slowest_steps"`
}

type SelfAugmentHistorySkipped struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type SelfAugmentIteration struct {
	Iteration int          `json:"iteration"`
	Seed      int64        `json:"seed"`
	Steps     []StepResult `json:"steps"`
}

type StepResult struct {
	Label      string `json:"label"`
	Command    string `json:"command,omitempty"`
	OK         bool   `json:"ok"`
	DurationMS int64  `json:"duration_ms"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Error      string `json:"error,omitempty"`
}

func selfAugment(iterations int, baseSeed int64, verbose bool) (SelfAugmentResult, error) {
	started := time.Now()
	result := SelfAugmentResult{
		Iterations:  iterations,
		BaseSeed:    baseSeed,
		HarnessRoot: harnessRoot(),
		InspiredBy:  "/Users/habin/workspace/eye-tracking-scroll/scripts/self-augment.js",
		LoopContract: []string{
			"minimum 10 iterations",
			"seeded per-iteration randomized git preflight fuzz",
			"repeat core invariant, build, CLI/MCP schema and response contract golden, CLI, docs, command policy, MCP, state, and native integration smoke checks",
			"fail fast on the first failed step",
		},
	}
	if iterations < 10 {
		err := fmt.Errorf("self-augmentation requires at least 10 iterations; use --iterations=10 or higher")
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = summarizeSelfAugment(result)
		return result, err
	}

	for iteration := 1; iteration <= iterations; iteration++ {
		seed := baseSeed + int64(iteration) - 1
		if verbose {
			fmt.Printf("\n=== Self-augmentation iteration %d/%d seed=%d ===\n", iteration, iterations, seed)
		}
		run := SelfAugmentIteration{Iteration: iteration, Seed: seed}
		tempDir, err := os.MkdirTemp("", "agent-harness-self-augment-*")
		if err != nil {
			step := failedStep("create temp workspace", err)
			run.Steps = append(run.Steps, step)
			result.Runs = append(result.Runs, run)
			result.ElapsedMS = time.Since(started).Milliseconds()
			return result, err
		}
		tempBin := filepath.Join(tempDir, "harness")

		steps := []func() StepResult{
			func() StepResult { return validateHarnessInvariants(result.HarnessRoot) },
			func() StepResult {
				return runCommandStep(result.HarnessRoot, "go test", 120*time.Second, "", "go", "test", "./...", "-count=1")
			},
			func() StepResult {
				return runCommandStep(result.HarnessRoot, "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/harness", "-run", "Golden", "-count=1")
			},
			func() StepResult {
				return runCommandStep(result.HarnessRoot, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/harness")
			},
			func() StepResult { return validateInspect(tempBin, result.HarnessRoot) },
			func() StepResult { return validateDocsIndex(tempBin, result.HarnessRoot) },
			func() StepResult { return validateCommandPolicy(tempBin, result.HarnessRoot) },
			func() StepResult { return validateMCP(tempBin, result.HarnessRoot) },
			func() StepResult { return validateStateRoundtrip(tempBin, result.HarnessRoot, seed) },
			func() StepResult { return validatePreflightFuzz(tempBin, result.HarnessRoot, seed) },
			func() StepResult { return validateNativeIntegration(result.HarnessRoot) },
		}

		for _, stepFn := range steps {
			step := stepFn()
			run.Steps = append(run.Steps, step)
			if verbose {
				printStep(step)
			}
			if !step.OK {
				_ = os.RemoveAll(tempDir)
				result.Runs = append(result.Runs, run)
				result.ElapsedMS = time.Since(started).Milliseconds()
				result.OK = false
				result.Summary = summarizeSelfAugment(result)
				return result, fmt.Errorf("%s failed: %s", step.Label, step.Error)
			}
		}
		_ = os.RemoveAll(tempDir)
		result.Runs = append(result.Runs, run)
	}

	result.OK = true
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.Summary = summarizeSelfAugment(result)
	if verbose {
		fmt.Printf("\nSelf-augmentation pipeline passed %d iterations in %.1fs.\n", iterations, float64(result.ElapsedMS)/1000)
	}
	return result, nil
}

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	summary := SelfAugmentSummary{
		TotalRuns:    len(result.Runs),
		StepLabels:   []string{},
		SlowestSteps: []SelfAugmentSlowStep{},
	}
	seenLabels := map[string]bool{}
	for _, run := range result.Runs {
		for _, step := range run.Steps {
			summary.TotalSteps++
			if step.OK {
				summary.PassedSteps++
			} else {
				summary.FailedSteps++
				if summary.FailedStep == "" {
					summary.FailedIteration = run.Iteration
					summary.FailedSeed = run.Seed
					summary.FailedStep = step.Label
				}
			}
			if !seenLabels[step.Label] {
				seenLabels[step.Label] = true
				summary.StepLabels = append(summary.StepLabels, step.Label)
			}
			summary.SlowestSteps = append(summary.SlowestSteps, SelfAugmentSlowStep{
				Iteration:  run.Iteration,
				Seed:       run.Seed,
				Label:      step.Label,
				DurationMS: step.DurationMS,
			})
		}
	}
	sort.Slice(summary.SlowestSteps, func(i, j int) bool {
		if summary.SlowestSteps[i].DurationMS != summary.SlowestSteps[j].DurationMS {
			return summary.SlowestSteps[i].DurationMS > summary.SlowestSteps[j].DurationMS
		}
		if summary.SlowestSteps[i].Iteration != summary.SlowestSteps[j].Iteration {
			return summary.SlowestSteps[i].Iteration < summary.SlowestSteps[j].Iteration
		}
		return summary.SlowestSteps[i].Label < summary.SlowestSteps[j].Label
	})
	if len(summary.SlowestSteps) > 5 {
		summary.SlowestSteps = summary.SlowestSteps[:5]
	}
	if summary.StepLabels == nil {
		summary.StepLabels = []string{}
	}
	if summary.SlowestSteps == nil {
		summary.SlowestSteps = []SelfAugmentSlowStep{}
	}
	return summary
}

func saveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	if key == "" {
		key = "self-augment-latest"
	}
	snapshot := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            result.OK,
		Iterations:    result.Iterations,
		BaseSeed:      result.BaseSeed,
		ElapsedMS:     result.ElapsedMS,
		HarnessRoot:   result.HarnessRoot,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Summary:       result.Summary,
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := core.StateWrite(key, string(b))
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: core.StateDir(), Error: err.Error()}
		return err
	}
	result.StateCheckpoint = &SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return nil
}

func compareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	result := SelfAugmentCompareResult{
		OK:                      false,
		StateDir:                core.StateDir(),
		BaselineKey:             baselineKey,
		CandidateKey:            candidateKey,
		MaxElapsedRegressionPct: maxElapsedRegressionPct,
		MissingStepLabels:       []string{},
		AddedStepLabels:         []string{},
		Regressions:             []string{},
		Warnings:                []string{},
	}
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	if strings.TrimSpace(candidateKey) == "" {
		return result, fmt.Errorf("candidate-key is required")
	}
	if maxElapsedRegressionPct < 0 {
		return result, fmt.Errorf("max elapsed regression pct must be non-negative")
	}
	baseline, err := readSelfAugmentStateSnapshot(baselineKey)
	if err != nil {
		return result, fmt.Errorf("read baseline summary: %w", err)
	}
	candidate, err := readSelfAugmentStateSnapshot(candidateKey)
	if err != nil {
		return result, fmt.Errorf("read candidate summary: %w", err)
	}
	result.BaselineSummary = baseline.Summary
	result.CandidateSummary = candidate.Summary
	result.BaselineSnapshotGeneratedAt = baseline.GeneratedAt
	result.CandidateSnapshotGeneratedAt = candidate.GeneratedAt
	result.BaselineSlowestSteps = baseline.Summary.SlowestSteps
	result.CandidateSlowestSteps = candidate.Summary.SlowestSteps
	result.ElapsedDeltaMS = candidate.ElapsedMS - baseline.ElapsedMS
	if baseline.ElapsedMS > 0 {
		result.ElapsedDeltaPct = float64(result.ElapsedDeltaMS) * 100 / float64(baseline.ElapsedMS)
	} else if candidate.ElapsedMS > 0 {
		result.Warnings = append(result.Warnings, "baseline_elapsed_zero")
	}
	result.FailedStepsDelta = candidate.Summary.FailedSteps - baseline.Summary.FailedSteps
	result.TotalStepsDelta = candidate.Summary.TotalSteps - baseline.Summary.TotalSteps
	result.MissingStepLabels = missingStrings(baseline.Summary.StepLabels, candidate.Summary.StepLabels)
	result.AddedStepLabels = missingStrings(candidate.Summary.StepLabels, baseline.Summary.StepLabels)
	if baseline.OK && !candidate.OK {
		result.Regressions = append(result.Regressions, "candidate_not_ok")
	}
	if result.FailedStepsDelta > 0 {
		result.Regressions = append(result.Regressions, fmt.Sprintf("failed_steps_increased_by_%d", result.FailedStepsDelta))
	}
	if result.ElapsedDeltaPct > maxElapsedRegressionPct {
		result.Regressions = append(result.Regressions, fmt.Sprintf("elapsed_ms_increased_by_%.2f_pct", result.ElapsedDeltaPct))
	}
	for _, label := range result.MissingStepLabels {
		result.Regressions = append(result.Regressions, "missing_step_label:"+label)
	}
	if result.TotalStepsDelta != 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("total_steps_delta_%+d", result.TotalStepsDelta))
	}
	for _, label := range result.AddedStepLabels {
		result.Warnings = append(result.Warnings, "added_step_label:"+label)
	}
	sort.Strings(result.MissingStepLabels)
	sort.Strings(result.AddedStepLabels)
	sort.Strings(result.Regressions)
	sort.Strings(result.Warnings)
	result.Regressed = len(result.Regressions) > 0
	result.OK = true
	return result, nil
}

func promoteSelfAugmentBaseline(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error) {
	result := SelfAugmentPromoteResult{
		OK:          false,
		StateDir:    core.StateDir(),
		FromKey:     fromKey,
		BaselineKey: baselineKey,
		Confirm:     confirm,
		DryRun:      !confirm,
	}
	if strings.TrimSpace(fromKey) == "" {
		return result, fmt.Errorf("from-key is required")
	}
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	snapshot, err := readSelfAugmentStateSnapshot(fromKey)
	if err != nil {
		return result, fmt.Errorf("read source summary: %w", err)
	}
	result.SnapshotGeneratedAt = snapshot.GeneratedAt
	result.Summary = snapshot.Summary
	if !confirm {
		result.OK = true
		return result, nil
	}
	if err := writeSelfAugmentSnapshotRecord(core.StateDir(), baselineKey, snapshot); err != nil {
		return result, err
	}
	state, err := core.StateRead(baselineKey)
	if err != nil {
		return result, err
	}
	result.OK = true
	result.Promoted = true
	result.Path = state.Path
	result.Bytes = state.Record.Bytes
	return result, nil
}

func selfAugmentHistory(prefix string, limit int) (SelfAugmentHistoryResult, error) {
	result := SelfAugmentHistoryResult{
		OK:       false,
		StateDir: core.StateDir(),
		Prefix:   prefix,
		Limit:    limit,
		Entries:  []SelfAugmentHistoryEntry{},
		Skipped:  []SelfAugmentHistorySkipped{},
		Warnings: []string{},
	}
	if limit < 0 {
		return result, fmt.Errorf("limit must be non-negative")
	}
	list, err := core.StateList()
	if err != nil {
		return result, err
	}
	for _, record := range list.Records {
		if prefix != "" && !strings.HasPrefix(record.Key, prefix) {
			continue
		}
		state, err := core.StateRead(record.Key)
		if err != nil {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "state_read:" + err.Error()})
			continue
		}
		var snapshot SelfAugmentStateSnapshot
		if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "not_json_summary"})
			continue
		}
		if snapshot.Kind != "self_augment_summary" {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "kind:" + snapshot.Kind})
			continue
		}
		if snapshot.SchemaVersion != 1 {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: fmt.Sprintf("schema:%d", snapshot.SchemaVersion)})
			continue
		}
		if _, ok := parseSelfAugmentTimestamp(snapshot.GeneratedAt); !ok {
			result.Warnings = append(result.Warnings, "invalid_generated_at:"+record.Key)
		}
		result.Entries = append(result.Entries, SelfAugmentHistoryEntry{
			Key:          record.Key,
			UpdatedAt:    record.UpdatedAt,
			Bytes:        record.Bytes,
			GeneratedAt:  snapshot.GeneratedAt,
			OK:           snapshot.OK,
			Iterations:   snapshot.Iterations,
			BaseSeed:     snapshot.BaseSeed,
			ElapsedMS:    snapshot.ElapsedMS,
			TotalRuns:    snapshot.Summary.TotalRuns,
			TotalSteps:   snapshot.Summary.TotalSteps,
			FailedSteps:  snapshot.Summary.FailedSteps,
			StepLabels:   nonNilStringSlice(snapshot.Summary.StepLabels),
			SlowestSteps: nonNilSlowStepSlice(snapshot.Summary.SlowestSteps),
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		left, leftOK := parseSelfAugmentTimestamp(result.Entries[i].GeneratedAt)
		right, rightOK := parseSelfAugmentTimestamp(result.Entries[j].GeneratedAt)
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && !left.Equal(right) {
			return left.After(right)
		}
		leftUpdated, leftUpdatedOK := parseSelfAugmentTimestamp(result.Entries[i].UpdatedAt)
		rightUpdated, rightUpdatedOK := parseSelfAugmentTimestamp(result.Entries[j].UpdatedAt)
		if leftUpdatedOK != rightUpdatedOK {
			return leftUpdatedOK
		}
		if leftUpdatedOK && !leftUpdated.Equal(rightUpdated) {
			return leftUpdated.After(rightUpdated)
		}
		return result.Entries[i].Key < result.Entries[j].Key
	})
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Key < result.Skipped[j].Key })
	sort.Strings(result.Warnings)
	result.TotalMatches = len(result.Entries)
	if limit > 0 && len(result.Entries) > limit {
		result.Entries = result.Entries[:limit]
	}
	result.Returned = len(result.Entries)
	result.OK = true
	return result, nil
}

func parseSelfAugmentTimestamp(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func nonNilStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func nonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	if items == nil {
		return []SelfAugmentSlowStep{}
	}
	return items
}

func readSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	state, err := core.StateRead(key)
	if err != nil {
		return SelfAugmentStateSnapshot{}, err
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		return SelfAugmentStateSnapshot{}, err
	}
	if snapshot.Kind != "self_augment_summary" {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q contains kind %q, want self_augment_summary", key, snapshot.Kind)
	}
	if snapshot.SchemaVersion != 1 {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q has unsupported self-augment summary schema %d", key, snapshot.SchemaVersion)
	}
	return snapshot, nil
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	key, err := core.NormalizeStateKey(key)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	record := core.StateRecord{
		SchemaVersion: core.StateCurrentSchemaVersion,
		Key:           key,
		Content:       string(content),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Bytes:         len(content),
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, key+".json"), append(b, '\n'), 0o600)
}

func missingStrings(want, have []string) []string {
	haveSet := map[string]bool{}
	for _, item := range have {
		haveSet[item] = true
	}
	missing := []string{}
	for _, item := range want {
		if !haveSet[item] {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}

func validateHarnessInvariants(root string) StepResult {
	started := time.Now()
	errs := []string{}
	required := []string{
		"AGENTS.md",
		"CLAUDE.md",
		filepath.Join("agent_docs", "USAGE.md"),
		filepath.Join("agent_docs", "COMMIT_POLICY.md"),
		filepath.Join("skills", skillName, "SKILL.md"),
		filepath.Join("skills", skillName, "agents", "openai.yaml"),
		filepath.Join("skills", skillName, "scripts", "git_preflight.py"),
		filepath.Join("internal", "core", "docs.go"),
		filepath.Join("internal", "core", "inspect.go"),
		filepath.Join("internal", "core", "policy.go"),
		filepath.Join("internal", "core", "preflight.go"),
		filepath.Join("internal", "core", "state.go"),
		filepath.Join("cmd", "harness", "contract_golden_test.go"),
		filepath.Join("cmd", "harness", "response_contract_golden_test.go"),
		filepath.Join("cmd", "harness", "self_augment_summary_test.go"),
		filepath.Join("cmd", "harness", "testdata", "usage.golden.txt"),
		filepath.Join("cmd", "harness", "testdata", "mcp_tools.golden.json"),
		filepath.Join("cmd", "harness", "testdata", "mcp_resources.golden.json"),
		filepath.Join("cmd", "harness", "testdata", "response_contracts.golden.json"),
		filepath.Join(".mcp.json"),
		filepath.Join(".claude", "skills", skillName, "SKILL.md"),
	}
	for _, rel := range required {
		if !exists(filepath.Join(root, rel)) {
			errs = append(errs, "missing "+rel)
		}
	}
	if err := validateSkillShape(filepath.Join(root, "skills", skillName)); err != nil {
		errs = append(errs, err.Error())
	}
	if hits := forbiddenNameHits(root); len(hits) > 0 {
		errs = append(errs, "forbidden legacy name hits: "+strings.Join(hits, "; "))
	}
	return assertionStep("harness invariants", started, errs)
}

func validateSkillShape(skillDir string) error {
	body, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return err
	}
	text := string(body)
	if !strings.HasPrefix(text, "---\n") {
		return fmt.Errorf("SKILL.md missing YAML frontmatter")
	}
	front := strings.SplitN(text, "---", 3)
	if len(front) < 3 || !strings.Contains(front[1], "name: "+skillName) || !strings.Contains(front[1], "description:") {
		return fmt.Errorf("SKILL.md frontmatter must include name and description")
	}
	if !exists(filepath.Join(skillDir, "agents", "openai.yaml")) {
		return fmt.Errorf("agents/openai.yaml missing")
	}
	return nil
}

func forbiddenNameHits(root string) []string {
	forbidden := []string{"m" + "16kh", "m" + "16h", "M" + "16H", "m" + "16"}
	var hits []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "bin" || name == ".omx" {
				return filepath.SkipDir
			}
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil || bytes.Contains(b, []byte{0}) {
			return nil
		}
		text := string(b)
		for _, needle := range forbidden {
			if strings.Contains(text, needle) {
				rel, _ := filepath.Rel(root, path)
				hits = append(hits, rel+" contains "+needle)
				break
			}
		}
		return nil
	})
	sort.Strings(hits)
	if len(hits) > 20 {
		return hits[:20]
	}
	return hits
}

func validateInspect(binary, root string) StepResult {
	step := runCommandStep(root, "inspect smoke", 30*time.Second, "", binary, "inspect", "--json")
	if !step.OK {
		return step
	}
	var info core.InspectInfo
	if err := json.Unmarshal([]byte(step.Stdout), &info); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := []string{}
	if !info.OK {
		errs = append(errs, "inspect ok=false")
	}
	if len(info.Skills) == 0 {
		errs = append(errs, "no skills listed")
	}
	if !info.Integration.ProjectClaudeMCPConfig {
		errs = append(errs, "project Claude MCP config missing")
	}
	if strings.Contains(step.Stdout, "m"+"16") {
		errs = append(errs, "inspect output contains legacy "+"m"+"16 name")
	}
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func validateDocsIndex(binary, root string) StepResult {
	step := runCommandStep(root, "docs index smoke", 30*time.Second, "", binary, "docs", "--json")
	if !step.OK {
		return step
	}
	var index core.DocsIndexResult
	if err := json.Unmarshal([]byte(step.Stdout), &index); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := []string{}
	if !index.OK {
		errs = append(errs, "docs index ok=false")
	}
	if index.HarnessRoot != root {
		errs = append(errs, "docs index harness root mismatch")
	}
	if len(index.Docs) == 0 {
		errs = append(errs, "no docs indexed")
	}
	wantDocs := []string{"AGENTS.md", "CLAUDE.md", "agent_docs/COMMIT_POLICY.md", "agent_docs/USAGE.md"}
	for _, want := range wantDocs {
		if !docIndexContains(index.Docs, want) {
			errs = append(errs, "missing doc "+want)
		}
	}
	for _, doc := range index.Docs {
		if doc.Title == "" {
			errs = append(errs, "missing title for "+doc.RelPath)
			break
		}
		if strings.Contains(doc.RelPath, "m"+"16") || strings.Contains(doc.Title, "m"+"16") {
			errs = append(errs, "docs index contains legacy "+"m"+"16 name")
			break
		}
	}
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func docIndexContains(docs []core.DocIndexInfo, relPath string) bool {
	for _, doc := range docs {
		if doc.RelPath == relPath {
			return true
		}
	}
	return false
}

func validateCommandPolicy(binary, root string) StepResult {
	started := time.Now()
	tempWorkspace, err := os.MkdirTemp("", "agent-harness-policy-*")
	if err != nil {
		return failedStep("command policy smoke", err)
	}
	defer os.RemoveAll(tempWorkspace)
	outside, err := os.MkdirTemp("", "agent-harness-policy-outside-*")
	if err != nil {
		return failedStep("command policy smoke", err)
	}
	defer os.RemoveAll(outside)

	stdoutParts := []string{}
	commands := []string{}
	allowed := runCommandStep(root, "policy allow", 30*time.Second, "", binary, "policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "git", "status", "--short")
	stdoutParts = append(stdoutParts, allowed.Stdout)
	commands = append(commands, allowed.Command)
	if !allowed.OK {
		return combineFailedStep("command policy smoke", started, allowed, stdoutParts, commands)
	}
	var allowedEval core.CommandPolicyEvaluation
	if err := json.Unmarshal([]byte(allowed.Stdout), &allowedEval); err != nil {
		return assertionStepWithOutput("command policy smoke", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !allowedEval.OK || !allowedEval.Allowed {
		return assertionStepWithOutput("command policy smoke", started, []string{"read-only git status was not allowed"}, stdoutParts, commands)
	}

	deniedOutside := runCommandStep(root, "policy deny outside", 30*time.Second, "", binary, "policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", outside, "--", "git", "status", "--short")
	stdoutParts = append(stdoutParts, deniedOutside.Stdout)
	commands = append(commands, deniedOutside.Command)
	if !deniedOutside.OK {
		return combineFailedStep("command policy smoke", started, deniedOutside, stdoutParts, commands)
	}
	var outsideEval core.CommandPolicyEvaluation
	if err := json.Unmarshal([]byte(deniedOutside.Stdout), &outsideEval); err != nil {
		return assertionStepWithOutput("command policy smoke", started, []string{err.Error()}, stdoutParts, commands)
	}
	if outsideEval.Allowed || !containsString(outsideEval.DenyReasons, "cwd_outside_workspace") {
		return assertionStepWithOutput("command policy smoke", started, []string{"outside cwd was not denied"}, stdoutParts, commands)
	}

	deniedShell := runCommandStep(root, "policy deny shell", 30*time.Second, "", binary, "policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "sh", "-c", "echo ok")
	stdoutParts = append(stdoutParts, deniedShell.Stdout)
	commands = append(commands, deniedShell.Command)
	if !deniedShell.OK {
		return combineFailedStep("command policy smoke", started, deniedShell, stdoutParts, commands)
	}
	var shellEval core.CommandPolicyEvaluation
	if err := json.Unmarshal([]byte(deniedShell.Stdout), &shellEval); err != nil {
		return assertionStepWithOutput("command policy smoke", started, []string{err.Error()}, stdoutParts, commands)
	}
	if shellEval.Allowed || !containsString(shellEval.DenyReasons, "shell_interpreter_not_allowed") {
		return assertionStepWithOutput("command policy smoke", started, []string{"shell command was not denied"}, stdoutParts, commands)
	}

	marker := filepath.Join(tempWorkspace, "marker")
	fakeRun := runCommandStep(root, "policy fake-run", 30*time.Second, "", binary, "policy", "fake-run", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--write", "--", "touch", "marker")
	stdoutParts = append(stdoutParts, fakeRun.Stdout)
	commands = append(commands, fakeRun.Command)
	if !fakeRun.OK {
		return combineFailedStep("command policy smoke", started, fakeRun, stdoutParts, commands)
	}
	var fakeResult core.CommandFakeRunResult
	if err := json.Unmarshal([]byte(fakeRun.Stdout), &fakeResult); err != nil {
		return assertionStepWithOutput("command policy smoke", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !fakeResult.OK || fakeResult.Executed || !fakeResult.Policy.Allowed {
		return assertionStepWithOutput("command policy smoke", started, []string{"fake-run did not report accepted non-execution"}, stdoutParts, commands)
	}
	if exists(marker) {
		return assertionStepWithOutput("command policy smoke", started, []string{"fake-run created marker; command executed unexpectedly"}, stdoutParts, commands)
	}

	return StepResult{
		Label:      "command policy smoke",
		Command:    strings.Join(commands, " && "),
		OK:         true,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     tail(strings.Join(stdoutParts, "\n"), 8*1024),
	}
}

func validateMCP(binary, root string) StepResult {
	tempState, err := os.MkdirTemp("", "agent-harness-mcp-state-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer os.RemoveAll(tempState)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"self-augment","version":"0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"harness://state"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"harness://docs"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"harness://command-policy"}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"state_prune","arguments":{"max_age":"1h"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"state_doctor","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"state_migrate","arguments":{}}}`,
	}, "\n") + "\n"
	step := runCommandStepEnv(root, "MCP smoke", 30*time.Second, input, []string{"HARNESS_STATE_DIR=" + tempState}, binary, "mcp")
	if !step.OK {
		return step
	}
	lines := splitLines(step.Stdout)
	if len(lines) != 9 {
		step.OK = false
		step.Error = fmt.Sprintf("expected 9 MCP responses, got %d", len(lines))
		return step
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			step.OK = false
			step.Error = fmt.Sprintf("response %d is invalid JSON: %v", i+1, err)
			return step
		}
		if _, ok := obj["result"]; !ok {
			step.OK = false
			step.Error = fmt.Sprintf("response %d has no result", i+1)
			return step
		}
	}
	if !strings.Contains(step.Stdout, "atomic_commit_preflight") || !strings.Contains(step.Stdout, "docs_index") || !strings.Contains(step.Stdout, "command_policy_check") || !strings.Contains(step.Stdout, "state_write") || !strings.Contains(step.Stdout, "state_prune") || !strings.Contains(step.Stdout, "state_doctor") || !strings.Contains(step.Stdout, "state_migrate") || !strings.Contains(step.Stdout, "self_augment_history") || !strings.Contains(step.Stdout, "self_augment_compare") || !strings.Contains(step.Stdout, "self_augment_promote") || !strings.Contains(step.Stdout, "dry_run") || !strings.Contains(step.Stdout, "healthy") || !strings.Contains(step.Stdout, "to_schema") || !strings.Contains(step.Stdout, "Lore:") {
		step.OK = false
		step.Error = "MCP smoke did not expose expected tool/resource"
	}
	step.Stdout = tail(step.Stdout, 8*1024)
	return step
}

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	started := time.Now()
	tempState, err := os.MkdirTemp("", "agent-harness-state-roundtrip-*")
	if err != nil {
		return failedStep("state roundtrip", err)
	}
	defer os.RemoveAll(tempState)

	key := fmt.Sprintf("self-augment-%d", seed)
	content := fmt.Sprintf("seed=%d\nLore: state roundtrip\n", seed)
	env := []string{"HARNESS_STATE_DIR=" + tempState}
	stdoutParts := []string{}
	commands := []string{}

	write := runCommandStepEnv(root, "state write", 30*time.Second, "", env, binary, "state", "write", "--key", key, "--value", content, "--json")
	stdoutParts = append(stdoutParts, write.Stdout)
	commands = append(commands, write.Command)
	if !write.OK {
		return combineFailedStep("state roundtrip", started, write, stdoutParts, commands)
	}
	var writeResult core.StateResult
	if err := json.Unmarshal([]byte(write.Stdout), &writeResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !writeResult.OK || writeResult.Record.Key != key || writeResult.Record.Content != content || writeResult.Record.Bytes != len([]byte(content)) {
		return assertionStepWithOutput("state roundtrip", started, []string{"write result did not match expected record"}, stdoutParts, commands)
	}

	read := runCommandStepEnv(root, "state read", 30*time.Second, "", env, binary, "state", "read", "--key", key, "--json")
	stdoutParts = append(stdoutParts, read.Stdout)
	commands = append(commands, read.Command)
	if !read.OK {
		return combineFailedStep("state roundtrip", started, read, stdoutParts, commands)
	}
	var readResult core.StateResult
	if err := json.Unmarshal([]byte(read.Stdout), &readResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !readResult.OK || readResult.Record.Key != key || readResult.Record.Content != content || readResult.Record.Bytes != len([]byte(content)) {
		return assertionStepWithOutput("state roundtrip", started, []string{"read result did not match expected record"}, stdoutParts, commands)
	}

	list := runCommandStepEnv(root, "state list", 30*time.Second, "", env, binary, "state", "list", "--json")
	stdoutParts = append(stdoutParts, list.Stdout)
	commands = append(commands, list.Command)
	if !list.OK {
		return combineFailedStep("state roundtrip", started, list, stdoutParts, commands)
	}
	var listResult core.StateListResult
	if err := json.Unmarshal([]byte(list.Stdout), &listResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !listResult.OK || !containsString(listResult.Keys, key) {
		return assertionStepWithOutput("state roundtrip", started, []string{"state list did not include roundtrip key"}, stdoutParts, commands)
	}

	oldKey := key + "-old"
	oldWrite := runCommandStepEnv(root, "state old write", 30*time.Second, "", env, binary, "state", "write", "--key", oldKey, "--value", "old state", "--json")
	stdoutParts = append(stdoutParts, oldWrite.Stdout)
	commands = append(commands, oldWrite.Command)
	if !oldWrite.OK {
		return combineFailedStep("state roundtrip", started, oldWrite, stdoutParts, commands)
	}
	var oldWriteResult core.StateResult
	if err := json.Unmarshal([]byte(oldWrite.Stdout), &oldWriteResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	oldWriteResult.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	b, err := json.MarshalIndent(oldWriteResult.Record, "", "  ")
	if err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if err := os.WriteFile(oldWriteResult.Path, append(b, '\n'), 0o600); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}

	pruneDry := runCommandStepEnv(root, "state prune dry-run", 30*time.Second, "", env, binary, "state", "prune", "--max-age", "1h", "--json")
	stdoutParts = append(stdoutParts, pruneDry.Stdout)
	commands = append(commands, pruneDry.Command)
	if !pruneDry.OK {
		return combineFailedStep("state roundtrip", started, pruneDry, stdoutParts, commands)
	}
	var pruneDryResult core.StatePruneResult
	if err := json.Unmarshal([]byte(pruneDry.Stdout), &pruneDryResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !pruneDryResult.OK || !pruneDryResult.DryRun || !containsString(pruneDryResult.DeletedKeys, oldKey) || !containsString(pruneDryResult.KeptKeys, key) {
		return assertionStepWithOutput("state roundtrip", started, []string{"state prune dry-run did not classify old/fresh keys"}, stdoutParts, commands)
	}

	pruneConfirm := runCommandStepEnv(root, "state prune confirm", 30*time.Second, "", env, binary, "state", "prune", "--max-age", "1h", "--confirm", "--json")
	stdoutParts = append(stdoutParts, pruneConfirm.Stdout)
	commands = append(commands, pruneConfirm.Command)
	if !pruneConfirm.OK {
		return combineFailedStep("state roundtrip", started, pruneConfirm, stdoutParts, commands)
	}
	var pruneConfirmResult core.StatePruneResult
	if err := json.Unmarshal([]byte(pruneConfirm.Stdout), &pruneConfirmResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !pruneConfirmResult.OK || pruneConfirmResult.DryRun || !pruneConfirmResult.Confirm || !containsString(pruneConfirmResult.DeletedKeys, oldKey) {
		return assertionStepWithOutput("state roundtrip", started, []string{"state prune confirm did not delete old key"}, stdoutParts, commands)
	}

	listAfterPrune := runCommandStepEnv(root, "state list after prune", 30*time.Second, "", env, binary, "state", "list", "--json")
	stdoutParts = append(stdoutParts, listAfterPrune.Stdout)
	commands = append(commands, listAfterPrune.Command)
	if !listAfterPrune.OK {
		return combineFailedStep("state roundtrip", started, listAfterPrune, stdoutParts, commands)
	}
	var listAfterPruneResult core.StateListResult
	if err := json.Unmarshal([]byte(listAfterPrune.Stdout), &listAfterPruneResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !containsString(listAfterPruneResult.Keys, key) || containsString(listAfterPruneResult.Keys, oldKey) {
		return assertionStepWithOutput("state roundtrip", started, []string{"state prune did not preserve fresh key and remove old key"}, stdoutParts, commands)
	}

	legacyKey := key + "-legacy"
	legacyRecord := core.StateRecord{
		Key:       legacyKey,
		Content:   "legacy state",
		UpdatedAt: "2000-01-01T00:00:00Z",
		Bytes:     len([]byte("legacy state")),
	}
	legacyBytes, err := json.MarshalIndent(legacyRecord, "", "  ")
	if err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if err := os.WriteFile(filepath.Join(tempState, legacyKey+".json"), append(legacyBytes, '\n'), 0o600); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	migrateDry := runCommandStepEnv(root, "state migrate dry-run", 30*time.Second, "", env, binary, "state", "migrate", "--json")
	stdoutParts = append(stdoutParts, migrateDry.Stdout)
	commands = append(commands, migrateDry.Command)
	if !migrateDry.OK {
		return combineFailedStep("state roundtrip", started, migrateDry, stdoutParts, commands)
	}
	var migrateDryResult core.StateMigrateResult
	if err := json.Unmarshal([]byte(migrateDry.Stdout), &migrateDryResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !migrateDryResult.OK || !migrateDryResult.DryRun || !containsString(migrateDryResult.CandidateKeys, legacyKey) || len(migrateDryResult.MigratedKeys) != 0 {
		return assertionStepWithOutput("state roundtrip", started, []string{"state migrate dry-run did not classify legacy key"}, stdoutParts, commands)
	}
	migrateConfirm := runCommandStepEnv(root, "state migrate confirm", 30*time.Second, "", env, binary, "state", "migrate", "--confirm", "--json")
	stdoutParts = append(stdoutParts, migrateConfirm.Stdout)
	commands = append(commands, migrateConfirm.Command)
	if !migrateConfirm.OK {
		return combineFailedStep("state roundtrip", started, migrateConfirm, stdoutParts, commands)
	}
	var migrateConfirmResult core.StateMigrateResult
	if err := json.Unmarshal([]byte(migrateConfirm.Stdout), &migrateConfirmResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !migrateConfirmResult.OK || migrateConfirmResult.DryRun || !migrateConfirmResult.Confirm || !containsString(migrateConfirmResult.MigratedKeys, legacyKey) {
		return assertionStepWithOutput("state roundtrip", started, []string{"state migrate confirm did not migrate legacy key"}, stdoutParts, commands)
	}
	migratedRead := runCommandStepEnv(root, "state migrated read", 30*time.Second, "", env, binary, "state", "read", "--key", legacyKey, "--json")
	stdoutParts = append(stdoutParts, migratedRead.Stdout)
	commands = append(commands, migratedRead.Command)
	if !migratedRead.OK {
		return combineFailedStep("state roundtrip", started, migratedRead, stdoutParts, commands)
	}
	var migratedReadResult core.StateResult
	if err := json.Unmarshal([]byte(migratedRead.Stdout), &migratedReadResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if migratedReadResult.Record.SchemaVersion != core.StateCurrentSchemaVersion || migratedReadResult.Record.Content != legacyRecord.Content {
		return assertionStepWithOutput("state roundtrip", started, []string{"state migrate did not preserve content or set current schema"}, stdoutParts, commands)
	}
	doctorHealthy := runCommandStepEnv(root, "state doctor after migrate", 30*time.Second, "", env, binary, "state", "doctor", "--json")
	stdoutParts = append(stdoutParts, doctorHealthy.Stdout)
	commands = append(commands, doctorHealthy.Command)
	if !doctorHealthy.OK {
		return combineFailedStep("state roundtrip", started, doctorHealthy, stdoutParts, commands)
	}
	var doctorHealthyResult core.StateDoctorResult
	if err := json.Unmarshal([]byte(doctorHealthy.Stdout), &doctorHealthyResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !doctorHealthyResult.OK || !doctorHealthyResult.Healthy {
		return assertionStepWithOutput("state roundtrip", started, []string{"state doctor was not healthy after migrating legacy fixture"}, stdoutParts, commands)
	}

	baselineCompareKey := key + "-compare-base"
	candidateCompareKey := key + "-compare-candidate"
	compareSummary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: seed, Label: "go test", DurationMS: 1000},
		},
	}
	if err := writeSelfAugmentSnapshotRecord(tempState, baselineCompareKey, SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      seed,
		ElapsedMS:     1000,
		HarnessRoot:   root,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       compareSummary,
	}); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if err := writeSelfAugmentSnapshotRecord(tempState, candidateCompareKey, SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      seed,
		ElapsedMS:     1100,
		HarnessRoot:   root,
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       compareSummary,
	}); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	compareOK := runCommandStepEnv(root, "self augment compare ok", 30*time.Second, "", env, binary, "self-augment", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--json")
	stdoutParts = append(stdoutParts, compareOK.Stdout)
	commands = append(commands, compareOK.Command)
	if !compareOK.OK {
		return combineFailedStep("state roundtrip", started, compareOK, stdoutParts, commands)
	}
	var compareOKResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareOK.Stdout), &compareOKResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !compareOKResult.OK || compareOKResult.Regressed || compareOKResult.ElapsedDeltaMS != 100 {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-augment compare reported unexpected non-regression result"}, stdoutParts, commands)
	}
	compareRegression := runCommandStepEnv(root, "self augment compare regression", 30*time.Second, "", env, binary, "self-augment", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--max-elapsed-regression-pct", "5", "--json")
	stdoutParts = append(stdoutParts, compareRegression.Stdout)
	commands = append(commands, compareRegression.Command)
	if !compareRegression.OK {
		return combineFailedStep("state roundtrip", started, compareRegression, stdoutParts, commands)
	}
	var compareRegressionResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareRegression.Stdout), &compareRegressionResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !compareRegressionResult.OK || !compareRegressionResult.Regressed || len(compareRegressionResult.Regressions) == 0 {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-augment compare did not report expected elapsed regression"}, stdoutParts, commands)
	}
	promotedBaselineKey := key + "-promoted-baseline"
	promoteDry := runCommandStepEnv(root, "self augment promote dry-run", 30*time.Second, "", env, binary, "self-augment", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--json")
	stdoutParts = append(stdoutParts, promoteDry.Stdout)
	commands = append(commands, promoteDry.Command)
	if !promoteDry.OK {
		return combineFailedStep("state roundtrip", started, promoteDry, stdoutParts, commands)
	}
	var promoteDryResult SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(promoteDry.Stdout), &promoteDryResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !promoteDryResult.OK || !promoteDryResult.DryRun || promoteDryResult.Promoted {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-augment promote dry-run mutated state or did not report dry-run"}, stdoutParts, commands)
	}
	if _, err := core.StateRead(promotedBaselineKey); err == nil {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-augment promote dry-run wrote baseline unexpectedly"}, stdoutParts, commands)
	}
	promoteConfirm := runCommandStepEnv(root, "self augment promote confirm", 30*time.Second, "", env, binary, "self-augment", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--confirm", "--json")
	stdoutParts = append(stdoutParts, promoteConfirm.Stdout)
	commands = append(commands, promoteConfirm.Command)
	if !promoteConfirm.OK {
		return combineFailedStep("state roundtrip", started, promoteConfirm, stdoutParts, commands)
	}
	var promoteConfirmResult SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(promoteConfirm.Stdout), &promoteConfirmResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !promoteConfirmResult.OK || promoteConfirmResult.DryRun || !promoteConfirmResult.Promoted {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-augment promote confirm did not write baseline"}, stdoutParts, commands)
	}
	comparePromoted := runCommandStepEnv(root, "self augment compare promoted", 30*time.Second, "", env, binary, "self-augment", "compare", "--baseline-key", promotedBaselineKey, "--candidate-key", candidateCompareKey, "--json")
	stdoutParts = append(stdoutParts, comparePromoted.Stdout)
	commands = append(commands, comparePromoted.Command)
	if !comparePromoted.OK {
		return combineFailedStep("state roundtrip", started, comparePromoted, stdoutParts, commands)
	}
	var comparePromotedResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(comparePromoted.Stdout), &comparePromotedResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !comparePromotedResult.OK || comparePromotedResult.Regressed || comparePromotedResult.ElapsedDeltaMS != 0 {
		return assertionStepWithOutput("state roundtrip", started, []string{"promoted baseline did not compare cleanly with candidate"}, stdoutParts, commands)
	}
	history := runCommandStepEnv(root, "self augment history", 30*time.Second, "", env, binary, "self-augment", "history", "--prefix", key+"-", "--json")
	stdoutParts = append(stdoutParts, history.Stdout)
	commands = append(commands, history.Command)
	if !history.OK {
		return combineFailedStep("state roundtrip", started, history, stdoutParts, commands)
	}
	var historyResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(history.Stdout), &historyResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	historyKeys := []string{}
	for _, entry := range historyResult.Entries {
		historyKeys = append(historyKeys, entry.Key)
	}
	if !historyResult.OK || historyResult.TotalMatches < 3 || !containsString(historyKeys, baselineCompareKey) || !containsString(historyKeys, candidateCompareKey) || !containsString(historyKeys, promotedBaselineKey) {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-augment history did not list saved baseline/candidate/promoted summaries"}, stdoutParts, commands)
	}

	corruptPath := filepath.Join(tempState, "corrupt.json")
	if err := os.WriteFile(corruptPath, []byte("{not json\n"), 0o600); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	doctor := runCommandStepEnv(root, "state doctor", 30*time.Second, "", env, binary, "state", "doctor", "--json")
	stdoutParts = append(stdoutParts, doctor.Stdout)
	commands = append(commands, doctor.Command)
	if !doctor.OK {
		return combineFailedStep("state roundtrip", started, doctor, stdoutParts, commands)
	}
	var doctorResult core.StateDoctorResult
	if err := json.Unmarshal([]byte(doctor.Stdout), &doctorResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if !doctorResult.OK || doctorResult.Healthy || !containsString(doctorResult.ValidKeys, key) || !stateDoctorHasIssueCode(doctorResult.Issues, "invalid_json") {
		return assertionStepWithOutput("state roundtrip", started, []string{"state doctor did not report corrupt fixture and preserve valid key"}, stdoutParts, commands)
	}

	return StepResult{
		Label:      "state roundtrip",
		Command:    strings.Join(commands, " && "),
		OK:         true,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     tail(strings.Join(stdoutParts, "\n"), 8*1024),
	}
}

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	started := time.Now()
	tempRepo, err := os.MkdirTemp("", "agent-harness-preflight-fuzz-*")
	if err != nil {
		return failedStep("preflight fuzz", err)
	}
	defer os.RemoveAll(tempRepo)
	if code, _, stderr := core.GitCmd(tempRepo, "init", "-q"); code != 0 {
		return failedStep("preflight fuzz", fmt.Errorf("git init: %s", stderr))
	}
	if err := os.WriteFile(filepath.Join(tempRepo, "file.txt"), []byte("seed="+strconv.FormatInt(seed, 10)+"\n"), 0o644); err != nil {
		return failedStep("preflight fuzz", err)
	}
	if code, _, stderr := core.GitCmd(tempRepo, "add", "file.txt"); code != 0 {
		return failedStep("preflight fuzz", fmt.Errorf("git add: %s", stderr))
	}
	msg := "docs(test): add seeded sample"
	body := "Lore:\n- Intent: Validate seeded preflight fuzz.\n- Why: Self-augmentation needs deterministic git fixtures.\n- Changes:\n  - Add sample file.\n- Verify: harness self-augment\n- Risk: Low"
	commitArgs := []string{"-c", "user.name=Self Augment", "-c", "user.email=self-augment@example.invalid", "commit", "-q", "-m", msg, "-m", body}
	if code, _, stderr := core.GitCmd(tempRepo, commitArgs...); code != 0 {
		return failedStep("preflight fuzz", fmt.Errorf("git commit: %s", stderr))
	}
	secretName := ".env"
	if seed%2 == 0 {
		secretName = "nested.secret"
	}
	if err := os.WriteFile(filepath.Join(tempRepo, secretName), []byte("TOKEN=redacted\n"), 0o600); err != nil {
		return failedStep("preflight fuzz", err)
	}
	step := runCommandStep(root, "preflight fuzz", 30*time.Second, "", binary, "preflight", "--json", tempRepo)
	step.DurationMS = time.Since(started).Milliseconds()
	if !step.OK {
		return step
	}
	var preflight core.PreflightResult
	if err := json.Unmarshal([]byte(step.Stdout), &preflight); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := []string{}
	if !preflight.OK {
		errs = append(errs, "preflight ok=false")
	}
	if preflight.CommitStyleHints["conventional_subjects"] != float64(1) {
		errs = append(errs, "conventional subject not detected")
	}
	if preflight.CommitStyleHints["lore_bodies"] != float64(1) {
		errs = append(errs, "Lore body not detected")
	}
	if len(preflight.SecretLikePaths) == 0 {
		errs = append(errs, "secret-like path not detected")
	}
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func validateNativeIntegration(root string) StepResult {
	started := time.Now()
	home, _ := os.UserHomeDir()
	errs := []string{}
	paths := []string{
		filepath.Join(home, ".codex", "skills", skillName, "SKILL.md"),
		filepath.Join(home, ".claude", "skills", skillName, "SKILL.md"),
		filepath.Join(root, ".claude", "skills", skillName, "SKILL.md"),
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
	}
	for _, path := range paths {
		if !exists(path) {
			errs = append(errs, "missing "+path)
		}
	}
	if b, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(b), "[mcp_servers.agent_harness]") {
		errs = append(errs, "Codex MCP config missing agent_harness")
	}
	return assertionStep("native integration", started, errs)
}

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return runCommandStepEnv(dir, label, timeout, stdin, nil, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	step := StepResult{
		Label:      label,
		Command:    strings.Join(append([]string{name}, args...), " "),
		OK:         err == nil,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     tail(stdout.String(), 256*1024),
		Stderr:     tail(stderr.String(), 256*1024),
	}
	if ctx.Err() == context.DeadlineExceeded {
		step.OK = false
		step.Error = "timeout after " + timeout.String()
	} else if err != nil {
		step.Error = err.Error()
	}
	return step
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	step := StepResult{
		Label:      label,
		Command:    strings.Join(commands, " && "),
		OK:         false,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     tail(strings.Join(stdoutParts, "\n"), 8*1024),
		Stderr:     child.Stderr,
		Error:      child.Label + ": " + child.Error,
	}
	if step.Error == child.Label+": " {
		step.Error = child.Label + " failed"
	}
	return step
}

func assertionStep(label string, started time.Time, errs []string) StepResult {
	step := StepResult{Label: label, OK: len(errs) == 0, DurationMS: time.Since(started).Milliseconds()}
	if len(errs) > 0 {
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	step := assertionStep(label, started, errs)
	step.Command = strings.Join(commands, " && ")
	step.Stdout = tail(strings.Join(stdoutParts, "\n"), 8*1024)
	return step
}

func failedStep(label string, err error) StepResult {
	return StepResult{Label: label, OK: false, Error: err.Error()}
}

func printStep(step StepResult) {
	if step.OK {
		fmt.Printf("→ %s ok (%dms)\n", step.Label, step.DurationMS)
		return
	}
	fmt.Printf("→ %s failed (%dms): %s\n", step.Label, step.DurationMS, step.Error)
	if step.Stdout != "" {
		fmt.Printf("  stdout:\n%s\n", indentLines(step.Stdout))
	}
	if step.Stderr != "" {
		fmt.Printf("  stderr:\n%s\n", indentLines(step.Stderr))
	}
}

func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func indentLines(s string) string {
	lines := splitLines(s)
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func runMCP() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeRPCError(nil, -32700, "Parse error", err.Error())
			continue
		}
		if len(req.ID) == 0 {
			handleNotification(req)
			continue
		}
		result, rpcErr := handleRequest(req)
		if rpcErr != nil {
			writeRPCError(req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			continue
		}
		writeRPCResult(req.ID, result)
	}
	return scanner.Err()
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int
	Message string
	Data    any
}

func handleNotification(req rpcRequest) {
	// notifications/initialized and cancellation notifications intentionally require no response.
	fmt.Fprintln(os.Stderr, "harness mcp notification:", req.Method)
}

func handleRequest(req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
			},
			"serverInfo":   map[string]any{"name": "agent-harness", "version": version},
			"instructions": "Use harness tools for shared Codex/Claude harness inspection, atomic commit preflight, state checkpoints, self-augmentation, and commit policy context.",
		}, nil
	case "tools/list":
		return map[string]any{"tools": mcpTools()}, nil
	case "tools/call":
		return handleToolCall(req.Params)
	case "resources/list":
		return map[string]any{"resources": mcpResources()}, nil
	case "resources/read":
		return handleResourceRead(req.Params)
	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found", Data: req.Method}
	}
}

func mcpTools() []map[string]any {
	return []map[string]any{
		{
			"name":        "harness_inspect",
			"description": "Inspect the agent harness installation, shared skills, docs, and native Codex/Claude integration status.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"repo": map[string]any{"type": "string", "description": "Optional target repository path."}}},
		},
		{
			"name":        "atomic_commit_preflight",
			"description": "Run a read-only git preflight for the atomic-commit-push workflow: branch/upstream, staged/unstaged/untracked files, secret-like paths, and commit style hints.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Git repository path. Defaults to the agent project directory."}}},
		},
		{
			"name":        "commit_policy",
			"description": "Return the Conventional Commit + Lore body policy used by this harness.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "skill_manifest",
			"description": "List shared skills exposed by the harness and their metadata.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "docs_index",
			"description": "Return a lightweight index of AGENTS.md, CLAUDE.md, and agent_docs markdown files: relative path, title, headings, and byte size.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "command_policy_check",
			"description": "Evaluate whether an argv-based command request is allowed by the harness command policy without executing it.",
			"inputSchema": commandPolicyInputSchema(),
		},
		{
			"name":        "command_fake_run",
			"description": "Run the command policy and return a fake runner result. This never executes the command; it only proves policy acceptance/denial and audit metadata.",
			"inputSchema": commandPolicyInputSchema(),
		},
		{
			"name":        "state_write",
			"description": "Write a small agent state checkpoint to HARNESS_STATE_DIR or ~/.local/state/agent-harness. Keys allow [A-Za-z0-9._-] and cannot contain path traversal.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"key", "content"}, "properties": map[string]any{
				"key":     map[string]any{"type": "string", "description": "State key, max 128 chars, no path separators."},
				"content": map[string]any{"type": "string", "description": "State checkpoint content."},
			}},
		},
		{
			"name":        "state_read",
			"description": "Read an agent state checkpoint by key.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"key"}, "properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "State key."},
			}},
		},
		{
			"name":        "state_list",
			"description": "List agent state checkpoint keys and metadata.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "state_prune",
			"description": "Prune old agent state checkpoints. Defaults to dry-run; pass confirm=true to delete records older than max_age.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"max_age"}, "properties": map[string]any{
				"max_age": map[string]any{"type": "string", "description": "Duration such as 720h or 168h. Must be positive."},
				"confirm": map[string]any{"type": "boolean", "description": "When true, delete matching records; false or omitted performs a dry-run."},
			}},
		},
		{
			"name":        "state_doctor",
			"description": "Inspect agent state checkpoint files for invalid JSON, key mismatches, byte-count drift, invalid timestamps, and unexpected files without modifying state.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "state_migrate",
			"description": "Migrate valid legacy state checkpoints to the current schema. Defaults to dry-run; pass confirm=true to rewrite eligible records.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"confirm": map[string]any{"type": "boolean", "description": "When true, rewrite legacy records; false or omitted performs a dry-run."},
			}},
		},
		{
			"name":        "self_augment",
			"description": "Run the harness self-augmentation validation loop inspired by eye-tracking-scroll: at least 10 seeded iterations of invariants, build/test, CLI, MCP, state roundtrip, native integration, and git preflight fuzz checks. Can optionally save a compact summary checkpoint to harness state.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"iterations": map[string]any{"type": "integer", "description": "Iteration count; must be at least 10."},
				"seed":       map[string]any{"type": "integer", "description": "Base seed for deterministic per-iteration fuzz fixtures."},
				"save_state": map[string]any{"type": "boolean", "description": "When true, save compact summary to harness state after the run."},
				"state_key":  map[string]any{"type": "string", "description": "State key for save_state; defaults to self-augment-latest."},
			}},
		},
		{
			"name":        "self_augment_history",
			"description": "List saved self-augment summary checkpoints from harness state, sorted by snapshot generation time for quick baseline/candidate discovery.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"prefix": map[string]any{"type": "string", "description": "State key prefix to scan; defaults to self-augment. Use empty string to scan all keys."},
				"limit":  map[string]any{"type": "integer", "description": "Maximum entries to return; defaults to 20, 0 returns all."},
			}},
		},
		{
			"name":        "self_augment_compare",
			"description": "Compare two saved self-augment summary checkpoints from harness state and report elapsed-time, failed-step, and step-label regressions.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"baseline_key", "candidate_key"}, "properties": map[string]any{
				"baseline_key":               map[string]any{"type": "string", "description": "State key containing the baseline self-augment summary snapshot."},
				"candidate_key":              map[string]any{"type": "string", "description": "State key containing the candidate self-augment summary snapshot."},
				"max_elapsed_regression_pct": map[string]any{"type": "number", "description": "Allowed elapsed_ms increase percentage before regression; defaults to 20."},
			}},
		},
		{
			"name":        "self_augment_promote",
			"description": "Promote a saved self-augment summary checkpoint to a baseline state key. Defaults to dry-run; pass confirm=true to write the baseline.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"from_key", "baseline_key"}, "properties": map[string]any{
				"from_key":     map[string]any{"type": "string", "description": "State key containing the candidate self-augment summary snapshot to promote."},
				"baseline_key": map[string]any{"type": "string", "description": "State key to write as the promoted baseline."},
				"confirm":      map[string]any{"type": "boolean", "description": "When true, write baseline_key; false or omitted performs a dry-run."},
			}},
		},
	}
}

func commandPolicyInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"workspace_root", "cwd", "argv"},
		"properties": map[string]any{
			"workspace_root":  map[string]any{"type": "string", "description": "Workspace root boundary."},
			"cwd":             map[string]any{"type": "string", "description": "Command working directory; must be inside workspace_root."},
			"argv":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Command argv array. Shell strings are not accepted."},
			"timeout":         map[string]any{"type": "string", "description": "Duration such as 30s or 2m; max 15m."},
			"env_allowlist":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed environment variable names."},
			"network_allowed": map[string]any{"type": "boolean", "description": "Whether network access is allowed."},
			"write_allowed":   map[string]any{"type": "boolean", "description": "Whether workspace writes are allowed."},
			"shell_allowed":   map[string]any{"type": "boolean", "description": "Whether shell interpreter argv[0] is allowed."},
			"shell_reason":    map[string]any{"type": "string", "description": "Required reason when shell_allowed is true."},
		},
	}
}

func commandPolicyRequestFromArgs(args map[string]any) core.CommandPolicyRequest {
	return core.CommandPolicyRequest{
		WorkspaceRoot:  stringArg(args, "workspace_root"),
		CWD:            stringArg(args, "cwd"),
		Argv:           stringSliceArg(args, "argv"),
		Timeout:        stringArgWithDefault(args, "timeout", "30s"),
		EnvAllowlist:   stringSliceArg(args, "env_allowlist"),
		NetworkAllowed: boolArg(args, "network_allowed"),
		WriteAllowed:   boolArg(args, "write_allowed"),
		ShellAllowed:   boolArg(args, "shell_allowed"),
		ShellReason:    stringArg(args, "shell_reason"),
		AuditLogID:     stringArg(args, "audit_log_id"),
	}
}

func handleToolCall(params json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params", Data: err.Error()}
	}
	var payload any
	switch call.Name {
	case "harness_inspect":
		payload = inspectHarness(stringArg(call.Arguments, "repo"))
	case "atomic_commit_preflight":
		payload = core.GitPreflight(resolveTarget(stringArg(call.Arguments, "path")), harnessRoot())
	case "commit_policy":
		text, err := readHarnessFile("agent_docs", "COMMIT_POLICY.md")
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Cannot read commit policy", Data: err.Error()}
		}
		return textResult(text), nil
	case "skill_manifest":
		payload = core.ListSkills(harnessRoot(), skillName)
	case "docs_index":
		payload = core.DocsIndex(harnessRoot(), version)
	case "command_policy_check":
		payload = core.EvaluateCommandPolicy(commandPolicyRequestFromArgs(call.Arguments))
	case "command_fake_run":
		payload = core.FakeRunCommand(commandPolicyRequestFromArgs(call.Arguments))
	case "state_write":
		result, err := core.StateWrite(stringArg(call.Arguments, "key"), stringArg(call.Arguments, "content"))
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "State write failed", Data: err.Error()}
		}
		payload = result
	case "state_read":
		result, err := core.StateRead(stringArg(call.Arguments, "key"))
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "State read failed", Data: err.Error()}
		}
		payload = result
	case "state_list":
		result, err := core.StateList()
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "State list failed", Data: err.Error()}
		}
		payload = result
	case "state_prune":
		maxAge, err := time.ParseDuration(stringArg(call.Arguments, "max_age"))
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "State prune failed", Data: "invalid max_age: " + err.Error()}
		}
		result, err := core.StatePrune(maxAge, boolArg(call.Arguments, "confirm"))
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "State prune failed", Data: err.Error()}
		}
		payload = result
	case "state_doctor":
		result, err := core.StateDoctor()
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "State doctor failed", Data: err.Error()}
		}
		payload = result
	case "state_migrate":
		result, err := core.StateMigrate(boolArg(call.Arguments, "confirm"))
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "State migrate failed", Data: err.Error()}
		}
		payload = result
	case "self_augment":
		iterations := intArg(call.Arguments, "iterations", 10)
		seed := int64Arg(call.Arguments, "seed", time.Now().Unix())
		result, err := selfAugment(iterations, seed, false)
		if boolArg(call.Arguments, "save_state") {
			saveErr := saveSelfAugmentSummary(&result, stringArgWithDefault(call.Arguments, "state_key", "self-augment-latest"))
			if err == nil && saveErr != nil {
				err = saveErr
			}
		}
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Self-augmentation failed", Data: result}
		}
		payload = result
	case "self_augment_history":
		result, err := selfAugmentHistory(
			stringArgWithDefault(call.Arguments, "prefix", "self-augment"),
			intArg(call.Arguments, "limit", 20),
		)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-augment history failed", Data: err.Error()}
		}
		payload = result
	case "self_augment_compare":
		result, err := compareSelfAugmentSummaries(
			stringArg(call.Arguments, "baseline_key"),
			stringArg(call.Arguments, "candidate_key"),
			floatArg(call.Arguments, "max_elapsed_regression_pct", 20),
		)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-augment compare failed", Data: err.Error()}
		}
		payload = result
	case "self_augment_promote":
		result, err := promoteSelfAugmentBaseline(
			stringArg(call.Arguments, "from_key"),
			stringArg(call.Arguments, "baseline_key"),
			boolArg(call.Arguments, "confirm"),
		)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-augment promote failed", Data: err.Error()}
		}
		payload = result
	default:
		return nil, &rpcError{Code: -32602, Message: "Unknown tool", Data: call.Name}
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return textResult(string(b)), nil
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func mcpResources() []map[string]any {
	return []map[string]any{
		{"uri": "harness://commit-policy", "name": "Commit policy", "description": "Conventional Commit + Lore body policy.", "mimeType": "text/markdown"},
		{"uri": "harness://skill/atomic-commit-push", "name": "atomic-commit-push skill", "description": "Shared native skill instructions.", "mimeType": "text/markdown"},
		{"uri": "harness://agents", "name": "Agent root rules", "description": "AGENTS.md root operating contract.", "mimeType": "text/markdown"},
		{"uri": "harness://docs", "name": "Agent docs index", "description": "JSON index of harness agent-facing markdown docs.", "mimeType": "application/json"},
		{"uri": "harness://command-policy", "name": "Command policy summary", "description": "JSON summary of command policy boundaries and fake runner behavior.", "mimeType": "application/json"},
		{"uri": "harness://state", "name": "State checkpoint index", "description": "JSON index of harness state checkpoints.", "mimeType": "application/json"},
	}
}

func handleResourceRead(params json.RawMessage) (any, *rpcError) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params", Data: err.Error()}
	}
	if req.URI == "harness://docs" {
		result := core.DocsIndex(harnessRoot(), version)
		b, _ := json.MarshalIndent(result, "", "  ")
		return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "application/json", "text": string(b)}}}, nil
	}
	if req.URI == "harness://command-policy" {
		b, _ := json.MarshalIndent(core.CommandPolicySummary(), "", "  ")
		return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "application/json", "text": string(b)}}}, nil
	}
	if req.URI == "harness://state" {
		result, err := core.StateList()
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Cannot read state index", Data: err.Error()}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "application/json", "text": string(b)}}}, nil
	}
	var rel []string
	switch req.URI {
	case "harness://commit-policy":
		rel = []string{"agent_docs", "COMMIT_POLICY.md"}
	case "harness://skill/atomic-commit-push":
		rel = []string{"skills", skillName, "SKILL.md"}
	case "harness://agents":
		rel = []string{"AGENTS.md"}
	default:
		return nil, &rpcError{Code: -32602, Message: "Unknown resource", Data: req.URI}
	}
	text, err := readHarnessFile(rel...)
	if err != nil {
		return nil, &rpcError{Code: -32000, Message: "Cannot read resource", Data: err.Error()}
	}
	return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "text/markdown", "text": text}}}, nil
}

func writeRPCResult(id json.RawMessage, result any) {
	msg := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result}
	b, _ := json.Marshal(msg)
	fmt.Println(string(b))
}

func writeRPCError(id json.RawMessage, code int, message string, data any) {
	msg := map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": code, "message": message, "data": data}}
	if id != nil {
		msg["id"] = json.RawMessage(id)
	} else {
		msg["id"] = nil
	}
	b, _ := json.Marshal(msg)
	fmt.Println(string(b))
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func stringArgWithDefault(args map[string]any, key, fallback string) string {
	if v := stringArg(args, key); v != "" {
		return v
	}
	return fallback
}

func stringSliceArg(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []string:
		return append([]string{}, v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(v)
	default:
		return nil
	}
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		return err == nil && parsed
	default:
		return false
	}
}

func intArg(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func int64Arg(args map[string]any, key string, fallback int64) int64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func floatArg(args map[string]any, key string, fallback float64) float64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func readHarnessFile(parts ...string) (string, error) {
	path := filepath.Join(append([]string{harnessRoot()}, parts...)...)
	b, err := os.ReadFile(path)
	return string(b), err
}

func harnessRoot() string {
	if env := os.Getenv("HARNESS_ROOT"); env != "" {
		if root, err := filepath.Abs(env); err == nil {
			return root
		}
	}
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		starts = append(starts, d, filepath.Dir(d))
	}
	for _, start := range starts {
		if root, ok := findUp(start, filepath.Join("skills", skillName, "SKILL.md")); ok {
			return root
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func findUp(start, marker string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if exists(filepath.Join(d, marker)) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

func resolveTarget(arg string) string {
	if arg == "" {
		if env := os.Getenv("CLAUDE_PROJECT_DIR"); env != "" {
			arg = env
		} else if env := os.Getenv("PWD"); env != "" {
			arg = env
		} else if cwd, err := os.Getwd(); err == nil {
			arg = cwd
		} else {
			arg = "."
		}
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return arg
	}
	return abs
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func stateDoctorHasIssueCode(issues []core.StateDoctorIssue, want string) bool {
	for _, issue := range issues {
		if issue.Code == want {
			return true
		}
	}
	return false
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
