package harnessapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/cmd/harness/qualitycli"
	"agent-harness/internal/core"
	"agent-harness/internal/port"
)

func TestCommandStepFacadeWrappers(t *testing.T) {
	step := runCommandStep("", "echo", time.Second, "", "sh", "-c", "printf ok")
	if !step.OK || step.Stdout != "ok" {
		t.Fatalf("runCommandStep = %#v", step)
	}
	envStep := runCommandStepEnv("", "env", time.Second, "", []string{"HARNESSAPP_TEST_ENV=ok"}, "sh", "-c", "printf $HARNESSAPP_TEST_ENV")
	if !envStep.OK || envStep.Stdout != "ok" {
		t.Fatalf("runCommandStepEnv = %#v", envStep)
	}
	budgetStep := runCommandStepEnvWithBudget("", "budget", time.Second, "", nil, 2, "sh", "-c", "printf abc")
	if !budgetStep.OK || !budgetStep.StdoutTruncated {
		t.Fatalf("runCommandStepEnvWithBudget = %#v", budgetStep)
	}
	if got := mergeEnvOverrides([]string{"A=1", "B=1"}, []string{"A=2"}); !containsString(got, "A=2") || !containsString(got, "B=1") {
		t.Fatalf("mergeEnvOverrides = %#v", got)
	}
	if key, ok := envEntryKey("A=1"); !ok || key != "A" {
		t.Fatalf("envEntryKey = %q %v", key, ok)
	}
	if out, truncated, n := budgetCommandOutput("abcdef", 3); out == "" || !truncated || n != 6 {
		t.Fatalf("budgetCommandOutput = %q %v %d", out, truncated, n)
	}
	started := time.Now()
	child := failedStep("child", errors.New("boom"))
	if combined := combineFailedStep("parent", started, child, []string{"out"}, []string{"cmd"}); combined.OK {
		t.Fatalf("combineFailedStep = %#v", combined)
	}
	if assertionStep("assert", started, []string{"bad"}).OK {
		t.Fatal("assertionStep should fail")
	}
	if assertionStepWithOutput("assert", started, []string{"bad"}, []string{"out"}, []string{"cmd"}).OK {
		t.Fatal("assertionStepWithOutput should fail")
	}
	printStep(StepResult{Label: "covered", OK: true})
	if tail("abcdef", 3) == "" {
		t.Fatal("tail returned empty")
	}
	if out, truncated, n := tailWithBudget("abcdef", 3); out == "" || !truncated || n != 6 {
		t.Fatalf("tailWithBudget = %q %v %d", out, truncated, n)
	}
	if !strings.Contains(indentLines("a\nb"), "  a") {
		t.Fatal("indentLines did not indent")
	}
}

func TestAppAndRootCommandFacadeWrappers(t *testing.T) {
	var buf bytes.Buffer
	fprintString(&buf, "hello")
	fprintUsage(&buf)
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("buffer = %q", buf.String())
	}
	cmd := rootCommand()
	if cmd.Version != version || len(cmd.Runners) == 0 {
		t.Fatalf("root command = %#v", cmd)
	}
	if rootSubcommandErrorExitCode("unknown", errors.New("bad")) != 1 {
		t.Fatal("default root subcommand exit code changed")
	}
	if code := RunRootCommand([]string{"--version"}); code != 0 {
		t.Fatalf("RunRootCommand --version = %d", code)
	}
}

