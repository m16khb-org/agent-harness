package toolconformance

import (
	"encoding/json"
	"fmt"

	"issueops/internal/contract/failurecause"
)

const (
	FixtureManifestVersion        = 1
	ReportSchemaVersion           = 1
	ExactValid                    = "exact_valid"
	UnknownKey                    = "unknown_key"
	CoercibleTypeDrift            = "coercible_type_drift"
	NoncoercibleTypeDrift         = "noncoercible_type_drift"
	ValidButSemanticallyDifferent = "valid_but_semantically_different"
	InvalidJSON                   = "invalid_json"
	NoCall                        = "no_call"
	MultipleCalls                 = "multiple_calls"
	MissingRequired               = "missing_required"
	EnumMismatch                  = "enum_mismatch"
)

type ToolDescriptor struct {
	Name        string
	InputSchema map[string]any
}
type Diagnostic struct {
	Path     string `json:"path"`
	Code     string `json:"code"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}
type CallObservation struct {
	RawArguments []byte
	CallCount    int
}
type Classification string
type CaseResult struct {
	Classification  Classification `json:"classification"`
	AdvertisedValid bool           `json:"advertised_valid"`
	CanonicalValid  bool           `json:"canonical_valid"`
	Diagnostics     []Diagnostic   `json:"diagnostics"`
}
type BaselineCase struct {
	FixtureID              string         `json:"fixture_id"`
	PayloadClass           string         `json:"payload_class"`
	ExpectedClassification Classification `json:"expected_classification"`
	Arguments              map[string]any `json:"arguments"`
}
type GateDecision string

type EpisodeStatus string

const (
	EpisodeCompleted  EpisodeStatus = "completed"
	EpisodeIncomplete EpisodeStatus = "incomplete"
)

const (
	GateBaselinePassed          GateDecision = "baseline_passed"
	GateInconclusive            GateDecision = "inconclusive"
	GateDeferHardening          GateDecision = "defer_hardening"
	GateNeedsReproduction       GateDecision = "needs_reproduction"
	GateAuthorizeHardening      GateDecision = "authorize_hardening"
	GateUnreproducedObservation GateDecision = "unreproduced_observation"
)

type EpisodeReport struct {
	Status               EpisodeStatus           `json:"status"`
	Host                 string                  `json:"host"`
	HostVersion          string                  `json:"host_version"`
	RequestedModel       string                  `json:"requested_model"`
	ObservedModel        string                  `json:"observed_model"`
	FixtureID            string                  `json:"fixture_id"`
	SchemaSHA256         string                  `json:"schema_sha256"`
	Profile              string                  `json:"profile"`
	Attempt              int                     `json:"attempt"`
	DurationMS           int64                   `json:"duration_ms"`
	AmbientToolCount     int                     `json:"ambient_tool_count"`
	CallCount            int                     `json:"call_count"`
	RawArgumentsSHA256   string                  `json:"raw_arguments_sha256,omitempty"`
	EvidenceID           string                  `json:"evidence_id,omitempty"`
	CanonicalArguments   any                     `json:"canonical_arguments,omitempty"`
	Classification       Classification          `json:"classification,omitempty"`
	AdvertisedValid      bool                    `json:"advertised_valid"`
	CanonicalValid       bool                    `json:"canonical_valid"`
	Diagnostics          []Diagnostic            `json:"diagnostics"`
	DiagnosticSignature  string                  `json:"diagnostic_signature,omitempty"`
	FailureCause         failurecause.Cause      `json:"failure_cause"`
	FailureCauseReason   string                  `json:"failure_cause_reason"`
	FailureCauseEvidence []failurecause.Evidence `json:"failure_cause_evidence"`
}

type HostReport struct {
	Host              string          `json:"host"`
	Version           string          `json:"version"`
	RequestedModel    string          `json:"requested_model"`
	ObservedModel     string          `json:"observed_model"`
	AttemptCount      int             `json:"attempt_count"`
	CompletedEpisodes int             `json:"completed_episodes"`
	Cases             []EpisodeReport `json:"cases"`
}

type BenchmarkCounts struct {
	Attempts                 int `json:"attempts"`
	Completed                int `json:"completed"`
	ModelDenominator         int `json:"model_denominator"`
	EnvironmentFailures      int `json:"environment_failures"`
	TransportFailures        int `json:"transport_failures"`
	NoCalls                  int `json:"no_calls"`
	ValidSemanticDifferences int `json:"valid_semantic_differences"`
	SchemaDriftObservations  int `json:"schema_drift_observations"`
}

type GateReport struct {
	Decision               GateDecision `json:"decision"`
	NextReproductionTarget string       `json:"next_reproduction_target,omitempty"`
	ConfirmedSignature     string       `json:"confirmed_signature,omitempty"`
	ConfirmedCount         int          `json:"confirmed_count,omitempty"`
}

type BenchmarkEvidence struct {
	ReportPath              string `json:"report_path,omitempty"`
	CandidateRegressionPath string `json:"candidate_regression_path,omitempty"`
	TrackedFixturePath      string `json:"tracked_fixture_path,omitempty"`
}

type BenchmarkReport struct {
	OK            bool              `json:"ok"`
	SchemaVersion int               `json:"schema_version"`
	RunID         string            `json:"run_id"`
	Profile       string            `json:"profile"`
	ProfileSHA256 string            `json:"profile_sha256,omitempty"`
	CaseCount     int               `json:"case_count"`
	Counts        BenchmarkCounts   `json:"counts"`
	Gate          GateReport        `json:"gate"`
	Hosts         []HostReport      `json:"hosts"`
	Warnings      []string          `json:"warnings"`
	Evidence      BenchmarkEvidence `json:"evidence"`
}

func ParseEpisodeStatus(value string) (EpisodeStatus, error) {
	switch EpisodeStatus(value) {
	case EpisodeCompleted, EpisodeIncomplete:
		return EpisodeStatus(value), nil
	default:
		return "", fmt.Errorf("unknown episode status %q", value)
	}
}

func (value *EpisodeStatus) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseEpisodeStatus(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}
func ValidGateDecision(value GateDecision) bool {
	switch value {
	case GateBaselinePassed, GateInconclusive, GateDeferHardening, GateNeedsReproduction, GateAuthorizeHardening, GateUnreproducedObservation:
		return true
	default:
		return false
	}
}
func ParseClassification(value string) (Classification, error) {
	switch value {
	case ExactValid, ValidButSemanticallyDifferent, UnknownKey, CoercibleTypeDrift, NoncoercibleTypeDrift, InvalidJSON, NoCall, MultipleCalls, MissingRequired, EnumMismatch:
		return Classification(value), nil
	default:
		return "", fmt.Errorf("unknown classification %q", value)
	}
}
func (value *Classification) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseClassification(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}
func (value *GateDecision) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseGateDecision(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}
func ParseGateDecision(value string) (GateDecision, error) {
	decision := GateDecision(value)
	if !ValidGateDecision(decision) {
		return "", fmt.Errorf("unknown gate decision %q", value)
	}
	return decision, nil
}

type Fixture struct {
	ID                string         `json:"id"`
	ProbeTool         string         `json:"probe_tool"`
	SourceTool        string         `json:"source_tool"`
	SchemaSHA256      string         `json:"schema_sha256"`
	ExpectedArguments map[string]any `json:"expected_arguments"`
}

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
