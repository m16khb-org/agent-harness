# Karpathy-First 프롬프트 증강 훅 설계

작성: 2026-07-03 · 개정: 사용자 방향 확정 반영 · 상태: **구현 완료** (커밋 대기)

> 구현 노트 (설계와 달라진 점):
> - 지시 블록은 여러 줄 `[prompt-engineering-first]` 블록 대신 기존 배너 형식과 일치하는 **컴팩트 한 줄**
>   (`- prompt-engineering-first: …`)로 구현 — Codex 전사 인라인 렌더 제약(컨텍스트 길이 테스트 상한
>   500→900자 의도적 조정, `hook_prompt_test.go`에 주석 명시).
> - 제외 조건에 "2룬 이하 극단문" 추가 (예: "x", "y", "응" — 증강할 원문 없음).
> - systemMessage 가시성은 **claude/reasonix 호스트 전용** — Codex 어댑터는 userView가
>   힌트 컨텍스트를 대체하는 의미라 별도 채널이 없음(`hook_user_prompt.go` 주석 명시).
> - 구현 파일: `internal/core/hookprompt/karpathy_first.go`(판정+상수),
>   `hook_prompt.go`(통합), `rules.go`(dispatch 키워드 확장),
>   `cmd/issueops/hookcli/hook_user_prompt.go`(systemMessage 방출),
>   테스트 `karpathy_first_test.go` + CLI 가시성 테스트 2건.
> - §6 kill switch 구현 완료: request field `DisableKarpathyFirst`,
>   CLI `--disable-prompt-engineering-first`, env `ISSUEOPS_DISABLE_KARPATHY_FIRST`.
> - hook-metrics 발동/제외 태깅(§3.5)은 이번 패스에서 **미구현** — dogfood 관찰은
>   systemMessage 육안 확인으로 시작하고, 계측 필요성이 확인되면 후속 작업으로 추가.

## 1. 확정된 방향 (사용자 결정)

유저가 프롬프트를 입력하면 메인 에이전트가 **바로 작업을 시작하지 않는다**. 대신:

1. UserPromptSubmit 훅이 "prompt-engineering-first" 지시를 additionalContext로 주입한다.
2. 메인 에이전트가 같은 턴 안에서 prompt-engineering 스킬(one-shot 경량 패스)로 유저 원문을
   **증강된 프롬프트**로 재작성한다.
3. 증강 프롬프트를 응답 서두에 명시적으로 보여준 뒤, **그 증강 프롬프트를 기준으로**
   작업을 진행한다.

핵심: 증강의 실행 주체는 훅(Go)도 외부 LLM도 아닌 **메인 에이전트 자신**이다. 훅은
증강을 강제하는 지시자 역할만 한다. external LLM 호출안(구 Phase 3)은 **폐기**.

기존 자산 (재사용, 재발명 금지):
- `internal/core/hookprompt/rules.go:171-176` — prompt-engineering 라우팅 룰. 현재는 키워드 매칭
  시에만 발동하는 "권고 힌트". 본 설계는 이를 "필수 선행 단계 지시"로 격상.
- `internal/core/hookprompt/hook_prompt.go:40-97` — UserPromptSubmit 힌트 빌더(주입 지점).
- `~/.claude/skills/prompt-engineering/SKILL.md` — one-shot 경량 패스 규정: 입출력 계약,
  제약 상단·포맷 하단 배치, 1-2 sanity check. 증강 시 이 경량 패스만 적용
  (풀 테스트 스위트·A/B·버저닝 세리머니는 one-shot 프롬프트에 부적용이 스킬 자체 규정).

## 2. 하드 제약 (설계를 결정하는 사실)

1. UserPromptSubmit은 프롬프트를 치환할 수 없다(`additionalContext`만 지원). →
   원문은 항상 함께 전달되며, 증강본은 에이전트가 턴 안에서 생성한다. 원문이 남아 있으므로
   증강이 의도를 왜곡하면 유저가 즉시 발견·교정할 수 있다(감사 가능성 확보).
