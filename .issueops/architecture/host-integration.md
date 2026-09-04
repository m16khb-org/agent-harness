# Host integration: Codex, Claude, and Omo thin adapters

> Family index: [`../ARCHITECTURE.md`](../ARCHITECTURE.md). This module owns the
> Codex/Claude/Omo integration map, the shared pioneer-skills layer, and the
> host-adapter change checklist. Core/port/adapter structure and package
> boundaries live in [`hexagonal-core.md`](hexagonal-core.md); runtime and MCP
> topology live in [`runtime.md`](runtime.md).

## Codex / Claude / Omo integration map

| Host | 최소 통합 | 권장 통합 | 주의 |
|------|----------|----------|------|
| Codex | `AGENTS.md`와 shell에서 `issueops` 실행 | `~/.codex/skills/*` native skills + `~/.codex/config.toml` MCP server + `~/.codex/hooks.json` SessionStart context hook | plugin에는 core logic을 넣지 않고, 대상 repo 파일도 기본으로 생성하지 않는다 |
| Claude Code | `CLAUDE.md`와 shell에서 `issueops` 실행 | `~/.claude/skills/*` native skills + user-scope MCP server + `~/.claude/settings.json` SessionStart context hook | hook에서 위험 명령을 직접 실행하지 않는다. `.claude/skills`/`.claude/settings.json`/`.mcp.json` repo-local 파일은 project-local opt-in을 명시한 경우에만 사용한다 |
| Omo native | shell에서 `issueops` 실행 | `~/.omo/agent/skills/*` native skills + `~/.omo/mcp.json` MCP server + `~/.omo/extensions/issueops.js` (`session_start`, `session_compact`) | Omo 설치를 대행하지 않으며 readiness gate도 만들지 않는다. `.omo/skills`/`.omo/mcp.json` repo-local 파일은 project-local opt-in을 명시한 경우에만 사용한다 |

`configs/upstream.json`의 선언형 upstream catalog는 shared skill layer와 분리되어 있다. Native activation 후 Claude Code에만 없는 plugin/Git skill을 선택적으로 provision한다. Provision 실패는 install/readiness 성공에 영향을 주지 않는다. Codex와 Omo에는 `skills/`의 first-party 원본만 기본으로 연결한다.

## Pioneer Skills Layer

issueops는 `skills/` 디렉토리를 공용 스킬 33개의 단일 출처(single source of truth)로 관리한다. 그중 12개는 `internal/domain/pioneerskill/catalog.go`가 고정하는 pioneer skill catalog이고, 나머지 21개는 host·workflow·문서·QA용 operational skill이다. 수량은 `issueops inspect --json`과 각 `skills/<name>/SKILL.md`로 검증한다. namesake 설명과 사용 계약은 각 skill의 frontmatter와 identity를 참조한다.

### 스킬 목록과 IssueOps 연동

| 스킬 | 역할 | IssueOps phase |
|------|------|---------------|
| `implementation-planning` | Strategic Planning — decision-complete 계획 수립 | problem, grill, issue, plan |
| `verified-execution` | Evidence-Bound Execution — 증거 기반 목표 실행 | implement, ai-slop-clean, feedback, pr, cleanup |
| `web-research` | Web Research — 출처 인용 다중 소스 조사 | grill, issue, feedback |
| `requirements-analysis` | Risk-driven planning-document analysis — Kordoc·OCR·시각 증거 조정 | grill, issue, plan, compatibility-review |
| `design-review` | Devil's-advocate design/plan critic — 구현 전 계획 적대 검증 | plan, compatibility-review |
| `database-design` | Database Design & Optimization — 정규화·인덱스·쿼리 최적화 | issue, plan, implement |
| `algorithm-optimization` | Algorithm Design & Complexity Optimization | plan, implement, ai-slop-clean |
| `meeting-notes` | Meeting-record augmentation / team-memory | 직접 연동 없음 |
| `issueops-debugging` | Systematic Debugging — 7단계 과학적 디버깅 | implement, feedback |
| `code-quality-metrics` | Signal-to-Noise Quality Measurement | ai-slop-clean |
| `prompt-engineering` | Prompt Engineering & Optimization | plan, ai-slop-clean, pr |
| `git-operations` | Git Operations — atomic commit, bisect, rebase, worktree | implement, pr, cleanup |

### Cross-reference mesh

스킬 간 참조는 hub-and-spoke 토폴로지를 따른다:

- **Hub**: `verified-execution`과 operational skill `issueops`가 실행과 조정을 담당한다.
- **Spoke**: 전문 스킬은 hub를 통해 간접 연결되며, 직접 cross-reference도 유지한다.
- 각 스킬의 cross-reference와 IssueOps 연동 여부는 해당 `SKILL.md`를 기준으로 한다. 독립적인 역할에는 직접 연동을 강제하지 않는다.

### 설계 원칙

- **Language/tech agnostic**: 어떤 스킬도 특정 언어·프레임워크를 강제하지 않는다(6f31c55에서 검증 완료). 언어별 예시는 동등한 명령어를 여러 언어로 제시한다.
- **Namesake philosophy**: 각 스킬의 방법론은 이름의 과학자가 남긴 핵심 기여에서 파생된다(예: Codd → 정규화 이론, Dijkstra → 구조적 프로그래밍 + 최단 경로).
- **Host-neutral**: `skills/` 원본 하나로 Codex, Claude Code, Omo native에서 모든 스킬을 동일하게 사용한다.
- **Executable-fence contract**: shipped `sh`/`bash`/`zsh`/`shell` fences는
  `scripts/verify-skill-shell.py`의 syntax·failure propagation·safe expansion
  gate를 통과한다. MCP pseudo-call이나 설명용 예시는 `text` fence로 둔다.
- **Evaluation honesty**: canonical catalog 12종 coverage, deterministic
  reproduction fixtures, fresh-context/real-artifact evaluation은 서로 다른
  evidence layer다. Fixture 통과를 blind holdout이나 실제 skill invocation으로
  부르지 않는다.

## Architecture change checklist

- core behavior를 변경하면 CLI, MCP, worker adapter가 같은 결과를 내는지 테스트한다.
- command policy를 변경하면 CAUTIONS와 TESTING에 위험과 검증 내용을 업데이트한다.
- guard 변경: portable anti-pattern rule은 `internal/adapter/guard/guard_test.go`로 block/warn/review 판정을 고정하고, CLI/contract golden을 함께 갱신한다.
- host adapter 변경: core contract를 복제하지 않았는지 확인하고 `internal/adapter` contract matrix golden으로 Codex/Claude/Omo 설치 표면이 drift되지 않았는지 검증한다.
- shared skill 변경: `skills/<name>` 원본과 user-level host skill 연결(`~/.codex/skills`, `~/.claude/skills`, `~/.omo/agent/skills`)이 같은 대상을 가리키는지 확인한다. repo-local skill link는 기본 설치에 포함하지 않는다.
- state 위치 변경: migration/backward compatibility와 cleanup 전략을 문서화한다.
