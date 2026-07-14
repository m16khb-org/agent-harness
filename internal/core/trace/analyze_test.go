package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/failurecause"
	corestate "agent-harness/internal/core/state"
)

func TestTraceAnalyzeSelfVerifySummary(t *testing.T) {
	input := filepath.Join(t.TempDir(), "summary.json")
	body := `{
  "summary": {
    "failed_steps": 2,
    "failure_class": "deterministic",
    "failed_step": "contract golden tests",
    "failure_clusters": [
      {"step": "contract golden tests", "count": 2, "seeds": [100, 101]}
    ],
    "rerun_commands": ["go test ./cmd/harness -run Golden -count=1"],
    "failure_cause": "transport",
    "failure_cause_evidence": [
      {"cause": "transport", "code": "mcp-framing", "source": "conformance_probe"}
    ]
  }
}`
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: input})
	if err != nil {
		t.Fatalf("TraceAnalyze: %v", err)
	}
	if !result.OK || result.InputSource != "file" || result.FindingCount != 1 {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	finding := result.Findings[0]
	if finding.FailureClass != "deterministic" {
		t.Fatalf("unexpected failure class: %+v", finding)
	}
	if !strings.Contains(finding.RecurringPattern, "contract golden tests failed 2 time") {
		t.Fatalf("unexpected recurring pattern: %+v", finding)
	}
	if finding.VerificationCommand != "go test ./cmd/harness -run Golden -count=1" {
		t.Fatalf("unexpected verification command: %+v", finding)
	}
	if finding.FailureCause != failurecause.Transport {
		t.Fatalf("unexpected failure cause: %+v", finding)
	}
	if len(finding.FailureCauseEvidence) != 1 || finding.FailureCauseEvidence[0] != (failurecause.Evidence{
		Cause:  failurecause.Transport,
		Code:   "mcp-framing",
		Source: "conformance_probe",
	}) {
		t.Fatalf("unexpected failure cause evidence: %+v", finding.FailureCauseEvidence)
	}
}

func TestTraceAnalyzeDocUpkeepJSONLRedactsSecretEvidence(t *testing.T) {
	input := filepath.Join(t.TempDir(), "queue.jsonl")
	body := `{"kind":"code_change","target_docs":["OPERATIONS.md"],"summary":"TOKEN=secret-value","evidence":["cmd/harness/main.go"],"source":"test"}
{"event":"step_end","step":"go test","ok":false}
`
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: input})
	if err != nil {
		t.Fatalf("TraceAnalyze: %v", err)
	}
	if result.FindingCount != 2 {
		t.Fatalf("expected doc upkeep and failed step findings: %+v", result)
	}
	encoded := stringifyTraceResult(result)
	if strings.Contains(encoded, "secret-value") {
		t.Fatalf("trace analysis leaked secret fixture: %s", encoded)
	}
	if !containsString(result.TraceTypes, "doc_upkeep_jsonl") || !containsString(result.TraceTypes, "self_verify_progress_jsonl") {
		t.Fatalf("unexpected trace types: %+v", result.TraceTypes)
	}
}

func TestTraceAnalyzeSingleDocUpkeepJSON(t *testing.T) {
	input := filepath.Join(t.TempDir(), "event.json")
	body := `{"kind":"code_change","target_docs":["AGENT_WORKFLOW.md"],"summary":"workflow changed","source":"test"}`
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: input})
	if err != nil {
		t.Fatalf("TraceAnalyze: %v", err)
	}
	if result.FindingCount != 1 || !containsString(result.TraceTypes, "doc_upkeep_json") {
		t.Fatalf("expected single doc-upkeep finding: %+v", result)
	}
	finding := result.Findings[0]
	if finding.FailureCause != failurecause.Unknown || finding.FailureCauseEvidence == nil {
		t.Fatalf("doc upkeep failure cause defaults = %+v", finding)
	}
}

func TestTraceAnalyzeGuardFindingsAndWarnings(t *testing.T) {
	input := filepath.Join(t.TempDir(), "guard.json")
	body := `{
  "guard": {
    "findings": [
      {"rule":"search-routing"},
      {"rule":"search-routing"},
      {"message":"missing rule"}
    ]
  }
}`
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: input})
	if err != nil {
		t.Fatalf("TraceAnalyze guard: %v", err)
	}
	if result.FindingCount != 2 || !containsString(result.TraceTypes, "guard_result") {
		t.Fatalf("unexpected guard result: %+v", result)
	}
	encoded := stringifyTraceResult(result)
	for _, want := range []string{"guard_guard_finding", "guard_search-routing", "search-routing reported 2 time"} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("guard findings missing %q: %+v", want, result.Findings)
		}
	}
	for _, finding := range result.Findings {
		if finding.FailureCause != failurecause.Unknown || finding.FailureCauseEvidence == nil {
			t.Fatalf("guard failure cause defaults = %+v", finding)
		}
	}

	unknown := filepath.Join(t.TempDir(), "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"other":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	noFindings, err := TraceAnalyze(TraceAnalyzeRequest{Input: unknown})
	if err != nil {
		t.Fatalf("TraceAnalyze unknown: %v", err)
	}
	if !containsString(noFindings.Warnings, "no_supported_trace_findings") {
		t.Fatalf("expected no-supported warning: %+v", noFindings)
	}
}