2. user-prompt 훅은 매 턴 동기 핫패스다. → 훅 자체는 결정적 문자열 주입만 수행(~0ms).
   비용은 훅이 아니라 **에이전트 턴 내부**(증강 단계의 토큰/시간)로 이동한다.
3. prompt-engineering SKILL.md는 466줄이다. 매 턴 Skill 도구로 풀 로드하면 턴당 수천 토큰이
   반복 소모된다. → 지시 블록에 **증류된 one-shot 체크리스트를 내장**해 스킬 미로드로도
   증강이 가능하게 하고, 복잡한 턴에서만 풀 스킬 로드를 허용한다(§3 발동 정책).

## 3. 설계

### 3.1 훅 주입 블록 (결정적, LLM 없음)

`BuildUserPromptMCPHints`(hook_prompt.go:40)에 증강 지시 단계를 추가. 발동 시
additionalContext에 다음 블록을 주입한다:

```
[prompt-engineering-first]
이 턴은 원문 요청을 바로 실행하지 말 것. 먼저 prompt-engineering one-shot 패스로 아래 원칙에 따라
증강 프롬프트를 작성하고, 응답 서두에 "증강된 요청:"으로 명시한 뒤 그것을 기준으로 진행하라:
- 입출력 계약: 대상·범위·경계 / 산출물 형식·완료 판정 기준을 명시
- 제약 우선 배치: 금지사항·불변 조건을 증강 프롬프트 상단에 고정
- 모호성 해소: 다의적이면 가장 그럴듯한 해석 1개를 명시 채택 (원문 의도 왜곡 금지)
- 복잡 작업(다단계·다파일·서브에이전트 dispatch 예정)이면 prompt-engineering 스킬을 로드해 정식 적용
증강 프롬프트가 원문과 실질적으로 동일하면 "증강 불필요" 한 줄로 표기하고 바로 진행하라.
```

마지막 줄이 중요: 사소한 프롬프트에 증강 세리머니를 강제하면 매 턴 노이즈가 되므로,
에이전트에게 스킵 판단 여지를 명시적으로 준다.

### 3.2 발동 정책 — 기본 발동, 제외는 최소화

사용자 피드백 반영: "너무 발동 안 한다" → 키워드 매칭으로 발동을 좁히지 않는다.
**기본은 전 프롬프트 발동**이고, 제외는 증강할 원문 자체가 없는 경우로만 한정한다:

| 제외 조건 | 근거 |
|---|---|
| 슬래시 커맨드(`/`로 시작) | 이미 스킬이 프롬프트를 정의 |
| 선택지 응답(`1`, `2번`, "추천대로" 등 극단문 지시) | 증강할 원문이 없음 |
| `그대로:` 접두사 | 사용자 명시 opt-out |

(초안에 있던 "N자 미만 단문 제외" 임계는 삭제 — 짧은 요청일수록 모호해서 오히려 증강
가치가 크고, 사소한 경우는 지시 블록 마지막 줄의 "증강 불필요" 스킵이 에이전트 판단으로
처리한다. 훅은 넓게 발동하고, 세밀한 스킵은 에이전트에 위임하는 2단 구조.)

보완: 기존 prompt-engineering 라우팅 룰(rules.go:174-175)의 키워드도 확장한다 — 이 룰은
prompt-engineering-first와 별개로 "서브에이전트 dispatch 하드닝" 힌트를 담당하므로, "구현해",
"만들어줘", "분석해줘", "리팩토링" 등 dispatch 가능성이 높은 동사류를 추가해 발동률을
높인다.

### 3.3 사용자 가시성 — 발동 사실을 유저가 알 수 있어야 한다

두 계층으로 보장한다:

