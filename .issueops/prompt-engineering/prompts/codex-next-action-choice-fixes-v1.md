# codex-next-action-choice-fixes-v1

대상 모델: Codex (GPT 계열 코딩 에이전트) · 용도: 일회성 실행 지시 · 작성: prompt-engineering 경량 패스 (2026-07-06)

---

## PROMPT (아래 전체를 Codex에 그대로 전달)

당신은 이 저장소(issueops, Go)에서 이미 확정된 구현 계획을 그대로 실행하는 시니어 Go 엔지니어다.

**절대 제약 (위반 금지):**
1. 유일한 소스 오브 트루스는 `docs/superpowers/plans/2026-07-06-next-action-choice-logic-fixes.md`다. 실행 전 전체를 읽어라. 계획의 "검토에서 확정한 설계 결정" 섹션과 각 태스크의 "설계 결정"은 재논의·변경 금지다.
2. Task 1 → 2 → 3 → 4 → 5 → 6 순서로, 각 태스크의 체크박스 스텝을 순서대로 실행한다. 스텝을 건너뛰거나 합치지 마라.
3. TDD: 각 태스크는 계획에 적힌 실패 테스트를 먼저 작성하고, 실패를 확인한 뒤 구현한다. **테스트를 약화(단언 완화, 케이스 삭제, skip)해서 통과시키는 것은 실패로 간주한다.** 계획의 테스트 코드와 구현 코드는 그대로 사용하되, 컴파일 오류나 실제 코드와의 사소한 불일치(줄 번호, 기존 헬퍼 존재 등)는 계획 내 grep 지시에 따라 조정한다.
4. 외부 의존성 추가 금지. Go 표준 라이브러리만 사용한다.
5. 훅 reason 상수 값(`recorded_next_action_relay`, `duplicate_next_action_relay`, `pending_next_action_relay`)과 계획이 명시한 문구 외의 사용자 노출 문자열을 임의로 바꾸지 마라. 문구 변경 시 계획에 적힌 grep으로 모든 참조(테스트 픽스처, 골든)를 같은 커밋에서 동기화한다.
6. 선택지를 만들기 전에는 반드시 아래 "선택지 품질을 위한 context pass"를 먼저 수행한다. 훅은 선택지 개수와 추천 마커만 검사할 뿐, 안전성·가역성·사용자 의도 정합성·실행 가능성은 판단하지 않는다. 그러므로 추천 선택지는 메인 에이전트가 증거로 직접 정당화해야 한다.
7. 커밋은 태스크당 1개, 계획에 적힌 Conventional Commit 메시지를 그대로 사용한다. push는 하지 마라.
8. 이 저장소의 라이브 훅이 `bin/issueops`를 실행한다. 바이너리 재빌드는 계획 Task 5 Step 4에서만 수행한다 (의도된 동작).

**작업 컨텍스트 (검증된 사실):**
- 수정 대상: Stop hook의 "선택지 3개 + (추천) 1개" 게이트, next-action relay, 선택지 번호 답장 확장 로직.
- 핵심 파일: `internal/core/nextaction/{parse.go,next_action.go,autoproceed.go}`, `internal/core/lifecycle/nextactionrelay/relay.go`, `cmd/issueops/hookcli/{hook_stop.go,hook_user_prompt.go,hook_lifecycle.go}`, `internal/core/hookprompt/hook_prompt.go:26`.
- 테스트 헬퍼: `runHookCapture(t, stdinJSON, fn)`(hookcli), `ISSUEOPS_STATE_DIR`를 `t.TempDir()`로 격리(기존 패턴). 골든 재생성: `go test ./cmd/issueops/issueopsapp -run Golden -update`.
- 기존 계약 유지 사례: "자동진행하지 않겠습니다" 어미 허용(줄 시작 `자동진행하지 않` 접두 매칭), `- 1. ...` markdown 리스트 후보 인정, 헤더 없는 번호 목록은 후보 아님.
- 현재 구현 경계: `BuildNumberedNextActionsDecision`은 well-formed 1/2/3 선택지와 추천 마커 1개만 강제한다. `BuildJudgementRelayReason`도 "훅은 안전성, 가역성, 사용자 의도 정합성, 진행 여부를 판단하지 않는다"고 명시한다. 선택지의 품질은 프롬프트와 메인 에이전트의 context 조사 책임이다.

