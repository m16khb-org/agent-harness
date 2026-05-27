package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-harness/internal/core"
)

const version = "0.1.0"
const skillName = "atomic-commit-push"
const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024

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
	case "self-verify":
		if err := runSelfVerify(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-verify:", err)
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
	case "llm-wiki":
		if err := runLLMWiki(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "llm-wiki:", err)
			os.Exit(1)
		}
	case "install-native":
		if err := runInstallNative(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "install-native:", err)
			os.Exit(1)
		}
	case "daemon":
		if err := runDaemon(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "daemon:", err)
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
  harness llm-wiki inventory [--root PATH] [--project PATH] [--json]
  harness llm-wiki session-context [--root PATH] [--project PATH] [--json]
  harness llm-wiki search --query TEXT [--root PATH] [--limit N] [--json]
  harness llm-wiki read --page PATH_OR_SLUG [--root PATH] [--json]
  harness llm-wiki capture --title TITLE (--content TEXT|--input FILE|--stdin) [--type session|concept|entity|summary] [--json]
  harness daemon start|status|stop [--json]
  harness install-native [--llm-wiki-root PATH] [--project-local] [--no-claude-user-hook] [--dry-run] [--json]
  harness self-verify [--iterations=10] [--seed=N] [--target-score=95] [--progress=none|jsonl] [--save-state] [--state-key KEY] [--json]
  harness self-verify history [--prefix PREFIX] [--limit N] [--retention-limit N] [--prune-retention] [--confirm] [--json]
  harness self-verify compare --baseline-key KEY --candidate-key KEY [--max-elapsed-regression-pct N] [--fail-on-regression] [--json]
  harness self-verify promote --from-key KEY --baseline-key KEY [--confirm] [--json]
  harness self-augment [--cycles=1] [--target-score=95] [--save-state] [--state-key KEY] [--json]
  harness self-augment lesson [--candidate ID] --lesson TEXT --next-action TEXT [--source TEXT] [--severity info|warning|error] [--state-key KEY] [--json]
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
	fmt.Printf("Claude user SessionStart hook: %v\n", info.Integration.ClaudeUserHook)
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

func runSelfVerify(args []string) error {
	if len(args) > 0 && args[0] == "history" {
		return runSelfVerifyHistory(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return runSelfVerifyCompare(args[1:])
	}
	if len(args) > 0 && args[0] == "promote" {
		return runSelfVerifyPromote(args[1:])
	}
	fs := flag.NewFlagSet("self-verify", flag.ContinueOnError)
	iterations := fs.Int("iterations", 10, "number of self-verification loop iterations; must be at least 10")
	seed := fs.Int64("seed", time.Now().Unix(), "base seed for randomized checks")
	targetScore := fs.Float64("target-score", 95, "exclusive per-goal score threshold; every concrete goal must score above this value to terminate")
	saveState := fs.Bool("save-state", false, "save compact self-verification summary to harness state")
	stateKey := fs.String("state-key", "self-verify-latest", "state key for --save-state")
	progress := fs.String("progress", "none", "progress output mode: none or jsonl; jsonl writes JSON Lines events to stderr")
	jsonOut := fs.Bool("json", false, "print JSON summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *targetScore < 0 || *targetScore >= 100 {
		return fmt.Errorf("target-score must be >= 0 and < 100")
	}
	progressReporter, err := newSelfVerifyProgressReporter(*progress, os.Stderr)
	if err != nil {
		return err
	}
	result, err := selfVerifyWithProgress(*iterations, *seed, *targetScore, !*jsonOut, progressReporter)
	saveErr := error(nil)
	if *saveState {
		saveErr = saveSelfVerificationSummary(&result, *stateKey)
	}
	if *jsonOut {
		_ = printJSON(result)
	}
	if err == nil && saveErr != nil {
		return saveErr
	}
	return err
}

func runSelfVerifyCompare(args []string) error {
	fs := flag.NewFlagSet("self-verify compare", flag.ContinueOnError)
	baselineKey := fs.String("baseline-key", "", "state key containing the baseline self-verification summary snapshot")
	candidateKey := fs.String("candidate-key", "", "state key containing the candidate self-verification summary snapshot")
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
		fmt.Printf("self-verify compare %s: elapsed_delta=%dms failed_steps_delta=%d\n", status, result.ElapsedDeltaMS, result.FailedStepsDelta)
		for _, regression := range result.Regressions {
			fmt.Println("- " + regression)
		}
	}
	if *failOnRegression && result.Regressed {
		return fmt.Errorf("self-verification summary regression detected")
	}
	return nil
}