1. **훅 레벨 (터미널 표시)**: 발동 시 hook 출력에 `systemMessage`를 함께 방출한다.
   기존 경로 재사용 — `internal/adapter/hook/output.go:134`가 이미 userView를
   systemMessage로 방출하는 패턴을 갖고 있다. 표시 예:
   `🧪 prompt-engineering-first 발동 — 에이전트가 증강 프롬프트를 먼저 작성합니다 (스킵: "그대로:" 접두사)`
   제외된 턴에는 아무것도 표시하지 않는다(무발동 = 무노이즈).
2. **에이전트 레벨 (응답 본문)**: 증강 수행 시 응답 서두에 `증강된 요청:` 블록을 반드시
   표기(3.1 지시 블록에 명시). 유저는 원문이 어떻게 재해석됐는지 눈으로 확인하고 어긋나면
   즉시 교정할 수 있다. "증강 불필요" 스킵도 한 줄로 표기되므로 침묵 스킵은 없다.

### 3.4 서브에이전트 dispatch 연쇄 (기존 룰과의 접합)

rules.go:171의 기존 prompt-engineering 룰(서브에이전트 dispatch 프롬프트 하드닝)은 그대로 유지된다.
결과적으로 2단 증강 체계: 유저 원문 → (prompt-engineering-first) 증강 프롬프트 → 서브에이전트
dispatch 시 dispatch 프롬프트도 prompt-engineering 하드닝. PreToolUse `updatedInput` 기반 강제
치환(구 Phase 2)은 **후속 선택지로 보류** — 에이전트가 지시를 따르는 한 불필요하며,
계약 골든 변경을 수반하므로 별도 결정 지점.

### 3.5 관측 (성공 판정 근거)

- 주입 발동/제외 카운트를 기존 hook-metrics 이벤트에 태그로 기록(새 상태 파일 금지 —
  v3 계획의 무한성장 파일 교훈).
- dogfood 기간 중 "증강된 요청:" 표기 빈도와 "증강 불필요" 비율을 관찰해 임계값(N자)과
  제외 조건을 조정한다.

## 4. 비목표 (Non-goals)

- 훅/외부 LLM에 의한 프롬프트 치환·정제 (사용자 확정: 증강 주체는 메인 에이전트).
- 매 턴 prompt-engineering SKILL.md 풀 로드 강제 (증류 체크리스트로 대체, 복잡 턴만 풀 로드).
- 사용자 원문의 폐기 — 증강본은 원문 옆에 놓이는 실행 기준이지 원문의 대체물이 아니다.
- 새 상태 파일 추가.

## 5. 테스트 계획

- `hookprompt/hook_prompt_test.go`: 주입/제외 판별 — 슬래시 커맨드, 선택지 응답
  ("1", "2번", "추천대로"), `그대로:` 접두사 strip 및 라우팅 비오염, 그 외 전 프롬프트
  기본 주입 확인 (짧은 일반 요청도 주입되는지 포함).
- 렌더 골든: `[prompt-engineering-first]` 블록 포맷 (기존 `pioneer_routing_test.go` 패턴 준수).
- 가시성: 발동 시 훅 출력에 systemMessage 포함, 제외 시 미포함
  (`adapter/hook/output.go` 경로 테스트).
- rules.go 키워드 확장: 확장 동사류("구현해", "만들어줘" 등) 매칭 테스트 추가.
- 성능: 변경 전/후 `hook user-prompt` p50 재측정 (결정적 주입이므로 회귀 없음을 수치 확인).
- 행동 검증(dogfood): 실제 세션에서 증강 프롬프트가 서두에 표기되고 그 기준으로 작업이
  진행되는지, 사소 프롬프트에서 "증강 불필요" 스킵이 작동하는지 관찰.

## 6. 후속 피처 A — 세션/전역 kill switch (구현 완료)

**목표**: dogfood 중 증강이 방해되면 revert/재빌드 없이 환경변수 하나로 기능을 끈다.
프롬프트 단위 opt-out(`그대로:`)의 상위 스위치.

