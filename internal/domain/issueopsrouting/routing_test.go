package issueopsrouting

import (
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestAppendRejectsTraceBeyondBound(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		RoutingTrace: make([]issueopscontract.SkillRoutingEntry, MaxTraceEntries),
	}
	if _, _, err := Append(record, issueopscontract.SkillRoutingEntry{
		Phase: "plan",
		Skill: "codd",
	}); err == nil {
		t.Fatal("full routing trace must be rejected")
	}
}
