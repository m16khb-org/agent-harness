---
name: ADR.md
description: Architecture decision record index and accepted baseline.
---
# Architecture Decision Records

작성일: 2026-05-25.

> 이 파일의 날짜별 항목은 append-only 결정 이력이다. 과거 항목의 retired host/schema/command 명칭은 당시 근거를 보존하는 역사 표기이며 현재 지원 계약이 아니다. 현재 운영 표면은 루트 `AGENTS.md`, `ARCHITECTURE.md`, `OPERATIONS.md`와 가장 최근의 명시적 superseding 결정이 정한다.

이 색인은 accepted architecture decision의 정규 입구다. 각 결정은
`adr/decisions/` 아래 하나의 immutable record로 저장되고 본 색인으로 링크된다.

## Accepted baseline

- **아키텍처:** 외부 하네스 코어(CLI/MCP/worker)에 얇은 host adapter를 얹는
  hybrid 구조. 근거는 [plugin vs external worker](adr/decisions/2026-05-25-plugin-vs-external-worker.md)
  와 [Go 언어 선택](adr/decisions/2026-05-25-go-language-selection.md) 결정에 있다.
- **First-party hosts:** Codex, Claude Code, Omo native가 같은 shared skill,
  MCP, lifecycle activation contract를 사용한다. 근거는
  [Omo native first-party host](adr/decisions/2026-08-12-omo-native-first-party-host.md)
  결정에 있다.
- **구현 로드맵:** phase별 계획, 목표 아키텍처, MVP 범위, 위험, 다음 작업 후보는
  [adr/roadmap.md](adr/roadmap.md) 가 소유한다.
- **결정 규칙:** status model, naming, authoring 규칙과 archived history
  ledger는 [adr/README.md](adr/README.md) 가 소유한다.

## Decision index

Reverse-chronological. Same-day records use a distinguishing slug. Status model
and supersession rules live in [adr/README.md](adr/README.md).

