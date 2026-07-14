package toolconformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const regressionFixtureLimit = 64 << 10

type RegressionFixture struct {
	SchemaVersion               int            `json:"schema_version"`
	FixtureID                   string         `json:"fixture_id"`
	SourceTool                  string         `json:"source_tool"`
	ProbeTool                   string         `json:"probe_tool"`
	SourceSchemaSHA256          string         `json:"source_schema_sha256"`
	Host                        string         `json:"host"`
	HostVersion                 string         `json:"host_version"`
	ModelLabel                  string         `json:"model_label"`
	CanonicalArguments          any            `json:"canonical_arguments"`
	RawArgumentsSHA256          string         `json:"raw_arguments_sha256"`
	ExpectedClassification      Classification `json:"expected_classification"`
	ExpectedDiagnostics         []Diagnostic   `json:"expected_diagnostics"`
	ExpectedDiagnosticSignature string         `json:"expected_diagnostic_signature"`
	ConfirmedEvidenceIDs        []string       `json:"confirmed_evidence_ids"`
	ExpectedHandlerCallCount    int            `json:"expected_handler_call_count"`
	ExpectedFinalResult         map[string]any `json:"expected_final_result"`
	ExpectedStateUnchanged      bool           `json:"expected_state_unchanged"`
}

type ReplayResult struct {
	OK                  bool           `json:"ok"`
	Classification      Classification `json:"classification"`
	Diagnostics         []Diagnostic   `json:"diagnostics"`
	DiagnosticSignature string         `json:"diagnostic_signature"`
	HandlerCalls        int            `json:"handler_calls"`
	FinalResult         map[string]any `json:"final_result"`
	StateBeforeSHA256   string         `json:"state_before_sha256"`
	StateAfterSHA256    string         `json:"state_after_sha256"`
}

func LoadRegressionFixture(path string) (RegressionFixture, error) {
	file, err := os.Open(path)
	if err != nil {
		return RegressionFixture{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, regressionFixtureLimit+1))
	if err != nil {
		return RegressionFixture{}, err
	}
	if len(data) > regressionFixtureLimit {
		return RegressionFixture{}, fmt.Errorf("regression_fixture_too_large")
	}
	var fixture RegressionFixture
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return RegressionFixture{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return RegressionFixture{}, fmt.Errorf("regression_fixture_trailing_json")
	}
	if err := validateRegressionFixture(fixture); err != nil {
		return RegressionFixture{}, err
	}
	return fixture, nil
}

func validateRegressionFixture(fixture RegressionFixture) error {
	if fixture.SchemaVersion != 1 {
		return fmt.Errorf("unsupported_regression_schema_version:%d", fixture.SchemaVersion)
	}
	if fixture.FixtureID == "" || fixture.SourceTool == "" || fixture.ProbeTool == "" || fixture.Host == "" ||
		fixture.HostVersion == "" || fixture.ModelLabel == "" || fixture.CanonicalArguments == nil {
		return fmt.Errorf("invalid_regression_fixture")
	}
	if !validEvidenceID(fixture.SourceSchemaSHA256) || !validEvidenceID(fixture.RawArgumentsSHA256) {
		return fmt.Errorf("invalid_regression_fixture_digest")
	}
	if !schemaDriftClassification(fixture.ExpectedClassification) {
		return fmt.Errorf("regression_classification_not_drift")
	}
	diagnostics := append([]Diagnostic(nil), fixture.ExpectedDiagnostics...)
	sortDiagnostics(diagnostics)
	if !jsonDeepEqual(diagnostics, fixture.ExpectedDiagnostics) ||
		fixture.ExpectedDiagnosticSignature != DiagnosticSignature(fixture.ExpectedClassification, diagnostics) {
		return fmt.Errorf("regression_diagnostic_signature_mismatch")
	}
	distinctEvidence := map[string]bool{}
	for _, id := range fixture.ConfirmedEvidenceIDs {
		if !validEvidenceID(id) {
			return fmt.Errorf("invalid_regression_evidence_id")
		}
		distinctEvidence[id] = true
	}
	if len(distinctEvidence) < 2 {
		return fmt.Errorf("regression_not_confirmed")
	}
	if fixture.ExpectedHandlerCallCount != 0 || !fixture.ExpectedStateUnchanged {
		return fmt.Errorf("invalid_regression_behavioral_expectation")
	}
	expectedFinal := InvalidToolArgumentsResult(fixture.SourceTool, diagnostics)
	if !jsonDeepEqual(fixture.ExpectedFinalResult, expectedFinal) {
		return fmt.Errorf("invalid_regression_final_result")
	}
	return nil
}

