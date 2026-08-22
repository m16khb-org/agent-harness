package gates

import (
	"regexp"
	"strings"
)

// 게이트 상태. unlazy의 미충족 규칙을 그대로 따른다:
//   - 미충족 = 체크박스 미체크 + ABANDON 없음, 또는 체크박스 체크 + EVIDENCE pending.
//     후자가 이 시스템이 잡으려는 실패 그 자체이므로 미체크보다 "나쁜" 상태로
//     별도 분류한다.
//   - ABANDON된 게이트는 해결으로 간주하되 보고서에 나열한다.
const (
	StateMet             = "met"
	StateUnchecked       = "unchecked"
	StateEvidencePending = "evidence_pending"
	StateAbandoned       = "abandoned"
)

// ExpectMatches는 EXPECT 값이 명령 출력(stdout+stderr 결합)과 일치하는지 판정한다.
// 기본은 부분 문자열 매치, `/pattern/flags` 형태면 정규식 매치다(unlazy 규칙).
// 지원 플래그는 i(대소문자 무시), m(멀티라인), s(개행 매치)이며 그 외 JS 플래그는
// 무시한다. 컴파일 실패는 false다 — 패닉이나 오류가 아니다.
func ExpectMatches(expect, output string) bool {
	expect = strings.TrimSpace(expect)
	if strings.HasPrefix(expect, "/") {
		if m := regexp.MustCompile(`^/(.+)/([a-z]*)$`).FindStringSubmatch(expect); m != nil {
			pattern := m[1]
			for _, flag := range m[2] {
				switch flag {
				case 'i':
					pattern = "(?i)" + pattern
				case 'm':
					pattern = "(?m)" + pattern
				case 's':
					pattern = "(?s)" + pattern
				}
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false
			}
			return re.MatchString(output)
		}
	}
	return strings.Contains(output, expect)
}

// EvidenceTail은 증거로 기록할 결정적 출력 꼬리를 만든다. unlazy처럼 비어 있지
// 않은 라인의 마지막 2개를 " | "로 잇고 max 문자로 자른다.
func EvidenceTail(output string, max int) string {
	if max <= 0 {
		max = 200
	}
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	kept := make([]string, 0, 2)
	for i := len(lines) - 1; i >= 0 && len(kept) < 2; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			kept = append([]string{trimmed}, kept...)
		}
	}
	joined := strings.Join(kept, " | ")
	if joined == "" {
		joined = "(no output)"
	}
	if len(joined) > max {
		joined = joined[:max]
	}
	return joined
}

// EvidencePending는 증거가 아직 기록되지 않았다는 unlazy 판정이다.
// EVIDENCE가 없거나 대소문자 구분 없이 "pending"이면 pending이다.
func EvidencePending(evidence string) bool {
	evidence = strings.TrimSpace(evidence)
	return evidence == "" || strings.EqualFold(evidence, "pending")
}

// State는 게이트 하나의 현재 상태를 판정한다.
func State(gate Gate) string {
	if gate.Abandoned {
		return StateAbandoned
	}
	if !gate.Checked {
		return StateUnchecked
	}
	if EvidencePending(gate.Evidence) {
		return StateEvidencePending
	}
	return StateMet
}

// Summary는 게이트 모음의 충족 요약이다. Complete는 unlazy와 같이
// 미충족(unchecked + evidence_pending)이 0개인지로 판정한다. ABANDON은 해결로
// 센다 — 모두 포기한 파일도 complete다.
type Summary struct {
	Total     int
	Met       int
	Unmet     int
	Abandoned int
	Complete  bool
}

func Summarize(gates []Gate) Summary {
	summary := Summary{Total: len(gates)}
	for _, gate := range gates {
		switch State(gate) {
		case StateMet:
			summary.Met++
		case StateAbandoned:
			summary.Abandoned++
		case StateUnchecked, StateEvidencePending:
			summary.Unmet++
		}
	}
	summary.Complete = summary.Unmet == 0
	return summary
}
