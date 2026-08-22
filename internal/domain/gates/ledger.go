// Package gates는 unlazy 호환 태스크 게이트 ledger의 순수 파서/직렬화 규칙을
// 소유한다. 형식은 unlazy v2의 references/gates.md 계약을 따른다:
//
//	# Gates: <scope>
//
//	- [ ] G1: <outcome>
//	  CHECK: <command>
//	  EXPECT: <substring or /regex/>
//	  EVIDENCE: pending
//
//	ABANDON: G2 <reason>
//
// 파서는 원본 바이트를 보존한다. 라인 내용을 바꿀 때는 해당 라인만 교체하며,
// 개행 스타일(CRLF 포함)과 게이트 외 라인은 그대로 둔다. 체크박스는 주장이고
// EVIDENCE가 증명이다 — checked + pending evidence는 미충족으로 판정한다.
package gates

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// gateLineRe는 unlazy의 /^- \[( |x|X)\] (.*)$/와 같다.
	gateLineRe = regexp.MustCompile(`^- \[( |x|X)\] (.*)$`)
	// attrLineRe는 게이트 라인 아래 들여쓴 CHECK/EXPECT/EVIDENCE 속성 라인.
	attrLineRe = regexp.MustCompile(`^[ \t]+(CHECK|EXPECT|EVIDENCE):[ \t]?(.*)$`)
	// abandonLineRe는 /^ABANDON:\s*(\S+)\s*(.*)$/.
	abandonLineRe = regexp.MustCompile(`^ABANDON:[ \t]*(\S+)[ \t]*(.*)$`)
)

// Gate는 게이트 하나의 파싱 결과와 파일 내 위치다.
type Gate struct {
	ID            string
	Title         string
	Checked       bool
	CheckCmd      string
	Expect        string
	Evidence      string
	Abandoned     bool
	AbandonReason string
	// CheckboxLine은 "- [ ]"/"- [x]" 라인의 0-based 인덱스.
	CheckboxLine int
	// EvidenceLine은 EVIDENCE: 라인의 0-based 인덱스. 없으면 -1.
	EvidenceLine int
}

// Ledger는 게이트 파일 전체의 파싱 결과다.
type Ledger struct {
	Lines []string
	Gates []Gate
}

// Parse는 게이트 파일 원문을 라인 배열과 게이트 목록으로 바꾼다.
//
// unlazy gate-check.mjs의 파싱 규칙을 그대로 따른다:
//   - `- [ ]`/`- [x]` 라인에서 게이트가 시작한다. ID는 제목의 첫 `token:` 접두어,
//     없으면 "line<N>"(1-based 라인 번호)이다.
//   - 다음 게이트 전까지의 들여쓴 CHECK/EXPECT/EVIDENCE 라인은 직전 게이트 속성이다.
//   - `^#` 헤딩 라인과 `- ` 목록 라인은 속성 첨부를 끊는다.
//   - `ABANDON: <id> <reason>`은 파일 어디서든 해당 게이트를 정직한 포기로 표시한다.
func Parse(text string) Ledger {
	lines := strings.Split(text, "\n")
	ledger := Ledger{Lines: lines}
	current := -1 // index into ledger.Gates; -1 = no open gate
	abandons := map[string]string{}
	for i, line := range lines {
		if m := gateLineRe.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(m[2])
			id := gateID(title, i)
			ledger.Gates = append(ledger.Gates, Gate{
				ID:           id,
				Title:        stripGateIDPrefix(title),
				Checked:      strings.ToLower(m[1]) == "x",
				CheckboxLine: i,
				EvidenceLine: -1,
			})
			current = len(ledger.Gates) - 1
			continue
		}
		if m := attrLineRe.FindStringSubmatch(line); m != nil && current >= 0 {
			gate := &ledger.Gates[current]
			value := strings.TrimSpace(m[2])
			switch m[1] {
			case "CHECK":
				gate.CheckCmd = value
			case "EXPECT":
				gate.Expect = value
			case "EVIDENCE":
				gate.Evidence = value
				gate.EvidenceLine = i
			}
			continue
		}
		if m := abandonLineRe.FindStringSubmatch(line); m != nil {
			abandons[strings.TrimSuffix(m[1], ":")] = abandonReason(m[2])
			continue
		}
		// unlazy처럼 헤딩과 목록 라인은 속성 첨부를 끊는다. 빈 라인은 유지한다.
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- ") {
			current = -1
		}
	}
	for i := range ledger.Gates {
		reason, ok := abandons[ledger.Gates[i].ID]
		if ok {
			ledger.Gates[i].Abandoned = true
			ledger.Gates[i].AbandonReason = reason
		}
	}
	return ledger
}