func ReplayRegression(fixture RegressionFixture, descriptors []ToolDescriptor, stateDir string) (ReplayResult, error) {
	if err := validateRegressionFixture(fixture); err != nil {
		return ReplayResult{}, err
	}
	var descriptor *ToolDescriptor
	for index := range descriptors {
		if descriptors[index].Name == fixture.SourceTool {
			descriptor = &descriptors[index]
			break
		}
	}
	if descriptor == nil {
		return ReplayResult{}, fmt.Errorf("source_tool_not_found:%s", fixture.SourceTool)
	}
	fixtures, _, err := LoadManifest(descriptors)
	if err != nil {
		return ReplayResult{}, err
	}
	var sourceFixture *Fixture
	for index := range fixtures {
		if fixtures[index].ID == fixture.FixtureID {
			sourceFixture = &fixtures[index]
			break
		}
	}
	if sourceFixture == nil || sourceFixture.SourceTool != fixture.SourceTool || sourceFixture.ProbeTool != fixture.ProbeTool {
		return ReplayResult{}, fmt.Errorf("regression_fixture_identity_mismatch")
	}
	actualSHA, err := CanonicalSchemaSHA256(descriptor.InputSchema)
	if err != nil {
		return ReplayResult{}, err
	}
	if actualSHA != fixture.SourceSchemaSHA256 || actualSHA != sourceFixture.SchemaSHA256 {
		return ReplayResult{}, fmt.Errorf("source_schema_hash_mismatch:%s", fixture.SourceTool)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return ReplayResult{}, err
	}
	statePath := filepath.Join(stateDir, "state.json")
	state := []byte(`{"stable":true}`)
	if err := os.WriteFile(statePath, state, 0o600); err != nil {
		return ReplayResult{}, err
	}
	before := sha256.Sum256(state)
	raw, err := json.Marshal(fixture.CanonicalArguments)
	if err != nil {
		return ReplayResult{}, err
	}
	result, err := Classify(CallObservation{RawArguments: raw, CallCount: 1}, descriptor.InputSchema, sourceFixture.ExpectedArguments)
	if err != nil {
		return ReplayResult{}, err
	}
	handlerCalls := 0
	fakeHandler := func() map[string]any {
		handlerCalls++
		_ = os.WriteFile(statePath, []byte(`{"stable":false}`), 0o600)
		return map[string]any{"ok": true}
	}
	finalResult := InvalidToolArgumentsResult(fixture.SourceTool, result.Diagnostics)
	if result.CanonicalValid {
		finalResult = fakeHandler()
	}
	afterState, err := os.ReadFile(statePath)
	if err != nil {
		return ReplayResult{}, err
	}
	after := sha256.Sum256(afterState)
	replay := ReplayResult{
		Classification:      result.Classification,
		Diagnostics:         result.Diagnostics,
		DiagnosticSignature: DiagnosticSignature(result.Classification, result.Diagnostics),
		HandlerCalls:        handlerCalls,
		FinalResult:         finalResult,
		StateBeforeSHA256:   hex.EncodeToString(before[:]),
		StateAfterSHA256:    hex.EncodeToString(after[:]),
	}
	replay.OK = replay.Classification == fixture.ExpectedClassification &&
		replay.DiagnosticSignature == fixture.ExpectedDiagnosticSignature &&
		replay.HandlerCalls == fixture.ExpectedHandlerCallCount &&
		(!fixture.ExpectedStateUnchanged || replay.StateBeforeSHA256 == replay.StateAfterSHA256) &&
		jsonDeepEqual(replay.FinalResult, fixture.ExpectedFinalResult)
	return replay, nil
}

func InvalidToolArgumentsResult(tool string, diagnostics []Diagnostic) map[string]any {
	return map[string]any{
		"ok":      false,
		"isError": true,
		"error": map[string]any{
			"code":        "invalid_tool_arguments",
			"tool":        tool,
			"diagnostics": append([]Diagnostic(nil), diagnostics...),
		},
	}
}

func jsonDeepEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}