**설계**: 기존 `ISSUEOPS_ENABLE_LLM_HINTS` 패턴(`hook_user_prompt.go:40`,
`hookenv.Bool`)을 그대로 복제한다. core는 env를 직접 읽지 않고 요청 필드로 받는다
(테스트 용이성 + 기존 관례).

| 변경 파일 | 내용 |
|---|---|
| `internal/core/hookprompt/hook_prompt.go` | `HookUserPromptRequest`에 `DisableKarpathyFirst bool` 추가; true면 발동 강제 해제(접두사 strip은 유지) |
| `cmd/issueops/hookcli/hook_user_prompt.go` | `--disable-prompt-engineering-first` 플래그 + `hookenv.Bool("ISSUEOPS_DISABLE_KARPATHY_FIRST")` OR 결합 |
| 테스트 | core: 필드 true 시 지시/notice 미발동 + 라우팅 정상. CLI: `t.Setenv`로 env 켜고 systemMessage/지시 라인 부재 확인 |

- **계약 영향**: 요청 필드 추가는 omitempty 없는 입력 계약이지만 request는 golden 대상
  아님(response만 golden). 영향 없음 예상 — 구현 시 `go test ./cmd/issueops/...`로 확인.
- **검증 기준**: env 설정/해제 각각에서 실바이너리 출력 확인 + 패키지 테스트 green.
- **구현 근거**: `go test ./internal/core/hookprompt ./cmd/issueops/hookcli -count=1` green.
- **규모/리스크**: ~30 LOC, Low. 롤백은 커밋 revert.

## 7. 후속 피처 B — Phase 2: PreToolUse `updatedInput` dispatch 프롬프트 강제 하드닝 (착수 보류)

**착수 조건(선행)**: dogfood에서 에이전트가 prompt-engineering-first 지시를 무시하고 서브에이전트를
날프롬프트로 dispatch하는 사례가 관찰될 것. 관찰 전 착수 금지(§6-4, YAGNI).

2026-07-03 현재 repo 검색에서 관련 관찰 기록은 확인되지 않았다(`rg "날프롬프트|prompt-engineering-first 지시를 무시|updatedInput|harden-subagent"` 결과가 이 계획 문서에만 한정). 따라서 아래 구현 계획은 유지하되 착수하지 않는다.

### Dogfood 관찰 로그

- 2026-07-03 15:58 KST: `./bin/issueops hook user-prompt --host claude --prompt '§7 dogfood 관찰 진행'` 결과 `additionalContext`에 `- prompt-engineering-first:`가 포함되고 `systemMessage`로 prompt-engineering-first notice가 표시됨. 동일 입력의 `--json` 결과도 `karpathy_first: true`.
- 2026-07-03 15:58 KST: repo 검색(`rg "날프롬프트|prompt-engineering-first 지시를 무시|updatedInput|harden-subagent" .issueops internal cmd skills scripts AGENTS.md CLAUDE.md`) 결과는 이 계획 문서의 설계/게이트 문구에만 한정됨.
- 판정: 이번 dogfood에서는 main agent가 prompt-engineering-first 지시에 따라 증강 요청을 먼저 표기했고, sub-agent dispatch도 발생하지 않았다. §7 착수 조건인 "지시 무시 + raw sub-agent prompt dispatch"는 **미관찰** 상태이므로 구현 착수 금지 상태를 유지한다.

### §7 착수 후보 관찰 템플릿

§7 착수 조건을 여는 관찰은 단순 체감이나 기억이 아니라 아래 템플릿을 채운 재현 가능한 기록이어야 한다. 전문 prompt를 문서에 붙이지 말고 bounded excerpt, hash, length만 기록한다.

