---
name: CONVENTIONS.md
description: Coding conventions, package structure, and layer boundaries.
---

# 구현 컨벤션

이 파일은 구현 컨벤션 family의 canonical index다. 모든 agent가 알아야 할
핵심 불변식을 보존하고, 상세 규칙은 아래 module로 연결한다. 각 module은 다시
이 index로 돌아온다.

> 현재 `agent-harness`는 초기 문서 단계다. Go 코드가 추가되면 이 문서를 우선
> 적용한다.

## Module map

| Module | 책임 |
|--------|------|
| [`conventions/go-and-packages.md`](conventions/go-and-packages.md) | Go 네이밍/파일 구조, core/port/adapter layer 경계, dependency 원칙·fitness ratchet·concrete-adapter 제거 순서, SOLID/YAGNI/KISS, shared skill packaging |
| [`conventions/cli-mcp-and-output.md`](conventions/cli-mcp-and-output.md) | CLI·MCP tool schema, `--json`/exit code 계약, response·single-object 출력 형태, api-doc gate, prompt 구조, project-docs bootstrap 표면, 커밋 메시지 형식 |
| [`conventions/state-policy-and-hooks.md`](conventions/state-policy-and-hooks.md) | worker lifecycle, config/env 우선순위, logging/secret hygiene, hook 출력, 언어 무관 guard, policy tier, doctor·lifecycle state, IssueOps 상태머신 reducer 계약, 외부 orchestration adapter 경계 |

각 module은 이 index로 링크하고, 다른 family의 규칙을 복제하지 않는다.

## Canonical single-owner pointers

이 family이 아닌 정규 소유자에게 링크한다:

- 커밋 메시지 정책(Conventional Commit + Lore): [`COMMIT_POLICY.md`](COMMIT_POLICY.md)
- OpenAPI/endpoint 문서화 요구사항: [`OPEN_API_SPEC.md`](OPEN_API_SPEC.md)
- 기술 선택: [`TECH_STACK.md`](TECH_STACK.md)
- agent 실행 순서: [`AGENT_WORKFLOW.md`](AGENT_WORKFLOW.md)
- 헌법 우선순위·안전: [`CONSTITUTION.md`](CONSTITUTION.md)

## 핵심 불변식 (canonical 요약)

모든 상세 경계·ratchet·schema 계약은 각 module이 소유한다. 아래는 모든
agent가 즉시 알아야 할 canonical 요약이다.

### 의존 방향 / layer

- `internal/domain`은 contract와 순수 domain helper에만 의존하고,
  adapter나 `cmd/...`를 import하지 않는다.
- `internal/application`은 contract/domain/port를 조합하며 concrete adapter나
  `cmd/...`를 import하지 않는다.
- `internal/port`는 contract 외 `internal/...` concrete 구현에 의존하지 않는다.
- `internal/adapter/*`는 composition root(`cmd/harness/harnessapp`)에서만
  조립된다. legacy adapter edge는 0이다.
- `cmd/harness/*cli`는 transport parse/render/dispatch를 소유하고 공통 DTO,
  catalog, 판정은 contract/domain/application에서 가져온다.
- 상세 layer 책임표, boundary ratchet, concrete-adapter 제거 순서, SOLID
  적용 지침은 [`conventions/go-and-packages.md`](conventions/go-and-packages.md).

### CLI / MCP / 출력

- subcommand는 동사 중심으로 짧게: `inspect`, `state`, `run`, `mcp`,
  `worker`.
- 사람 출력과 `--json` 출력을 구분한다. `--json` field는 snake_case.
- exit code: `0` 성공 · `1` 일반 실패(대부분의 flag/usage 오류 포함) · `2` 알 수
  없는 subcommand와 `gates` usage 오류 · `3` policy/guard 거부.
- MCP tool 이름은 snake_case `<capability>_<verb>`(예: `harness_inspect`,
  `issueops_execution`). CLI와 MCP는 같은 core request/response DTO를 공유한다.
- 단일 객체 응답은 wrapper로 다시 감싸지 않고 top-level로 spread한다.
- 상세 schema·golden·prompt 구조·api-doc gate·커밋 형식은
  [`conventions/cli-mcp-and-output.md`](conventions/cli-mcp-and-output.md).

