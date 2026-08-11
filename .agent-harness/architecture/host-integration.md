# Host integration: Codex and Claude thin adapters

> Family index: [`../ARCHITECTURE.md`](../ARCHITECTURE.md). This module owns the
> Codex/Claude integration map, the shared pioneer-skills layer, and the
> host-adapter change checklist. Core/port/adapter structure and package
> boundaries live in [`hexagonal-core.md`](hexagonal-core.md); runtime and MCP
> topology live in [`runtime.md`](runtime.md).

## Codex / Claude integration map

| Host | 최소 통합 | 권장 통합 | 주의 |
|------|----------|----------|------|
| Codex | `AGENTS.md` + shell에서 `agent-harness` 실행 | `~/.codex/skills/*` native skills + `~/.codex/config.toml` MCP server + `~/.codex/hooks.json` SessionStart/PostCompact context hooks | plugin에 core logic을 넣지 않는다. 대상 repo 파일을 기본 생성하지 않는다 |
| Claude Code | `CLAUDE.md` + shell에서 `agent-harness` 실행 | `~/.claude/skills/*` native skills + user-scope MCP server + `~/.claude/settings.json` SessionStart/PostCompact context hooks | hook에서 위험 명령을 직접 실행하지 않는다. `.claude/skills`/`.claude/settings.json`/`.mcp.json` repo-local 파일은 explicit project-local opt-in에서만 쓴다 |

## Pioneer Skills Layer

agent-harness는 12개의 pioneer skill을 `skills/` 디렉토리에 단일 진실 원천(single source of truth)으로 관리한다. 각 스킬은 컴퓨터 과학 선구자의 이름을 따서 명명되었으며, 그 선구자의 핵심 통찰을 설계 철학으로 삼는다. 자세한 namesake 설명과 실제 사용 계약은 각 `skills/<name>/SKILL.md`의 frontmatter와 identity를 참조한다.

### 스킬 목록과 IssueOps 연동

| 스킬 | 역할 | IssueOps phase |
|------|------|---------------|
| `von-neumann` | Strategic Planning — decision-complete 계획 수립 | problem, grill, issue, plan |
| `turing` | Evidence-Bound Execution — 증거 기반 목표 실행 | implement, ai-slop-clean, feedback, pr, cleanup |
| `berners-lee` | Web Research — 출처 인용 다중 소스 조사 | grill, issue, feedback |
| `boehm` | Risk-driven planning-document analysis — Kordoc·OCR·시각 증거 조정 | grill, issue, plan, compatibility-review |
| `brooks` | Devil's-advocate design/plan critic — 구현 전 계획 적대 검증 | plan, compatibility-review |
| `codd` | Database Design & Optimization — 정규화·인덱스·쿼리 최적화 | issue, plan, implement |
| `dijkstra` | Algorithm Design & Complexity Optimization | plan, implement, ai-slop-clean |
| `engelbart` | Meeting-record augmentation / team-memory | 직접 연동 없음 |
| `hopper` | Systematic Debugging — 7단계 과학적 디버깅 | implement, feedback |
| `shannon` | Signal-to-Noise Quality Measurement | ai-slop-clean |
| `karpathy` | Prompt Engineering & Optimization | plan, ai-slop-clean, pr |
| `torvalds` | Git Operations — atomic commit, bisect, rebase, worktree | implement, pr, cleanup |

### Cross-reference mesh

스킬 간 참조는 hub-and-spoke 토폴로지를 따른다:

- **Hub**: `turing`과 operational skill인 `issueops`가 실행·조정 중심
- **Spoke**: 전문 스킬들이 hub를 통해 간접 연결되며, 직접 cross-reference도 유지
- 각 스킬의 실제 cross-reference와 IssueOps 연동 여부는 해당 `SKILL.md`가 기준이며, 독립적인 역할은 직접 연동을 강제하지 않음

### 설계 원칙

- **Language/tech agnostic**: 어떤 스킬도 특정 언어·프레임워크를 강제하지 않는다(6f31c55에서 검증 완료). 모든 언어별 예시는 여러 언어의 동등한 명령어를 나란히 제시한다.
- **Namesake philosophy**: 각 스킬의 방법론은 그 이름이 된 과학자의 핵심 기여에서 파생된다(예: Codd → 정규화 이론, Dijkstra → 구조적 프로그래밍 + 최단 경로).
- **Host-neutral**: 모든 스킬은 `skills/` 원본 하나로 Codex와 Claude Code에서 동일하게 사용된다.

## Architecture change checklist

- core behavior 변경: CLI, MCP, worker adapter가 같은 결과를 내는지 테스트한다.
- command policy 변경: CAUTIONS와 TESTING에 위험과 검증을 업데이트한다.
- guard 변경: portable anti-pattern rule은 `internal/core/guard/guard_test.go`로 block/warn/review 판정을 고정하고, CLI/contract golden을 함께 갱신한다.
- host adapter 변경: core contract를 복제하지 않았는지 확인하고 `internal/adapter` contract matrix golden으로 Codex/Claude 설치 표면이 drift되지 않았는지 검증한다.
- shared skill 변경: `skills/<name>` 원본과 user-level host skill 연결(`~/.codex/skills`, `~/.claude/skills`)이 같은 대상을 가리키는지 확인한다. repo-local skill link는 기본 설치에 포함하지 않는다.
- state 위치 변경: migration/backward compatibility와 cleanup 전략을 문서화한다.