func TestHostAndPathFacadeWrappers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	t.Setenv("HARNESSAPP_BOOL", "true")
	t.Setenv("HARNESSAPP_FLOAT", "1.5")
	if err := os.MkdirAll(filepath.Join(root, "skills", skillName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", skillName, "SKILL.md"), []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, err := readHarnessFile("skills", skillName, "SKILL.md"); err != nil || text != "skill" {
		t.Fatalf("readHarnessFile = %q err=%v", text, err)
	}
	if harnessRoot() != root {
		t.Fatalf("harnessRoot = %q", harnessRoot())
	}
	if found, ok := findUp(filepath.Join(root, "skills", skillName), "SKILL.md"); !ok || found != filepath.Join(root, "skills", skillName) {
		t.Fatalf("findUp = %q %v", found, ok)
	}
	if resolveTarget(root) != root || !exists(filepath.Join(root, "skills", skillName, "SKILL.md")) {
		t.Fatal("path facade wrappers failed")
	}
	if lines := splitLines("a\nb\n"); len(lines) != 2 {
		t.Fatalf("splitLines = %#v", lines)
	}
	if csv := splitCSV("a, b,,"); len(csv) != 2 || csv[1] != "b" {
		t.Fatalf("splitCSV = %#v", csv)
	}
	if !containsString([]string{"a"}, "a") {
		t.Fatal("containsString failed")
	}
	if !stateDoctorHasIssueCode([]core.StateDoctorIssue{{Code: "x"}}, "x") {
		t.Fatal("stateDoctorHasIssueCode failed")
	}

	input := []byte(`{"repo":"repo","source":"codex","paths":["a"],"prompt":"continue","last_assistant_message":"done","transcript_path":"transcript.jsonl","tool_name":"Shell","command":"go test","project_path":"/tmp/project"}`)
	_ = hookArgValue([]string{"--input", "file"}, "input")
	_ = repoFromHookInput(input)
	_ = sourceFromHookInput(input)
	_ = pathsFromHookInput(input)
	_ = promptFromHookInput(input)
	_ = lastAssistantMessageFromHookInput(input)
	_ = transcriptPathFromHookInput(input)
	_ = toolNameFromHookInput(input)
	_ = commandFromHookInput(input)
	_ = projectPathFromHookInput(input)
	if !envBool("HARNESSAPP_BOOL") || envFloat("HARNESSAPP_FLOAT") != 1.5 {
		t.Fatal("env wrappers failed")
	}
	_ = isStopHookContinuationPrompt("<hook_prompt>다음 행동 판단 지점</hook_prompt>")
	if got := readLastAssistantMessageFromTranscript(filepath.Join(root, "missing.jsonl")); got != "" {
		t.Fatalf("missing transcript = %q", got)
	}
}

