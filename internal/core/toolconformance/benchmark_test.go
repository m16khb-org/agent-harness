package toolconformance_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	core "agent-harness/internal/core/toolconformance"
	"agent-harness/internal/port"
)

type fakeProbeRunner struct {
	host      string
	fixtures  map[string]core.Fixture
	responses map[string][]map[string]any
	failCode  string
	calls     map[string]int
}

func (f *fakeProbeRunner) Name() string { return f.host }

func (f *fakeProbeRunner) Preflight(context.Context, port.HostProbeRequest) port.HostProbePreflight {
	return port.HostProbePreflight{Ready: true, Host: f.host, Version: f.host + "-1", RequestedModel: "default", ObservedModel: "default"}
}

func (f *fakeProbeRunner) Run(_ context.Context, request port.HostProbeRequest) port.HostProbeResult {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	index := f.calls[request.FixtureID]
	f.calls[request.FixtureID]++
	if f.failCode != "" {
		return port.HostProbeResult{Host: f.host, HostVersion: f.host + "-1", RequestedModel: request.Model, ObservedModel: request.Model, FixtureID: request.FixtureID, Profile: request.Profile, Attempt: request.Attempt, Cause: "transport", Code: f.failCode, EvidenceSource: f.host + "_runner"}
	}
	arguments := f.fixtures[request.FixtureID].ExpectedArguments
	if values := f.responses[request.FixtureID]; index < len(values) {
		arguments = values[index]
	}
	encoded, _ := json.Marshal(arguments)
	rawSHA := sha256.Sum256(encoded)
	evidenceSHA := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d", f.host, request.FixtureID, request.RunToken, request.Attempt)))
	classification := core.Classification(core.ExactValid)
	advertisedValid, canonicalValid := true, true
	diagnostics := []core.Diagnostic{}
	if _, exists := arguments["requireUnique"]; exists {
		classification = core.Classification(core.UnknownKey)
		canonicalValid = false
		diagnostics = []core.Diagnostic{{Path: "/requireUnique", Code: core.UnknownKey, Expected: "declared property", Actual: "boolean"}}
	}
	diagnosticsJSON, _ := json.Marshal(diagnostics)
	return port.HostProbeResult{
		Completed:              true,
		Host:                   f.host,
		HostVersion:            f.host + "-1",
		RequestedModel:         request.Model,
		ObservedModel:          request.Model,
		FixtureID:              request.FixtureID,
		SchemaSHA256:           request.SchemaSHA256,
		Profile:                request.Profile,
		Attempt:                request.Attempt,
		DurationMS:             1,
		AmbientToolCount:       1,
		CallCount:              1,
		RawArgumentsSHA256:     fmt.Sprintf("%x", rawSHA),
		CanonicalArgumentsJSON: string(encoded),
		EvidenceID:             fmt.Sprintf("%x", evidenceSHA),
		Classification:         string(classification),
		AdvertisedValid:        advertisedValid,
		CanonicalValid:         canonicalValid,
		DiagnosticsJSON:        string(diagnosticsJSON),
	}
}

func TestLiveGateNineExactEpisodesDeferHardening(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runners := map[string]port.HostProbeRunner{}
	for _, host := range []string{"codex", "claude", "gjc"} {
		runners[host] = &fakeProbeRunner{host: host, fixtures: fixtures, responses: map[string][]map[string]any{}}
	}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex", "claude", "gjc"}, Models: map[string]string{"gjc": "test-model"}, Profile: "clean", TargetCompleted: 1, MaxAttemptsPerCase: 3, HarnessBinary: "/harness", RunID: "exact",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: runners, Now: func() time.Time { return time.Unix(1, 0) }, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Gate.Decision != core.GateDeferHardening || report.Counts.Completed != 9 || report.Counts.ModelDenominator != 9 {
		t.Fatalf("report=%+v", report)
	}
}

func TestLiveGateIncompleteEpisodeFailsClosed(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, failCode: "probe_result_missing"}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "incomplete",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": runner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Gate.Decision != core.GateInconclusive || report.Counts.TransportFailures != 1 || report.Counts.ModelDenominator != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestLiveGateResumeConfirmsOnlyRepeatedDiagnosticSignature(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	invalid := cloneArguments(fixtures["empty_object"].ExpectedArguments)
	invalid["requireUnique"] = true
	initialRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	initial, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "initial",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": initialRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if initial.Gate.Decision != core.GateNeedsReproduction {
		t.Fatalf("initial gate=%s", initial.Gate.Decision)
	}
	reproductionRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	confirmed, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 10, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "confirmed", Previous: &initial,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": reproductionRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Gate.Decision != core.GateAuthorizeHardening || confirmed.Gate.ConfirmedCount != 2 || confirmed.Counts.Completed != 10 {
		t.Fatalf("confirmed=%+v", confirmed)
	}
}

func TestLiveGateResumeRejectsStaleSchemaEvidence(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	invalid := cloneArguments(fixtures["empty_object"].ExpectedArguments)
	invalid["requireUnique"] = true
	initialRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	initial, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "initial",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": initialRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	initial.Hosts[0].Cases[0].SchemaSHA256 = "stale"

	reproductionRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 10, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "reproduction", Previous: &initial,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": reproductionRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if got := reproductionRunner.calls["empty_object"]; got != 10 {
		t.Fatalf("fresh calls=%d want=10", got)
	}
	if report.Gate.Decision != core.GateNeedsReproduction || report.Gate.ConfirmedCount != 0 {
		t.Fatalf("stale evidence influenced gate: %+v", report.Gate)
	}
}

func TestContextPressureProfileIsFixedSizeAndHash(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	promptA, hashA := core.BuildEpisodePrompt(fixtures["empty_object"], "context-pressure")
	promptB, hashB := core.BuildEpisodePrompt(fixtures["empty_object"], "context-pressure")
	if promptA != promptB || hashA == "" || hashA != hashB {
		t.Fatalf("context profile is not deterministic")
	}
	clean, cleanHash := core.BuildEpisodePrompt(fixtures["empty_object"], "clean")
	if cleanHash != "" || len(promptA)-len(clean) < 32<<10 {
		t.Fatalf("context bytes=%d hash=%q", len(promptA)-len(clean), cleanHash)
	}
}

func benchmarkFixtures(t *testing.T) map[string]core.Fixture {
	t.Helper()
	fixtures, _, err := core.LoadManifest(catalogDescriptors())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]core.Fixture{}
	for _, fixture := range fixtures {
		out[fixture.ID] = fixture
	}
	return out
}