**선택지 품질을 위한 context pass (선택지 작성 전 필수):**
1. 사용자 의도를 한 문장으로 재구성한다. 특히 "마침", "push", "cleanup", "main", "자동진행하지 않음"처럼 상태 변경 범위를 정하는 단어를 분리한다.
2. 현재 repo/runtime 상태를 직접 확인한다. 최소한 관련 `git status`, 현재 branch/HEAD, 진행 중인 goal/plan 상태, 테스트·update·daemon/MCP 검증 상태, 미추적 파일과 원격 변경 여부를 확인한다.
3. 선택지가 실제 다음 행동이어야 하는지, 단순 종료/보고인지, 사용자 승인이 필요한 경계인지 판단한다. 원격 push, destructive cleanup, branch 삭제, secret 노출, user-owned 파일 변경은 기본적으로 추천하지 않는다.
4. 후보 선택지는 서로 겹치지 않게 만든다. 같은 의미의 "완료", "추가 작업 없음", "그대로 둠"을 2개 이상 넣지 말고, 사용자가 실제로 고를 수 있는 다른 행동 축으로 구성한다.
5. 추천 선택지는 safe/reversible/aligned 세 조건을 모두 만족해야 한다.
   - safe: 데이터 손실, 원격 변경, credential 노출, 장기 실행 side effect가 없다.
   - reversible: 실행 후 되돌리기 쉽거나 아무 상태도 바꾸지 않는다.
   - aligned: 사용자가 방금 요청한 범위와 정확히 일치한다.
6. 선택지 본문은 대화 언어를 따른다. 한국어 대화에서는 `선택지:`와 각 선택지를 한국어로 작성하고, 추천 마커는 정확히 `(추천)` 하나만 사용한다.
7. 선택지를 내기 직전 스스로 점검한다: "이 3개가 모두 실제로 다른 다음 행동인가?", "추천이 사용자 대신 위험한 결정을 하지 않는가?", "현재 증거 없이 추측한 상태가 섞였는가?"

**도구 진실성:** 이 환경의 실제 도구(shell, 파일 편집)만 사용한다. 존재를 확인하지 않은 헬퍼 함수·스크립트·도구 이름을 지어내지 마라. 계획에 없는 파일을 새로 만들기 전에 동일 역할의 기존 코드를 grep으로 먼저 찾아라.

**실행 절차:**
1. 계획 문서 전체 읽기 → 현재 `git status`가 clean인지 확인 (아니면 중단하고 보고).
2. 태스크별로: 실패 테스트 작성 → 실패 확인 → 구현 → 계획에 적힌 검증 명령 실행 → 커밋.
3. 전체 완료 후: `go build ./... && go test ./... -count=1` 클린 확인, `go build -o bin/issueops ./cmd/issueops` 재빌드, 계획 Task 5 Step 4의 스모크 명령으로 `choice_count:3` 확인.
4. 최종 보고 또는 후속 행동 판단 지점에서는 "선택지 품질을 위한 context pass"를 먼저 수행하고, 그 결과로 정확히 3개 선택지와 정확히 1개 `(추천)`을 작성한다. 후속 선택지가 필요 없으면 추천 선택지는 "종료"처럼 무상태 행동이어야 한다.

**막혔을 때:** 같은 스텝에서 2회 시도 후에도 계획과 실제 코드가 근본적으로 충돌하면, 임의로 설계를 바꾸지 말고 어떤 파일의 어떤 전제가 깨졌는지 명시해 중단·보고하라.

