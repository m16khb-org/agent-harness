# 이슈 #100 — hookinput 내용 heuristic 화이트리스트 전환

이슈: https://github.com/m16khb-org/issueops/issues/100

## 재평가 결론 (design-review devil's-advocate revise 반영)

#95의 재평가 조건 충족. 근거 체인(design-review 지적으로 PreToolUse 직접 근거 보강):

- Codex upstream `pre_tool_use.rs` `PreToolUseCommandInput`: `session_id, turn_id, agent_id, agent_type, transcript_path, cwd, hook_event_name, model, permission_mode, tool_name, tool_input, tool_use_id` — **top-level 명령·경로 문자열 없음**, "Shell-like tools pass `{ "command": ... }` as `tool_input`" 원문 명시. `post_tool_use.rs`는 여기에 `tool_response`만 추가.
- Claude Code 스키마 동형(레포 fixture·CAUTIONS 537-538 관측과 정합).
- top-level 문자열 추출 의존은 합성 fixture 1건(`hook_input_test.go` top-level `note`) — design-review 전수 조사로 깨지는 테스트가 이 1개뿐임을 확인.

전환한다.

## 변경

1. `hook_paths.go` walk의 string 케이스(`*** Begin Patch` 스캔 + `.go`/`.issueops`/`testdata/` 내용 heuristic)를 **`insideToolInput` 또는 부모 키가 `command`/`cmd`일 때만** 수행하도록 게이트.
   - command/cmd carve-out 결정(design-review 지적 명시 해소): `CommandFromHookInput`(hook_tool_input.go)이 top-level `command`/`cmd`를 여전히 1순위 추출하는데, 게이트가 같은 문자열의 patch 스캔만 소리 없이 잃으면 legacy top-level command 형상에서 worktreeguard가 block→allow로 약화된다(`ShellCommandGuardPaths`는 heredoc patch 본문을 파싱하지 않아 base 폴백). 실제 host는 양쪽 다 command를 tool_input에 싣지만, stale hook 현실(CAUTIONS 537)을 감안해 command 계열 키는 스캔을 보존해 대칭을 유지한다.
2. 키 기반 추출(path·`_path`·file·filename)·filesystem alias·tool_response/tool_result/transcript 계열 subtree 제외는 불변. 갱신되는 fixture에 **키 기반 추출 보존이 의도**라는 주석을 남긴다(다음 재평가 반복 방지, design-review 지적).

## TDD 순서

1. RED: 비-tool_input·비-command 위치의 경로형 문자열·patch 문자열(top-level `note`, 임의 중간 맵)이 추출되지 않음을 단언 → 현재 코드에서 실패. command/cmd 키의 patch 스캔 보존도 함께 단언(현재도 통과 — 계약 고정).
2. GREEN: string 케이스에 게이트 추가.
3. 계약 갱신: `TestPathsFromHookInputCollectsExplicitPatchAndInlinePaths`의 top-level `note`·`patch` 기대를 새 계약(미추출)으로 갱신, tool_input 내부 동등 케이스로 보존 증명, 키 기반 보존 의도 주석.
4. 회귀: hookcli 전체·전체 모듈 green.

## 비범위

- 키 기반 추출 규칙 변경, CommandFromHookInput 변경.

## 위험

- 미지의 host 필드에 정당 경로 문자열 가능성 — 양 host `pre/post_tool_use.rs` 전수 확정으로 수용, 발견 시 해당 키를 키 기반 추출로 승격하는 좁은 경로 존재.

## 역할 분담

- 계획·리뷰 설계: 메인 세션(Fable 5). 구현(TDD 편집·테스트 실행): Opus 5 서브에이전트(사용자 지시) — 결과 diff·테스트 증거는 메인 세션이 검증 후 커밋.
