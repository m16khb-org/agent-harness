package contractcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mcpadapter "agent-harness/internal/adapter/mcp"
	"agent-harness/internal/core/toolconformance"
)

type LiveRequest struct {
	Hosts              []string
	Models             []string
	Profile            string
	Only               string
	ResumeReport       string
	TargetCompleted    int
	MaxAttemptsPerCase int
	EvidenceDir        string
	Previous           *toolconformance.BenchmarkReport
}

type ReplayOutcome struct {
	HandlerCalls      int
	StateBeforeSHA256 string
	StateAfterSHA256  string
	Classification    toolconformance.Classification
	Diagnostics       []toolconformance.Diagnostic
	FinalResult       map[string]any
}

type ConformanceDependencies struct {
	Catalog          func() []mcpadapter.Tool
	Root             func() string
	RunProcess       func(context.Context, LiveRequest) (toolconformance.BenchmarkReport, error)
	EvaluateBaseline func() (caseCount int, ok bool, err error)
	Replay           func(context.Context, string, string) (ReplayOutcome, error)
}

var conformanceDependencies ConformanceDependencies

func init() { conformanceDependencies = defaultConformanceDependencies() }

func defaultConformanceDependencies() ConformanceDependencies {
	return ConformanceDependencies{
		Catalog: mcpadapter.AdvertisedTools,
		Root: func() string {
			root, err := os.Getwd()
			if err != nil {
				return "."
			}
			return root
		},
		RunProcess: func(context.Context, LiveRequest) (toolconformance.BenchmarkReport, error) {
			return toolconformance.BenchmarkReport{}, fmt.Errorf("live_runner_unavailable")
		},
		EvaluateBaseline: evaluateSyntheticBaseline,
		Replay: func(_ context.Context, fixturePath, stateDir string) (ReplayOutcome, error) {
			fixture, err := toolconformance.LoadRegressionFixture(fixturePath)
			if err != nil {
				return ReplayOutcome{}, err
			}
			replayed, err := toolconformance.ReplayRegression(fixture, conformanceDescriptors(), stateDir)
			if err != nil {
				return ReplayOutcome{}, err
			}
			if !replayed.OK {
				return ReplayOutcome{}, fmt.Errorf("replay_expectation_failed")
			}
			return ReplayOutcome{
				HandlerCalls: replayed.HandlerCalls, StateBeforeSHA256: replayed.StateBeforeSHA256, StateAfterSHA256: replayed.StateAfterSHA256,
				Classification: replayed.Classification, Diagnostics: replayed.Diagnostics, FinalResult: replayed.FinalResult,
			}, nil
		},
	}
}

// ConfigureConformance injects harness-owned dependencies without exposing the
// capture server to the production MCP catalog. The restore function is used by
// focused tests; application wiring intentionally retains configured values.
func ConfigureConformance(overrides ConformanceDependencies) func() {
	previous := conformanceDependencies
	if overrides.Catalog != nil {
		conformanceDependencies.Catalog = overrides.Catalog
	}
	if overrides.Root != nil {
		conformanceDependencies.Root = overrides.Root
	}
	if overrides.RunProcess != nil {
		conformanceDependencies.RunProcess = overrides.RunProcess
	}
	if overrides.EvaluateBaseline != nil {
		conformanceDependencies.EvaluateBaseline = overrides.EvaluateBaseline
	}
	if overrides.Replay != nil {
		conformanceDependencies.Replay = overrides.Replay
	}
	return func() { conformanceDependencies = previous }
}

func runConformance(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing conformance subcommand")
	}
	switch args[0] {
	case "baseline":
		return runConformanceBaseline(args[1:])
	case "live":
		return runConformanceLive(args[1:])
	case "replay":
		return runConformanceReplay(args[1:])
	case "serve":
		return runConformanceServe(args[1:])
	default:
		return fmt.Errorf("unknown conformance subcommand %q", args[0])
	}
}

func conformanceDescriptors() []toolconformance.ToolDescriptor {
	out := []toolconformance.ToolDescriptor{}
	for _, t := range conformanceDependencies.Catalog() {
		out = append(out, toolconformance.ToolDescriptor{Name: t.Name, InputSchema: t.InputSchema})
	}
	return out
}