func runSelfVerifyHistory(args []string) error {
	fs := flag.NewFlagSet("self-verify history", flag.ContinueOnError)
	prefix := fs.String("prefix", "self-verify", "state key prefix to scan; empty string scans all keys")
	limit := fs.Int("limit", 20, "maximum entries to return; 0 returns all")
	retentionLimit := fs.Int("retention-limit", 0, "maximum matching summaries to retain by newest-first ordering; 0 disables retention planning")
	pruneRetention := fs.Bool("prune-retention", false, "delete retention candidates; dry-run unless --confirm is also set")
	confirm := fs.Bool("confirm", false, "confirm deletion when used with --prune-retention")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := selfAugmentHistory(*prefix, *limit, selfAugmentHistoryRetentionOptions{
		Limit:          *retentionLimit,
		PruneRequested: *pruneRetention,
		Confirm:        *confirm,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("self-verify history: %d/%d entries from %s (prefix=%q)\n", result.Returned, result.TotalMatches, result.StateDir, result.Prefix)
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
	if result.Retention != nil {
		retention := result.Retention
		action := "planned"
		if retention.PruneRequested && !retention.Confirm {
			action = "would delete"
		}
		if retention.PruneRequested && retention.Confirm {
			action = "deleted"
		}
		fmt.Printf("retention: retain=%d candidates=%d %s=%d\n", retention.Limit, len(retention.CandidateKeys), action, len(retention.DeletedKeys))
	}
	return nil
}

func runSelfVerifyPromote(args []string) error {
	fs := flag.NewFlagSet("self-verify promote", flag.ContinueOnError)
	fromKey := fs.String("from-key", "", "state key containing the candidate self-verification summary snapshot")
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
	fmt.Printf("%s self-verification summary %q to baseline %q\n", action, result.FromKey, result.BaselineKey)
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

const (
	selfVerificationSummaryKind     = "self_verification_summary"
	legacySelfAugmentSummaryKind    = "self_augment_summary"
	selfVerificationKoreanName      = "자기 검증 루프"
	selfAugmentationKoreanName      = "자가 증강 루프"
	defaultLoopTargetScoreExclusive = 95.0
)

type SelfAugmentResult struct {
	OK                  bool                        `json:"ok"`
	LoopKind            string                      `json:"loop_kind"`
	KoreanName          string                      `json:"korean_name"`
	Iterations          int                         `json:"iterations"`
	BaseSeed            int64                       `json:"base_seed"`
	TargetScore         float64                     `json:"target_score"`
	TerminationEligible bool                        `json:"termination_eligible"`
	ElapsedMS           int64                       `json:"elapsed_ms"`
	HarnessRoot         string                      `json:"harness_root"`
	InspiredBy          string                      `json:"inspired_by"`
	LoopContract        []string                    `json:"loop_contract"`
	Summary             SelfAugmentSummary          `json:"summary"`
	StateCheckpoint     *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	Runs                []SelfAugmentIteration      `json:"runs"`
}

type SelfAugmentSummary struct {
	TotalRuns           int                              `json:"total_runs"`
	TotalSteps          int                              `json:"total_steps"`
	PassedSteps         int                              `json:"passed_steps"`
	FailedSteps         int                              `json:"failed_steps"`
	TargetScore         float64                          `json:"target_score"`
	Contract            SelfVerificationContract         `json:"contract"`
	MinimumGoalScore    float64                          `json:"minimum_goal_score"`
	TerminationEligible bool                             `json:"termination_eligible"`
	GoalScores          []SelfVerificationGoalScore      `json:"goal_scores"`
	Coverage            []SelfVerificationCoverage       `json:"coverage"`
	CoverageGaps        []string                         `json:"coverage_gaps"`
	RerunCommands       []string                         `json:"rerun_commands,omitempty"`
	FailureClass        string                           `json:"failure_class,omitempty"`
	FailureClassReason  string                           `json:"failure_class_reason,omitempty"`
	FailureClusters     []SelfVerificationFailureCluster `json:"failure_clusters,omitempty"`
	FailedIteration     int                              `json:"failed_iteration,omitempty"`
	FailedSeed          int64                            `json:"failed_seed,omitempty"`
	FailedStep          string                           `json:"failed_step,omitempty"`
	StepLabels          []string                         `json:"step_labels"`
	SlowestSteps        []SelfAugmentSlowStep            `json:"slowest_steps"`
}

type SelfVerificationContract struct {
	Name           string   `json:"name"`
	Version        int      `json:"version"`
	Hash           string   `json:"hash"`
	RequiredFields []string `json:"required_fields"`
	GoalNames      []string `json:"goal_names"`
	CoverageClaims []string `json:"coverage_claims"`
}

type SelfVerificationGoalScore struct {
	Name           string   `json:"name"`
	KoreanName     string   `json:"korean_name"`
	Score          float64  `json:"score"`
	TargetScore    float64  `json:"target_score"`
	Passed         bool     `json:"passed"`
	EvidenceLabels []string `json:"evidence_labels"`
	PassedChecks   int      `json:"passed_checks"`
	TotalChecks    int      `json:"total_checks"`
}

type SelfVerificationCoverage struct {
	Claim          string   `json:"claim"`
	EvidenceLabels []string `json:"evidence_labels"`
	Covered        bool     `json:"covered"`
	MissingLabels  []string `json:"missing_labels"`
}

type SelfVerificationFailureCluster struct {
	Step  string  `json:"step"`
	Seeds []int64 `json:"seeds"`
	Count int     `json:"count"`
}

type SelfAugmentSlowStep struct {
	Iteration  int    `json:"iteration"`
	Seed       int64  `json:"seed"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
}

type SelfVerifyProgressEvent struct {
	Event       string `json:"event"`
	LoopKind    string `json:"loop_kind,omitempty"`
	Iteration   int    `json:"iteration,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`
	Seed        int64  `json:"seed,omitempty"`
	StepIndex   int    `json:"step_index,omitempty"`
	StepCount   int    `json:"step_count,omitempty"`
	Step        string `json:"step,omitempty"`
	OK          *bool  `json:"ok,omitempty"`
	ElapsedMS   int64  `json:"elapsed_ms"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
	LastSuccess string `json:"last_success,omitempty"`
	Error       string `json:"error,omitempty"`
}

type selfVerifyProgressReporter struct {
	mode        string
	writer      io.Writer
	started     time.Time
	lastSuccess string
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
	LoopKind      string             `json:"loop_kind,omitempty"`
	KoreanName    string             `json:"korean_name,omitempty"`
	OK            bool               `json:"ok"`
	Iterations    int                `json:"iterations"`
	BaseSeed      int64              `json:"base_seed"`
	TargetScore   float64            `json:"target_score,omitempty"`
	ElapsedMS     int64              `json:"elapsed_ms"`
	HarnessRoot   string             `json:"harness_root"`
	GeneratedAt   string             `json:"generated_at"`
	Summary       SelfAugmentSummary `json:"summary"`
}

type SelfAugmentCompareResult struct {
	OK                           bool                            `json:"ok"`
	StateDir                     string                          `json:"state_dir"`
	BaselineKey                  string                          `json:"baseline_key"`
	CandidateKey                 string                          `json:"candidate_key"`
	MaxElapsedRegressionPct      float64                         `json:"max_elapsed_regression_pct"`
	Regressed                    bool                            `json:"regressed"`
	ElapsedDeltaMS               int64                           `json:"elapsed_delta_ms"`
	ElapsedDeltaPct              float64                         `json:"elapsed_delta_pct"`
	FailedStepsDelta             int                             `json:"failed_steps_delta"`
	TotalStepsDelta              int                             `json:"total_steps_delta"`
	BaselineMinimumGoalScore     float64                         `json:"baseline_minimum_goal_score"`
	CandidateMinimumGoalScore    float64                         `json:"candidate_minimum_goal_score"`
	MissingStepLabels            []string                        `json:"missing_step_labels"`
	AddedStepLabels              []string                        `json:"added_step_labels"`
	Regressions                  []string                        `json:"regressions"`
	Warnings                     []string                        `json:"warnings"`
	BaselineSummary              SelfAugmentSummary              `json:"baseline_summary"`
	CandidateSummary             SelfAugmentSummary              `json:"candidate_summary"`
	BaselineSnapshotGeneratedAt  string                          `json:"baseline_snapshot_generated_at"`
	CandidateSnapshotGeneratedAt string                          `json:"candidate_snapshot_generated_at"`
	BaselineSlowestSteps         []SelfAugmentSlowStep           `json:"baseline_slowest_steps"`
	CandidateSlowestSteps        []SelfAugmentSlowStep           `json:"candidate_slowest_steps"`
	SlowStepRegressions          []SelfAugmentSlowStepRegression `json:"slow_step_regressions"`
}

type SelfAugmentSlowStepRegression struct {
	Label               string  `json:"label"`
	BaselineDurationMS  int64   `json:"baseline_duration_ms"`
	CandidateDurationMS int64   `json:"candidate_duration_ms"`
	DeltaMS             int64   `json:"delta_ms"`
	DeltaPct            float64 `json:"delta_pct"`
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
	OK           bool                         `json:"ok"`
	StateDir     string                       `json:"state_dir"`
	Prefix       string                       `json:"prefix"`
	Limit        int                          `json:"limit"`
	TotalMatches int                          `json:"total_matches"`
	Returned     int                          `json:"returned"`
	Retention    *SelfAugmentHistoryRetention `json:"retention,omitempty"`
	Entries      []SelfAugmentHistoryEntry    `json:"entries"`
	Skipped      []SelfAugmentHistorySkipped  `json:"skipped"`
	Warnings     []string                     `json:"warnings"`
}

type SelfAugmentHistoryRetention struct {
	Enabled        bool     `json:"enabled"`
	Limit          int      `json:"limit"`
	TotalMatches   int      `json:"total_matches"`
	RetainedKeys   []string `json:"retained_keys"`
	CandidateKeys  []string `json:"candidate_keys"`
	DeletedKeys    []string `json:"deleted_keys"`
	PruneRequested bool     `json:"prune_requested"`
	Confirm        bool     `json:"confirm"`
	DryRun         bool     `json:"dry_run"`
	Recommendation string   `json:"recommendation"`
}

type selfAugmentHistoryRetentionOptions struct {
	Limit          int
	PruneRequested bool
	Confirm        bool
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
	Label           string `json:"label"`
	Command         string `json:"command,omitempty"`
	OK              bool   `json:"ok"`
	DurationMS      int64  `json:"duration_ms"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutBytes     int    `json:"stdout_bytes,omitempty"`
	StderrBytes     int    `json:"stderr_bytes,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
	Error           string `json:"error,omitempty"`
}

func selfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool) (SelfAugmentResult, error) {
	return selfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil)
}

func selfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *selfVerifyProgressReporter) (SelfAugmentResult, error) {
	started := time.Now()
	result := SelfAugmentResult{
		LoopKind:    "self_verification",
		KoreanName:  selfVerificationKoreanName,
		Iterations:  iterations,
		BaseSeed:    baseSeed,
		TargetScore: targetScore,
		HarnessRoot: harnessRoot(),
		InspiredBy:  "/Users/habin/workspace/eye-tracking-scroll/scripts/self-augment.js",
		LoopContract: []string{
			"minimum 10 iterations",
			"tests and QA are first-class stages, not optional follow-ups",
			"seeded per-iteration randomized git preflight fuzz",
			"repeat core invariant, tests, risk-tier QA, build, CLI/MCP schema and response contract golden, CLI, docs, command policy, MCP, state, and native integration smoke checks",
			"terminate only when every concrete goal score is greater than target_score",
			"fail fast on the first failed step and report goal scores for recovery",
		},
	}
	if progress != nil {
		progress.started = started
		progress.emit(SelfVerifyProgressEvent{
			Event:      "loop_start",
			LoopKind:   result.LoopKind,
			Iterations: iterations,
			Seed:       baseSeed,
		})
	}
	if iterations < 10 {
		err := fmt.Errorf("self-verification requires at least 10 iterations; use --iterations=10 or higher")
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = summarizeSelfVerification(result, targetScore)
		if progress != nil {
			progress.emit(SelfVerifyProgressEvent{
				Event:      "loop_end",
				LoopKind:   result.LoopKind,
				Iterations: iterations,
				Seed:       baseSeed,
				OK:         boolPtr(false),
				Error:      err.Error(),
			})
		}
		return result, err
	}

	for iteration := 1; iteration <= iterations; iteration++ {
		seed := baseSeed + int64(iteration) - 1
		if verbose {
			fmt.Printf("\n=== Self-verification iteration %d/%d seed=%d ===\n", iteration, iterations, seed)
		}
		run := SelfAugmentIteration{Iteration: iteration, Seed: seed}
		tempDir, err := os.MkdirTemp("", "agent-harness-self-verify-*")
		if err != nil {
			step := failedStep("create temp workspace", err)
			run.Steps = append(run.Steps, step)
			result.Runs = append(result.Runs, run)
			result.ElapsedMS = time.Since(started).Milliseconds()
			result.Summary = summarizeSelfVerification(result, targetScore)
			if progress != nil {
				progress.emit(SelfVerifyProgressEvent{
					Event:      "loop_end",
					LoopKind:   result.LoopKind,
					Iterations: iterations,
					Seed:       baseSeed,
					OK:         boolPtr(false),
					Error:      err.Error(),
				})
			}
			return result, err
		}
		tempBin := filepath.Join(tempDir, "harness")

		steps := []selfVerifyPlannedStep{
			{Label: "harness invariants", Run: func() StepResult { return validateHarnessInvariants(result.HarnessRoot) }},
			{Label: "go test", Run: func() StepResult {
				return runCommandStep(result.HarnessRoot, "go test", 120*time.Second, "", "go", "test", "./...", "-count=1")
			}},
			{Label: "contract golden tests", Run: func() StepResult {
				return runCommandStep(result.HarnessRoot, "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/harness", "-run", "Golden", "-count=1")
			}},
			{Label: "risk QA tier", Run: func() StepResult { return validateRiskQATier(result.HarnessRoot) }},
			{Label: "go build", Run: func() StepResult {
				return runCommandStep(result.HarnessRoot, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/harness")
			}},
			{Label: "inspect smoke", Run: func() StepResult { return validateInspect(tempBin, result.HarnessRoot) }},
			{Label: "docs index smoke", Run: func() StepResult { return validateDocsIndex(tempBin, result.HarnessRoot) }},
			{Label: "command policy smoke", Run: func() StepResult { return validateCommandPolicy(tempBin, result.HarnessRoot) }},
			{Label: "MCP smoke", Run: func() StepResult { return validateMCP(tempBin, result.HarnessRoot) }},
			{Label: "state roundtrip", Run: func() StepResult { return validateStateRoundtrip(tempBin, result.HarnessRoot, seed) }},
			{Label: "parallel isolation", Run: func() StepResult { return validateParallelTempIsolation(tempBin, result.HarnessRoot, seed) }},
			{Label: "daemon resilience", Run: func() StepResult { return validateDaemonRestartResilience(tempBin, result.HarnessRoot, seed) }},
			{Label: "preflight fuzz", Run: func() StepResult { return validatePreflightFuzz(tempBin, result.HarnessRoot, seed) }},
			{Label: "native integration", Run: func() StepResult { return validateNativeIntegration(result.HarnessRoot) }},
			{Label: "redaction audit", Run: func() StepResult { return validateRedactionAudit(result.HarnessRoot) }},
			{Label: "QA gate", Run: func() StepResult { return validateQAGate(result.HarnessRoot) }},
		}

		if progress != nil {
			progress.emit(SelfVerifyProgressEvent{
				Event:      "iteration_start",
				LoopKind:   result.LoopKind,
				Iteration:  iteration,
				Iterations: iterations,
				Seed:       seed,
				StepCount:  len(steps),
			})
		}
		for index, plannedStep := range steps {
			if progress != nil {
				progress.emit(SelfVerifyProgressEvent{
					Event:      "step_start",
					LoopKind:   result.LoopKind,
					Iteration:  iteration,
					Iterations: iterations,
					Seed:       seed,
					StepIndex:  index + 1,
					StepCount:  len(steps),
					Step:       plannedStep.Label,
				})
			}
			step := plannedStep.Run()
			run.Steps = append(run.Steps, step)
			if progress != nil {
				progress.emitStepEnd(result.LoopKind, iteration, iterations, seed, index+1, len(steps), step)
			}
			if verbose {
				printStep(step)
			}
			if !step.OK {
				_ = os.RemoveAll(tempDir)
				result.Runs = append(result.Runs, run)
				result.ElapsedMS = time.Since(started).Milliseconds()
				result.OK = false
				result.Summary = summarizeSelfVerification(result, targetScore)
				if progress != nil {
					progress.emit(SelfVerifyProgressEvent{
						Event:      "loop_end",
						LoopKind:   result.LoopKind,
						Iterations: iterations,
						Seed:       baseSeed,
						OK:         boolPtr(false),
						Error:      fmt.Sprintf("%s failed: %s", step.Label, step.Error),
					})
				}
				return result, fmt.Errorf("%s failed: %s", step.Label, step.Error)
			}
		}
		_ = os.RemoveAll(tempDir)
		result.Runs = append(result.Runs, run)
		if progress != nil {
			progress.emit(SelfVerifyProgressEvent{
				Event:      "iteration_end",
				LoopKind:   result.LoopKind,
				Iteration:  iteration,
				Iterations: iterations,
				Seed:       seed,
				OK:         boolPtr(true),
				StepCount:  len(steps),
			})
		}
	}

	result.OK = true
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.Summary = summarizeSelfVerification(result, targetScore)
	result.TerminationEligible = result.Summary.TerminationEligible
	result.OK = result.TerminationEligible
	if verbose {
		fmt.Printf("\nSelf-verification pipeline passed %d iterations in %.1fs.\n", iterations, float64(result.ElapsedMS)/1000)
	}
	if progress != nil {
		progress.emit(SelfVerifyProgressEvent{
			Event:      "loop_end",
			LoopKind:   result.LoopKind,
			Iterations: iterations,
			Seed:       baseSeed,
			OK:         boolPtr(result.OK),
		})
	}
	return result, nil
}

type selfVerifyPlannedStep struct {
	Label string
	Run   func() StepResult
}

func newSelfVerifyProgressReporter(mode string, writer io.Writer) (*selfVerifyProgressReporter, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" || mode == "none" {
		return nil, nil
	}
	if mode != "jsonl" {
		return nil, fmt.Errorf("unsupported self-verify progress mode %q; use none or jsonl", mode)
	}
	if writer == nil {
		writer = io.Discard
	}
	return &selfVerifyProgressReporter{mode: mode, writer: writer, started: time.Now()}, nil
}

func (r *selfVerifyProgressReporter) emit(event SelfVerifyProgressEvent) {
	if r == nil || r.mode == "" {
		return
	}
	if event.ElapsedMS == 0 {
		event.ElapsedMS = time.Since(r.started).Milliseconds()
	}
	if event.LastSuccess == "" {
		event.LastSuccess = r.lastSuccess
	}
	b, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(r.writer, `{"event":"progress_error","error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(r.writer, string(b))
}

func (r *selfVerifyProgressReporter) emitStepEnd(loopKind string, iteration, iterations int, seed int64, stepIndex, stepCount int, step StepResult) {
	if r == nil {
		return
	}
	lastSuccess := r.lastSuccess
	if step.OK {
		lastSuccess = step.Label
	}
	event := SelfVerifyProgressEvent{
		Event:       "step_end",
		LoopKind:    loopKind,
		Iteration:   iteration,
		Iterations:  iterations,
		Seed:        seed,
		StepIndex:   stepIndex,
		StepCount:   stepCount,
		Step:        step.Label,
		OK:          boolPtr(step.OK),
		DurationMS:  step.DurationMS,
		LastSuccess: lastSuccess,
		Error:       step.Error,
	}
	r.emit(event)
	if step.OK {
		r.lastSuccess = step.Label
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return summarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	summary := SelfAugmentSummary{
		TotalRuns:    len(result.Runs),
		TargetScore:  targetScore,
		Contract:     selfVerificationContract(),
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
	summary.GoalScores = scoreSelfVerificationGoals(result, targetScore)
	summary.Coverage, summary.CoverageGaps = selfVerificationCoverage(summary.StepLabels)
	if summary.FailedStep != "" {
		summary.RerunCommands = selfVerifyRerunCommands(summary.FailedStep, result.Iterations, result.BaseSeed, targetScore)
		summary.FailureClass, summary.FailureClassReason, summary.FailureClusters = classifySelfVerificationFailure(result, summary)
	}
	summary.MinimumGoalScore = 100
	if len(summary.GoalScores) == 0 {
		summary.MinimumGoalScore = 0
	}
	summary.TerminationEligible = result.OK
	for _, goal := range summary.GoalScores {
		if goal.Score < summary.MinimumGoalScore {
			summary.MinimumGoalScore = goal.Score
		}
		if !goal.Passed {
			summary.TerminationEligible = false
		}
	}
	return summary
}

type selfVerificationGoalDefinition struct {
	Name       string
	KoreanName string
	Labels     []string
}

type selfVerificationCoverageDefinition struct {
	Claim  string
	Labels []string
}

func selfVerificationContract() SelfVerificationContract {
	contract := SelfVerificationContract{
		Name:    "self_verification_summary",
		Version: 2,
		RequiredFields: []string{
			"total_runs",
			"total_steps",
			"passed_steps",
			"failed_steps",
			"target_score",
			"contract",
			"minimum_goal_score",
			"termination_eligible",
			"goal_scores",
			"coverage",
			"coverage_gaps",
			"step_labels",
			"slowest_steps",
		},
		GoalNames:      []string{},
		CoverageClaims: []string{},
	}
	for _, goal := range selfVerificationGoalDefinitions() {
		contract.GoalNames = append(contract.GoalNames, goal.Name)
	}
	for _, coverage := range selfVerificationCoverageDefinitions() {
		contract.CoverageClaims = append(contract.CoverageClaims, coverage.Claim)
	}
	b, _ := json.Marshal(contract)
	sum := sha256.Sum256(b)
	contract.Hash = hex.EncodeToString(sum[:])
	return contract
}

func selfVerificationGoalDefinitions() []selfVerificationGoalDefinition {
	return []selfVerificationGoalDefinition{
		{
			Name:       "test_suite",
			KoreanName: "테스트 스위트",
			Labels:     []string{"go test", "contract golden tests"},
		},
		{
			Name:       "risk_qa",
			KoreanName: "위험도 기반 QA",
			Labels:     []string{"risk QA tier"},
		},
		{
			Name:       "build_release",
			KoreanName: "빌드 산출물",
			Labels:     []string{"go build"},
		},
		{
			Name:       "qa_smoke",
			KoreanName: "QA 스모크",
			Labels:     []string{"harness invariants", "inspect smoke", "docs index smoke", "QA gate"},
		},
		{
			Name:       "policy_security",
			KoreanName: "정책·보안",
			Labels:     []string{"command policy smoke", "preflight fuzz", "redaction audit"},
		},
		{
			Name:       "mcp_state_regression",
			KoreanName: "MCP·상태 회귀",
			Labels:     []string{"MCP smoke", "state roundtrip"},
		},
		{
			Name:       "concurrency_isolation",
			KoreanName: "동시성 격리",
			Labels:     []string{"parallel isolation"},
		},
		{
			Name:       "daemon_resilience",
			KoreanName: "데몬 복구력",
			Labels:     []string{"daemon resilience"},
		},
		{
			Name:       "native_integration",
			KoreanName: "네이티브 통합",
			Labels:     []string{"native integration"},
		},
	}
}

func selfVerificationCoverageDefinitions() []selfVerificationCoverageDefinition {
	return []selfVerificationCoverageDefinition{
		{Claim: "core repository invariants", Labels: []string{"harness invariants"}},
		{Claim: "test suite contract", Labels: []string{"go test", "contract golden tests"}},
		{Claim: "risk-tier static and race QA", Labels: []string{"risk QA tier"}},
		{Claim: "release build artifact", Labels: []string{"go build"}},
		{Claim: "CLI inspect/docs smoke", Labels: []string{"inspect smoke", "docs index smoke"}},
		{Claim: "command policy boundary", Labels: []string{"command policy smoke"}},
		{Claim: "MCP and state regression", Labels: []string{"MCP smoke", "state roundtrip"}},
		{Claim: "parallel temp isolation", Labels: []string{"parallel isolation"}},
		{Claim: "daemon restart resilience", Labels: []string{"daemon resilience"}},
		{Claim: "git preflight fuzz", Labels: []string{"preflight fuzz"}},
		{Claim: "native integration", Labels: []string{"native integration"}},
		{Claim: "secret redaction audit", Labels: []string{"redaction audit"}},
		{Claim: "documentation QA gate", Labels: []string{"QA gate"}},
	}
}

func selfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	labelSet := map[string]bool{}
	for _, label := range stepLabels {
		labelSet[label] = true
	}
	coverage := []SelfVerificationCoverage{}
	gaps := []string{}
	for _, definition := range selfVerificationCoverageDefinitions() {
		item := SelfVerificationCoverage{
			Claim:          definition.Claim,
			EvidenceLabels: append([]string{}, definition.Labels...),
			Covered:        true,
			MissingLabels:  []string{},
		}
		for _, label := range definition.Labels {
			if !labelSet[label] {
				item.Covered = false
				item.MissingLabels = append(item.MissingLabels, label)
				gaps = append(gaps, definition.Claim+": missing "+label)
			}
		}
		coverage = append(coverage, item)
	}
	return coverage, gaps
}

func selfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	commands := []string{}
	if command, ok := selfVerifyStepRerunCommand(failedStep); ok {
		commands = append(commands, command)
	}
	if iterations < 10 {
		iterations = 10
	}
	commands = append(commands, fmt.Sprintf("./bin/harness self-verify --iterations=%d --seed=%d --target-score=%s --progress=jsonl --json", iterations, baseSeed, formatScore(targetScore)))
	return commands
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	switch label {
	case "go test":
		return "go test ./... -count=1", true
	case "contract golden tests":
		return "go test ./cmd/harness -run Golden -count=1", true
	case "risk QA tier":
		return "go vet ./... && go test -race ./... -count=1", true
	case "go build":
		return "go build -o bin/harness ./cmd/harness", true
	case "inspect smoke":
		return "./bin/harness inspect --json", true
	case "docs index smoke":
		return "./bin/harness docs --json", true
	case "command policy smoke":
		return "./bin/harness policy check --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short", true
	case "MCP smoke":
		return "./bin/harness mcp", true
	case "state roundtrip":
		return "tmp_state=\"$(mktemp -d)\" && HARNESS_STATE_DIR=\"$tmp_state\" ./bin/harness state migrate --json; rm -rf \"$tmp_state\"", true
	case "parallel isolation":
		return "./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json", true
	case "daemon resilience":
		return "tmp_daemon=\"$(mktemp -d)\" && HARNESS_DAEMON_DIR=\"$tmp_daemon\" ./bin/harness daemon start --json && HARNESS_DAEMON_DIR=\"$tmp_daemon\" ./bin/harness daemon stop --json; rm -rf \"$tmp_daemon\"", true
	case "preflight fuzz":
		return "./bin/harness preflight --json \"$PWD\"", true
	case "native integration":
		return "./scripts/install-native.sh && ./bin/harness install-native --dry-run --json", true
	case "redaction audit", "QA gate", "harness invariants":
		return "go test ./cmd/harness -run Test -count=1", true
	default:
		return "", false
	}
}

func formatScore(score float64) string {
	if score == float64(int64(score)) {
		return strconv.FormatInt(int64(score), 10)
	}
	return strconv.FormatFloat(score, 'f', -1, 64)
}

func classifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	clusters := selfVerificationFailureClusters(result)
	if summary.FailedSteps == 0 {
		return "", "", nil
	}
	if len(clusters) == 0 {
		return "unknown", "summary reports failed steps but no failed step details were captured", nil
	}
	if summary.FailedSteps < summary.TotalRuns {
		return "intermittent", "only some completed seeds failed", clusters
	}
	if len(clusters) == 1 && clusters[0].Count == 1 {
		return "single_failure_observation", "self-verify is fail-fast; rerun the same seed before calling the failure flaky or deterministic", clusters
	}
	if len(clusters) == 1 {
		return "deterministic", "all completed failing seeds failed at the same step", clusters
	}
	return "mixed", "multiple failure steps were observed across completed seeds", clusters
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	byStep := map[string][]int64{}
	for _, run := range result.Runs {
		for _, step := range run.Steps {
			if step.OK {
				continue
			}
			byStep[step.Label] = append(byStep[step.Label], run.Seed)
		}
	}
	steps := make([]string, 0, len(byStep))
	for step := range byStep {
		steps = append(steps, step)
	}
	sort.Strings(steps)
	clusters := []SelfVerificationFailureCluster{}
	for _, step := range steps {
		seeds := append([]int64{}, byStep[step]...)
		sort.Slice(seeds, func(i, j int) bool { return seeds[i] < seeds[j] })
		clusters = append(clusters, SelfVerificationFailureCluster{
			Step:  step,
			Seeds: seeds,
			Count: len(seeds),
		})
	}
	return clusters
}

type RiskQATierPlan struct {
	Tier         string   `json:"tier"`
	ChangedPaths []string `json:"changed_paths"`
	Reasons      []string `json:"reasons"`
	Commands     []string `json:"commands"`
}

func validateRiskQATier(root string) StepResult {
	started := time.Now()
	plan := planRiskQATier(root)
	planJSON := riskQATierPlanJSON(plan)
	stdoutParts := []string{planJSON}
	commands := []string{}
	if len(plan.Commands) == 0 {
		return StepResult{
			Label:      "risk QA tier",
			OK:         true,
			DurationMS: time.Since(started).Milliseconds(),
			Stdout:     planJSON,
		}
	}
	for _, command := range plan.Commands {
		var step StepResult
		switch command {
		case "go test -race ./... -count=1":
			step = runCommandStep(root, "risk QA race test", 180*time.Second, "", "go", "test", "-race", "./...", "-count=1")
		case "go vet ./...":
			step = runCommandStep(root, "risk QA static vet", 120*time.Second, "", "go", "vet", "./...")
		default:
			step = failedStep("risk QA tier", fmt.Errorf("unknown risk QA command %q", command))
		}
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		if !step.OK {
			return combineFailedStep("risk QA tier", started, step, stdoutParts, commands)
		}
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "risk QA tier",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func planRiskQATier(root string) RiskQATierPlan {
	paths, warnings := gitChangedPaths(root)
	plan := planRiskQATierFromPaths(paths)
	plan.Reasons = append(plan.Reasons, warnings...)
	sort.Strings(plan.Reasons)
	return plan
}

func planRiskQATierFromPaths(paths []string) RiskQATierPlan {
	plan := RiskQATierPlan{Tier: "standard", ChangedPaths: uniqueSortedStrings(paths), Reasons: []string{}, Commands: []string{}}
	if len(plan.ChangedPaths) == 0 {
		plan.Reasons = append(plan.Reasons, "working tree has no local changes")
		return plan
	}
	goChanged := false
	sensitive := false
	for _, path := range plan.ChangedPaths {
		if strings.HasSuffix(path, ".go") {
			goChanged = true
		}
		if isRiskSensitivePath(path) {
			sensitive = true
		}
	}
	if goChanged {
		plan.Tier = "static"
		plan.Reasons = append(plan.Reasons, "go changes detected")
		plan.Commands = append(plan.Commands, "go vet ./...")
	}
	if goChanged && sensitive {
		plan.Tier = "elevated"
		plan.Reasons = append(plan.Reasons, "go changes touch policy, MCP, adapter, daemon, state, or harness orchestration surfaces")
		plan.Commands = append([]string{"go test -race ./... -count=1"}, plan.Commands...)
	}
	if !goChanged {
		plan.Reasons = append(plan.Reasons, "no Go changes detected; race/static tier skipped")
	}
	sort.Strings(plan.Reasons)
	return plan
}

func gitChangedPaths(root string) ([]string, []string) {
	cmd := exec.Command("git", "-C", root, "status", "--short", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, []string{"git status unavailable: " + err.Error()}
	}
	paths := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		path := parseGitStatusPath(line)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return uniqueSortedStrings(paths), nil
}

func parseGitStatusPath(line string) string {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if len(line) > 3 {
		line = line[3:]
	} else {
		line = strings.TrimSpace(line)
	}
	if strings.Contains(line, " -> ") {
		parts := strings.Split(line, " -> ")
		line = parts[len(parts)-1]
	}
	line = strings.Trim(line, ` "`)
	return filepath.ToSlash(line)
}

func isRiskSensitivePath(path string) bool {
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "cmd/harness/") || strings.HasPrefix(path, "internal/") {
		return true
	}
	for _, token := range []string{"daemon", "worker", "policy", "state", "mcp", "adapter", "install", "hook", "self_augment", "self-augment"} {
		if strings.Contains(path, token) {
			return true
		}
	}
	return false
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(filepath.ToSlash(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func riskQATierPlanJSON(plan RiskQATierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	goals := selfVerificationGoalDefinitions()
	scores := make([]SelfVerificationGoalScore, 0, len(goals))
	runCount := result.Iterations
	if runCount < 1 {
		runCount = len(result.Runs)
	}
	for _, goal := range goals {
		passed := 0
		total := 0
		for iteration := 1; iteration <= runCount; iteration++ {
			steps := map[string]StepResult{}
			for _, run := range result.Runs {
				if run.Iteration != iteration {
					continue
				}
				for _, step := range run.Steps {
					steps[step.Label] = step
				}
				break
			}
			for _, label := range goal.Labels {
				total++
				if step, ok := steps[label]; ok && step.OK {
					passed++
				}
			}
		}
		score := 0.0
		if total > 0 {
			score = float64(passed) * 100 / float64(total)
		}
		scores = append(scores, SelfVerificationGoalScore{
			Name:           goal.Name,
			KoreanName:     goal.KoreanName,
			Score:          score,
			TargetScore:    targetScore,
			Passed:         score > targetScore,
			EvidenceLabels: append([]string{}, goal.Labels...),
			PassedChecks:   passed,
			TotalChecks:    total,
		})
	}
	return scores
}

func saveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	if key == "" {
		key = "self-verify-latest"
	}
	snapshot := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		LoopKind:      result.LoopKind,
		KoreanName:    result.KoreanName,
		OK:            result.OK,
		Iterations:    result.Iterations,
		BaseSeed:      result.BaseSeed,
		TargetScore:   result.TargetScore,
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

func saveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return saveSelfVerificationSummary(result, key)
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
		SlowStepRegressions:     []SelfAugmentSlowStepRegression{},
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
	result.BaselineMinimumGoalScore = baseline.Summary.MinimumGoalScore
	result.CandidateMinimumGoalScore = candidate.Summary.MinimumGoalScore
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
	if baseline.Summary.TerminationEligible && !candidate.Summary.TerminationEligible {
		result.Regressions = append(result.Regressions, "candidate_not_termination_eligible")
	}
	if baseline.Summary.MinimumGoalScore > 0 && candidate.Summary.MinimumGoalScore < baseline.Summary.MinimumGoalScore {
		result.Regressions = append(result.Regressions, fmt.Sprintf("minimum_goal_score_decreased_by_%.2f", baseline.Summary.MinimumGoalScore-candidate.Summary.MinimumGoalScore))
	}
	if result.FailedStepsDelta > 0 {
		result.Regressions = append(result.Regressions, fmt.Sprintf("failed_steps_increased_by_%d", result.FailedStepsDelta))
	}
	if result.ElapsedDeltaPct > maxElapsedRegressionPct {
		result.Regressions = append(result.Regressions, fmt.Sprintf("elapsed_ms_increased_by_%.2f_pct", result.ElapsedDeltaPct))
	}
	result.SlowStepRegressions = compareSlowestStepRegressions(baseline.Summary.SlowestSteps, candidate.Summary.SlowestSteps, maxElapsedRegressionPct)
	for _, regression := range result.SlowStepRegressions {
		result.Regressions = append(result.Regressions, fmt.Sprintf("slow_step:%s_increased_by_%.2f_pct", regression.Label, regression.DeltaPct))
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

func compareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	baselineByLabel := maxSlowStepDurationByLabel(baseline)
	candidateByLabel := maxSlowStepDurationByLabel(candidate)
	regressions := []SelfAugmentSlowStepRegression{}
	for label, candidateDuration := range candidateByLabel {
		baselineDuration, ok := baselineByLabel[label]
		if !ok || baselineDuration <= 0 {
			continue
		}
		delta := candidateDuration - baselineDuration
		if delta <= 0 {
			continue
		}
		deltaPct := float64(delta) * 100 / float64(baselineDuration)
		if deltaPct <= maxRegressionPct {
			continue
		}
		regressions = append(regressions, SelfAugmentSlowStepRegression{
			Label:               label,
			BaselineDurationMS:  baselineDuration,
			CandidateDurationMS: candidateDuration,
			DeltaMS:             delta,
			DeltaPct:            deltaPct,
		})
	}
	sort.Slice(regressions, func(i, j int) bool {
		if regressions[i].DeltaPct != regressions[j].DeltaPct {
			return regressions[i].DeltaPct > regressions[j].DeltaPct
		}
		return regressions[i].Label < regressions[j].Label
	})
	return regressions
}

func maxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	out := map[string]int64{}
	for _, step := range steps {
		if step.Label == "" {
			continue
		}
		if step.DurationMS > out[step.Label] {
			out[step.Label] = step.DurationMS
		}
	}
	return out
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

func selfAugmentHistory(prefix string, limit int, retentionOptions ...selfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
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
	retention := selfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		retention = retentionOptions[0]
	}
	if retention.Limit < 0 {
		return result, fmt.Errorf("retention-limit must be non-negative")
	}
	if retention.Confirm && !retention.PruneRequested {
		return result, fmt.Errorf("confirm requires --prune-retention")
	}
	if retention.PruneRequested && retention.Limit <= 0 {
		return result, fmt.Errorf("prune-retention requires a positive --retention-limit")
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
		if !isSelfVerificationSummaryKind(snapshot.Kind) {
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
	if retention.Limit > 0 {
		if err := applySelfAugmentHistoryRetention(&result, retention); err != nil {
			return result, err
		}
		sort.Strings(result.Warnings)
	}
	if limit > 0 && len(result.Entries) > limit {
		result.Entries = result.Entries[:limit]
	}
	result.Returned = len(result.Entries)
	result.OK = true
	return result, nil
}

func applySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options selfAugmentHistoryRetentionOptions) error {
	retention := &SelfAugmentHistoryRetention{
		Enabled:        true,
		Limit:          options.Limit,
		TotalMatches:   result.TotalMatches,
		RetainedKeys:   []string{},
		CandidateKeys:  []string{},
		DeletedKeys:    []string{},
		PruneRequested: options.PruneRequested,
		Confirm:        options.Confirm,
		DryRun:         options.PruneRequested && !options.Confirm,
		Recommendation: "within_retention_budget",
	}
	for i, entry := range result.Entries {
		if i < options.Limit {
			retention.RetainedKeys = append(retention.RetainedKeys, entry.Key)
			continue
		}
		retention.CandidateKeys = append(retention.CandidateKeys, entry.Key)
	}
	if len(retention.CandidateKeys) > 0 {
		retention.Recommendation = fmt.Sprintf("prune %d history checkpoint(s) beyond retention-limit=%d after reviewing dry-run output", len(retention.CandidateKeys), options.Limit)
		result.Warnings = append(result.Warnings, fmt.Sprintf("history_retention_candidates:%d", len(retention.CandidateKeys)))
	}
	if options.PruneRequested && options.Confirm {
		for _, key := range retention.CandidateKeys {
			state, err := core.StateRead(key)
			if err != nil {
				return fmt.Errorf("read retention candidate %q: %w", key, err)
			}
			if err := os.Remove(state.Path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete retention candidate %q: %w", key, err)
			}
			retention.DeletedKeys = append(retention.DeletedKeys, key)
		}
		retention.Recommendation = fmt.Sprintf("deleted %d history checkpoint(s) beyond retention-limit=%d", len(retention.DeletedKeys), options.Limit)
	}
	result.Retention = retention
	return nil
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
	if !isSelfVerificationSummaryKind(snapshot.Kind) {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q contains kind %q, want %s", key, snapshot.Kind, selfVerificationSummaryKind)
	}
	if snapshot.SchemaVersion != 1 {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q has unsupported self-verification summary schema %d", key, snapshot.SchemaVersion)
	}
	return snapshot, nil
}

func isSelfVerificationSummaryKind(kind string) bool {
	return kind == selfVerificationSummaryKind || kind == legacySelfAugmentSummaryKind
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
		filepath.Join("configs", "claude", "hooks", "session-start-llm-wiki.settings.json"),
		filepath.Join("scripts", "session-start-llm-wiki.sh"),
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
		for _, needle := range forbiddenLegacyNeedles() {
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

func forbiddenLegacyNeedles() []string {
	return []string{"m" + "16kh", "m" + "16h", "M" + "16H", "m" + "16"}
}

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	sanitized := text
	replacements := []string{}
	if abs, err := filepath.Abs(root); err == nil {
		replacements = append(replacements, abs)
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements = append(replacements, home)
	}
	for _, runtimePath := range replacements {
		if runtimePath == "" || runtimePath == string(filepath.Separator) {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, runtimePath, "$RUNTIME_PATH")
	}
	for _, needle := range forbiddenLegacyNeedles() {
		if strings.Contains(sanitized, needle) {
			return true
		}
	}
	return false
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
	if !info.Integration.ClaudeUserHook && !info.Integration.ProjectClaudeHook {
		errs = append(errs, "Claude SessionStart hook missing")
	}
	if containsForbiddenLegacyOutsideRuntimePaths(step.Stdout, root) {
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
	wantDocs := []string{"AGENTS.md", "CLAUDE.md", "GENIUS_THINK.md", "agent_docs/COMMIT_POLICY.md", "agent_docs/SELF_AUGMENTATION.md", "agent_docs/USAGE.md"}
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

	deniedOutsidePath := runCommandStep(root, "policy deny outside path arg", 30*time.Second, "", binary, "policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "cat", filepath.Join(outside, "note.txt"))
	stdoutParts = append(stdoutParts, deniedOutsidePath.Stdout)
	commands = append(commands, deniedOutsidePath.Command)
	if !deniedOutsidePath.OK {
		return combineFailedStep("command policy smoke", started, deniedOutsidePath, stdoutParts, commands)
	}
	var outsidePathEval core.CommandPolicyEvaluation
	if err := json.Unmarshal([]byte(deniedOutsidePath.Stdout), &outsidePathEval); err != nil {
		return assertionStepWithOutput("command policy smoke", started, []string{err.Error()}, stdoutParts, commands)
	}
	if outsidePathEval.Allowed || !containsString(outsidePathEval.DenyReasons, "path_outside_workspace") {
		return assertionStepWithOutput("command policy smoke", started, []string{"outside path arg was not denied"}, stdoutParts, commands)
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

	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "command policy smoke",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func validateMCP(binary, root string) StepResult {
	tempState, err := os.MkdirTemp("", "agent-harness-mcp-state-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer os.RemoveAll(tempState)
	tempWiki := filepath.Join(tempState, "llm-wiki")
	if err := writeSelfAugmentWikiFixture(tempWiki); err != nil {
		return failedStep("MCP smoke", err)
	}
	daemonDir, err := os.MkdirTemp("", "ahd-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer os.RemoveAll(daemonDir)
	env := []string{
		"HARNESS_STATE_DIR=" + tempState,
		"HARNESS_DAEMON_DIR=" + daemonDir,
		"LLM_WIKI_ROOT=" + tempWiki,
	}
	defer runCommandStepEnv(root, "MCP daemon stop", 5*time.Second, "", env, binary, "daemon", "stop", "--json")

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"self-verify","version":"0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"harness://state"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"harness://docs"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"harness://command-policy"}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"state_prune","arguments":{"max_age":"1h"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"state_doctor","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"state_migrate","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"resources/read","params":{"uri":"harness://llm-wiki/session-context"}}`,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"llm_wiki_search","arguments":{"query":"llm wiki","limit":3}}}`,
	}, "\n") + "\n"
	step := runCommandStepEnvWithBudget(root, "MCP smoke", 30*time.Second, input, env, 0, binary, "mcp")
	if !step.OK {
		return step
	}
	lines := splitLines(step.Stdout)
	if len(lines) != 11 {
		step.OK = false
		step.Error = fmt.Sprintf("expected 11 MCP responses, got %d", len(lines))
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
	if !strings.Contains(step.Stdout, "atomic_commit_preflight") || !strings.Contains(step.Stdout, "docs_index") || !strings.Contains(step.Stdout, "command_policy_check") || !strings.Contains(step.Stdout, "state_write") || !strings.Contains(step.Stdout, "state_prune") || !strings.Contains(step.Stdout, "state_doctor") || !strings.Contains(step.Stdout, "state_migrate") || !strings.Contains(step.Stdout, "self_augment") || !strings.Contains(step.Stdout, "self_augment_lesson") || !strings.Contains(step.Stdout, "self_verify") || !strings.Contains(step.Stdout, "self_verify_history") || !strings.Contains(step.Stdout, "self_verify_compare") || !strings.Contains(step.Stdout, "self_verify_promote") || !strings.Contains(step.Stdout, "llm_wiki_session_context") || !strings.Contains(step.Stdout, "llm_wiki_search") || !strings.Contains(step.Stdout, "dry_run") || !strings.Contains(step.Stdout, "healthy") || !strings.Contains(step.Stdout, "to_schema") || !strings.Contains(step.Stdout, "Lore:") || !strings.Contains(step.Stdout, "LLM Wiki Session Context") {
		step.OK = false
		step.Error = "MCP smoke did not expose expected tool/resource"
	}
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(step.Stdout, selfVerifyAggregateOutputBudgetBytes)
	return step
}

func writeSelfAugmentWikiFixture(root string) error {
	files := map[string]string{
		"00-meta/AGENTS.md":                    "# LLM Wiki Operating Schema\n\nUse sources as read-only evidence.\n",
		"00-meta/index.md":                     "# Wiki Index\n\n- [[llm-wiki-pattern]]\n- [[karpathy-llm-wiki-gist]]\n",
		"00-meta/log.md":                       "# Wiki Log\n\n",
		"10-sources/karpathy-llm-wiki-gist.md": "---\ntitle: Karpathy LLM Wiki Gist\ntype: source\nstatus: active\ntags: [llm-wiki]\n---\n\nSource card for the LLM Wiki pattern.\n",
		"20-wiki/concepts/llm-wiki-pattern.md": "---\ntitle: LLM Wiki Pattern\ntype: concept\nstatus: active\ntags: [llm-wiki]\n---\n\nThe LLM Wiki pattern keeps durable, interlinked markdown knowledge.\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	started := time.Now()
	tempState, err := os.MkdirTemp("", "agent-harness-state-roundtrip-*")
	if err != nil {
		return failedStep("state roundtrip", err)
	}
	defer os.RemoveAll(tempState)

	key := fmt.Sprintf("self-verify-%d", seed)
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
		Kind:          selfVerificationSummaryKind,
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
		Kind:          selfVerificationSummaryKind,
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
	compareOK := runCommandStepEnv(root, "self verify compare ok", 30*time.Second, "", env, binary, "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--json")
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
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify compare reported unexpected non-regression result"}, stdoutParts, commands)
	}
	compareRegression := runCommandStepEnv(root, "self verify compare regression", 30*time.Second, "", env, binary, "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--max-elapsed-regression-pct", "5", "--json")
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
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify compare did not report expected elapsed regression"}, stdoutParts, commands)
	}
	promotedBaselineKey := key + "-promoted-baseline"
	promoteDry := runCommandStepEnv(root, "self verify promote dry-run", 30*time.Second, "", env, binary, "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--json")
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
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify promote dry-run mutated state or did not report dry-run"}, stdoutParts, commands)
	}
	if _, err := core.StateRead(promotedBaselineKey); err == nil {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify promote dry-run wrote baseline unexpectedly"}, stdoutParts, commands)
	}
	promoteConfirm := runCommandStepEnv(root, "self verify promote confirm", 30*time.Second, "", env, binary, "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--confirm", "--json")
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
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify promote confirm did not write baseline"}, stdoutParts, commands)
	}
	comparePromoted := runCommandStepEnv(root, "self verify compare promoted", 30*time.Second, "", env, binary, "self-verify", "compare", "--baseline-key", promotedBaselineKey, "--candidate-key", candidateCompareKey, "--json")
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
	history := runCommandStepEnv(root, "self verify history", 30*time.Second, "", env, binary, "self-verify", "history", "--prefix", key+"-", "--json")
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
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify history did not list saved baseline/candidate/promoted summaries"}, stdoutParts, commands)
	}

	retentionDry := runCommandStepEnv(root, "self verify history retention dry-run", 30*time.Second, "", env, binary, "self-verify", "history", "--prefix", key+"-", "--retention-limit", "1", "--prune-retention", "--json")
	stdoutParts = append(stdoutParts, retentionDry.Stdout)
	commands = append(commands, retentionDry.Command)
	if !retentionDry.OK {
		return combineFailedStep("state roundtrip", started, retentionDry, stdoutParts, commands)
	}
	var retentionDryResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(retentionDry.Stdout), &retentionDryResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if retentionDryResult.Retention == nil || !retentionDryResult.Retention.DryRun || retentionDryResult.Retention.Confirm || retentionDryResult.Retention.Limit != 1 || len(retentionDryResult.Retention.CandidateKeys) == 0 || len(retentionDryResult.Retention.DeletedKeys) != 0 {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify history retention dry-run did not classify prune candidates safely"}, stdoutParts, commands)
	}

	retentionConfirm := runCommandStepEnv(root, "self verify history retention confirm", 30*time.Second, "", env, binary, "self-verify", "history", "--prefix", key+"-", "--retention-limit", "1", "--prune-retention", "--confirm", "--json")
	stdoutParts = append(stdoutParts, retentionConfirm.Stdout)
	commands = append(commands, retentionConfirm.Command)
	if !retentionConfirm.OK {
		return combineFailedStep("state roundtrip", started, retentionConfirm, stdoutParts, commands)
	}
	var retentionConfirmResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(retentionConfirm.Stdout), &retentionConfirmResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if retentionConfirmResult.Retention == nil || retentionConfirmResult.Retention.DryRun || !retentionConfirmResult.Retention.Confirm || len(retentionConfirmResult.Retention.DeletedKeys) == 0 {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify history retention confirm did not delete prune candidates"}, stdoutParts, commands)
	}

	historyAfterRetention := runCommandStepEnv(root, "self verify history after retention", 30*time.Second, "", env, binary, "self-verify", "history", "--prefix", key+"-", "--json")
	stdoutParts = append(stdoutParts, historyAfterRetention.Stdout)
	commands = append(commands, historyAfterRetention.Command)
	if !historyAfterRetention.OK {
		return combineFailedStep("state roundtrip", started, historyAfterRetention, stdoutParts, commands)
	}
	var historyAfterRetentionResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(historyAfterRetention.Stdout), &historyAfterRetentionResult); err != nil {
		return assertionStepWithOutput("state roundtrip", started, []string{err.Error()}, stdoutParts, commands)
	}
	if historyAfterRetentionResult.TotalMatches > 1 {
		return assertionStepWithOutput("state roundtrip", started, []string{"self-verify history retention confirm left too many matching summaries"}, stdoutParts, commands)
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

	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "state roundtrip",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

type parallelIsolationProbe struct {
	Worker       int      `json:"worker"`
	TempRoot     string   `json:"temp_root"`
	StateDir     string   `json:"state_dir"`
	DaemonDir    string   `json:"daemon_dir"`
	WikiRoot     string   `json:"wiki_root"`
	ArtifactPath string   `json:"artifact_path"`
	Key          string   `json:"key"`
	Commands     []string `json:"commands"`
	Error        string   `json:"error,omitempty"`
}

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	started := time.Now()
	const workers = 3
	results := make(chan parallelIsolationProbe, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			results <- runParallelIsolationProbe(binary, root, seed, worker)
		}(worker)
	}
	wg.Wait()
	close(results)

	probes := []parallelIsolationProbe{}
	errs := []string{}
	seenPaths := map[string]string{}
	for probe := range results {
		probes = append(probes, probe)
		if probe.Error != "" {
			errs = append(errs, fmt.Sprintf("worker %d: %s", probe.Worker, probe.Error))
		}
		for label, path := range map[string]string{
			"temp_root":     probe.TempRoot,
			"state_dir":     probe.StateDir,
			"daemon_dir":    probe.DaemonDir,
			"wiki_root":     probe.WikiRoot,
			"artifact_path": probe.ArtifactPath,
		} {
			if strings.TrimSpace(path) == "" {
				errs = append(errs, fmt.Sprintf("worker %d has empty %s", probe.Worker, label))
				continue
			}
			if previous, ok := seenPaths[path]; ok {
				errs = append(errs, fmt.Sprintf("path collision: %s reused by %s and worker %d %s", path, previous, probe.Worker, label))
				continue
			}
			seenPaths[path] = fmt.Sprintf("worker %d %s", probe.Worker, label)
		}
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].Worker < probes[j].Worker })
	stdoutBytes, _ := json.MarshalIndent(map[string]any{
		"workers": workers,
		"probes":  probes,
	}, "", "  ")
	stdoutText, stdoutTruncated, stdoutOriginalBytes := tailWithBudget(string(stdoutBytes), selfVerifyAggregateOutputBudgetBytes)
	if len(errs) > 0 {
		return StepResult{
			Label:           "parallel isolation",
			OK:              false,
			DurationMS:      time.Since(started).Milliseconds(),
			Stdout:          stdoutText,
			StdoutBytes:     stdoutOriginalBytes,
			StdoutTruncated: stdoutTruncated,
			Error:           strings.Join(errs, "; "),
		}
	}
	commands := []string{}
	for _, probe := range probes {
		commands = append(commands, probe.Commands...)
	}
	return StepResult{
		Label:           "parallel isolation",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutOriginalBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func runParallelIsolationProbe(binary, root string, seed int64, worker int) parallelIsolationProbe {
	probe := parallelIsolationProbe{Worker: worker, Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Commands: []string{}}
	tempRoot, err := os.MkdirTemp("", fmt.Sprintf("agent-harness-parallel-%d-%d-*", seed, worker))
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	probe.TempRoot = tempRoot
	defer os.RemoveAll(tempRoot)
	probe.StateDir = filepath.Join(tempRoot, "state")
	probe.DaemonDir = filepath.Join(tempRoot, "daemon")
	probe.WikiRoot = filepath.Join(tempRoot, "llm-wiki")
	buildDir := filepath.Join(tempRoot, "build")
	probe.ArtifactPath = filepath.Join(buildDir, "harness")
	if err := os.MkdirAll(buildDir, 0o700); err != nil {
		probe.Error = err.Error()
		return probe
	}
	if err := os.WriteFile(probe.ArtifactPath, []byte("parallel isolation artifact\n"), 0o600); err != nil {
		probe.Error = err.Error()
		return probe
	}
	if err := writeSelfAugmentWikiFixture(probe.WikiRoot); err != nil {
		probe.Error = err.Error()
		return probe
	}
	env := []string{
		"HARNESS_STATE_DIR=" + probe.StateDir,
		"HARNESS_DAEMON_DIR=" + probe.DaemonDir,
		"LLM_WIKI_ROOT=" + probe.WikiRoot,
	}
	value := fmt.Sprintf("worker=%d seed=%d", worker, seed)
	write := runCommandStepEnv(root, fmt.Sprintf("parallel state write %d", worker), 30*time.Second, "", env, binary, "state", "write", "--key", probe.Key, "--value", value, "--json")
	probe.Commands = append(probe.Commands, write.Command)
	if !write.OK {
		probe.Error = "state write failed: " + write.Error
		return probe
	}
	read := runCommandStepEnv(root, fmt.Sprintf("parallel state read %d", worker), 30*time.Second, "", env, binary, "state", "read", "--key", probe.Key, "--json")
	probe.Commands = append(probe.Commands, read.Command)
	if !read.OK {
		probe.Error = "state read failed: " + read.Error
		return probe
	}
	var readResult core.StateResult
	if err := json.Unmarshal([]byte(read.Stdout), &readResult); err != nil {
		probe.Error = "state read parse failed: " + err.Error()
		return probe
	}
	if readResult.Record.Key != probe.Key || readResult.Record.Content != value {
		probe.Error = "state read returned another worker's content"
		return probe
	}
	list := runCommandStepEnv(root, fmt.Sprintf("parallel state list %d", worker), 30*time.Second, "", env, binary, "state", "list", "--json")
	probe.Commands = append(probe.Commands, list.Command)
	if !list.OK {
		probe.Error = "state list failed: " + list.Error
		return probe
	}
	var listResult core.StateListResult
	if err := json.Unmarshal([]byte(list.Stdout), &listResult); err != nil {
		probe.Error = "state list parse failed: " + err.Error()
		return probe
	}
	if len(listResult.Keys) != 1 || listResult.Keys[0] != probe.Key {
		probe.Error = fmt.Sprintf("state list leaked keys across workers: %v", listResult.Keys)
		return probe
	}
	wiki := runCommandStepEnv(root, fmt.Sprintf("parallel wiki inventory %d", worker), 30*time.Second, "", env, binary, "llm-wiki", "inventory", "--json")
	probe.Commands = append(probe.Commands, wiki.Command)
	if !wiki.OK {
		probe.Error = "llm-wiki inventory failed: " + wiki.Error
		return probe
	}
	var wikiResult core.LLMWikiInventory
	if err := json.Unmarshal([]byte(wiki.Stdout), &wikiResult); err != nil {
		probe.Error = "llm-wiki inventory parse failed: " + err.Error()
		return probe
	}
	if filepath.Clean(wikiResult.Root) != filepath.Clean(probe.WikiRoot) {
		probe.Error = "llm-wiki inventory used another root"
		return probe
	}
	daemon := runCommandStepEnv(root, fmt.Sprintf("parallel daemon status %d", worker), 30*time.Second, "", env, binary, "daemon", "status", "--json")
	probe.Commands = append(probe.Commands, daemon.Command)
	if !daemon.OK {
		probe.Error = "daemon status failed: " + daemon.Error
		return probe
	}
	var daemonResult daemonStatus
	if err := json.Unmarshal([]byte(daemon.Stdout), &daemonResult); err != nil {
		probe.Error = "daemon status parse failed: " + err.Error()
		return probe
	}
	if filepath.Clean(daemonResult.Paths.Dir) != filepath.Clean(probe.DaemonDir) {
		probe.Error = "daemon status used another daemon dir"
		return probe
	}
	return probe
}

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	started := time.Now()
	// Keep this prefix short because Unix socket paths are length-limited on
	// macOS. Long self-verify prefixes can make the daemon fail before it can
	// write a useful status file.
	tempDaemon, err := os.MkdirTemp("", fmt.Sprintf("ahd-%d-*", seed))
	if err != nil {
		return failedStep("daemon resilience", err)
	}
	defer os.RemoveAll(tempDaemon)
	paths := daemonPaths{
		Dir:    tempDaemon,
		Socket: filepath.Join(tempDaemon, "agent-harness.sock"),
		PID:    filepath.Join(tempDaemon, "agent-harness.pid"),
		Lock:   filepath.Join(tempDaemon, "agent-harness.lock"),
		Log:    filepath.Join(tempDaemon, "agent-harness.log"),
	}
	if err := os.WriteFile(paths.Lock, []byte("999999\n"), 0o600); err != nil {
		return failedStep("daemon resilience", err)
	}
	old := time.Now().Add(-2 * time.Minute)
	_ = os.Chtimes(paths.Lock, old, old)
	if err := os.WriteFile(paths.Socket, []byte("stale socket placeholder\n"), 0o600); err != nil {
		return failedStep("daemon resilience", err)
	}
	stdoutParts := []string{}
	commands := []string{}
	env := []string{"HARNESS_DAEMON_DIR=" + tempDaemon}
	runDaemonJSON := func(label string, args ...string) (daemonStatus, StepResult, error) {
		step := runCommandStepEnv(root, label, 30*time.Second, "", env, binary, args...)
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		var status daemonStatus
		if step.Stdout != "" {
			if err := json.Unmarshal([]byte(step.Stdout), &status); err != nil {
				return status, step, fmt.Errorf("parse %s JSON: %w", label, err)
			}
		}
		if !step.OK {
			return status, step, fmt.Errorf("%s failed: %s", label, step.Error)
		}
		return status, step, nil
	}
	defer runCommandStepEnv(root, "daemon resilience cleanup stop", 30*time.Second, "", env, binary, "daemon", "stop", "--json")

	errs := []string{}
	startStatus, startStep, startErr := runDaemonJSON("daemon resilience start", "daemon", "start", "--json")
	if startErr != nil {
		return combineFailedStep("daemon resilience", started, startStep, stdoutParts, commands)
	}
	if !startStatus.OK || !startStatus.Running || startStatus.PID <= 0 {
		errs = append(errs, "daemon did not start from stale lock/socket fixture")
	}
	if exists(paths.Lock) {
		errs = append(errs, "stale daemon lock remained after start")
	}
	if info, err := os.Stat(paths.Socket); err != nil {
		errs = append(errs, "daemon socket missing after start: "+err.Error())
	} else if info.Mode().Perm() != 0o600 {
		errs = append(errs, fmt.Sprintf("daemon socket mode = %o, want 600", info.Mode().Perm()))
	}

	runningStatus, statusStep, statusErr := runDaemonJSON("daemon resilience status", "daemon", "status", "--json")
	if statusErr != nil {
		return combineFailedStep("daemon resilience", started, statusStep, stdoutParts, commands)
	}
	if !runningStatus.Running || filepath.Clean(runningStatus.Paths.Dir) != filepath.Clean(tempDaemon) {
		errs = append(errs, "daemon status did not report running temp daemon")
	}

	stopStatus, stopStep, stopErr := runDaemonJSON("daemon resilience stop", "daemon", "stop", "--json")
	if stopErr != nil {
		return combineFailedStep("daemon resilience", started, stopStep, stdoutParts, commands)
	}
	if !stopStatus.OK || stopStatus.Running {
		errs = append(errs, "daemon stop did not report stopped state")
	}

	afterStatus, afterStep, afterErr := runDaemonJSON("daemon resilience after status", "daemon", "status", "--json")
	if afterErr != nil {
		return combineFailedStep("daemon resilience", started, afterStep, stdoutParts, commands)
	}
	if afterStatus.Running || exists(paths.Socket) || exists(paths.PID) {
		errs = append(errs, "daemon stop left running socket or pid file")
	}
	if len(errs) > 0 {
		return assertionStepWithOutput("daemon resilience", started, errs, stdoutParts, commands)
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "daemon resilience",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
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
	body := "Lore:\n- Intent: Validate seeded preflight fuzz.\n- Why: Self-verification needs deterministic git fixtures.\n- Changes:\n  - Add sample file.\n- Verify: harness self-verify\n- Risk: Low"
	commitArgs := []string{"-c", "user.name=Self Verify", "-c", "user.email=self-verify@example.invalid", "commit", "-q", "-m", msg, "-m", body}
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
	stdoutParts := []string{}
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "configs", "claude", "hooks", "session-start-llm-wiki.settings.json"),
		filepath.Join(root, "scripts", "session-start-llm-wiki.sh"),
	}
	nativeSkills, err := core.ListSkillNames(root)
	if err != nil {
		errs = append(errs, "list native skills: "+err.Error())
	}
	for _, nativeSkill := range nativeSkills {
		paths = append(paths,
			filepath.Join(home, ".codex", "skills", nativeSkill, "SKILL.md"),
			filepath.Join(home, ".claude", "skills", nativeSkill, "SKILL.md"),
		)
	}
	for _, path := range paths {
		if !exists(path) {
			errs = append(errs, "missing "+path)
		}
	}
	if b, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(b), "[mcp_servers.agent_harness]") {
		errs = append(errs, "Codex MCP config missing agent_harness")
	}
	if b, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json")); err != nil || !strings.Contains(string(b), "session-start-llm-wiki.sh") {
		errs = append(errs, "Claude user SessionStart hook missing")
	}
	duplicateWarnings := detectClaudeMCPDuplicateWarnings(claudeMCPDuplicateWarningFixture())
	warningBytes, _ := json.MarshalIndent(map[string]any{
		"duplicate_mcp_warning_fixture": duplicateWarnings,
	}, "", "  ")
	stdoutParts = append(stdoutParts, string(warningBytes))
	if len(duplicateWarnings) != 1 || duplicateWarnings[0].Server != "agent-harness" || !strings.Contains(duplicateWarnings[0].Message, "multiple scopes") {
		errs = append(errs, "Claude duplicate MCP warning fixture was not classified")
	}
	if len(errs) > 0 {
		return assertionStepWithOutput("native integration", started, errs, stdoutParts, nil)
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "native integration",
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

type ClaudeMCPDuplicateWarning struct {
	Server      string   `json:"server"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions"`
}

func detectClaudeMCPDuplicateWarnings(output string) []ClaudeMCPDuplicateWarning {
	warnings := []ClaudeMCPDuplicateWarning{}
	current := -1
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "└"))
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "│"))
		if strings.Contains(trimmed, "[Warning]") && strings.Contains(trimmed, "defined in multiple scopes") {
			server := ""
			if before, after, ok := strings.Cut(trimmed, `Server "`); ok {
				_ = before
				if name, _, ok := strings.Cut(after, `"`); ok {
					server = name
				}
			}
			warnings = append(warnings, ClaudeMCPDuplicateWarning{
				Server:      server,
				Message:     strings.TrimSpace(trimmed),
				Suggestions: []string{},
			})
			current = len(warnings) - 1
			continue
		}
		if current >= 0 && strings.Contains(trimmed, "Suggestion:") {
			_, suggestion, _ := strings.Cut(trimmed, "Suggestion:")
			warnings[current].Suggestions = append(warnings[current].Suggestions, strings.TrimSpace(suggestion))
		}
	}
	return warnings
}

func claudeMCPDuplicateWarningFixture() string {
	return `MCP Config Diagnostics

For help configuring MCP servers, see: https://code.claude.com/docs/en/mcp

[Conflicting scopes]
 └ [Warning] Server "agent-harness" is defined in multiple scopes with different endpoints: user (/Users/example/agent-harness/bin/harness mcp), project (./bin/harness mcp). OAuth tokens are stored per endpoint, so authenticating in one context will not carry over.
   Suggestion: Keep the correct endpoint and remove the others: ` + "`claude mcp remove agent-harness -s user`" + ` or ` + "`claude mcp remove agent-harness -s project`" + `
`
}

var secretMaterialPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "private_key", re: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{name: "aws_access_key_id", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{name: "github_token", re: regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`)},
	{name: "openai_token", re: regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`)},
	{name: "secret_assignment", re: regexp.MustCompile(`(?i)\b(token|secret|password|api[_-]?key|access[_-]?key)\s*[:=]\s*["']?([^\s"',}]+)`)},
}

