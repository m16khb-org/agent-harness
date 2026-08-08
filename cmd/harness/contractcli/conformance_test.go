package contractcli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agent-harness/internal/adapter/toolconformance"
	mcpadapter "agent-harness/internal/domain/mcp"
)

func TestConformanceBaselineFailsWithJSONWhenInjectedCaseFails(t *testing.T) {
	restore := ConfigureConformance(ConformanceDependencies{
		EvaluateBaseline: func() (int, bool, error) { return 10, false, nil },
	})
	defer restore()
	if err := runConformanceBaseline([]string{"--json"}); err == nil || err.Error() != "baseline_failed" {
		t.Fatalf("err=%v", err)
	}
}

func TestConformanceBaselineRunsRegressionFixturesInDeterministicOrderAndAllowsAbsentDirectory(t *testing.T) {
	root := t.TempDir()
	dir := regressionDirectory(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"z-last.json", "a-first.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var calls []string
	restore := ConfigureConformance(ConformanceDependencies{
		Root: func() string { return root },
		Replay: func(_ context.Context, fixturePath, stateDir string) (ReplayOutcome, error) {
			calls = append(calls, filepath.Base(fixturePath))
			if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("unchanged"), 0o600); err != nil {
				return ReplayOutcome{}, err
			}
			digest := sha256.Sum256([]byte("unchanged"))
			return ReplayOutcome{HandlerCalls: 0, StateBeforeSHA256: fmt.Sprintf("%x", digest), StateAfterSHA256: fmt.Sprintf("%x", digest)}, nil
		},
	})
	defer restore()
	if err := runConformanceBaseline(nil); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a-first.json", "z-last.json"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	calls = nil
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := runConformanceBaseline(nil); err != nil {
		t.Fatalf("absent regression directory: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("absent directory replayed=%v", calls)
	}
}

func TestConformanceReplayUsesFakeHandlerAndUnchangedTemporaryStateDigest(t *testing.T) {
	called := 0
	restore := ConfigureConformance(ConformanceDependencies{
		Replay: func(_ context.Context, fixturePath, stateDir string) (ReplayOutcome, error) {
			called++
			if filepath.Base(fixturePath) != "fixture.json" {
				return ReplayOutcome{}, fmt.Errorf("unexpected fixture %s", fixturePath)
			}
			state := []byte("stable state")
			if err := os.WriteFile(filepath.Join(stateDir, "state.json"), state, 0o600); err != nil {
				return ReplayOutcome{}, err
			}
			digest := sha256.Sum256(state)
			return ReplayOutcome{HandlerCalls: 0, StateBeforeSHA256: fmt.Sprintf("%x", digest), StateAfterSHA256: fmt.Sprintf("%x", digest)}, nil
		},
	})
	defer restore()
	if err := runConformanceReplay([]string{"--fixture", "fixture.json", "--json"}); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("handler runs=%d", called)
	}
}

func TestConformanceLiveRequiresExplicitOptInBeforeInjectedProcess(t *testing.T) {
	old, had := os.LookupEnv("HARNESS_TOOL_CONFORMANCE_LIVE")
	defer func() {
		if had {
			_ = os.Setenv("HARNESS_TOOL_CONFORMANCE_LIVE", old)
		} else {
			_ = os.Unsetenv("HARNESS_TOOL_CONFORMANCE_LIVE")
		}
	}()
	processCalls := 0
	restore := ConfigureConformance(ConformanceDependencies{RunProcess: func(context.Context, LiveRequest) (toolconformance.BenchmarkReport, error) {
		processCalls++
		return toolconformance.BenchmarkReport{}, nil
	}})
	defer restore()
	_ = os.Unsetenv("HARNESS_TOOL_CONFORMANCE_LIVE")
	if err := runConformanceLive([]string{"--hosts", "codex", "--model", "codex=default", "--profile", "clean", "--target-completed", "1", "--max-attempts-per-case", "3"}); err == nil || err.Error() != "live_opt_in_required" {
		t.Fatalf("err=%v", err)
	}
	if processCalls != 0 {
		t.Fatalf("process calls=%d", processCalls)
	}
}