func evaluateSyntheticBaseline() (int, bool, error) {
	fixtures, cases, err := toolconformance.LoadManifest(conformanceDescriptors())
	if err != nil {
		return 0, false, err
	}
	byID := map[string]toolconformance.Fixture{}
	schema := map[string]map[string]any{}
	byTool := map[string]map[string]any{}
	for _, d := range conformanceDescriptors() {
		byTool[d.Name] = d.InputSchema
	}
	for _, f := range fixtures {
		byID[f.ID] = f
		schema[f.ID] = byTool[f.SourceTool]
	}
	ok := true
	for _, c := range cases {
		raw, marshalErr := json.Marshal(c.Arguments)
		got, classifyErr := toolconformance.Classify(toolconformance.CallObservation{RawArguments: raw, CallCount: 1}, schema[c.FixtureID], byID[c.FixtureID].ExpectedArguments)
		ok = ok && marshalErr == nil && classifyErr == nil && got.Classification == c.ExpectedClassification
	}
	return len(cases), ok, nil
}

func runConformanceBaseline(args []string) error {
	fs := flag.NewFlagSet("contract conformance baseline", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report, err := evaluateBaselineReport()
	if err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("cases=%d ok=%t\n", report.CaseCount, report.OK)
	}
	if !report.OK {
		return fmt.Errorf("baseline_failed")
	}
	return nil
}

func evaluateBaselineReport() (toolconformance.BenchmarkReport, error) {
	caseCount, ok, err := conformanceDependencies.EvaluateBaseline()
	if err != nil {
		return toolconformance.BenchmarkReport{}, err
	}
	regressions, err := regressionFixtures(regressionDirectory(conformanceDependencies.Root()))
	if err != nil {
		return toolconformance.BenchmarkReport{}, err
	}
	for _, fixture := range regressions {
		outcome, replayErr := replayFixture(fixture)
		caseCount++
		ok = ok && replayErr == nil && outcome.HandlerCalls == 0 && outcome.StateBeforeSHA256 != "" && outcome.StateBeforeSHA256 == outcome.StateAfterSHA256
	}
	decision := toolconformance.GateBaselinePassed
	if !ok {
		decision = toolconformance.GateInconclusive
	}
	return toolconformance.BenchmarkReport{
		OK: ok, SchemaVersion: toolconformance.ReportSchemaVersion, RunID: "deterministic-baseline",
		Profile: "deterministic", CaseCount: caseCount,
		Counts: toolconformance.BenchmarkCounts{Attempts: caseCount, Completed: caseCount},
		Gate:   toolconformance.GateReport{Decision: decision},
		Hosts:  []toolconformance.HostReport{}, Warnings: []string{},
	}, nil
}

func regressionDirectory(root string) string {
	return filepath.Join(root, "internal", "core", "toolconformance", "testdata", "regressions")
}

func regressionFixtures(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fixtures := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			fixtures = append(fixtures, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(fixtures)
	return fixtures, nil
}

func runConformanceLive(args []string) error {
	fs := flag.NewFlagSet("contract conformance live", flag.ContinueOnError)
	hosts := fs.String("hosts", "codex,claude", "comma-separated hosts")
	profile := fs.String("profile", "clean", "clean or context-pressure")
	only := fs.String("only", "", "host:fixture")
	resume := fs.String("resume-report", "", "previous report")
	target := fs.Int("target-completed", 1, "1, 10, or 20")
	maxAttempts := fs.Int("max-attempts-per-case", 3, "1 through 3")
	evidenceDir := fs.String("evidence-dir", ".agent-harness/evidence/tool-conformance", "ignored evidence directory")
	jsonOut := fs.Bool("json", false, "print JSON")
	var models stringSlice
	fs.Var(&models, "model", "host=value")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profile != "clean" && *profile != "context-pressure" {
		return fmt.Errorf("invalid profile %q", *profile)
	}
	if *target != 1 && *target != 10 && *target != 20 {
		return fmt.Errorf("invalid target-completed %d", *target)
	}
	if *maxAttempts < 1 || *maxAttempts > 3 {
		return fmt.Errorf("invalid max-attempts-per-case %d", *maxAttempts)
	}
	request := LiveRequest{
		Hosts: splitNonEmpty(*hosts), Models: models, Profile: *profile,
		Only: *only, ResumeReport: *resume, TargetCompleted: *target,
		MaxAttemptsPerCase: *maxAttempts, EvidenceDir: *evidenceDir,
	}
	if len(request.Hosts) == 0 {
		return fmt.Errorf("hosts_required")
	}
	if os.Getenv("HARNESS_TOOL_CONFORMANCE_LIVE") != "1" {
		return fmt.Errorf("live_opt_in_required")
	}
	baseline, err := evaluateBaselineReport()
	if err != nil || !baseline.OK {
		return fmt.Errorf("baseline_failed_before_live")
	}
	if request.ResumeReport != "" {
		previous, loadErr := loadBenchmarkReport(request.ResumeReport)
		if loadErr != nil {
			return loadErr
		}
		request.Previous = &previous
	}
	report, err := conformanceDependencies.RunProcess(context.Background(), request)
	if err != nil {
		return err
	}
	if err := persistLiveReport(&report, request); err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("gate=%s completed=%d attempts=%d report=%s\n", report.Gate.Decision, report.Counts.Completed, report.Counts.Attempts, report.Evidence.ReportPath)
	}
	if !report.OK {
		return fmt.Errorf("conformance_inconclusive")
	}
	return nil
}