```text
- 시각/host/session:
  - observed_at_kst:
  - host: codex|claude|reasonix
  - session_or_transcript:
- 원문 prompt와 hook evidence:
  - user_prompt_excerpt:
  - hook_json_command:
  - karpathy_first: true|false
  - additional_context_has_karpathy_first: true|false
  - user_visible_notice: present|absent|unknown
- main-agent 응답 evidence:
  - response_started_with_augmented_request: true|false
  - response_started_with_no_augmentation_needed: true|false
  - transcript_line_or_summary:
- dispatch evidence:
  - tool_name: Task|Agent
  - tool_prompt_hash:
  - tool_prompt_length:
  - tool_prompt_bounded_excerpt:
  - raw_prompt_evidence:
- 영향/필요성:
  - missing_contract_or_guardrail:
  - observed_failure_or_rework:
- 재현/반복성:
  - reproduction_count:
  - high_risk_single_case_reason:
- 판정:
  - opens_section_7_gate: true|false
  - next_step: keep advisory|run updatedInput spike
```

판정 기준: `opens_section_7_gate=true`는 UserPromptSubmit에서 prompt-engineering-first가 주입됐는데도 main agent가 `증강된 요청:`/`증강 불필요` 없이 `Task`/`Agent`를 raw prompt로 dispatch했고, 그 결과 누락된 계약·제약·tool truth·privacy guardrail 중 하나 이상이 실제 실패나 재작업으로 이어졌을 때만 허용한다.

**목표**: `Task`/`Agent` tool 호출의 `prompt` 파라미터를 PreToolUse에서 결정적으로
하드닝해 치환한다(진짜 "바꿔서 들어가게"가 가능한 유일한 지점). LLM 호출 없음 —
PreToolUse는 매 tool 호출 크리티컬 패스이므로(`hook_pre_tool_use.go:69-71` 주석)
결정적 문자열 연산만 허용.

**단계별 변경**:

1. **입력 파싱** — `hookinput`에 `SubagentPromptFromHookInput` 추가: `tool_name`이
   `Task`/`Agent`일 때 `tool_input.prompt`(string)와 **전체 tool_input 맵**을 반환.
   updatedInput은 부분 패치가 아니라 수정된 입력 전체를 담아야 하므로 원본 맵 보존 필수.
2. **하드닝 함수** — `internal/core/hookprompt/karpathy_dispatch.go`:
   `HardenSubagentPrompt(prompt) (string, bool)`. 컴팩트 계약 헤더(입출력 계약·제약
   상단·포맷 하단 지시)를 프리펜드. **멱등성 마커**(헤더 첫 줄 고정 문자열) 존재 시
   무변경 반환 — 재시도/중첩 하드닝 방지.
3. **결과 계약** — `model/types.go`의 `HookPreToolUseDecisionResult`에
   `UpdatedInput map[string]any \`json:"updated_input,omitempty"\`` 추가.
   ⚠️ `response_contracts.golden.json` 갱신 필요 — TESTING.md §4에 따라 **의도적 계약
   변경임을 커밋 메시지에 명시**하고 diff를 사람이 리뷰.
4. **어댑터** — `HostHookOutput`에 `FormatUpdatedInput(updatedInput map[string]any)`
   추가. Claude/Reasonix: `hookSpecificOutput{hookEventName, updatedInput}`.
   Codex: updatedInput 채널이 없으므로 Noop(기능은 Claude 계열 전용, 주석 명문화).
   인터페이스 확장이라 어댑터 3종 + 테스트 동시 수정.
5. **CLI** — `--harden-subagent-prompts` 플래그(기본 OFF, 기존 enforce-* 패턴).
   decision이 allow이고 하드닝이 변경을 만든 경우에만 updatedInput 방출, 아니면 Noop.
6. **감사 가시성** — 치환은 사용자에게 보이지 않으므로 발동 시 systemMessage 한 줄
   (`🧪 dispatch 프롬프트 하드닝 적용`) + hook-metrics 이벤트(kind=subagent_prompt_hardened,
   전문이 아닌 before/after 길이·해시만 기록 — 무한성장 방지, c88f472의 bounded 로그 원칙).
