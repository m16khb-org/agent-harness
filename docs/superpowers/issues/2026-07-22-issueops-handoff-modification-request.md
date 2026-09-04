# IssueOps handoff owner 수정 요청 경로 추가

## 문제

IssueOps ownership handoff가 PR/MR을 만든 뒤 source/main 세션에서 수정 요청이
들어와도 active owner에게 안전하게 전달할 정식 경로가 없다. raw terminal
steering이나 unfenced orchestration send는 sealed lifecycle identity와 권한
경계를 우회하고, source가 worker 파일을 직접 수정하면 ownership transfer를
깨뜨린다.

또한 선택지에서 `handoff owner` 실행 계약이 `격리된 구현 worktree`로 축약된
뒤 numeric-choice relay가 축약 문구를 원문 요청으로 승격해 source/main이 직접
구현한 semantic downgrade가 관찰됐다. 이 이슈의 구현은 owner-only mutation을
유지하며, 관련 선택지·coordinator liveness 개선은 #65와 별도 설계 문서에서
추적한다.

## 목표

- PR/MR phase의 active ownership handoff에 fresh exact-source native session이
  bounded 수정 요청을 보낼 수 있다.
- 요청은 sealed coordinator/worker mailbox, task, dispatch identity만 사용한다.
- 외부 전송 전에 durable intent를 기록하고 결과를 같은 request key에 CAS로
  반영한다.
- owner만 feedback, 코드, commit, phase와 publish receipt를 변경한다.
- owner의 재게시 HEAD는 이전 published HEAD의 fast-forward descendant여야 한다.
- CLI와 MCP가 같은 core DTO와 validation을 사용한다.
- handoff 이후 source/main은 non-blocking coordinator로 남는다.

## 완료 기준

- schema v8 optional bounded `modification_requests` projection과 envelope
  validation/round-trip 회귀 테스트가 존재한다.
- request normalization, redaction, request-key idempotency와 32-entry limit가
  TDD로 검증된다.
- intent-before-send, exact-once invocation, sent/failed CAS와 concurrent duplicate
  처리가 검증된다.
- narrow Orca adapter가 literal argv를 사용하고 response identity mismatch를
  fail-closed 처리한다.
- CLI `issueops handoff request-modification`과 MCP `issueops_handoff` action이 같은
  core 계약을 노출한다.
- source/owner hook allowlist가 전용 경로만 허용하고 raw steering은 계속
  차단한다.
- owner 수정 후 publication은 fast-forward descendant만 허용하며 stale 또는
  non-fast-forward HEAD를 거부한다.
- 문서, contract golden, focused/full/race Go tests와 build가 통과한다.

## 비목표

- source/main에서 worker 파일, owner shell, feedback, phase 또는 publish receipt를
  직접 변경하지 않는다.
- completed, cleanup, closed, cancelled, recovery handoff를 다시 열지 않는다.
- 자동 retry, force-push, branch 교체, PR/MR 재생성, merge 또는 cleanup을 하지
  않는다.
- 일반-purpose 메시징 command를 추가하지 않는다.
- 이번 구현 범위에서 모든 next-action candidate를 typed schema로 바꾸지 않는다.

## 구현 경계

- 기존 Task 1 변경은 detached worktree의 atomic commit으로 보존한 뒤 canonical
  IssueOps owner worktree에서 적용한다.
- owner는 승인된 설계와 계획 순서대로 TDD를 계속한다.
- source/main은 handoff receipt 이후 bounded status polling만 수행하며 구현을
  중복 실행하지 않는다.
- #65와 `2026-07-22-issueops-handoff-intent-and-coordinator-liveness.md`의 liveness
  불변식을 지킨다.

## 관련 이슈·라벨 판단

- deterministic remote score에서 #65는 `0.4463`, `enhancement`는 `0.6750`,
  `bug`는 `0.5125`로 기본 threshold `0.70`을 넘지 못했다.
- #65는 자동 related link로 선택하지 않는다. 다만 handoff liveness와 source/main
  coordinator responsiveness의 기존 body-of-record이므로 설계·검증 근거로만
  명시한다.
- 라벨은 `enhancement`를 manual override로 선택한다. 이 이슈의 주 산출물이 기존
  동작의 단순 교정이 아니라 typed bounded modification-request CLI/MCP/state
  capability 추가라는 점이 provider label 설명과 직접 일치한다.
- `bug`와 `documentation`은 적용하지 않는다. semantic downgrade와 liveness는
  #65 및 별도 설계 문서에서 추적하고, 문서 변경은 주 산출물이 아니다.
- 큰 이슈 분할은 하지 않는다. 각 task가 같은 handoff envelope, DTO와 lifecycle
  state를 순차적으로 확장하며 하나의 owner가 최종 contract를 함께 검증해야 한다.

## 검증

- focused package tests
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go build -o <temporary-path> ./cmd/issueops`
- CLI/MCP response contract golden
- source/main과 worker root의 clean/dirty ownership 확인
