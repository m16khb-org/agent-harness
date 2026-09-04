# 이슈 #95 — PostToolUse tool_response 경로 오추출 수정

이슈: https://github.com/m16khb-org/issueops/issues/95

## 문제

`PathsFromHookInput`(`cmd/issueops/hookcli/hookinput/hook_paths.go`)의 walk가 stdin 전체를 순회하며, 단일행 문자열이 `.go`/`.issueops`/`testdata/`를 포함하면 경로로 승격시킨다(line 37 heuristic). PostToolUse stdin에는 `tool_response`(diff/patch/결과 텍스트)가 포함되므로 응답 텍스트 속 repo-상대 문자열이 경로로 오추출되고, base(소스 체크아웃) 기준으로 해석되어 `SourceCheckoutMisdirectWarning` 오탐과 `source_misdirect_warnings` 지표 오염을 일으킨다. 실측: cycle io-68daee41e071에서 canonical worktree 내 plan 파일 Edit 직후 소스 체크아웃 misdirect 경고 1건 적립, 소스는 clean.

## 변경 (design-review devil's-advocate revise 반영)

`hook_paths.go`의 walk 키 필터에 `tool_response`·`tool_result` **subtree 제외**를 추가한다(값 추출과 하위 재귀 모두 건너뜀 — `transcript_path` 선례와 동일하게 `_path` suffix 검사 이전에 continue, `insideToolInput` 게이트로 tool_input 내부 동명 키는 보존). `tool_input` 하위 추출(file_path·filesystem alias·patch 문자열·내용 heuristic)은 불변.

왜 화이트리스트(내용 heuristic을 insideToolInput으로 게이트)가 아닌가: 그것이 본질 수정임은 인정하되, top-level 문자열 추출에 의존하는 기존 fixture(hook_input_test.go의 top-level note 케이스)의 계약 변경을 수반해 회귀 반경이 커진다. 이번 수정은 관측된 결함 클래스(도구 결과 echo 텍스트)의 최소 증분이며, 블랙리스트 성장(transcript_path→agent_transcript_path→tool_response·tool_result) 위험은 이 문단과 이슈 본문에 기록해 다음 오탐 시 화이트리스트 전환을 재평가한다.

Codex 커버리지 한계: `tool_response` 키명 근거는 전부 Claude 형상이다. Codex PostToolUse payload의 출력 필드명은 저장소 내 fixture·관측이 없어 미검증 — live payload 캡처 전에는 Codex 포함 수정 완료를 주장하지 않는다(후속 항목). `tool_result`는 저비용 방어적 추가로 포함하되 투기적 일반화임을 명시한다.

기존 적립분 처리: io-68daee41e071에 이미 적립된 `source_misdirect_warnings: 1`은 해당 사이클의 `cleanup finish`가 레코드를 삭제하며 함께 소멸한다 — 별도 reset 경로는 만들지 않는다.

부수 개선: 같은 paths를 쓰는 doc-upkeep 이벤트(`RecordLifecycleToolUse`)와 lint-gate(`LintEditedGoFiles`)의 tool_response 노이즈도 함께 제거된다. tool_response 경로에 의존하는 정당한 소비자는 없다(ADR의 "PostToolUse는 tool_response를 파싱하지 않는다" KILL 결정).

## TDD 순서

1. RED(추출기): 실제 사고 형상(Claude PostToolUse 전체 키)을 본뜬 fixture 2변형 — (a) `.go` 포함 단일행 diff 문자열형 tool_response, (b) 중첩 `file_path` 키와 `*** Begin Patch` 문자열을 품은 구조체형 tool_response — 에서 오추출을 단언하는 테스트가 수정 전 실패. 동시에 (c) tool_input 내부 `tool_result` 키 인자 보존 단언.
2. RED(통합): fake execution record + 사고 형상 stdin으로 PostToolUse 훅 경로에서 `misdirect_warning`이 빈 문자열임을 단언 — 수정 전 실패.
3. GREEN: walk에 tool_response/tool_result subtree 제외 추가.
4. 회귀: hookinput·hookcli 패키지, 전체 모듈 테스트 green.

## 수용 기준 매핑

- AC-01: tool_response 하위 문자열·중첩 키 미추출 — RED→GREEN 3케이스 + 통합 테스트.
- AC-02: tool_input 추출 불변 — 기존 테스트 + (c) 보존 케이스.
- AC-03: 전체 green — 모듈 테스트.

## 비범위

- `SourceCheckoutMisdirectWarning` 판정 로직, 내용 heuristic 자체 제거(화이트리스트 전환은 다음 오탐 시 재평가), Codex live payload 캡처(후속).

## 위험

- tool_response에서 정당한 경로 신호를 잃을 가능성 — 경로 추출 계약은 도구가 하려던 일(tool_input)이므로 수용. PreToolUse에는 tool_response가 없어 차단 경로 영향 없음.
- paths가 0이 되면 base(cwd) fallback으로 target이 잡히는 설계는 유지된다 — owner가 소스 체크아웃 cwd에서 도구를 돌리면 misdirect가 뜨는 것은 의도된 동작이며 회귀로 오인하지 않는다.