func TestUpdateAndAPIDocFacadeWrappers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	previousRunner := installScriptCommandRunner
	previousDaemonRefresh := postInstallDaemonRefresh
	previousMCPRefresh := postInstallMCPProxyRefresh
	previousDaemonLister := daemonProcessLister
	previousDaemonTerminator := daemonProcessTerminator
	previousMCPProxyLister := mcpProxyProcessLister
	previousMCPProxyTerminator := mcpProxyTerminator
	t.Cleanup(func() {
		installScriptCommandRunner = previousRunner
		postInstallDaemonRefresh = previousDaemonRefresh
		postInstallMCPProxyRefresh = previousMCPRefresh
		daemonProcessLister = previousDaemonLister
		daemonProcessTerminator = previousDaemonTerminator
		mcpProxyProcessLister = previousMCPProxyLister
		mcpProxyTerminator = previousMCPProxyTerminator
		resetUpdateFacadeDeps()
	})
	installScriptCommandRunner = func(string, ...string) error { return nil }
	postInstallDaemonRefresh = func() (bool, error) { return true, nil }
	postInstallMCPProxyRefresh = func() (int, error) { return 0, nil }
	daemonProcessLister = func() ([]daemonProcess, error) { return []daemonProcess{{PID: 11, Command: "daemon"}}, nil }
	daemonProcessTerminator = func(pid int) error { return nil }
	mcpProxyProcessLister = func() ([]mcpProxyProcess, error) { return []mcpProxyProcess{{PID: 22, Command: "mcp"}}, nil }
	mcpProxyTerminator = func(pid int) error { return nil }

	if err := runUpdate([]string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if err := runBootstrap([]string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if err := runInstallScriptCommand("update", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if terminated, err := terminateStaleDaemonProcesses(); err != nil || terminated != 1 {
		t.Fatalf("terminateStaleDaemonProcesses = %d err=%v", terminated, err)
	}
	binary := filepath.Join(root, "bin", "agent-harness")
	if parsed, ok := parseDaemonProcess("11 "+binary+" daemon --internal", binary); !ok || parsed.PID != 11 {
		t.Fatalf("parseDaemonProcess = %#v %v", parsed, ok)
	}
	if refreshed, err := refreshRunningMCPProxiesAfterInstall(); err != nil || refreshed != 1 {
		t.Fatalf("refreshRunningMCPProxiesAfterInstall = %d err=%v", refreshed, err)
	}
	if parsed, ok := parseMCPProxyProcess("22 "+binary+" mcp", binary); !ok || parsed.PID != 22 {
		t.Fatalf("parseMCPProxyProcess = %#v %v", parsed, ok)
	}
	if _, err := listDaemonProcesses(); err != nil {
		t.Fatal(err)
	}
	if _, err := listMCPProxyProcesses(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := printJSONTo(&buf, map[string]any{"ok": true}); err != nil || !strings.Contains(buf.String(), `"ok"`) {
		t.Fatalf("printJSONTo = %q err=%v", buf.String(), err)
	}
	if !isAPIDocReviewGateError(errAPIDocReviewGateFailed) || !isAPIDocStaticGateError(errAPIDocStaticGateFailed) || !isSelfVerificationGateError(errSelfVerificationGateFailed) {
		t.Fatal("gate error wrappers failed")
	}
	if len(apiDocReviewSchema()) == 0 {
		t.Fatal("apiDocReviewSchema empty")
	}
	if !isAPIDocCandidate("src/user.controller.ts") {
		t.Fatal("isAPIDocCandidate failed")
	}
	if got := normalizeAPIDocFiles(root, []string{filepath.Join(root, "src", "user.controller.ts")}); len(got) != 1 {
		t.Fatalf("normalizeAPIDocFiles = %#v", got)
	}
	_ = checkNestControllerStatic("user.controller.ts", "@Controller('users')\nexport class UserController {}")
	_ = checkNestDTOStatic("dto.ts", "export class UserDto {}")
	_ = buildAPIDocReviewPrompt([]string{"a.ts"}, "diff", "extra")
}

func TestSelfWorkflowAndLLMFacadeWrappers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("GENIUS_THINK quality inspect"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte("coverage signal"), 0o644); err != nil {
		t.Fatal(err)
	}
	history := SelfAugmentHistoryResult{Entries: []SelfAugmentHistoryEntry{}}
	if err := applySelfAugmentHistoryRetention(&history, selfAugmentHistoryRetentionOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := parseSelfAugmentTimestamp(time.Now().UTC().Format(time.RFC3339Nano)); !ok {
		t.Fatal("parseSelfAugmentTimestamp failed")
	}
	if got := nonNilStringSlice(nil); got == nil {
		t.Fatal("nonNilStringSlice returned nil")
	}
	if got := nonNilSlowStepSlice(nil); got == nil {
		t.Fatal("nonNilSlowStepSlice returned nil")
	}
	signals := collectSelfAugmentRepoSignals(root, 1, []string{"self-verify"}, "GENIUS_THINK")
	candidates := selfAugmentCandidates(signals)
	if len(candidates) == 0 {
		t.Fatal("selfAugmentCandidates empty")
	}
	if scoreBool(true) <= scoreBool(false) {
		t.Fatal("scoreBool failed")
	}
	if !allSelfAugmentGoalsPassed([]SelfAugmentGoal{{Passed: true}}) {
		t.Fatal("allSelfAugmentGoalsPassed failed")
	}
	if selectedCandidateID(nil) != "" {
		t.Fatal("selectedCandidateID nil should be empty")
	}
	_ = docsContainTerm(root, "coverage")
	_ = fileContainsTerm(root, "README.md", "quality")
	_ = dirContainsTerm(root, "docs", "signal")
	_ = selectGeniusFormulas("invert the problem and use first principles")
	_ = selfAugmentResearchInfluences()
	markSatisfiedSelfAugmentCandidate(&candidates[0], signals)
	_ = selfAugmentCandidateScore(candidates[0])
	_ = compareSlowestStepRegressions(nil, nil, 10)
	_ = compareStepBudgetRegressions(nil, nil, 10)
	if missing := missingStrings([]string{"a", "b"}, []string{"a"}); len(missing) != 1 || missing[0] != "b" {
		t.Fatalf("missingStrings = %#v", missing)
	}
	_ = stepDurationStatByLabel(nil)
	_ = maxSlowStepDurationByLabel(nil)
	_ = buildStepDurationStats(map[string][]int64{"a": {1, 2}})
	result := SelfAugmentResult{OK: true, Iterations: 0, Runs: []SelfAugmentIteration{}}
	summary := summarizeSelfAugment(result)
	_ = stepDurationStatsForCompare(summary)
	verifySummary := summarizeSelfVerification(result, 95)
	_, _, _ = classifySelfVerificationFailure(result, verifySummary)
	_ = selfVerificationFailureClusters(result)
	_ = selfVerifyRerunCommands("go test", 2, 100, 95)
	_, _ = selfVerifyStepRerunCommand("go test ./...")
	if formatScore(95.5) == "" {
		t.Fatal("formatScore empty")
	}
	_ = scoreSelfVerificationGoals(result, 95)
	if len(selfVerificationContract().RequiredFields) == 0 {
		t.Fatal("selfVerificationContract empty")
	}
	if coverage, _ := selfVerificationCoverage([]string{"go test ./..."}); coverage == nil {
		t.Fatal("selfVerificationCoverage nil")
	}
	if len(selfVerificationCoverageDefinitions()) == 0 {
		t.Fatal("selfVerificationCoverageDefinitions empty")
	}

	if err := validateSelfVerifyLLMEvalMode("advisory"); err != nil {
		t.Fatal(err)
	}
	if normalizeSelfVerifyLLMEvalMode("") == "" {
		t.Fatal("normalizeSelfVerifyLLMEvalMode empty")
	}
	if _, _, err := parseSelfVerifyLLMEvalEnv("off"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveSelfVerifyLLMEvalConfig(false, false, "", false, func(string) (string, bool) { return "", false }); err != nil {
		t.Fatal(err)
	}
	var eval SelfVerifyLLMEvalResult
	evalJSON := []byte(`{"ok":true,"mode":"advisory","execution_class":"read_only","read_only":true,"score":100,"summary":"ok","evidence_packet_bytes":10}`)
	if err := decodeSelfVerifyLLMEval(evalJSON, &eval); err != nil {
		t.Fatal(err)
	}
	if err := decodeSelfVerifyLLMEvalStrict(evalJSON, &eval); err != nil {
		t.Fatal(err)
	}
	if _, ok := extractSelfVerifyLLMEvalJSON(append([]byte("prefix "), evalJSON...)); !ok {
		t.Fatal("extractSelfVerifyLLMEvalJSON failed")
	}
	if boundedLLMEvalError("prefix", errors.New("bad"), strings.Repeat("x", 100)) == "" {
		t.Fatal("boundedLLMEvalError empty")
	}
	_, _ = applySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{})
	_, _ = applySelfVerifyLLMGate(result, 95)
	_, _, _ = buildSelfVerifyLLMEvalPrompt(result)
	if selfVerifyLLMResponseSchemaExample() == "" || len(selfVerifyLLMResponseFieldTypes()) == 0 {
		t.Fatal("LLM response schema wrappers empty")
	}
}

func TestRiskMCPAndIssueOpsPolicyFacadeWrappers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	riskStep := validateRiskQATierWithDeps(root, riskQATierDeps{
		plan: func(string) RiskQATierPlan {
			return RiskQATierPlan{Tier: "static", Commands: []string{"go test ./..."}, Reasons: []string{"test"}}
		},
		run: func(root string, command string) StepResult {
			return StepResult{OK: true, Label: "risk", Command: command}
		},
	})
	if !riskStep.OK {
		t.Fatalf("validateRiskQATierWithDeps = %#v", riskStep)
	}
	_ = validateRiskQATier(root)
	plan := planRiskQATierFromPaths([]string{"cmd/harness/harnessapp/mcp_facade.go"})
	if plan.Tier == "" || riskQATierPlanJSON(plan) == "" {
		t.Fatalf("risk plan = %#v", plan)
	}
	_ = planRiskQATier(root)
	if parseGitStatusPath(" M cmd/harness/harnessapp/mcp_facade.go") == "" {
		t.Fatal("parseGitStatusPath empty")
	}

	if _, err := prepareIssueOpsWorktreeTools(core.IssueOpsRecord{}); err == nil {
		t.Fatal("empty issueops record should not prepare worktree tools")
	}
	if issueOpsCleanupMerged("", false) {
		t.Fatal("empty cleanup should not be merged")
	}
	if err := verifyIssueOpsRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pr", URL: "not-a-url"}); err == nil {
		t.Fatal("invalid remote artifact URL should fail")
	}
	if _, _, err := parseCommandPolicyFlags("policy", []string{"--workspace-root", root, "--cwd", root, "--", "git", "status"}); err != nil {
		t.Fatalf("parseCommandPolicyFlags: %v", err)
	}
	if _, _, _, err := parseCommandPolicyRunFlags([]string{"--workspace-root", root, "--cwd", root, "--", "git", "status"}); err != nil {
		t.Fatalf("parseCommandPolicyRunFlags: %v", err)
	}
	if err := runIssueOps([]string{"unknown"}); err == nil {
		t.Fatal("unknown issueops subcommand should fail")
	}
	if err := runPolicy([]string{"unknown"}); err == nil {
		t.Fatal("unknown policy subcommand should fail")
	}

	configureMCPCLI()
	if len(mcpTools()) == 0 || len(mcpResources()) == 0 {
		t.Fatal("MCP catalog wrappers empty")
	}
	if result := textResult("hello"); result["content"] == nil {
		t.Fatalf("textResult = %#v", result)
	}
	if _, rpcErr := handleToolCall(json.RawMessage(`{"name":"unknown","arguments":{}}`)); rpcErr == nil {
		t.Fatal("unknown MCP tool should fail")
	}
	if _, rpcErr := handleResourceRead(json.RawMessage(`{"uri":"unknown://resource"}`)); rpcErr == nil {
		t.Fatal("unknown MCP resource should fail")
	}
	if result, rpcErr := handleRequest(rpcRequest{ID: json.RawMessage(`1`), Method: "initialize"}); rpcErr != nil || result == nil {
		t.Fatalf("initialize result=%#v err=%#v", result, rpcErr)
	}
	if _, rpcErr := handleRequest(rpcRequest{ID: json.RawMessage(`1`), Method: "unknown"}); rpcErr == nil {
		t.Fatal("unknown MCP request should fail")
	}
	var out bytes.Buffer
	if err := serveMCPStream(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &out, &bytes.Buffer{}); err != nil || !strings.Contains(out.String(), "serverInfo") {
		t.Fatalf("serveMCPStream out=%q err=%v", out.String(), err)
	}
	writeRPCResult(json.RawMessage(`1`), map[string]any{"ok": true})
	writeRPCError(json.RawMessage(`1`), -1, "bad", "data")
}

func TestCLIFacadeWrappers(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = runDocs([]string{"--json"})
	_ = runDocsWithRoot([]string{"--json"}, root)
	_ = runPreflight([]string{root})
	_ = runTrace([]string{"unknown"})
	_ = runTraceAnalyze([]string{"--help"})
	_ = runGuard([]string{"unknown"})
	_ = runGuardCheck([]string{"--repo", root, "--json"})
	_ = runQuality([]string{"unknown"})
	if err := runQualityInspectWithDeps([]string{"--json"}, qualityInspectDepsForHarnessAppTest()); err != nil {
		t.Fatalf("runQualityInspectWithDeps: %v", err)
	}
	_ = runInspect([]string{"--json", root})
	_ = runDoctor([]string{"--json"})

	if _, _, _, err := parseDraftWikiPathFlags("draft", []string{"--repo", root, "--json", "draft.md"}); err != nil {
		t.Fatalf("parseDraftWikiPathFlags: %v", err)
	}
	_, _ = draftWikiQueueMaterial(root, "", "material", false)
	_ = runProjectDraftWiki([]string{"unknown"})
	_ = runProjectDraftWikiSuggest([]string{"--repo", root, "--json"})

	_ = runInstall([]string{"--help"})
	_ = runInstallNative([]string{"--dry-run", "--json", "--skip-build"})
	_ = runInstallCommand("install-native", []string{"--dry-run", "--json", "--skip-build"})
	_ = validateInteractiveInstallInput(nil)
	printInstallNativeResult(port.NativeInstallResult{OK: true})
	_ = preferredShellRC(root)
	if _, err := appendShellPathLinePlan(filepath.Join(root, ".zshrc"), true); err != nil {
		t.Fatalf("appendShellPathLinePlan: %v", err)
	}
	_ = shellRCAlreadyAddsLocalBin(filepath.Join(root, ".zshrc"), root)

	_ = runProject([]string{"unknown"})
	_ = runProjectBootstrap([]string{"--repo", root, "--dry-run", "--json"})
	_ = runProjectDocs([]string{"--repo", root, "--json"})
	_ = runProjectRouteDocs([]string{"--repo", root, "--task", "general", "--json"})
	_ = runProjectRecord([]string{"--repo", root, "--kind", "note", "--title", "t", "--summary", "s", "--json"})
	_ = runProjectCommitSuggest([]string{"--repo", root, "--json"})
	_ = runProjectLintDiagnose([]string{"--repo", root, "--json"})

	_ = runState([]string{"unknown"})
	if err := runStateWrite([]string{"--key", "k", "--value", "v", "--json"}); err != nil {
		t.Fatalf("runStateWrite: %v", err)
	}
	_ = runStateRead([]string{"--key", "k", "--json"})
	_ = runStateList([]string{"--json"})
	_ = runStatePrune([]string{"--dry-run", "--json"})
	_ = runStateDoctor([]string{"--json"})
	_ = runStateMigrate([]string{"--json"})

	_ = runStatus([]string{"--repo", root, "--json"})
	_ = buildHarnessStatus(root)
	_ = runVerifyWork([]string{"--repo", root, "--json"})
	_ = buildVerifyWork(root, false, []string{"go", "test"})

	_ = runWorker([]string{"unknown"})
	_ = runWorkerEnqueue([]string{"--help"})
	_ = runWorkerDraftWiki([]string{"--help"})
	_ = runWorkerRun([]string{"--help"})
	_ = runWorkerStatus([]string{"--id", "missing", "--json"})
	_ = runWorkerList([]string{"--json"})
	_ = runWorkerCancel([]string{"--id", "missing", "--json"})
}

func TestSelfVerifyFacadeWrappers(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	t.Setenv("HARNESS_ROOT", t.TempDir())
	export := exportSelfVerificationCandidates()
	if export.Kind == "" {
		t.Fatalf("export = %#v", export)
	}
	catalog := selfVerificationCandidateCatalog()
	if len(catalog) == 0 {
		t.Fatal("self verification catalog empty")
	}
	_ = selfVerificationCandidateIDsByStatus(catalog, "open")
	_ = selectedSelfVerificationCandidateID(&catalog[0])
	_ = selectedSelfVerificationCandidateID(nil)
	if err := runSelfVerifyCandidatesWithDeps([]string{"--json"}, selfVerifyCandidatesDeps{
		export: func() SelfVerificationCandidateExportResult { return export },
		save:   func(*SelfVerificationCandidateExportResult, string) error { return nil },
	}); err != nil {
		t.Fatalf("runSelfVerifyCandidatesWithDeps: %v", err)
	}
	if err := saveSelfVerificationCandidateExport(&export, "candidate-export-test"); err != nil {
		t.Fatalf("saveSelfVerificationCandidateExport: %v", err)
	}
	baseline := SelfAugmentStateSnapshot{Kind: selfVerificationSummaryKind}
	candidate := SelfAugmentStateSnapshot{Kind: selfVerificationSummaryKind}
	_ = compareSelfAugmentSummariesFromSnapshots("base", "candidate", 10, baseline, candidate)
	_ = newSelfAugmentCompareResult("base", "candidate", 10)
	if _, err := compareSelfAugmentSummaries("missing-base", "missing-candidate", 10); err == nil {
		t.Fatal("missing summary compare should fail")
	}
	if _, err := selfAugmentHistory("", 1); err != nil {
		t.Fatalf("selfAugmentHistory: %v", err)
	}
	if err := runSelfVerifyPromoteWithDeps([]string{"--from-key", "a", "--baseline-key", "b", "--confirm"}, selfVerifyPromoteDeps{
		promote: func(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
			return SelfAugmentPromoteResult{OK: true, FromKey: fromKey, BaselineKey: baselineKey}, nil
		},
	}); err != nil {
		t.Fatalf("runSelfVerifyPromoteWithDeps: %v", err)
	}
	if _, err := promoteSelfAugmentBaseline("missing", "baseline", false, false); err == nil {
		t.Fatal("promote without confirm/missing source should fail")
	}
	if _, err := readSelfAugmentStateSnapshot("missing"); err == nil {
		t.Fatal("missing snapshot read should fail")
	}
	if !isSelfVerificationSummaryKind(selfVerificationSummaryKind) || boolPtr(true) == nil {
		t.Fatal("summary kind/boolPtr wrappers failed")
	}
	result := newSelfVerifyLoopResult(1, 100, 95)
	if result.Iterations != 1 {
		t.Fatalf("newSelfVerifyLoopResult = %#v", result)
	}
	emitSelfVerifyLoopStart(nil, "self-verify", 1, 100)
	emitSelfVerifyLoopEnd(nil, "self-verify", 1, 100, true, "")
	if err := saveSelfVerificationSummary(&result, "summary-test"); err != nil {
		t.Fatalf("saveSelfVerificationSummary: %v", err)
	}
	if err := saveSelfAugmentSummary(&result, "augment-summary-test"); err != nil {
		t.Fatalf("saveSelfAugmentSummary: %v", err)
	}
	_ = newSelfVerificationSummarySnapshot(result, time.Now())
	_ = plannedSelfVerifySteps(t.TempDir(), "", 100, nil)
	_ = cachedContractGoldenStep(StepResult{OK: false, Label: "go test"})
	_ = selfVerifyLoopDeps(false)
	_ = selfVerifyStepDeps()
	step := runCommandStepAdapter("", "adapter", time.Second, "", "sh", "-c", "printf ok")
	if !step.OK {
		t.Fatalf("runCommandStepAdapter = %#v", step)
	}
	_ = runSelfVerifyCandidates([]string{"--json"})
	_ = runSelfVerifyCompare([]string{"--baseline", "missing", "--candidate", "missing", "--json"})
	_ = runSelfVerifyHistory([]string{"--json", "--limit", "1"})
	_ = runSelfVerifyPromote([]string{"--from-key", "missing", "--baseline-key", "baseline"})
	_ = runSelfVerify([]string{"candidates", "--json"})
}

func qualityInspectDepsForHarnessAppTest() qualitycli.InspectDeps {
	return qualitycli.InspectDeps{
		Now:                  func() string { return "2026-01-01T00:00:00Z" },
		Coverage:             func(string) (string, error) { return "ok\tpkg\tcoverage: 100.0% of statements\n", nil },
		SelfAugmentOpenCount: func(string) (int, error) { return 0, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		CodeSNR: func(string) qualitycli.SNRResult {
			return qualitycli.SNRResult{SignalLines: 70, NoiseLines: 30, TotalLines: 100, Ratio: 0.7}
		},
	}
}
