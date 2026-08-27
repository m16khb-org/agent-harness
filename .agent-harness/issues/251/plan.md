# #251 Codex exec workdir hook plan

1. shell tool에 한해 effective CWD가 명시적 `tool_input.workdir`를 우선하는 실패 테스트를 추가한다.
2. 빈 값·비문자열·일반 `tool_input.cwd`는 기존 top-level cwd로 fallback하는 계약을 고정한다.
3. active holder 명령과 identity mismatch 회귀를 검증한 뒤 전체 테스트·race·vet·build를 실행한다.

완료 조건은 GitHub issue #251의 AC-01부터 AC-06까지다.
