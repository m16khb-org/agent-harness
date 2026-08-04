package issueops

import (
	"errors"
	"strings"
	"testing"
)

func TestBindGeneratedCommandRequiresCompleteBinaryProvenance(t *testing.T) {
	command, err := BindGeneratedCommand(
		"agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm",
		GeneratedCommandProvenance{LeaseGeneration: 7},
	)
	if err == nil {
		t.Fatal("incomplete binary provenance must fail")
	}
	if command != "" {
		t.Fatalf("observation failure must not fall back to an unbound command: %q", command)
	}
}

func TestBindGeneratedCommandUsesCanonicalExecutableAndAddsBoundedEvidence(t *testing.T) {
	command, err := BindGeneratedCommand(
		"agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm",
		GeneratedCommandProvenance{
			ExecutablePath:   "/repo/bin/agent-harness",
			ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			LeaseGeneration:  7,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(command, "'/repo/bin/agent-harness' issueops execution resume ") {
		t.Fatalf("generated command did not select its observed executable: %q", command)
	}
	for _, want := range []string{
		"--generated-by-executable '/repo/bin/agent-harness'",
		"--generated-by-sha256 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--generated-for-generation 7",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("generated command %q does not contain %q", command, want)
		}
	}
}

func TestConsumeGeneratedCommandProvenanceKeepsManualPATHCommandUnchanged(t *testing.T) {
	args := []string{"execution", "status", "--id", "io-1", "--json"}
	clean, evidence, present, err := ConsumeGeneratedCommandProvenance(args)
	if err != nil {
		t.Fatal(err)
	}
	if present || evidence != (GeneratedCommandProvenance{}) || strings.Join(clean, " ") != strings.Join(args, " ") {
		t.Fatalf("manual PATH invocation changed: clean=%q evidence=%+v present=%t", clean, evidence, present)
	}
}

func TestConsumeGeneratedCommandProvenanceRemovesOnlyCompleteEnvelope(t *testing.T) {
	args := []string{
		"execution", "release", "--id", "io-1", "--generation", "7",
		GeneratedByExecutableFlag, "/repo/bin/agent-harness",
		GeneratedBySHA256Flag, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		GeneratedForGenerationFlag, "7", "--json",
	}
	clean, evidence, present, err := ConsumeGeneratedCommandProvenance(args)
	if err != nil {
		t.Fatal(err)
	}
	if !present || evidence.LeaseGeneration != 7 || evidence.ExecutablePath != "/repo/bin/agent-harness" {
		t.Fatalf("parsed evidence = %+v present=%t", evidence, present)
	}
	want := []string{"execution", "release", "--id", "io-1", "--generation", "7", "--json"}
	if strings.Join(clean, " ") != strings.Join(want, " ") {
		t.Fatalf("clean args = %q, want %q", clean, want)
	}
}

func TestValidateGeneratedCommandInvocationRejectsStaleInstalledBinary(t *testing.T) {
	expected := GeneratedCommandProvenance{
		ExecutablePath:   "/worktree/bin/agent-harness",
		ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		LeaseGeneration:  7,
	}
	observed := GeneratedCommandProvenance{
		ExecutablePath:   "/installed/bin/agent-harness",
		ExecutableSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		LeaseGeneration:  7,
	}
	err := ValidateGeneratedCommandInvocation(expected, observed, 7)
	if err == nil {
		t.Fatal("stale installed binary must be rejected")
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "generated_command_binary_provenance_mismatch" {
		t.Fatalf("mismatch is not structured: %T %v", err, err)
	}
}

func TestObservationErrorDoesNotExposeAdapterDiagnostic(t *testing.T) {
	err := NewGeneratedCommandProvenanceObservationError(errors.New("credential=must-not-leak"))
	if strings.Contains(err.Error(), "credential") || strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("observation error exposed adapter diagnostic: %v", err)
	}
}
