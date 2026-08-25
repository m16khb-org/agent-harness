package toolconformance

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseEpisodeStatusAcceptsKnownValuesOnly(t *testing.T) {
	for _, valid := range []EpisodeStatus{EpisodeCompleted, EpisodeIncomplete} {
		parsed, err := ParseEpisodeStatus(string(valid))
		if err != nil || parsed != valid {
			t.Fatalf("ParseEpisodeStatus(%q) = %q, %v", valid, parsed, err)
		}
	}
	if _, err := ParseEpisodeStatus("finished"); err == nil {
		t.Fatal("unknown episode status must fail")
	}
}

func TestEpisodeStatusUnmarshalJSONRejectsUnknownValues(t *testing.T) {
	var status EpisodeStatus
	if err := json.Unmarshal([]byte(`"completed"`), &status); err != nil {
		t.Fatalf("valid unmarshal failed: %v", err)
	}
	if status != EpisodeCompleted {
		t.Fatalf("status = %q, want completed", status)
	}
	if err := json.Unmarshal([]byte(`"aborted"`), &status); err == nil {
		t.Fatal("unknown enum value must fail unmarshal")
	}
	if err := json.Unmarshal([]byte(`42`), &status); err == nil {
		t.Fatal("non-string episode status must fail unmarshal")
	}
}

func TestGateDecisionsRoundTripThroughJSON(t *testing.T) {
	valid := []GateDecision{
		GateBaselinePassed, GateInconclusive, GateDeferHardening,
		GateNeedsReproduction, GateAuthorizeHardening, GateUnreproducedObservation,
	}
	for _, decision := range valid {
		if !ValidGateDecision(decision) {
			t.Fatalf("%q must be a valid gate decision", decision)
		}
		raw, err := json.Marshal(decision)
		if err != nil {
			t.Fatalf("marshal %q failed: %v", decision, err)
		}
		var decoded GateDecision
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal %q failed: %v", decision, err)
		}
		if decoded != decision {
			t.Fatalf("gate decision round trip drift: %q vs %q", decoded, decision)
		}
	}
	if ValidGateDecision(GateDecision("auto_pass")) {
		t.Fatal("unknown gate decision must be invalid")
	}
	var decoded GateDecision
	if err := json.Unmarshal([]byte(`"auto_pass"`), &decoded); err == nil {
		t.Fatal("unknown gate decision must fail unmarshal")
	} else if !strings.Contains(err.Error(), "unknown gate decision") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClassificationsCoverAllContractCases(t *testing.T) {
	all := []Classification{
		ExactValid, ValidButSemanticallyDifferent, UnknownKey, CoercibleTypeDrift,
		NoncoercibleTypeDrift, InvalidJSON, NoCall, MultipleCalls, MissingRequired, EnumMismatch,
	}
	for _, classification := range all {
		parsed, err := ParseClassification(string(classification))
		if err != nil || parsed != classification {
			t.Fatalf("ParseClassification(%q) = %q, %v", classification, parsed, err)
		}
	}
	if _, err := ParseClassification("looks_fine"); err == nil {
		t.Fatal("unknown classification must fail")
	}
	var value Classification
	if err := json.Unmarshal([]byte(`"no_call"`), &value); err != nil || value != NoCall {
		t.Fatalf("classification unmarshal mismatch: %q, %v", value, err)
	}
}

func TestBenchmarkReportJSONRoundTripPreservesTypedEnums(t *testing.T) {
	report := BenchmarkReport{
		OK:            true,
		SchemaVersion: ReportSchemaVersion,
		RunID:         "run-1",
		Profile:       "codex-default",
		CaseCount:     1,
		Gate: GateReport{
			Decision:           GateBaselinePassed,
			ConfirmedSignature: "sig-1",
			ConfirmedCount:     1,
		},
		Counts: BenchmarkCounts{Attempts: 1, Completed: 1},
		Hosts: []HostReport{{
			Host:              "codex",
			AttemptCount:      1,
			CompletedEpisodes: 1,
			Cases: []EpisodeReport{{
				Status:         EpisodeCompleted,
				FixtureID:      "fixture-1",
				Classification: ExactValid,
				AdvertisedValid: true,
				CanonicalValid:  true,
			}},
		}},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded BenchmarkReport
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !decoded.OK || decoded.SchemaVersion != FixtureManifestVersion && decoded.SchemaVersion != ReportSchemaVersion {
		t.Fatalf("scalar fields drifted: %+v", decoded)
	}
	if decoded.Gate.Decision != GateBaselinePassed || decoded.Gate.ConfirmedCount != 1 {
		t.Fatalf("gate round trip drifted: %+v", decoded.Gate)
	}
	host := decoded.Hosts[0]
	if host.Cases[0].Status != EpisodeCompleted || host.Cases[0].Classification != ExactValid {
		t.Fatalf("typed enums drifted: %+v", host.Cases[0])
	}
}

func TestFixtureAndReplayContractsKeepRequiredFields(t *testing.T) {
	fixture := Fixture{
		ID:                "f-1",
		ProbeTool:         "probe_shell",
		SourceTool:        "shell",
		SchemaSHA256:      "abc",
		ExpectedArguments: map[string]any{"command": "ls"},
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture failed: %v", err)
	}
	var decodedFixture Fixture
	if err := json.Unmarshal(raw, &decodedFixture); err != nil {
		t.Fatalf("unmarshal fixture failed: %v", err)
	}
	if decodedFixture.ID != "f-1" || decodedFixture.ExpectedArguments["command"] != "ls" {
		t.Fatalf("fixture round trip drifted: %+v", decodedFixture)
	}

	replay := ReplayResult{
		OK:             true,
		Classification: MultipleCalls,
		Diagnostics:    []Diagnostic{{Path: "$", Code: MultipleCalls, Expected: "1", Actual: "2"}},
		HandlerCalls:   2,
	}
	rawReplay, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("marshal replay failed: %v", err)
	}
	var decodedReplay ReplayResult
	if err := json.Unmarshal(rawReplay, &decodedReplay); err != nil {
		t.Fatalf("unmarshal replay failed: %v", err)
	}
	if decodedReplay.Classification != MultipleCalls || decodedReplay.HandlerCalls != 2 ||
		len(decodedReplay.Diagnostics) != 1 || decodedReplay.Diagnostics[0].Code != MultipleCalls {
		t.Fatalf("replay round trip drifted: %+v", decodedReplay)
	}
}