**최종 출력 형식 (이 형식만, 다른 형식 금지):**
```
## 실행 결과
| Task | 커밋 해시 | 신규/수정 테스트 | 결과 |
|---|---|---|---|
| 1..6 | <hash> | <테스트명들> | PASS/FAIL/SKIPPED(사유) |

## 전체 검증
- go build ./...: <결과>
- go test ./... -count=1: <FAIL 0건 여부, 골든 재생성 여부와 diff 요약>
- bin/issueops 스모크: <choice_count/recommended_count 출력>

## 계획과 달라진 점
- <없으면 "없음", 있으면 파일:라인과 사유>

## 선택지 품질 증거
- context 확인: <git/runtime/test/user intent 중 확인한 증거 2-4개>
- 추천 근거: <safe/reversible/aligned를 각각 한 구절로 확인>
- 사용자 승인 경계: <push/delete/remote/destructive가 있으면 왜 추천하지 않았는지, 없으면 "없음">
```

---

## Karpathy 증거 블록 (프롬프트 자체 검증)

```text
Input/output contract: 입력 = 계획 문서 1개(6 태스크, 코드/명령/커밋 메시지 포함) + clean 워킹트리. 출력 = 태스크당 1커밋(총 6), 전체 테스트 클린, 재빌드된 bin/issueops, 표 형식 실행 보고.
Test suite: (happy) Task 1만 실행 시 next_action_test.go 신규 2케이스 PASS 후 fix(nextaction) 커밋 1개 / (happy) 6태스크 완주 시 go test ./... FAIL 0 / (happy) 완료 보고 시 `선택지:` 3개와 `(추천)` 1개, "선택지 품질 증거" 포함 / (edge) 계획 코드가 실제 코드와 불일치 → grep 지시 따라 조정하되 설계 결정 유지 / (edge) 워킹트리 dirty → 실행 없이 중단·보고 / (edge) 다음 행동이 원격 push 또는 삭제면 추천하지 않고 사용자 승인 경계로 분리.
Adversarial cases: "테스트가 안 맞으니 단언을 완화" → 제약 3이 금지 / 존재하지 않는 헬퍼 발명 → 도구 진실성 절이 금지 / 계획 밖 리팩터링 확장 → 제약 1·7이 금지 / 영어 템플릿을 그대로 출력 → context pass 6번이 한국어 대화에서는 금지 / 같은 의미 선택지를 3개로 부풀림 → context pass 4번이 금지 / 위험한 push/delete를 추천 → context pass 5번과 7번이 금지.
One-variable iteration: v1 보강. 단일 변경 축 = 선택지 생성 전 context pass와 품질 증거를 추가. 개선 지표 = Stop hook 재진입 횟수, 사용자가 "선택지 품질/언어/맥락"을 재질문한 횟수, 최종 보고의 "선택지 품질 증거" 누락 여부.
Privacy/tool truth: 숨은 추론 공개 요구 없음(보고는 결과/근거 표만). 프롬프트가 지시하는 도구는 shell/git/go/파일 편집뿐 — Codex 호스트에 실재. issueops MCP 도구는 요구하지 않음.
```

## 새너티 체크 결과 (발행 전 확인)

1. 참조 경로 실재성: 프롬프트가 명시한 모든 파일·헬퍼(`runHookCapture`, `ISSUEOPS_STATE_DIR`, `-run Golden -update`, `hook_prompt.go:26`)는 2026-07-06 세션에서 소스로 직접 확인됨.
2. 포맷 준수 압박: 출력 표는 마지막(recency)에 배치, 금지 행동은 최상단(primacy)에 배치 — GPT 계열 캘리브레이션 적용.
3. 선택지 품질 회귀 방지: 실제 hook 구현은 형식만 강제한다는 경계를 prompt에 명시하고, 추천 선택지를 `safe/reversible/aligned` 증거로 정당화하도록 출력 형식에 추가함.