func validateRedactionAudit(root string) StepResult {
	started := time.Now()
	errs := []string{}
	for _, path := range redactionAuditFiles(root) {
		b, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, "read redaction audit file "+path+": "+err.Error())
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		for _, finding := range findUnredactedSecretLike(string(b)) {
			errs = append(errs, filepath.ToSlash(rel)+": "+finding)
		}
	}
	return assertionStep("redaction audit", started, errs)
}

func redactionAuditFiles(root string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(path string) {
		if path == "" || seen[path] || !exists(path) {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, path := range core.ListDocs(root) {
		add(path)
	}
	for _, pattern := range []string{
		filepath.Join(root, "cmd", "harness", "testdata", "*"),
		filepath.Join(root, "internal", "adapter", "testdata", "*"),
		filepath.Join(root, "skills", "*", "SKILL.md"),
		filepath.Join(root, "skills", "*", "agents", "openai.yaml"),
	} {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			add(match)
		}
	}
	sort.Strings(out)
	return out
}

func findUnredactedSecretLike(text string) []string {
	findings := []string{}
	for lineNo, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || lineContainsAllowedSecretPlaceholder(line) {
			continue
		}
		for _, pattern := range secretMaterialPatterns {
			if pattern.re.MatchString(line) {
				findings = append(findings, fmt.Sprintf("line %d contains %s", lineNo+1, pattern.name))
			}
		}
	}
	return findings
}

