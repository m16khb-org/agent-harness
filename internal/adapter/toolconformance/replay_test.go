package toolconformance_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	core "issueops/internal/adapter/toolconformance"
)

func TestReplayRegressionRejectsInvalidArgumentsBeforeHandlerAndPreservesState(t *testing.T) {
	descriptors := catalogDescriptors()
	fixtures, _, err := core.LoadManifest(descriptors)
	if err != nil {
		t.Fatal(err)
	}
	fixture := fixtures[0]
	arguments := map[string]any{"requireUnique": true}
	var schema map[string]any
	for _, descriptor := range descriptors {
		if descriptor.Name == fixture.SourceTool {
			schema = descriptor.InputSchema
			break
		}
	}
	classified, err := core.Classify(core.CallObservation{RawArguments: []byte(`{"requireUnique":true}`), CallCount: 1}, schema, fixture.ExpectedArguments)
	if err != nil {
		t.Fatal(err)
	}
	rawSHA := sha256.Sum256([]byte(`{"requireUnique":true}`))
	evidenceOne := sha256.Sum256([]byte("episode-1"))
	evidenceTwo := sha256.Sum256([]byte("episode-2"))
	diagnosticSignature := core.DiagnosticSignature(classified.Classification, classified.Diagnostics)
	regression := core.RegressionFixture{
		SchemaVersion:               1,
		FixtureID:                   fixture.ID,
		SourceTool:                  fixture.SourceTool,
		ProbeTool:                   fixture.ProbeTool,
		SourceSchemaSHA256:          fixture.SchemaSHA256,
		Host:                        "claude",
		HostVersion:                 "test",
		ModelLabel:                  "test",
		CanonicalArguments:          arguments,
		RawArgumentsSHA256:          fmt.Sprintf("%x", rawSHA),
		ExpectedClassification:      classified.Classification,
		ExpectedDiagnostics:         classified.Diagnostics,
		ExpectedDiagnosticSignature: diagnosticSignature,
		ConfirmedEvidenceIDs:        []string{fmt.Sprintf("%x", evidenceOne), fmt.Sprintf("%x", evidenceTwo)},
		ExpectedHandlerCallCount:    0,
		ExpectedFinalResult:         core.InvalidToolArgumentsResult(fixture.SourceTool, classified.Diagnostics),
		ExpectedStateUnchanged:      true,
	}
	replayed, err := core.ReplayRegression(regression, descriptors, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.OK || replayed.HandlerCalls != 0 || replayed.StateBeforeSHA256 != replayed.StateAfterSHA256 {
		t.Fatalf("replay=%+v", replayed)
	}
}

func TestRegressionRequiresTwoConfirmedEvidenceIDs(t *testing.T) {
	path := t.TempDir() + "/fixture.json"
	writeJSONFile(t, path, map[string]any{
		"schema_version":         1,
		"source_tool":            "contract_schema",
		"source_schema_sha256":   "sha",
		"confirmed_evidence_ids": []string{"one"},
	})
	if _, err := core.LoadRegressionFixture(path); err == nil {
		t.Fatal("unconfirmed regression fixture accepted")
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