func loadBenchmarkReport(path string) (toolconformance.BenchmarkReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return toolconformance.BenchmarkReport{}, err
	}
	var report toolconformance.BenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		return toolconformance.BenchmarkReport{}, err
	}
	if report.SchemaVersion != toolconformance.ReportSchemaVersion {
		return toolconformance.BenchmarkReport{}, fmt.Errorf("unsupported report schema version %d", report.SchemaVersion)
	}
	return report, nil
}

func persistLiveReport(report *toolconformance.BenchmarkReport, request LiveRequest) error {
	root := conformanceDependencies.Root()
	evidenceRoot := request.EvidenceDir
	if !filepath.IsAbs(evidenceRoot) {
		evidenceRoot = filepath.Join(root, evidenceRoot)
	}
	relative, err := filepath.Rel(root, evidenceRoot)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence_dir_outside_harness_root")
	}
	runDir := filepath.Join(evidenceRoot, safeRunID(report.RunID))
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(runDir, 0o700); err != nil {
		return err
	}
	relativeRunDir := filepath.Join(relative, safeRunID(report.RunID))
	report.Evidence.ReportPath = filepath.Join(relativeRunDir, "report.json")
	if report.Gate.Decision == toolconformance.GateAuthorizeHardening {
		candidate, tracked, buildErr := buildCandidateRegression(*report)
		if buildErr != nil {
			return buildErr
		}
		candidatePath := filepath.Join(runDir, "candidate-regression.json")
		if err := writePrivateJSONFile(candidatePath, candidate); err != nil {
			return err
		}
		report.Evidence.CandidateRegressionPath = filepath.Join(relativeRunDir, "candidate-regression.json")
		report.Evidence.TrackedFixturePath = tracked
	}
	return writePrivateJSONFile(filepath.Join(runDir, "report.json"), report)
}

func safeRunID(value string) string {
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out.WriteRune(r)
		}
	}
	if out.Len() == 0 {
		return "run"
	}
	return out.String()
}

func writePrivateJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(parent, ".report-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func buildCandidateRegression(report toolconformance.BenchmarkReport) (toolconformance.RegressionFixture, string, error) {
	signature := report.Gate.ConfirmedSignature
	groups := map[string][]toolconformance.EpisodeReport{}
	keys := []string{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			if episode.Status != "completed" || episode.DiagnosticSignature != signature {
				continue
			}
			key := host.Host + "\x00" + episode.FixtureID
			if _, exists := groups[key]; !exists {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], episode)
		}
	}
	sort.Strings(keys)
	matches := []toolconformance.EpisodeReport{}
	for _, key := range keys {
		if len(groups[key]) >= 2 {
			matches = groups[key]
			break
		}
	}
	if len(matches) < 2 {
		return toolconformance.RegressionFixture{}, "", fmt.Errorf("confirmed_signature_evidence_missing")
	}
	fixtures, _, err := toolconformance.LoadManifest(conformanceDescriptors())
	if err != nil {
		return toolconformance.RegressionFixture{}, "", err
	}
	var fixture toolconformance.Fixture
	for _, candidate := range fixtures {
		if candidate.ID == matches[0].FixtureID {
			fixture = candidate
			break
		}
	}
	if fixture.ID == "" {
		return toolconformance.RegressionFixture{}, "", fmt.Errorf("confirmed_fixture_missing")
	}
	evidenceIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		if !validSHA256Text(match.EvidenceID) {
			return toolconformance.RegressionFixture{}, "", fmt.Errorf("confirmed_evidence_id_invalid")
		}
		evidenceIDs = append(evidenceIDs, match.EvidenceID)
	}
	sort.Strings(evidenceIDs)
	modelLabel := matches[0].ObservedModel
	if modelLabel == "" {
		modelLabel = matches[0].RequestedModel
	}
	regression := toolconformance.RegressionFixture{
		SchemaVersion: 1, FixtureID: fixture.ID, SourceTool: fixture.SourceTool, ProbeTool: fixture.ProbeTool,
		SourceSchemaSHA256: fixture.SchemaSHA256, Host: matches[0].Host,
		HostVersion: matches[0].HostVersion, ModelLabel: modelLabel,
		CanonicalArguments: matches[0].CanonicalArguments, RawArgumentsSHA256: matches[0].RawArgumentsSHA256,
		ExpectedClassification: matches[0].Classification, ExpectedDiagnostics: matches[0].Diagnostics,
		ExpectedDiagnosticSignature: signature, ConfirmedEvidenceIDs: evidenceIDs,
		ExpectedHandlerCallCount: 0,
		ExpectedFinalResult:      toolconformance.InvalidToolArgumentsResult(fixture.SourceTool, matches[0].Diagnostics),
		ExpectedStateUnchanged:   true,
	}
	name := matches[0].Host + "-" + fixture.ID + "-" + firstN(signature, 12) + ".json"
	tracked := filepath.Join("internal", "core", "toolconformance", "testdata", "regressions", name)
	return regression, tracked, nil
}

