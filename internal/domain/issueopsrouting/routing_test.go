package issueopsrouting

import (
	"strings"
	"testing"
	"time"

	issueopsroutingcontract "agent-harness/internal/contract/issueopsrouting"
)

// routing 도메인은 record-routing/routing-score의 순수 규칙 계층이다.
// 검증, 멱등 append, trace 상한, 점수 매칭(대소문자/공백 무시)을 잠근다.
func TestNewEntryValidatesAndNormalizes(t *testing.T) {
	at := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	entry, err := NewEntry("  plan  ", "codd", at)
	if err != nil || entry.Phase != "plan" || entry.Skill != "codd" || entry.At != "2026-08-21T12:00:00Z" {
		t.Fatalf("entry = %#v err = %v", entry, err)
	}
	for _, bad := range []struct {
		phase, skill string
		want         string
	}{
		{"", "codd", "phase is required"},
		{"plan", "", "skill is required"},
		{strings.Repeat("p", 65), "codd", "64 bytes"},
		{"plan", strings.Repeat("s", 65), "64 bytes"},
	} {
		if _, err := NewEntry(bad.phase, bad.skill, at); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Fatalf("NewEntry(%q,%q) error = %v want ~ %q", bad.phase, bad.skill, err, bad.want)
		}
	}
}

func TestAppendIsIdempotentAndBounded(t *testing.T) {
	at := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	entry, _ := NewEntry("grill", "boehm", at)
	record := issueopsroutingcontract.Record{}
	changed, _, err := Append(record, entry)
	if err != nil || !changed.OK || len(changed.RoutingTrace) != 1 || changed.UpdatedAt != entry.At {
		t.Fatalf("first append = %#v err = %v", changed, err)
	}
	// 같은 (phase, skill)는 대소문자를 무시하고 중복으로 취급한다.
	repeatUpper, _ := NewEntry("GRILL", "Boehm", at)
	again, changed2, err2 := Append(changed, repeatUpper)
	if err2 != nil || changed2 || len(again.RoutingTrace) != 1 {
		t.Fatalf("duplicate append must be a no-op: changed=%v err=%v trace=%d", changed2, err2, len(again.RoutingTrace))
	}
	full := issueopsroutingcontract.Record{RoutingTrace: make([]issueopsroutingcontract.Entry, MaxTraceEntries)}
	if _, changed3, err3 := Append(full, entry); err3 == nil || changed3 || !strings.Contains(err3.Error(), "trace is full") {
		t.Fatalf("bounded trace must reject: changed=%v err=%v", changed3, err3)
	}
}

func TestScoreMatchesPairingsLeniently(t *testing.T) {
	record := issueopsroutingcontract.Record{RoutingTrace: []issueopsroutingcontract.Entry{
		{Phase: "problem", Skill: "issueops"},
		{Phase: "plan", Skill: "codd"},
	}}
	result := ScoreRecord(record, []issueopsroutingcontract.Expected{
		{Phase: " PROBLEM ", Skill: "IssueOps"},
		{Phase: "plan", Skill: "codd"},
		{Phase: "implement", Skill: "dijkstra"},
	})
	if result.OK || len(result.Missing) != 1 || result.Missing[0].Skill != "dijkstra" {
		t.Fatalf("score record = %#v", result)
	}
	if got := Score(
		[]issueopsroutingcontract.Expected{{Phase: "plan", Skill: "codd"}},
		[]issueopsroutingcontract.Expected{{Phase: "PLAN", Skill: " codd "}},
	); !got.OK {
		t.Fatalf("lenient pairing must match: %#v", got)
	}
}
