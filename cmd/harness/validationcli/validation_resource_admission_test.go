package validationcli

import "testing"

func TestValidateResourceAdmissionUsesDeterministicSamples(t *testing.T) {
	step := ValidateResourceAdmission()
	if !step.OK {
		t.Fatalf("ValidateResourceAdmission() = %#v", step)
	}
	if step.Label != "resource admission contract" {
		t.Fatalf("label = %q", step.Label)
	}
}