func validSHA256Text(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func firstN(value string, count int) string {
	if len(value) <= count {
		return value
	}
	return value[:count]
}

func runConformanceReplay(args []string) error {
	fs := flag.NewFlagSet("contract conformance replay", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	fixture := fs.String("fixture", "", "fixture")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fixture == "" {
		return fmt.Errorf("fixture_required")
	}
	outcome, err := replayFixture(*fixture)
	if err != nil {
		return err
	}
	ok := outcome.HandlerCalls == 0 && outcome.StateBeforeSHA256 != "" && outcome.StateBeforeSHA256 == outcome.StateAfterSHA256
	result := map[string]any{
		"ok": ok, "classification": outcome.Classification, "diagnostics": outcome.Diagnostics,
		"handler_calls": outcome.HandlerCalls, "final_result": outcome.FinalResult,
		"state_digest": outcome.StateAfterSHA256,
	}
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf("replay_failed")
	}
	return nil
}

func replayFixture(fixture string) (ReplayOutcome, error) {
	stateDir, err := os.MkdirTemp("", "agent-harness-conformance-replay-")
	if err != nil {
		return ReplayOutcome{}, err
	}
	defer os.RemoveAll(stateDir)
	if err := os.Chmod(stateDir, 0o700); err != nil {
		return ReplayOutcome{}, err
	}
	return conformanceDependencies.Replay(context.Background(), fixture, stateDir)
}

func runConformanceServe(args []string) error {
	fs := flag.NewFlagSet("contract conformance serve", flag.ContinueOnError)
	id := fs.String("fixture-id", "", "fixture id")
	path := fs.String("result-file", "", "result file")
	token := fs.String("run-token", "", "run token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *path == "" || *token == "" {
		return fmt.Errorf("fixture-id, result-file, and run-token are required")
	}
	fixtures, _, err := toolconformance.LoadManifest(conformanceDescriptors())
	if err != nil {
		return err
	}
	for _, f := range fixtures {
		if f.ID == *id {
			return mcpadapter.ServeConformanceProbe(context.Background(), os.Stdin, os.Stdout, mcpadapter.ConformanceProbeConfig{FixtureID: f.ID, ProbeTool: f.ProbeTool, Schema: sourceSchema(f.SourceTool), SchemaSHA: f.SchemaSHA256, ExpectedArguments: f.ExpectedArguments, ResultPath: *path, RunToken: *token})
		}
	}
	return fmt.Errorf("unknown fixture %s", *id)
}

func sourceSchema(name string) map[string]any {
	for _, t := range conformanceDependencies.Catalog() {
		if t.Name == name {
			copy, err := cloneJSONMap(t.InputSchema)
			if err != nil {
				return nil
			}
			return copy
		}
	}
	return nil
}

func cloneJSONMap(value map[string]any) (map[string]any, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var copy map[string]any
	if err := json.Unmarshal(b, &copy); err != nil {
		return nil, err
	}
	return copy, nil
}

type stringSlice []string

func (values *stringSlice) String() string { return strings.Join(*values, ",") }
func (values *stringSlice) Set(value string) error {
	if value == "" {
		return fmt.Errorf("empty value")
	}
	*values = append(*values, value)
	return nil
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
