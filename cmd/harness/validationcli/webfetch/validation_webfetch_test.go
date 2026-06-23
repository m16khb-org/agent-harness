package webfetch

import "testing"

func TestValidateWebFetchBatteryPassesDeterministicBenchmark(t *testing.T) {
	step := Validate("/tmp/agent-harness", "/repo", 100)
	if !step.OK {
		t.Fatalf("Validate returned non-ok step: %#v", step)
	}
	if step.Label != "web fetch battery" {
		t.Fatalf("label=%q, want web fetch battery", step.Label)
	}
}