| Date | Decision | Record |
|---|---|---|
| 2026-08-10 | Default hooks are thin static context only | [record](adr/decisions/2026-08-10-default-hooks-thin-static-context.md) |
| 2026-08-08 | Dependency ratchet counts capability boundaries only | [record](adr/decisions/2026-08-08-dependency-ratchet-capability-boundary.md) |
| 2026-08-08 | Legacy baseline removed; ratchet becomes an invariant | [record](adr/decisions/2026-08-08-legacy-baseline-invariant.md) |
| 2026-08-08 | Port speaks in contract vocabulary | [record](adr/decisions/2026-08-08-port-contract-vocabulary.md) |
| 2026-08-08 | Contract cross-reference is composition, not implementation | [record](adr/decisions/2026-08-08-contract-cross-reference-composition.md) |
| 2026-08-04 | Post-completion base synchronization uses contract-owned authority | [record](adr/decisions/2026-08-04-post-completion-base-synchronization.md) |
| 2026-08-04 | Completed reseed requires stamped current completion provenance | [record](adr/decisions/2026-08-04-completed-reseed-stamped-provenance.md) |
| 2026-07-29 | Release vertical replaces the lease prototype | [record](adr/decisions/2026-07-29-release-vertical-replaces-lease-prototype.md) |
| 2026-07-28 | Lease differential contract owns stable v1 canonicalization | [record](adr/decisions/2026-07-28-lease-differential-v1-canonicalization.md) |
| 2026-07-27 | Architecture dependency fitness ratchet | [record](adr/decisions/2026-07-27-architecture-dependency-fitness-ratchet.md) |
| 2026-07-26 | Linked branches are pinned to the sealed base SHA | [record](adr/decisions/2026-07-26-linked-branches-pinned-to-sealed-base-sha.md) |
| 2026-07-24 | IssueOps planner/implementer dual structure (#78) | [record](adr/decisions/2026-07-24-issueops-planner-implementer-dual-structure.md) |
| 2026-07-24 | Canonical command with managed `ah` shorthand | [record](adr/decisions/2026-07-24-canonical-command-ah-shorthand.md) |
| 2026-07-24 | Workpool removal | [record](adr/decisions/2026-07-24-workpool-removal.md) |
| 2026-07-21 | IssueOps uses one ownership handoff contract | [record](adr/decisions/2026-07-21-issueops-ownership-handoff-contract.md) |
| 2026-07-19 | One operational-health authority and external one-time reconciliation | [record](adr/decisions/2026-07-19-operational-health-authority-and-external-reconciliation.md) |
| 2026-07-15 | Supervised handoff coordinator isolation and bounded self-heal | [record](adr/decisions/2026-07-15-supervised-handoff-coordinator-isolation.md) |
| 2026-07-14 | Evidence-first cross-host tool contract hardening | [record](adr/decisions/2026-07-14-evidence-first-cross-host-tool-contract.md) |
| 2026-07-09 | Pipe-capture immunity and pipe-capacity doctor check | [record](adr/decisions/2026-07-09-pipe-capture-immunity-doctor-check.md) |
| 2026-07-09 | Loop contracts | [record](adr/decisions/2026-07-09-loop-contracts.md) |
| 2026-07-08 | SQLite store maintenance policy | [record](adr/decisions/2026-07-08-sqlite-store-maintenance-policy.md) |
| 2026-07-07 | State storage moves from JSON files + flock to SQLite (sqlstore) | [record](adr/decisions/2026-07-07-sqlite-state-storage-migration.md) |
| 2026-07-07 | Standalone harness policy, upstream wiring removed | [record](adr/decisions/2026-07-07-standalone-harness-policy.md) |
| 2026-07-03 | Codex PreToolUse ask fallback | [record](adr/decisions/2026-07-03-codex-pretooluse-ask-fallback.md) |
| 2026-07-02 | External LLM calls emit per-call usage observation records | [record](adr/decisions/2026-07-02-external-llm-usage-observation.md) |
| 2026-07-02 | IssueOps regress rounds are capped with a human-decision escalation | [record](adr/decisions/2026-07-02-issueops-regress-round-cap.md) |
| 2026-07-02 | Self-augment planner consumes Reflexion lessons as score penalty | [record](adr/decisions/2026-07-02-self-augment-reflexion-lessons.md) |
| 2026-07-02 | External LLM stays Z.AI-only until a second provider is real | [record](adr/decisions/2026-07-02-external-llm-zai-only.md) |
| 2026-07-01 | Defer harness-side tool-error context injection | [record](adr/decisions/2026-07-01-defer-tool-error-context-injection.md) |
| 2026-07-01 | IssueOps phase transition is a pure reducer over the record | [record](adr/decisions/2026-07-01-issueops-phase-transition-pure-reducer.md) |
| 2026-07-01 | IssueOps devil's-advocate is a fail-closed loop, not just skill prose | [record](adr/decisions/2026-07-01-issueops-devils-advocate-fail-closed-loop.md) |
| 2026-07-01 | MCP transport: adopt go-sdk with a retained legacy JSON-RPC path | [record](adr/decisions/2026-07-01-mcp-transport-go-sdk-legacy-jsonrpc.md) |
| 2026-06-29 | IssueOps phase ledger, grill gate, and Brooks devil's-advocate regression | [record](adr/decisions/2026-06-29-issueops-phase-ledger-grill-gate-brooks.md) |
| 2026-06-26 | IssueOps compatibility review phase | [record](adr/decisions/2026-06-26-issueops-compatibility-review-phase.md) |
| 2026-06-24 | Skill local background separation | [record](adr/decisions/2026-06-24-skill-local-background-separation.md) |
| 2026-06-23 | IssueOps execution decision gate | [record](adr/decisions/2026-06-23-issueops-execution-decision-gate.md) |
| 2026-06-23 | IssueOps hook and state-machine boundary | [record](adr/decisions/2026-06-23-issueops-hook-state-machine-boundary.md) |
| 2026-06-18 | IssueOps plan-prep evidence gate | [record](adr/decisions/2026-06-18-issueops-plan-prep-evidence-gate.md) |
| 2026-06-18 | IssueOps implementation requires durable worktree tool preparation | [record](adr/decisions/2026-06-18-issueops-worktree-tool-preparation.md) |
| 2026-06-16 | internal/core *_facade.go is the intended public surface | [record](adr/decisions/2026-06-16-core-facades-intended-public-surface.md) |
| 2026-06-13 | Distribution decision gate | [record](adr/decisions/2026-06-13-distribution-decision-gate.md) |
| 2026-06-09 | Expose IssueOps gate contracts through MCP and skills | [record](adr/decisions/2026-06-09-expose-issueops-gate-contracts.md) |
| 2026-05-25 | Go language selection | [record](adr/decisions/2026-05-25-go-language-selection.md) |
| 2026-05-25 | Plugin vs external worker (external core + thin adapters) | [record](adr/decisions/2026-05-25-plugin-vs-external-worker.md) |

## Archived history

Hot reading path에서 벗어난 결정은 [adr/README.md](adr/README.md) 의 archived
ledger에 요약되어 있고, 전문은 `.agent-harness/archive/adr-history.md` 에 보존된다.
