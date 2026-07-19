package operationalhealth

import (
	"testing"
	"time"
)

func TestClassifyOperationalHealthRejectsBoundCycleWithoutFreshHeartbeat(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		Cycles: []Cycle{{
			ID:              "io-dead-owner",
			Repo:            "/repo",
			Branch:          "1-dead-owner",
			Phase:           "implement",
			HandoffState:    "claimed",
			Attempt:         1,
			OwnershipEpoch:  "epoch-1",
			ContextSHA256:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			WorkerSessionID: "session-1",
			WorkerAgentID:   "agent-1",
			TaskID:          "task-1",
		}},
		Bindings: []Binding{{
			CycleID: "io-dead-owner",
			Repo:    "/repo",
			Branch:  "1-dead-owner",
		}},
		Tasks: []OrcaTask{{ID: "task-1", Status: "ready"}},
	}

	result := Classify(snapshot, Options{Now: now})

	if result.Healthy {
		t.Fatalf("bound cycle without a fresh heartbeat classified healthy: %#v", result)
	}
	if !hasFinding(result.Findings, FindingDeadOwner, "io-dead-owner") {
		t.Fatalf("missing %s finding for dead owner: %#v", FindingDeadOwner, result.Findings)
	}
}

func hasFinding(findings []Finding, code, resourceID string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.ResourceID == resourceID {
			return true
		}
	}
	return false
}