func lineContainsAllowedSecretPlaceholder(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"redacted", "placeholder", "example", "fake", "dummy", "sample", "$secret", "$token", "<secret", "<token", "..."} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func validateQAGate(root string) StepResult {
	started := time.Now()
	errs := []string{}
	requiredDocs := map[string][]string{
		filepath.Join(root, "GENIUS_THINK.md"):                    {"천재적 사고", "Mermaid"},
		filepath.Join(root, "agent_docs", "SELF_AUGMENTATION.md"): {"자기 검증 루프", "자가 증강 루프", "95"},
		filepath.Join(root, "agent_docs", "TESTING.md"):           {"self-verify", "QA"},
	}
	for path, needles := range requiredDocs {
		b, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, "missing QA doc "+path)
			continue
		}
		text := string(b)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				errs = append(errs, fmt.Sprintf("%s missing %q", path, needle))
			}
		}
	}
	skills, err := core.ListSkillNames(root)
	if err != nil {
		errs = append(errs, "list skills: "+err.Error())
	}
	for _, want := range []string{"atomic-commit-push", "llm-wiki", "self-augment"} {
		if !containsString(skills, want) {
			errs = append(errs, "missing shared skill "+want)
		}
	}
	for _, skill := range skills {
		skillDir := filepath.Join(root, "skills", skill)
		skillMD := filepath.Join(skillDir, "SKILL.md")
		b, err := os.ReadFile(skillMD)
		if err != nil {
			errs = append(errs, "missing skill file "+skillMD)
			continue
		}
		text := string(b)
		if !strings.Contains(text, "\nname:") && !strings.HasPrefix(text, "---\nname:") {
			errs = append(errs, "skill missing name frontmatter "+skill)
		}
		if !strings.Contains(text, "\ndescription:") {
			errs = append(errs, "skill missing description frontmatter "+skill)
		}
		if !exists(filepath.Join(skillDir, "agents", "openai.yaml")) {
			errs = append(errs, "skill missing agents/openai.yaml "+skill)
		}
	}
	errs = append(errs, validateMermaidDocs(root)...)
	return assertionStep("QA gate", started, errs)
}

