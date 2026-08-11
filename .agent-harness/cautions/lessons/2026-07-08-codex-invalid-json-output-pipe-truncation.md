---
name: cautions/lessons/2026-07-08-codex-invalid-json-output-pipe-truncation.md
description: Dated lesson — Codex "invalid JSON output" was caused by a co-resident hook truncating piped stdout.
---

# 2026-07-08 — Codex hook "invalid JSON output"의 원인은 동거 훅의 파이프 truncation이었다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: Codex PreToolUse `hook returned invalid pre-tool-use JSON output` 실패 진단 세션
- Summary: 사용자에게 반복 표시된 PreToolUse/SessionStart "invalid JSON output" 실패의 원인은 agent-harness가 아니라, 같은 이벤트에 등록된 claude-mem codex 훅이 stdout이 파이프일 때 JSON을 512바이트에서 잘라 내보낸 것(+ 별건으로 `status` unknown top-level field). agent-harness 훅은 전 경로에서 스키마 유효했다.
- Context: Codex 0.142.5는 hook stdout을 `deny_unknown_fields` serde wire로 파싱하며, `{`로 시작하는 파싱 불가 stdout에만 정확히 이 오류를 낸다. claude-mem worker(node/bun)는 stdout이 파이프면 큰 출력(예: SessionStart context ~19KB, 관측 기록 많은 파일의 file-context)에서 flush 전에 종료해 정확히 512B만 전달했다. 파일 리다이렉트(동기)에서는 전체가 나오기 때문에 단독 실행 재현으로는 잡히지 않았고, `... | cat` 파이프 하류 비교로 확정했다. 계측 시에도 `node ... | tee`처럼 stdout을 다시 파이프로 만들면 잘림이 재유발되는 관찰자 효과가 있다.
- Resolution: (1) 진단 시 "invalid JSON output"은 unknown field / unknown enum / truncated-or-multi-object stdout 세 갈래로 좁힌다. (2) 같은 이벤트에 등록된 모든 훅을 용의선상에 두고(`~/.codex/hooks.json` + `[hooks.state]`의 plugin hooks), 각 훅 stdout을 파이프 하류(`| cat | wc -c`)에서 검증한다. (3) node 기반 훅 명령은 `_O=$(mktemp); ... > "$_O" 2>/dev/null || true; cat "$_O"; rm -f "$_O"` 패턴으로 stdout을 동기 기록 후 전달한다(claude-mem codex-hooks.json 5개 명령에 적용, 백업 `.harness.bak-20260708`). (4) 훅 명령 문자열이 바뀌면 codex trust 해시가 무효화되어 훅이 조용히 스킵되므로, 검증 전 TUI에서 재신뢰가 필요하고 "실패 0"이 "안 돌아서 0"인지 구분해야 한다.
- Evidence:
  - 동일 명령: 파일 리다이렉트 19,489B vs 파이프 512B(`Unterminated string`) 재현
  - openai/codex rust-v0.142.5 `hooks/src/engine/output_parser.rs` parse_json + `events/pre_tool_use.rs`의 오류 문자열 분기
  - 패치 후 `codex exec` e2e: SessionStart 4/4, PreToolUse 2/2 등 전 훅 Failed 0
- Alternatives / rejected options:
  - claude-mem minified worker 코드 직접 수정 — 거부: 업데이트로 유실되고 검증 부담이 큼; 명령 레벨 버퍼링이 더 단순.
  - 훅 stdout 무음화 — context 계열은 모델 컨텍스트 주입이 목적이라 무음화 불가(스키마 위반인 worker-start만 무음 처리).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