func TestConformanceLivePassesFullyParsedFlagsToInjectedProcessAfterOptIn(t *testing.T) {
	old, had := os.LookupEnv("HARNESS_TOOL_CONFORMANCE_LIVE")
	defer func() {
		if had {
			_ = os.Setenv("HARNESS_TOOL_CONFORMANCE_LIVE", old)
		} else {
			_ = os.Unsetenv("HARNESS_TOOL_CONFORMANCE_LIVE")
		}
	}()
	_ = os.Setenv("HARNESS_TOOL_CONFORMANCE_LIVE", "1")
	root := t.TempDir()
	prior := toolconformance.BenchmarkReport{
		OK: true, SchemaVersion: toolconformance.ReportSchemaVersion, RunID: "prior",
		Profile: "context-pressure", Gate: toolconformance.GateReport{Decision: toolconformance.GateNeedsReproduction},
		Hosts: []toolconformance.HostReport{}, Warnings: []string{},
	}
	priorPath := filepath.Join(root, "prior.json")
	if err := writePrivateJSONFile(priorPath, prior); err != nil {
		t.Fatal(err)
	}
	var got LiveRequest
	restore := ConfigureConformance(ConformanceDependencies{
		Root: func() string { return root },
		RunProcess: func(_ context.Context, request LiveRequest) (toolconformance.BenchmarkReport, error) {
			got = request
			return toolconformance.BenchmarkReport{
				OK: true, SchemaVersion: toolconformance.ReportSchemaVersion, RunID: "test-live",
				Profile: request.Profile, Gate: toolconformance.GateReport{Decision: toolconformance.GateDeferHardening},
				Hosts: []toolconformance.HostReport{}, Warnings: []string{},
			}, nil
		},
	})
	defer restore()
	args := []string{"--hosts", "codex,claude", "--model", "codex=default", "--model", "claude=test", "--profile", "context-pressure", "--only", "codex:empty_object", "--resume-report", priorPath, "--target-completed", "10", "--max-attempts-per-case", "2"}
	if err := runConformanceLive(args); err != nil {
		t.Fatal(err)
	}
	want := LiveRequest{Hosts: []string{"codex", "claude"}, Models: []string{"codex=default", "claude=test"}, Profile: "context-pressure", Only: "codex:empty_object", ResumeReport: priorPath, TargetCompleted: 10, MaxAttemptsPerCase: 2, EvidenceDir: ".agent-harness/evidence/tool-conformance", Previous: &prior}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestConformanceServeParsesRequiredFlags(t *testing.T) {
	if err := runConformanceServe(nil); err == nil {
		t.Fatal("serve missing flags accepted")
	}
	if err := runConformanceServe([]string{"--unknown"}); err == nil {
		t.Fatal("serve unknown flag accepted")
	}
}

func TestProductionCatalogDoesNotAdvertiseConformanceProbe(t *testing.T) {
	for _, tool := range mcpadapter.AdvertisedTools() {
		if len(tool.Name) >= len("harness_probe_") && tool.Name[:len("harness_probe_")] == "harness_probe_" {
			t.Fatalf("production catalog advertises probe %q", tool.Name)
		}
	}
}

func TestConformanceSourceSchemaCopiesConfiguredCatalogSource(t *testing.T) {
	source := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}}
	restore := ConfigureConformance(ConformanceDependencies{Catalog: func() []mcpadapter.Tool { return []mcpadapter.Tool{{Name: "source", InputSchema: source}} }})
	defer restore()
	copy := sourceSchema("source")
	copy["properties"].(map[string]any)["later"] = map[string]any{"type": "boolean"}
	if _, exists := source["properties"].(map[string]any)["later"]; exists {
		t.Fatalf("source schema mutated: %#v", source)
	}
}

func TestBuildCandidateRegressionRequiresRepeatedSignatureWithinOneHostFixture(t *testing.T) {
	signature := "0123456789abcdef"
	episode := func(host string, attempt int) toolconformance.EpisodeReport {
		return toolconformance.EpisodeReport{
			Status: "completed", Host: host, HostVersion: "test", ObservedModel: "test",
			FixtureID: "empty_object", Attempt: attempt, RawArgumentsSHA256: fmt.Sprintf("%064d", attempt), EvidenceID: fmt.Sprintf("%064x", attempt),
			Classification:      toolconformance.Classification(toolconformance.UnknownKey),
			Diagnostics:         []toolconformance.Diagnostic{{Code: toolconformance.UnknownKey, Path: "/requireUnique"}},
			DiagnosticSignature: signature,
			CanonicalArguments:  map[string]any{"requireUnique": true},
		}
	}
	report := toolconformance.BenchmarkReport{
		Gate: toolconformance.GateReport{Decision: toolconformance.GateAuthorizeHardening, ConfirmedSignature: signature, ConfirmedCount: 2},
		Hosts: []toolconformance.HostReport{
			{Host: "claude", Cases: []toolconformance.EpisodeReport{episode("claude", 1)}},
			{Host: "codex", Cases: []toolconformance.EpisodeReport{episode("codex", 1)}},
		},
	}
	if _, _, err := buildCandidateRegression(report); err == nil || err.Error() != "confirmed_signature_evidence_missing" {
		t.Fatalf("cross-target evidence accepted: %v", err)
	}

	report.Hosts[1].Cases = append(report.Hosts[1].Cases, episode("codex", 2))
	candidate, tracked, err := buildCandidateRegression(report)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Host != "codex" || len(candidate.ConfirmedEvidenceIDs) != 2 {
		t.Fatalf("candidate=%+v", candidate)
	}
	if want := "internal/adapter/toolconformance/testdata/regressions/codex-empty_object-0123456789ab.json"; tracked != want {
		t.Fatalf("tracked=%q want=%q", tracked, want)
	}
}
