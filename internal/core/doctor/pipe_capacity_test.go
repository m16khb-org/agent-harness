package doctor

import "testing"

func TestHarnessDoctorReportsPipeCapacityCheck(t *testing.T) {
	oldMeasure := measurePipeCapacity
	measurePipeCapacity = func() (int, error) { return pipeCapacityWarningThreshold, nil }
	t.Cleanup(func() { measurePipeCapacity = oldMeasure })

	result, err := HarnessDoctor(HarnessDoctorRequest{RepoRoot: t.TempDir(), HarnessRoot: t.TempDir(), Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheckForTest(result.Checks, "pipe_capacity")
	if !ok || !check.Healthy {
		t.Fatalf("expected healthy pipe_capacity check, got check=%+v ok=%v result=%+v", check, ok, result)
	}
	if result.PipeCapacityBytes != pipeCapacityWarningThreshold {
		t.Fatalf("pipe capacity bytes = %d, want %d", result.PipeCapacityBytes, pipeCapacityWarningThreshold)
	}
	if hasHarnessDoctorIssueForTest(result.Issues, "pipe_capacity_degraded") {
		t.Fatalf("did not expect degraded pipe warning: %+v", result.Issues)
	}
}

func TestHarnessDoctorWarnsOnDegradedPipeCapacity(t *testing.T) {
	oldMeasure := measurePipeCapacity
	measurePipeCapacity = func() (int, error) { return 512, nil }
	t.Cleanup(func() { measurePipeCapacity = oldMeasure })

	result, err := HarnessDoctor(HarnessDoctorRequest{RepoRoot: t.TempDir(), HarnessRoot: t.TempDir(), Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheckForTest(result.Checks, "pipe_capacity")
	if !ok || check.Healthy {
		t.Fatalf("expected unhealthy pipe_capacity check, got check=%+v ok=%v result=%+v", check, ok, result)
	}
	if result.PipeCapacityBytes != 512 {
		t.Fatalf("pipe capacity bytes = %d, want 512", result.PipeCapacityBytes)
	}
	if !hasHarnessDoctorIssueForTest(result.Issues, "pipe_capacity_degraded") {
		t.Fatalf("expected degraded pipe warning: %+v", result.Issues)
	}
}

func harnessDoctorCheckForTest(checks []HarnessDoctorCheck, name string) (HarnessDoctorCheck, bool) {
	for _, check := range checks {
		if check.Name == name {
			return check, true
		}
	}
	return HarnessDoctorCheck{}, false
}

func hasHarnessDoctorIssueForTest(issues []HarnessDoctorIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
