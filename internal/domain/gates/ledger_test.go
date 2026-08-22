package gates

import (
	"strings"
	"testing"
)

const sampleLedger = `# Gates: pricing section

Scope: one line

- [ ] G1: three tiers render with real copy
  CHECK: node check.js pricing --tiers
  EXPECT: 3/3 tiers ok
  EVIDENCE: pending

- [x] G2: manual outcome verified
  EVIDENCE: file:line quote of proof

- [X] G3: uppercase X checked

- [ ] no-id gate here

ABANDON: G4 not achievable in this environment
`

func TestParseExtractsGatesAttributesAndAbandons(t *testing.T) {
	ledger := Parse(sampleLedger)
	if len(ledger.Gates) != 4 {
		t.Fatalf("gate count = %d, want 4: %+v", len(ledger.Gates), ledger.Gates)
	}
	g1 := ledger.Gates[0]
	if g1.ID != "G1" || g1.Title != "three tiers render with real copy" || g1.Checked {
		t.Fatalf("G1 parsed wrong: %+v", g1)
	}
	if g1.CheckCmd != "node check.js pricing --tiers" || g1.Expect != "3/3 tiers ok" || g1.Evidence != "pending" {
		t.Fatalf("G1 attributes wrong: %+v", g1)
	}
	g2 := ledger.Gates[1]
	if !g2.Checked || g2.Evidence != "file:line quote of proof" || g2.CheckCmd != "" {
		t.Fatalf("G2 parsed wrong: %+v", g2)
	}
	if !ledger.Gates[2].Checked {
		t.Fatalf("uppercase X must count as checked: %+v", ledger.Gates[2])
	}
	if ledger.Gates[3].ID != "line15" {
		t.Fatalf("id-less gate should fall back to line<N>, got %q", ledger.Gates[3].ID)
	}
	// G4 ABANDON line names a gate that does not exist as a checkbox; unlazy
	// records it in the abandon map only. Our parser keeps it out of Gates.
	for _, gate := range ledger.Gates {
		if gate.ID == "G4" {
			t.Fatalf("ABANDON-only id should not create a gate: %+v", gate)
		}
	}
}

func TestParseAbandonMarksExistingGate(t *testing.T) {
	text := "- [ ] G1: outcome\n  EVIDENCE: pending\n\nABANDON: G1 cannot verify without network\n"
	ledger := Parse(text)
	if len(ledger.Gates) != 1 {
		t.Fatalf("gate count = %d, want 1", len(ledger.Gates))
	}
	if !ledger.Gates[0].Abandoned || ledger.Gates[0].AbandonReason != "cannot verify without network" {
		t.Fatalf("abandon not applied: %+v", ledger.Gates[0])
	}
}

func TestParseAttributesStopAtHeadingAndListLines(t *testing.T) {
	text := "# Gates: x\n\n- [ ] G1: first\n  CHECK: echo one\n\n## Sub heading\n  EVIDENCE: stray\n- [ ] G2: second\n  CHECK: echo two\n"
	ledger := Parse(text)
	if len(ledger.Gates) != 2 {
		t.Fatalf("gate count = %d, want 2", len(ledger.Gates))
	}
	if ledger.Gates[1].CheckCmd != "echo two" {
		t.Fatalf("stray indented line must not attach across a list boundary: %+v", ledger.Gates[1])
	}
	if ledger.Gates[0].Evidence != "" {
		t.Fatalf("stray EVIDENCE after a heading must not attach to G1: %+v", ledger.Gates[0])
	}
}

func TestParsePlainProseKeepsAttachmentLikeUnlazy(t *testing.T) {
	// unlazy gate-check.mjs는 헤딩(^#)과 목록(^- ) 라인만 속성 첨부를 끊는다.
	// 일반 산문 라인 뒤의 들여쓴 속성은 직전 게이트에 그대로 붙는다.
	text := "- [ ] G1: first\n  CHECK: echo one\nnote line\n  EVIDENCE: still attaches\n"
	ledger := Parse(text)
	if ledger.Gates[0].Evidence != "still attaches" {
		t.Fatalf("prose must not break attribute attachment: %+v", ledger.Gates[0])
	}
}

func TestRenderRoundTripPreservesBytes(t *testing.T) {
	ledger := Parse(sampleLedger)
	if got := Render(ledger); got != sampleLedger {
		t.Fatalf("round-trip changed the file:\n--- got ---\n%s\n--- want ---\n%s", got, sampleLedger)
	}
	crlf := "# Gates: x\r\n\r\n- [ ] G1: outcome\r\n  EVIDENCE: pending\r\n"
	if got := Render(Parse(crlf)); got != crlf {
		t.Fatalf("CRLF round-trip changed the file:\n%q\nwant\n%q", got, crlf)
	}
}

func TestMarkPassFlipsCheckboxAndWritesEvidence(t *testing.T) {
	text := "- [ ] G1: outcome\n  CHECK: true\n  EVIDENCE: pending\n"
	ledger := Parse(text)
	MarkPass(&ledger, 0, "3/3 tiers ok")
	if ledger.Lines[0] != "- [x] G1: outcome" {
		t.Fatalf("checkbox not flipped: %q", ledger.Lines[0])
	}
	if ledger.Lines[2] != "  EVIDENCE: 3/3 tiers ok" {
		t.Fatalf("evidence not recorded: %q", ledger.Lines[2])
	}
	if !ledger.Gates[0].Checked || ledger.Gates[0].Evidence != "3/3 tiers ok" {
		t.Fatalf("gate state not updated: %+v", ledger.Gates[0])
	}
}

func TestMarkPassInsertsEvidenceLineWhenMissing(t *testing.T) {
	text := "- [ ] G1: outcome\n  CHECK: true\n\n- [ ] G2: second\n"
	ledger := Parse(text)
	MarkPass(&ledger, 0, "ok")
	rendered := Render(ledger)
	if !strings.Contains(rendered, "- [x] G1: outcome\n  EVIDENCE: ok\n") {
		t.Fatalf("evidence line not inserted after checkbox:\n%s", rendered)
	}
	if ledger.Gates[1].CheckboxLine != 4 {
		t.Fatalf("later gate checkbox index not reindexed: %+v", ledger.Gates[1])
	}
	MarkPass(&ledger, 1, "second ok")
	if !strings.Contains(Render(ledger), "- [x] G2: second\n  EVIDENCE: second ok\n") {
		t.Fatalf("second gate pass not rendered:\n%s", Render(ledger))
	}
}

func TestAppendAbandonRecordsLine(t *testing.T) {
	ledger := Parse("- [ ] G1: outcome\n  EVIDENCE: pending\n")
	AppendAbandon(&ledger, "G1", "needs live credentials")
	rendered := Render(ledger)
	if !strings.Contains(rendered, "ABANDON: G1 needs live credentials\n") {
		t.Fatalf("abandon line missing:\n%s", rendered)
	}
	if !ledger.Gates[0].Abandoned {
		t.Fatal("gate not marked abandoned")
	}
}

func TestParseTrailingNewlineAndEmptyText(t *testing.T) {
	if ledger := Parse(""); len(ledger.Gates) != 0 || len(ledger.Lines) != 1 {
		t.Fatalf("empty text parse wrong: %+v", ledger)
	}
	ledger := Parse("- [ ] G1: x\n")
	if len(ledger.Gates) != 1 || len(ledger.Lines) != 2 || ledger.Lines[1] != "" {
		t.Fatalf("trailing newline parse wrong: %+v %q", ledger.Gates, ledger.Lines)
	}
}
