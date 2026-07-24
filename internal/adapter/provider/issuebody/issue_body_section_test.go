package issuebody

import (
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestMergeManagedSectionIdempotent(t *testing.T) {
	start, end, err := SectionMarkers(SectionDevilsAdvocate)
	if err != nil {
		t.Fatal(err)
	}
	sec := RenderDevilsAdvocateSection([]string{"gold-plating", "schedule optimism", "  "}, "2026-07-01T00:00:00Z")
	body := "original body\n"

	once := MergeManagedSection(body, sec, start, end)
	if !strings.Contains(once, "gold-plating") || !strings.HasPrefix(once, "original body") {
		t.Fatalf("append failed: %q", once)
	}
	if strings.Contains(once, "- \n") {
		t.Fatalf("blank finding should be dropped: %q", once)
	}

	sec2 := RenderDevilsAdvocateSection([]string{"new finding"}, "2026-07-02T00:00:00Z")
	twice := MergeManagedSection(once, sec2, start, end)
	if strings.Count(twice, start) != 1 || strings.Count(twice, end) != 1 {
		t.Fatalf("re-merge must not duplicate the block: %q", twice)
	}
	if strings.Contains(twice, "gold-plating") || !strings.Contains(twice, "new finding") {
		t.Fatalf("re-merge must replace the block content: %q", twice)
	}
	if !strings.HasPrefix(twice, "original body") {
		t.Fatalf("surrounding body must round-trip: %q", twice)
	}
}

func TestMergeManagedSectionEmptyBody(t *testing.T) {
	start, end, err := SectionMarkers(SectionDevilsAdvocate)
	if err != nil {
		t.Fatal(err)
	}
	sec := RenderDevilsAdvocateSection([]string{"x"}, "t")
	got := MergeManagedSection("", sec, start, end)
	if !strings.Contains(got, "x") || !strings.HasPrefix(got, start) {
		t.Fatalf("empty body should become just the section: %q", got)
	}
}

func TestSectionMarkersRejectsUnknownKind(t *testing.T) {
	if _, _, err := SectionMarkers("release-notes"); err == nil {
		t.Fatal("unknown section kind must be rejected")
	}
}

func completionFixture() port.IssueProviderCompletionSection {
	return port.IssueProviderCompletionSection{
		FinalHead:           "abc1234",
		RemoteArtifactURL:   "https://github.com/acme/repo/pull/9",
		VerificationSummary: []string{"go test ./... ok", "self-verify ok"},
		ArtifactManifest: []port.IssueProviderArtifactDigest{
			{Name: "plan", SHA256: strings.Repeat("a", 64)},
		},
		TuringSummary: "AC 전부 PASS",
		SpecBody:      "spec body 전문",
		PlanBody:      "plan body 전문",
	}
}

// AC-04: completion 섹션은 블록 헤딩 7종이 항상 존재해야 한다.
func TestRenderCompletionSectionContainsSevenBlocks(t *testing.T) {
	got := RenderCompletionSection(completionFixture(), "2026-07-24T00:00:00Z", 0)
	for _, heading := range []string{
		"### 최종 head", "### PR/MR", "### 검증 요약",
		"### Artifact manifest", "### Turing 요약", "### spec 전문", "### plan 전문",
	} {
		if !strings.Contains(got, heading) {
			t.Fatalf("completion section is missing block %q: %q", heading, got)
		}
	}
	if !strings.Contains(got, "<details>") || !strings.Contains(got, "plan body 전문") {
		t.Fatalf("collapsible full texts must be present: %q", got)
	}
	if strings.Contains(got, completionTruncationNotice) {
		t.Fatalf("no truncation expected without a limit: %q", got)
	}

	empty := RenderCompletionSection(port.IssueProviderCompletionSection{}, "t", 0)
	for _, heading := range []string{"### 최종 head", "### PR/MR", "### 검증 요약", "### Artifact manifest", "### Turing 요약", "### spec 전문", "### plan 전문"} {
		if !strings.Contains(empty, heading) {
			t.Fatalf("empty payload must keep block %q: %q", heading, empty)
		}
	}
	if !strings.Contains(empty, completionEmptyPlaceholder) {
		t.Fatalf("empty payload must render placeholders: %q", empty)
	}
}

// AC-04: 한도 초과 시 plan → spec → turing 우선순위로 절단하고 절단 문구를 남긴다.
func TestRenderCompletionSectionTruncatesByPriority(t *testing.T) {
	c := completionFixture()
	c.PlanBody = strings.Repeat("p", 4000)
	c.SpecBody = strings.Repeat("s", 400)
	full := RenderCompletionSection(c, "t", 0)
	got := RenderCompletionSection(c, "t", len(full)-2000)
	if strings.Contains(got, c.PlanBody) {
		t.Fatalf("plan body must be truncated first: len=%d", len(got))
	}
	if !strings.Contains(got, c.SpecBody) {
		t.Fatalf("spec body must survive when dropping plan is enough: %q", got[:200])
	}
	if !strings.Contains(got, completionTruncationNotice) {
		t.Fatalf("truncation notice must be present: %q", got[:200])
	}
	// 5차 m1: 절단이 일어나도 블록 헤딩 7종은 전부 남아야 한다.
	for _, heading := range []string{
		"### 최종 head", "### PR/MR", "### 검증 요약",
		"### Artifact manifest", "### Turing 요약", "### spec 전문", "### plan 전문",
	} {
		if !strings.Contains(got, heading) {
			t.Fatalf("truncated section is missing block %q", heading)
		}
	}
}

// C2 ④'가 재사용하는 CleanupAudit 렌더 계약을 고정한다(C3-F9).
func TestRenderCompletionSectionIncludesCleanupAudit(t *testing.T) {
	c := completionFixture()
	c.CleanupAudit = "cleanup 완료: worktree=/tmp/wt branch=80-finish oid=abc at=2026-07-24T00:00:00Z"
	got := RenderCompletionSection(c, "t", 0)
	if !strings.Contains(got, "### Cleanup 감사") || !strings.Contains(got, c.CleanupAudit) {
		t.Fatalf("cleanup audit block must render when set: %q", got)
	}
	without := RenderCompletionSection(completionFixture(), "t", 0)
	if strings.Contains(without, "### Cleanup 감사") {
		t.Fatalf("cleanup audit block must be absent when unset: %q", without)
	}
}

func TestRenderSectionRoutesByKind(t *testing.T) {
	if _, _, _, err := RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{Section: SectionCompletion}, "t", 0); err == nil {
		t.Fatal("completion section without payload must be rejected")
	}
	section, start, _, err := RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{
		Section: SectionCompletion, Completion: &port.IssueProviderCompletionSection{},
	}, "t", 0)
	if err != nil || !strings.HasPrefix(section, start) || start != CompletionStartMarker {
		t.Fatalf("completion render failed: %v %q", err, section)
	}
	section, start, _, err = RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{
		Section: SectionDevilsAdvocate, Findings: []string{"f"},
	}, "t", 0)
	if err != nil || !strings.HasPrefix(section, start) {
		t.Fatalf("devils-advocate render failed: %v %q", err, section)
	}
}

// 두 섹션은 서로의 블록을 건드리지 않아야 한다.
func TestCompletionAndDevilsAdvocateSectionsCoexist(t *testing.T) {
	daStart, daEnd, _ := SectionMarkers(SectionDevilsAdvocate)
	coStart, coEnd, _ := SectionMarkers(SectionCompletion)
	body := MergeManagedSection("base\n", RenderDevilsAdvocateSection([]string{"finding"}, "t"), daStart, daEnd)
	body = MergeManagedSection(body, RenderCompletionSection(completionFixture(), "t", 0), coStart, coEnd)
	if strings.Count(body, daStart) != 1 || strings.Count(body, coStart) != 1 {
		t.Fatalf("both sections must coexist exactly once: %q", body)
	}
	body2 := MergeManagedSection(body, RenderCompletionSection(completionFixture(), "t2", 0), coStart, coEnd)
	if !strings.Contains(body2, "finding") || strings.Count(body2, coStart) != 1 {
		t.Fatalf("re-merging completion must preserve devils-advocate block: %q", body2)
	}
}