7. **사전 검증 스파이크(구현 첫 단계)** — Claude Code가 실제로
   `hookSpecificOutput.updatedInput`을 적용하는지 최소 훅으로 실증(무해한 필드 에코 치환).
   호스트 버전에 따라 미지원이면 전체 계획 중단하고 advisory 유지 — 이 스파이크가
   게이트다.

**테스트**: hookinput 파싱(Task/Agent/기타 tool/입력 없음), 하드닝 멱등성·마커 보존,
CLI updatedInput 페이로드 형태(claude)·Noop(codex/플래그 OFF), golden 갱신 diff 리뷰,
`hook pre-tool-use` p50 전/후 재측정(크리티컬 패스 무회귀 필수).

**규모/리스크**: ~250 LOC + golden 1건. 리스크: (a) updatedInput 호스트 지원 여부 —
스파이크로 게이트, (b) 이중 하드닝 — 멱등성 마커로 차단, (c) Codex 비대칭 — 명문화로
수용. 롤백은 플래그 OFF(기본값)로 즉시, 코드 제거는 revert.

**실행 순서**: 7(스파이크 게이트) → 1·2(파싱+하드닝, 병행 가능) → 3(계약+golden) →
4·5(어댑터+CLI) → 6(감사) → 테스트/측정 → 플래그 OFF로 커밋 → dogfood 레포에서만
플래그 ON.

## 8. 후속 피처 C — 선택지 응답 복원 증강 (구현 완료)

**목표**: 사용자가 "1", "2번", "추천대로" 같은 선택지 응답만 입력해도, 그 선택지의
**전문을 복원**해 prompt-engineering-first 증강 대상으로 삼는다. (기존에는 선택지 응답을 통째로
증강 제외했음.)

**구조**: 훅은 대화 전사에 접근할 수 없지만, Stop 훅의 next-action relay가 선택지를
이미 파싱한다는 점을 활용:

1. `StopNextActionRelayRecord`에 `Candidates [](Index/Recommended/Text)` 추가
   (`model/types.go`) — Stop 훅 기록 시점에 전체 선택지 전문을 저장. additive 필드,
   스키마 버전 유지, 상태 레코드라 response golden 무관.
2. `nextactionrelay.Read` 신설 — 무변경 읽기. `lifecycle.ReadStopNextActionRelay` →
   `hookprompt` dependencies 배선.
3. `hookprompt.resolveChoiceExpansion` — 숫자 선택("2번")과 "추천대로"/"추천"만 복원
   대상(그 외 ack는 비확장). 6시간 초과 stale 레코드 무시. 복원 성공 시
   `사용자가 선택지 N번을 선택했다: "<전문>" — 이 선택지를 원문 요청으로 삼아 …` 지시와
   `🧪 … 선택지 N번 내용을 복원해 증강합니다` notice 주입.
4. CLI에서 relay clear를 Build **뒤로** 이동 — 복원이 소비보다 먼저.
   clear 동작 자체는 유지(소비 후 삭제, 중복 relay 억제 해제).
5. 게이트 공유: `ISSUEOPS_DISABLE_KARPATHY_FIRST`와 `그대로:` 접두사(`그대로: 1`)는
   복원 증강도 함께 끈다.

**한계(수용)**: relay 레코드가 있을 때만 동작하는 best-effort — Stop enforcement가
없는 레포에선 자연 비활성이고, 판단 턴이 tool 호출을 하면 post-tool-use가 레코드를
중간에 지울 수 있다(그 경우 기존 동작으로 무해하게 폴백).

## 9. 실행 순서 (전체)

1. 3.1 지시 블록 + 3.2 발동 정책 구현 (rules 또는 hook_prompt.go 증강 단계) + 테스트.
2. p50 전/후 측정으로 훅 지연 무회귀 확인.
3. dogfood 1~2주: 발동률·"증강 불필요" 비율·체감 품질 관찰 → 임계값/제외 조건 조정.
4. (보류 선택지) 에이전트가 지시를 무시하는 사례가 관찰되면 그때 PreToolUse
   `updatedInput` 강제 치환을 재평가.