func TestTraceAnalyzeInvalidJSONAndJSONLFallback(t *testing.T) {
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: invalid})
	if err != nil {
		t.Fatalf("TraceAnalyze invalid JSON warning result: %v", err)
	}
	if !containsStringPrefix(result.Warnings, "invalid_json:") {
		t.Fatalf("expected invalid_json warning: %+v", result)
	}

	jsonl := filepath.Join(t.TempDir(), "fallback.jsonl")
	if err := os.WriteFile(jsonl, []byte("{\"broken\":\n{\"event\":\"step_end\",\"step\":\"daemon build\",\"ok\":false}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fallback, err := TraceAnalyze(TraceAnalyzeRequest{Input: jsonl})
	if err != nil {
		t.Fatalf("TraceAnalyze JSONL fallback: %v", err)
	}
	if fallback.FindingCount != 1 || !strings.Contains(fallback.Findings[0].ProposedKnob, "daemon stale-lock") {
		t.Fatalf("expected JSONL fallback daemon finding: %+v", fallback)
	}
}

func TestDedupeTraceFindingsKeepsDistinctFailureCauses(t *testing.T) {
	findings := dedupeTraceFindings([]TraceAnalysisFinding{
		{
			FailureClass:     "shared_failure",
			FailureCause:     failurecause.Transport,
			RecurringPattern: "same pattern",
			ProposedKnob:     "same knob",
		},
		{
			FailureClass:     "shared_failure",
			FailureCause:     failurecause.Model,
			RecurringPattern: "same pattern",
			ProposedKnob:     "same knob",
		},
	})
	if len(findings) != 2 {
		t.Fatalf("distinct failure causes were deduped: %+v", findings)
	}
	if findings[0].FailureCause != failurecause.Model || findings[1].FailureCause != failurecause.Transport {
		t.Fatalf("findings were not deterministically sorted by failure cause: %+v", findings)
	}
	for _, finding := range findings {
		if finding.FailureCauseEvidence == nil {
			t.Fatalf("failure cause evidence must serialize as an array: %+v", finding)
		}
	}
}

func TestTraceAnalyzeRedactsFailureCauseEvidence(t *testing.T) {
	input := filepath.Join(t.TempDir(), "summary.json")
	body := `{
  "summary": {
    "failed_steps": 1,
    "failure_class": "deterministic",
    "failed_step": "contract probe",
    "failure_cause": "model",
    "failure_cause_evidence": [
      {"cause": "model", "code": "TOKEN=secret-value", "source": "authorization: Bearer secret-value"}
    ]
  }
}`
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: input})
	if err != nil {
		t.Fatalf("TraceAnalyze: %v", err)
	}
	evidence := result.Findings[0].FailureCauseEvidence
	if len(evidence) != 1 || evidence[0].Code != "redacted" || evidence[0].Source != "redacted" {
		t.Fatalf("failure cause evidence was not redacted: %+v", evidence)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-value") {
		t.Fatalf("trace analysis leaked failure cause evidence: %s", encoded)
	}
}
func TestTraceAnalyzeReadsStateKey(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := corestate.StateWrite("trace-fixture", `{"failed_steps":1,"failure_class":"intermittent","failed_step":"go test"}`); err != nil {
		t.Fatal(err)
	}
	result, err := TraceAnalyze(TraceAnalyzeRequest{Input: "trace-fixture"})
	if err != nil {
		t.Fatalf("TraceAnalyze state key: %v", err)
	}
	if result.InputSource != "state" || result.FindingCount != 1 {
		t.Fatalf("unexpected state trace result: %+v", result)
	}
}

func TestTraceAnalyzeRejectsEmptyInput(t *testing.T) {
	if _, err := TraceAnalyze(TraceAnalyzeRequest{}); err == nil {
		t.Fatal("expected missing input error")
	}
	input := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(input, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := TraceAnalyze(TraceAnalyzeRequest{Input: input}); err == nil {
		t.Fatal("expected empty input error")
	}
}

func stringifyTraceResult(result TraceAnalyzeResult) string {
	var b strings.Builder
	for _, finding := range result.Findings {
		b.WriteString(finding.FailureClass)
		b.WriteString(finding.RecurringPattern)
		b.WriteString(finding.ProposedKnob)
		b.WriteString(finding.OverfitRisk)
		b.WriteString(finding.VerificationCommand)
	}
	return b.String()
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsStringPrefix(items []string, prefix string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
