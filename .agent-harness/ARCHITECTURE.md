---
name: ARCHITECTURE.md
description: System structure, component boundaries, and responsibilities.
---

# agent-harness 아키텍처

이 파일은 아키텍처 문서 family의 canonical index다. 핵심 판단과 의존 방향
불변식을 보존하고, 상세 토폴로지·경계·runtime/state 제약은 아래 module로
연결한다. 각 module은 다시 이 index로 돌아온다.

> **핵심 판단**: Go로 작성한 외부 하네스 코어를 두고, Codex plugin, Claude
> Code 설정, Omo native extension은 core를 호출하는 얇은 adapter로 둔다(Hybrid 최종 구조). 자세한
> 선택지 비교와 판단 근거는
> [`architecture/hexagonal-core.md`](architecture/hexagonal-core.md)가
> 정규 소유자다.

## Module map

| Module | 책임 |
|--------|------|
| [`architecture/hexagonal-core.md`](architecture/hexagonal-core.md) | contract/domain/application/port/adapter 구조, package boundary, 의존 방향 ratchet, cross-host tool contract, operational-health boundary, hardening |
| [`architecture/runtime.md`](architecture/runtime.md) | 실행 모드(CLI/MCP/daemon/issueops/loop/worker), docs/state/config/log 토폴로지, lock 직렬화, command/policy model, MCP tool 설계, standalone runtime policy |
| [`architecture/host-integration.md`](architecture/host-integration.md) | Codex/Claude/Omo 통합 map, pioneer skills layer(host-neutral), host-adapter 변경 체크리스트 |
| [`architecture/issueops.md`](architecture/issueops.md) | IssueOps v1 execution 상태·schema 권위, capability vertical, operational surface, next_command 권위, actor model, Orca 경계, execution threat model, execution boundary |

## 의존 방향 불변식 (canonical)

모든 상세 경계와 ratchet 규칙은
[`architecture/hexagonal-core.md`](architecture/hexagonal-core.md)가 소유한다.
모든 agent가 알아야 할 canonical 요약:

- `internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete
  adapter나 `cmd/...`를 import하지 않는다.
- `internal/application`은 contract/domain/port를 조합하고 concrete adapter나
  `cmd/...`를 import하지 않는다.
- `internal/port`는 contract 외 `internal/...` concrete 구현에 의존하지 않는다.
- `internal/adapter/*`는 composition root(`cmd/harness/harnessapp`)에서만
  조립된다. legacy adapter edge는 0이다(`outbound` 하위 package 사이 정리
  edge와 공유 저장 엔진 `outbound/sqlstore` 예외만 허용).
- 새 host 추가 시 공통 domain/application 정책을 복제하지 않고
  `port.HostInstaller` 구현체와 composition-root wiring만 추가한다.
- `internal/architecture`는 위 규칙을 production import graph 기반 test-only
  ratchet으로 강제한다. baseline을 줄이는 변경만 같은 review에서 허용한다.

## 실행 모드와 표면 (요약)

CLI one-shot, `mcp` stdio proxy, `daemon` user-level backend, `issueops`,
`loop`, `worker` 부분 구현, 그리고 Phase 5/6의 Codex/Claude/Omo UX adapter.
각 모드의 도입 단계·용도·원칙 표와 daemon socket/lock, MCP schema/descriptor
설계, command-policy catalog와 기본 거부/허용 범주, standalone runtime policy는
[`architecture/runtime.md`](architecture/runtime.md)가 소유한다.

## State / process / lock 토폴로지 (요약)

- state root: `~/.local/state/agent-harness/`(`HARNESS_STATE_DIR`로 override).
- 모든 read-modify-write span은 해당 root의 `harness.lock.db`에서
  `BEGIN IMMEDIATE` sqlstore span으로 직렬화된다(프로세스 사막 시 자동 해제,
  span 중첩 금지).
- project lifecycle state는 `projects/<repo-id>/`에, loop state는
  `loop/<loop-id>.json`에, IssueOps v1 state는 `issueops_v1/harness.db`의
  `issueops_v1` bucket에 격리된다.
- config 추적 기준(project 지식은 git 추적, 전역 설정/state는 비추적, cache는
  `.gitignore`, secret은 keychain/env 참조)과 XDG 우선/fallback 원칙.

상세 schema, cleanup/integrity/diagnostics/migration, docs/project-bootstrap
표면은 [`architecture/runtime.md`](architecture/runtime.md)에 있다.

## IssueOps ownership (요약)

IssueOps v1 execution은 단일 authority다. Orca는 readiness·workspace·native
owner launch/inventory만 제공하고 generation/actor/CWD fence는 core가 소유한다.
한 record에 한 `Execution`, 한 canonical worktree, 한 active generation.
trust boundary는 exact native actor(host, session/agent ID, process
PID/start/executable receipt, canonical cwd, lifecycle ID, generation)다.
branch name·source cwd·generic session binding·terminal handle·stable diff는
쓰기 권위가 아니다. Hook은 mismatch mutation에 대한 default-deny guard일 뿐
scheduler나 lease grantor가 아니다.

상세 capability vertical(`execution release`/`reconcile`,
`issueopspublication`), operational surface, generated `next_command` 권위,
actor model, Orca 경계, 전체 threat model과 invariants, execution boundary와
post-merge cleanup 순서 계약은
[`architecture/issueops.md`](architecture/issueops.md)가 소유한다.

## Host integration (요약)

Codex/Claude/Omo는 repo 지침과 `agent-harness` 실행이 최소 통합,
user-scope native skill symlink + MCP server + `SessionStart`/`PostCompact`
context hook이 권장 통합. plugin이나 hook에 core logic/위험 명령을 넣지 않고,
repo-local 파일은 `--project-local` 명시 opt-in에서만 생성한다. pioneer
skills는 `skills/` 원본 하나로 host-neutral이며 hub-and-spoke cross-reference를
따른다. 통합 map 표, skill 목록/연동, host-adapter 변경 체크리스트는
[`architecture/host-integration.md`](architecture/host-integration.md)가
소유한다.

## 이 family의 갱신 절차

1. 변경 대상의 canonical owner module을 찾는다(위 Module map).
2. 한 module만 갱신한다. 역사적 결정은 ADR record로, 사고 교훈은 caution
   lesson으로 별도 파일을 쓴다.
3. 의존 방향이나 universal summary가 바뀔 때만 이 index를 갱신한다.
4. `skills/project-docs-optimize` validator(`scripts.check --mode check`)와
   `agent-harness docs --json`으로 family 계약을 검증한다.
5. diff에서 다른 family 중복이나 정보 누락을 확인한다.

이 index와 각 module은 모두 250 line 이하여야 한다. 상세를 요약으로 대체하지
않고 module로 옮긴다.