// Render는 라인 배열을 다시 파일 원문으로 합친다. Parse와 함께 round-trip하면
// 수정하지 않은 라인은 바이트 단위로 보존된다.
func Render(ledger Ledger) string {
	return strings.Join(ledger.Lines, "\n")
}

// MarkPass는 게이트의 체크박스를 [x]로 바꾸고 EVIDENCE 라인에 증거를 기록한다.
// EVIDENCE 라인이 없던 게이트는 체크박스 라인 바로 아래에 삽입한다 — unlazy
// gate-check.mjs는 이 경우 증거 라인을 만들지 않아 "checked but evidence pending"
// 상태가 파일에 남지만, 하네스는 checked+evidence 불변식을 파일까지 유지한다.
func MarkPass(ledger *Ledger, gateIndex int, evidence string) {
	gate := &ledger.Gates[gateIndex]
	if !gate.Checked {
		line := ledger.Lines[gate.CheckboxLine]
		if strings.HasPrefix(line, "- [ ]") {
			ledger.Lines[gate.CheckboxLine] = "- [x]" + line[len("- [ ]"):]
		}
		gate.Checked = true
	}
	indent := attrIndent(ledger.Lines, gate.CheckboxLine)
	if gate.EvidenceLine < 0 {
		ledger.Lines = append(ledger.Lines[:gate.CheckboxLine+1], append([]string{indent + "EVIDENCE: " + evidence}, ledger.Lines[gate.CheckboxLine+1:]...)...)
		gate.EvidenceLine = gate.CheckboxLine + 1
		reindexAfterInsert(ledger, gate.EvidenceLine, gateIndex)
	} else {
		ledger.Lines[gate.EvidenceLine] = indent + "EVIDENCE: " + evidence
	}
	gate.Evidence = evidence
}

// AppendAbandon은 파일의 마지막 비어 있지 않은 라인 뒤에 ABANDON 라인을
// 추가한다. 파일 끝 newline은 보존한다.
func AppendAbandon(ledger *Ledger, gateID, reason string) {
	insertAt := len(ledger.Lines)
	for insertAt > 0 && strings.TrimSpace(strings.TrimRight(ledger.Lines[insertAt-1], "\r")) == "" {
		insertAt--
	}
	line := "ABANDON: " + gateID + " " + strings.TrimSpace(reason)
	ledger.Lines = append(ledger.Lines[:insertAt], append([]string{line}, ledger.Lines[insertAt:]...)...)
	for i := range ledger.Gates {
		if ledger.Gates[i].CheckboxLine >= insertAt {
			ledger.Gates[i].CheckboxLine++
		}
		if ledger.Gates[i].EvidenceLine >= insertAt {
			ledger.Gates[i].EvidenceLine++
		}
		if ledger.Gates[i].ID == gateID {
			ledger.Gates[i].Abandoned = true
			ledger.Gates[i].AbandonReason = abandonReason(reason)
		}
	}
}

// reindexAfterInsert는 라인 삽입 이후 삽입 지점 뒤에 있는 게이트의 라인
// 인덱스를 보정한다. ownerIndex 게이트는 삽입된 EVIDENCE 라인의 주인이므로
// 제외한다(체크박스는 삽입 지점 앞에 있다).
func reindexAfterInsert(ledger *Ledger, insertedAt, ownerIndex int) {
	for i := range ledger.Gates {
		if i == ownerIndex {
			continue
		}
		if ledger.Gates[i].CheckboxLine >= insertedAt {
			ledger.Gates[i].CheckboxLine++
		}
		if ledger.Gates[i].EvidenceLine >= insertedAt {
			ledger.Gates[i].EvidenceLine++
		}
	}
}

func indentOf(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

func attrIndent(lines []string, checkboxLine int) string {
	for i := checkboxLine + 1; i < len(lines); i++ {
		if attrLineRe.MatchString(lines[i]) {
			return indentOf(lines[i])
		}
		if gateLineRe.MatchString(lines[i]) {
			break
		}
		if trimmed := strings.TrimRight(lines[i], "\r"); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
	}
	return "  "
}

func gateID(title string, lineIndex int) string {
	if token, _, ok := strings.Cut(title, ":"); ok && token != "" && !strings.ContainsAny(token, " \t") {
		return token
	}
	return "line" + strconv.Itoa(lineIndex+1)
}

func stripGateIDPrefix(title string) string {
	if token, rest, ok := strings.Cut(title, ":"); ok && token != "" && !strings.ContainsAny(token, " \t") {
		return strings.TrimSpace(rest)
	}
	return title
}

func abandonReason(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "(no reason)"
	}
	return raw
}