### State / policy / guard / hook / lifecycle

- worker는 로컬 전용, Unix socket/localhost binding. job은 idempotency
  key·timeout·cancellation을 갖는다. lifecycle state는 user state dir의
  `projects/<repo-id>/` namespace에 격리한다.
- config 우선순위: `flag → env → workspace config → user config → default`.
  secret 원문 저장 금지.
- 구조화 로그. secret은 adapter 경계에서 redaction한다.
- hook은 `SessionStart`/`PostCompact`만 context-only로 호출하고, 작업을
  대신 실행하거나 shared docs를 직접 수정하지 않는다.
- `guard check`는 언어 무관 1차 방어선. deterministic 신호만 core rule.
  차단은 사유를 함께 낸다.
- `PolicyTier`는 명명 envelope지 deny 판정 로직이 아니다. tier ladder:
  `read_only → workspace_write → network_access → shell_exception`.
- `doctor`는 기본 read-only. `state doctor`는 checkpoint store 무결성 전용.
- IssueOps 상태머신 reducer 계약과 외부 orchestration adapter 경계는
  [`conventions/state-policy-and-hooks.md`](conventions/state-policy-and-hooks.md).

## 이슈 산출물 레이아웃

이슈 하나의 수명주기 동안 생기는 산출물은 provider 이슈 번호로 한 폴더에 모은다(#480).
번호는 linked issue URL의 번호이며 lifecycle id(`io-…`)가 아니다.

```text
.agent-harness/issues/<issue-number>/
  plan.md      추적  기본 플랜(`issueops link-plan` 대상). 같은 번호에 플랜이 여럿이면 plan-<slug>.md
  gates.md     추적  `gates init/check` 원장. `gates check`와 IssueOps `gates_incomplete`가 1순위로 읽는다
  spec.md      추적  선택. superpowers 스펙을 이슈에 붙일 때
  review/      무시  fagan·review-agent-feedback 작업 파일(`<provider>-<mr번호>/`)
```

- 이슈가 없는 작업의 플랜은 `.agent-harness/plans/<slug>.md`, 리뷰 작업 파일은
  `.agent-harness/tmp/`에 둔다. 둘 다 이슈 번호를 알 수 없을 때만 쓰는 fallback이다.
- 옛 원장 경로(`.agent-harness/gates/*.md`, root `GATES.md`, `gates/*.md`)는 읽기 호환으로
  남지만, 현재 사이클의 이슈가 canonical과 호환 경로 양쪽에 원장을 두면 PR readiness가
  `duplicate_issue_artifact:<번호>`로 fail-closed된다.
- `.agent-harness/artifact/`(Orca 봉인 아티팩트)는 이 레이아웃에 포함하지 않는다.
  0600 불변 파일·레코드 절대 경로 계약이라 별도 이슈에서 다룬다.
- 이 표가 유일한 규정이다. 스킬(`issueops`, `von-neumann`, `fagan`, `review-agent-feedback`)과
  `CAUTIONS.md`는 여기를 가리킨다.

## 생성물 / dependency 보조 규칙

- 새 dependency는 표준 라이브러리로 명확히 부족할 때만 추가한다.
- 생성물은 직접 수정하지 않고 생성 스크립트와 source를 고친 뒤 재생성한다.
- dependency fitness ratchet, install adapter 계약, self-augment/self-verify
  교정 가드레일 상세는
  [`conventions/go-and-packages.md`](conventions/go-and-packages.md).

## 이 family의 갱신 절차

1. 변경 대상의 canonical owner module을 찾는다(위 Module map).
2. 한 module만 갱신한다. 역사적 결정은 ADR record로, 사고 교훈은 caution
   lesson으로 별도 파일을 쓴다.
3. 의존 방향·universal summary·module 구성이 바뀔 때만 이 index를 갱신한다.
4. `skills/project-docs-optimize` validator(`scripts.check --mode check`)와
   `agent-harness docs --json`으로 family 계약을 검증한다.
5. diff에서 다른 family 중복이나 정보 누락을 확인한다.

이 index와 각 module은 모두 250 line 이하여야 한다. 상세를 요약으로 대체하지
않고 module로 옮긴다.