var mermaidUnquotedBracketTextRe = regexp.MustCompile(`\[[^"\]]`)

func validateMermaidDocs(root string) []string {
	errs := []string{}
	for _, path := range core.ListDocs(root) {
		b, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, "read mermaid doc "+path+": "+err.Error())
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		for _, issue := range lintMermaidBlocks(filepath.ToSlash(rel), string(b)) {
			errs = append(errs, issue)
		}
	}
	return errs
}

func lintMermaidBlocks(relPath, text string) []string {
	errs := []string{}
	lines := strings.Split(text, "\n")
	inMermaid := false
	ignoreBlock := false
	currentHeading := ""
	ignoreNextMermaid := false
	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "harness:mermaid-lint ignore") {
			ignoreNextMermaid = true
		}
		if strings.HasPrefix(trimmed, "#") {
			currentHeading = trimmed
		}
		if strings.HasPrefix(trimmed, "```") {
			if !inMermaid {
				if strings.HasPrefix(trimmed, "```mermaid") {
					inMermaid = true
					ignoreBlock = ignoreNextMermaid || strings.Contains(currentHeading, "잘못된 예시")
					ignoreNextMermaid = false
				}
				continue
			}
			inMermaid = false
			ignoreBlock = false
			continue
		}
		if !inMermaid || ignoreBlock {
			continue
		}
		if strings.Contains(line, "<br>") {
			errs = append(errs, fmt.Sprintf("%s:%d mermaid uses <br>; use <br/>", relPath, lineNo))
		}
		if mermaidUnquotedBracketTextRe.MatchString(line) {
			errs = append(errs, fmt.Sprintf("%s:%d mermaid node text must start with a quote", relPath, lineNo))
		}
		if strings.HasPrefix(trimmed, "subgraph ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "subgraph "))
			if title != "" && !strings.HasPrefix(title, `"`) {
				errs = append(errs, fmt.Sprintf("%s:%d mermaid subgraph title must be quoted", relPath, lineNo))
			}
		}
	}
	return errs
}

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return runCommandStepEnv(dir, label, timeout, stdin, nil, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	return runCommandStepEnvWithBudget(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
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
	stdoutText, stdoutTruncated, stdoutBytes := budgetCommandOutput(stdout.String(), outputBudget)
	stderrText, stderrTruncated, stderrBytes := budgetCommandOutput(stderr.String(), outputBudget)
	step := StepResult{
		Label:           label,
		Command:         strings.Join(append([]string{name}, args...), " "),
		OK:              err == nil,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		Stderr:          stderrText,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     stderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}
	if ctx.Err() == context.DeadlineExceeded {
		step.OK = false
		step.Error = "timeout after " + timeout.String()
	} else if err != nil {
		step.Error = err.Error()
	}
	return step
}

func budgetCommandOutput(s string, budget int) (string, bool, int) {
	if budget <= 0 {
		return s, false, len(s)
	}
	return tailWithBudget(s, budget)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	step := StepResult{
		Label:           label,
		Command:         strings.Join(commands, " && "),
		OK:              false,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		Stderr:          child.Stderr,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     child.StderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: child.StderrTruncated,
		Error:           child.Label + ": " + child.Error,
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
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
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
	out, _, _ := tailWithBudget(s, max)
	return out
}

func tailWithBudget(s string, max int) (string, bool, int) {
	originalBytes := len(s)
	if max <= 0 {
		return "", originalBytes > 0, originalBytes
	}
	if originalBytes <= max {
		return s, false, originalBytes
	}
	tailBudget := max
	marker := fmt.Sprintf("[truncated: original_bytes=%d omitted_bytes=%d]\n", originalBytes, originalBytes-tailBudget)
	tailBudget = max - len(marker)
	if tailBudget < 0 {
		return marker[:max], true, originalBytes
	}
	marker = fmt.Sprintf("[truncated: original_bytes=%d omitted_bytes=%d]\n", originalBytes, originalBytes-tailBudget)
	return marker + s[originalBytes-tailBudget:], true, originalBytes
}

func indentLines(s string) string {
	lines := splitLines(s)
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func runMCP() error {
	if os.Getenv("HARNESS_MCP_DIRECT") == "1" || os.Getenv("HARNESS_DAEMON_DISABLE") == "1" {
		return serveMCPStream(os.Stdin, os.Stdout, os.Stderr)
	}
	return runMCPProxy()
}

func serveMCPStream(input io.Reader, output io.Writer, diagnostics io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeRPCErrorTo(output, nil, -32700, "Parse error", err.Error())
			continue
		}
		if len(req.ID) == 0 {
			handleNotificationTo(diagnostics, req)
			continue
		}
		result, rpcErr := handleRequest(req)
		if rpcErr != nil {
			writeRPCErrorTo(output, req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			continue
		}
		writeRPCResultTo(output, req.ID, result)
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
	handleNotificationTo(os.Stderr, req)
}

func handleNotificationTo(w io.Writer, req rpcRequest) {
	// notifications/initialized and cancellation notifications intentionally require no response.
	fmt.Fprintln(w, "harness mcp notification:", req.Method)
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
			"instructions": "This MCP endpoint is a proxy to the shared agent-harness daemon. At session start, read resource harness://llm-wiki/session-context or call llm_wiki_session_context to load the local LLM Wiki operating rules and inventory. Use llm-wiki only when durable knowledge, previous decisions, source-card citations, or reusable capture materially improves the task. Use other harness tools for shared Codex/Claude inspection, atomic commit preflight, state checkpoints, self-verification, self-augmentation, and commit policy context.",
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
			"description": "Return a lightweight index of AGENTS.md, CLAUDE.md, GENIUS_THINK.md, and agent_docs markdown files: relative path, title, headings, and byte size.",
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
			"name":        "daemon_status",
			"description": "Report whether the shared agent-harness daemon backing this MCP proxy is reachable, including socket and pid metadata.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "llm_wiki_inventory",
			"description": "Inspect the local LLM Wiki vault inventory, entry points, and page counts. Defaults to ~/workspace/knowledge-base/llm-wiki or LLM_WIKI_ROOT.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"root":         map[string]any{"type": "string", "description": "Optional LLM Wiki root path."},
				"project_path": map[string]any{"type": "string", "description": "Optional current project path for session context."},
			}},
		},
		{
			"name":        "llm_wiki_session_context",
			"description": "Return session-start instructions plus bounded index context for using the shared local LLM Wiki safely and selectively.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"root":         map[string]any{"type": "string", "description": "Optional LLM Wiki root path."},
				"project_path": map[string]any{"type": "string", "description": "Optional current project path."},
			}},
		},
		{
			"name":        "llm_wiki_search",
			"description": "Search canonical LLM Wiki markdown surfaces (00-meta, 10-sources, 20-wiki, 30-sessions) with a focused lexical query.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"query"}, "properties": map[string]any{
				"root":  map[string]any{"type": "string", "description": "Optional LLM Wiki root path."},
				"query": map[string]any{"type": "string", "description": "Focused search terms."},
				"limit": map[string]any{"type": "integer", "description": "Maximum results, default 10, max 50."},
			}},
		},
		{
			"name":        "llm_wiki_read",
			"description": "Read a bounded LLM Wiki page by relative path, slug, or wikilink. Refuses paths outside the vault and .obsidian.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"page"}, "properties": map[string]any{
				"root": map[string]any{"type": "string", "description": "Optional LLM Wiki root path."},
				"page": map[string]any{"type": "string", "description": "Relative path, slug, or wikilink."},
			}},
		},
		{
			"name":        "llm_wiki_capture",
			"description": "Create a concise durable LLM Wiki capture under 30-sessions or synthesized page under 20-wiki. Requires title and content; checks 00-meta/AGENTS.md before writing.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"title", "content"}, "properties": map[string]any{
				"root":         map[string]any{"type": "string", "description": "Optional LLM Wiki root path."},
				"title":        map[string]any{"type": "string", "description": "Capture title."},
				"content":      map[string]any{"type": "string", "description": "Markdown content to capture."},
				"type":         map[string]any{"type": "string", "description": "session, concept, entity, or summary. Defaults to session."},
				"status":       map[string]any{"type": "string", "description": "Page status. Defaults to active."},
				"tags":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Frontmatter tags."},
				"sources":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Source wikilinks."},
				"related":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Related wikilinks."},
				"project_path": map[string]any{"type": "string", "description": "Optional current project path."},
			}},
		},
		{
			"name":        "self_augment",
			"description": "Plan the 자가 증강 루프: use GENIUS_THINK.md, repo signals, and research-backed strategies to choose concrete feature/performance/quality improvements. The native skill performs implementation; this tool exposes the scoring contract and candidate curriculum, and can persist the chosen plan to harness state for durable Reflexion-style memory.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"cycles":       map[string]any{"type": "integer", "description": "Number of autonomous improvement cycles to plan; defaults to 1."},
				"target_score": map[string]any{"type": "number", "description": "Exclusive per-goal score threshold; defaults to 95."},
				"save_state":   map[string]any{"type": "boolean", "description": "When true, save the selected self-augmentation plan to harness state."},
				"state_key":    map[string]any{"type": "string", "description": "State key for save_state; defaults to self-augment-latest."},
			}},
		},
		{
			"name":        "self_augment_lesson",
			"description": "Store a Reflexion-style 자가 증강 lesson in harness state and return an llm-wiki capture draft for durable cross-session learning.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"lesson", "next_action"}, "properties": map[string]any{
				"candidate_id": map[string]any{"type": "string", "description": "Candidate id this lesson belongs to; defaults to current selected open candidate."},
				"lesson":       map[string]any{"type": "string", "description": "Lesson learned from failure, QA issue, or design concern."},
				"next_action":  map[string]any{"type": "string", "description": "Specific next action that should use this lesson."},
				"source":       map[string]any{"type": "string", "description": "Source that produced the lesson; defaults to self-augment."},
				"severity":     map[string]any{"type": "string", "description": "info, warning, or error. Defaults to info."},
				"state_key":    map[string]any{"type": "string", "description": "State key; defaults to self-augment-lesson-<candidate>-<timestamp>."},
			}},
		},
		{
			"name":        "self_verify",
			"description": "Run the 자기 검증 루프: at least 10 seeded iterations of tests, risk-tier QA, build, CLI/MCP schema and response contract golden checks, command policy, MCP, state roundtrip, native integration, and git preflight fuzz checks. Termination requires every concrete goal score to be greater than target_score.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"iterations":   map[string]any{"type": "integer", "description": "Iteration count; must be at least 10."},
				"seed":         map[string]any{"type": "integer", "description": "Base seed for deterministic per-iteration fuzz fixtures."},
				"target_score": map[string]any{"type": "number", "description": "Exclusive per-goal score threshold; defaults to 95."},
				"save_state":   map[string]any{"type": "boolean", "description": "When true, save compact summary to harness state after the run."},
				"state_key":    map[string]any{"type": "string", "description": "State key for save_state; defaults to self-verify-latest."},
			}},
		},
		{
			"name":        "self_verify_history",
			"description": "List saved 자기 검증 루프 summary checkpoints from harness state, sorted by snapshot generation time for quick baseline/candidate discovery.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"prefix":          map[string]any{"type": "string", "description": "State key prefix to scan; defaults to self-verify. Use empty string to scan all keys."},
				"limit":           map[string]any{"type": "integer", "description": "Maximum entries to return; defaults to 20, 0 returns all."},
				"retention_limit": map[string]any{"type": "integer", "description": "Maximum matching summaries to retain by newest-first ordering. Omit or use 0 to disable retention planning."},
				"prune_retention": map[string]any{"type": "boolean", "description": "When true, plan retention pruning. This is a dry-run unless confirm is also true."},
				"confirm":         map[string]any{"type": "boolean", "description": "When true with prune_retention, delete retention candidates beyond retention_limit."},
			}},
		},
		{
			"name":        "self_verify_compare",
			"description": "Compare two saved 자기 검증 루프 summary checkpoints from harness state and report elapsed-time, failed-step, step-label, and goal-score regressions.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"baseline_key", "candidate_key"}, "properties": map[string]any{
				"baseline_key":               map[string]any{"type": "string", "description": "State key containing the baseline self-verification summary snapshot."},
				"candidate_key":              map[string]any{"type": "string", "description": "State key containing the candidate self-verification summary snapshot."},
				"max_elapsed_regression_pct": map[string]any{"type": "number", "description": "Allowed elapsed_ms increase percentage before regression; defaults to 20."},
			}},
		},
		{
			"name":        "self_verify_promote",
			"description": "Promote a saved 자기 검증 루프 summary checkpoint to a baseline state key. Defaults to dry-run; pass confirm=true to write the baseline.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"from_key", "baseline_key"}, "properties": map[string]any{
				"from_key":     map[string]any{"type": "string", "description": "State key containing the candidate self-verification summary snapshot to promote."},
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
	case "daemon_status":
		payload = daemonStatusForMCP()
	case "llm_wiki_inventory":
		projectPath := stringArg(call.Arguments, "project_path")
		if projectPath == "" {
			projectPath = resolveTarget("")
		}
		result, err := core.LLMWikiInventoryFor(stringArg(call.Arguments, "root"), projectPath)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "LLM Wiki inventory failed", Data: err.Error()}
		}
		payload = result
	case "llm_wiki_session_context":
		projectPath := stringArg(call.Arguments, "project_path")
		if projectPath == "" {
			projectPath = resolveTarget("")
		}
		result, err := core.LLMWikiSessionContextFor(stringArg(call.Arguments, "root"), projectPath)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "LLM Wiki session context failed", Data: err.Error()}
		}
		payload = result
	case "llm_wiki_search":
		result, err := core.LLMWikiSearch(stringArg(call.Arguments, "root"), stringArg(call.Arguments, "query"), intArg(call.Arguments, "limit", 10))
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "LLM Wiki search failed", Data: err.Error()}
		}
		payload = result
	case "llm_wiki_read":
		result, err := core.LLMWikiRead(stringArg(call.Arguments, "root"), stringArg(call.Arguments, "page"))
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "LLM Wiki read failed", Data: err.Error()}
		}
		payload = result
	case "llm_wiki_capture":
		projectPath := stringArg(call.Arguments, "project_path")
		if projectPath == "" {
			projectPath = resolveTarget("")
		}
		result, err := core.LLMWikiCapture(core.LLMWikiCaptureRequest{
			Root:        stringArg(call.Arguments, "root"),
			Title:       stringArg(call.Arguments, "title"),
			Content:     stringArg(call.Arguments, "content"),
			Type:        stringArgWithDefault(call.Arguments, "type", "session"),
			Status:      stringArgWithDefault(call.Arguments, "status", "active"),
			Tags:        stringSliceArg(call.Arguments, "tags"),
			Sources:     stringSliceArg(call.Arguments, "sources"),
			Related:     stringSliceArg(call.Arguments, "related"),
			ProjectPath: projectPath,
		})
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "LLM Wiki capture failed", Data: err.Error()}
		}
		payload = result
	case "self_augment":
		result := planSelfAugmentation(SelfAugmentPlanRequest{
			Cycles:      intArg(call.Arguments, "cycles", 1),
			TargetScore: floatArg(call.Arguments, "target_score", defaultLoopTargetScoreExclusive),
		})
		if boolArg(call.Arguments, "save_state") {
			if err := saveSelfAugmentPlan(&result, stringArgWithDefault(call.Arguments, "state_key", "self-augment-latest")); err != nil {
				return nil, &rpcError{Code: -32000, Message: "Self-augmentation plan save failed", Data: result}
			}
		}
		payload = result
	case "self_augment_lesson":
		result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{
			CandidateID: stringArg(call.Arguments, "candidate_id"),
			Lesson:      stringArg(call.Arguments, "lesson"),
			NextAction:  stringArg(call.Arguments, "next_action"),
			Source:      stringArgWithDefault(call.Arguments, "source", "self-augment"),
			Severity:    stringArgWithDefault(call.Arguments, "severity", "info"),
			StateKey:    stringArg(call.Arguments, "state_key"),
		})
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-augmentation lesson save failed", Data: result}
		}
		payload = result
	case "self_verify":
		iterations := intArg(call.Arguments, "iterations", 10)
		seed := int64Arg(call.Arguments, "seed", time.Now().Unix())
		targetScore := floatArg(call.Arguments, "target_score", defaultLoopTargetScoreExclusive)
		result, err := selfVerify(iterations, seed, targetScore, false)
		if boolArg(call.Arguments, "save_state") {
			saveErr := saveSelfVerificationSummary(&result, stringArgWithDefault(call.Arguments, "state_key", "self-verify-latest"))
			if err == nil && saveErr != nil {
				err = saveErr
			}
		}
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Self-verification failed", Data: result}
		}
		payload = result
	case "self_verify_history", "self_augment_history":
		result, err := selfAugmentHistory(
			stringArgWithDefault(call.Arguments, "prefix", "self-verify"),
			intArg(call.Arguments, "limit", 20),
			selfAugmentHistoryRetentionOptions{
				Limit:          intArg(call.Arguments, "retention_limit", 0),
				PruneRequested: boolArg(call.Arguments, "prune_retention"),
				Confirm:        boolArg(call.Arguments, "confirm"),
			},
		)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-verify history failed", Data: err.Error()}
		}
		payload = result
	case "self_verify_compare", "self_augment_compare":
		result, err := compareSelfAugmentSummaries(
			stringArg(call.Arguments, "baseline_key"),
			stringArg(call.Arguments, "candidate_key"),
			floatArg(call.Arguments, "max_elapsed_regression_pct", 20),
		)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-verify compare failed", Data: err.Error()}
		}
		payload = result
	case "self_verify_promote", "self_augment_promote":
		result, err := promoteSelfAugmentBaseline(
			stringArg(call.Arguments, "from_key"),
			stringArg(call.Arguments, "baseline_key"),
			boolArg(call.Arguments, "confirm"),
		)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: "Self-verify promote failed", Data: err.Error()}
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
		{"uri": "harness://llm-wiki/session-context", "name": "LLM Wiki session context", "description": "Session-start instructions and bounded inventory for the shared local LLM Wiki.", "mimeType": "text/markdown"},
		{"uri": "harness://llm-wiki/inventory", "name": "LLM Wiki inventory", "description": "JSON inventory of the shared local LLM Wiki vault.", "mimeType": "application/json"},
		{"uri": "harness://llm-wiki/index", "name": "LLM Wiki index", "description": "Canonical llm-wiki catalog at 00-meta/index.md.", "mimeType": "text/markdown"},
		{"uri": "harness://llm-wiki/schema", "name": "LLM Wiki schema", "description": "Canonical llm-wiki agent operating rules at 00-meta/AGENTS.md.", "mimeType": "text/markdown"},
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
	if req.URI == "harness://llm-wiki/session-context" {
		result, err := core.LLMWikiSessionContextFor("", resolveTarget(""))
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Cannot read LLM Wiki session context", Data: err.Error()}
		}
		return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "text/markdown", "text": result.Text}}}, nil
	}
	if req.URI == "harness://llm-wiki/inventory" {
		result, err := core.LLMWikiInventoryFor("", resolveTarget(""))
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Cannot read LLM Wiki inventory", Data: err.Error()}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "application/json", "text": string(b)}}}, nil
	}
	if req.URI == "harness://llm-wiki/index" || req.URI == "harness://llm-wiki/schema" {
		root, err := core.ResolveLLMWikiRoot("")
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Cannot resolve LLM Wiki root", Data: err.Error()}
		}
		rel := "00-meta/index.md"
		if req.URI == "harness://llm-wiki/schema" {
			rel = "00-meta/AGENTS.md"
		}
		result, err := core.LLMWikiRead(root, rel)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "Cannot read LLM Wiki resource", Data: err.Error()}
		}
		return map[string]any{"contents": []map[string]any{{"uri": req.URI, "mimeType": "text/markdown", "text": result.Content}}}, nil
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
	writeRPCResultTo(os.Stdout, id, result)
}

func writeRPCError(id json.RawMessage, code int, message string, data any) {
	writeRPCErrorTo(os.Stdout, id, code, message, data)
}

func writeRPCResultTo(w io.Writer, id json.RawMessage, result any) {
	msg := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result}
	b, _ := json.Marshal(msg)
	fmt.Fprintln(w, string(b))
}

func writeRPCErrorTo(w io.Writer, id json.RawMessage, code int, message string, data any) {
	msg := map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": code, "message": message, "data": data}}
	if id != nil {
		msg["id"] = json.RawMessage(id)
	} else {
		msg["id"] = nil
	}
	b, _ := json.Marshal(msg)
	fmt.Fprintln(w, string(b))
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
