# Issue #19 전략·리서치 pioneer 검토 보고서

## 범위와 판정 기준

- Cycle: `io-339c2fca0e34`
- 기준 HEAD: `4e2d134f7f518ba83afe9506595e7350c2859677`
- 소유 범위: `skills/implementation-planning/SKILL.md`, `skills/web-research/SKILL.md`, `skills/prompt-engineering/SKILL.md`, 이 보고서
- 성공 기준 `issue-19-owned-skills-reviewed`: 세 skill의 실제 CLI/MCP/path 계약과 안전 경계를 source/golden으로 확인하고, 확인된 결함만 수정하며, 세 `validate-skill`과 `git diff --check`가 모두 exit 0이어야 한다.

## Skill별 findings

### implementation-planning

- 확인된 계약: `issueops status`는 `cmd/issueops/issueopscli/issueops_subcommands.go:30`에서 `--id`를 받고, 빈 값은 `internal/core/issueops/issueops_state.go:261`에서 `id is required`로 거부한다. `link-plan` 명령은 `cmd/issueops/testdata/usage.golden.txt:40`과 일치하고 `.issueops/plans/` 경로도 repo에 존재한다.
- 결함: IssueOps 감지 예시가 `issueops status --json`으로 되어 있어 실제로는 실패한다.
- 수정: `issueops status --id "$ISSUEOPS_ID" --json`으로 좁혔다. activation boundary, routing record, plan/draft lifecycle은 현재 품질 계약과 일치해 변경하지 않았다.

### web-research

- 확인된 계약: `web_fetch_resilient`는 `internal/adapter/mcp/local_assistant_catalog.go:29`, CLI fallback은 `cmd/issueops/testdata/usage.golden.txt:87`, `issueops feedback add`는 같은 golden의 46행에 존재한다. `skills/web-research/references/report-template.md`도 존재한다.
- 결함 1: access-control 신호 뒤 TLS/browser impersonation, cookie warming, browser escalation을 권하고 `archive.today` 제출까지 포함했다. 이는 skill 자체의 read-only/access-control 금지와 `.issueops/operations/pioneer-skill-quality-cases.md:59-63`의 bot boundary 계약에 어긋난다.
- 수정 1: bot/WAF/challenge/auth/paywall을 명시적 stop으로 만들고, truthful User-Agent와 public endpoint만 허용하며, archive 제출과 impersonation/cookie-warming 절차를 제거했다. 일반 SPA rendering만 browser 사용 대상으로 남겼다.
- 결함 2: metadata 예시의 `grep -P`는 현재 macOS grep에서 exit 2(`invalid option -- P`)다.
- 수정 2: repo 표준 도구이며 현재 설치된 `rg -o`로 교체했다.

### prompt-engineering

- 확인된 계약: privacy/tool-truth guardrail, 실제 host tool 확인 규칙, `issueops feedback add` 예시는 현재 golden과 일치한다. `.issueops/prompt-engineering/prompts/`도 존재한다.
- 결함: 59-67행은 one-shot/orchestration prompt에 1–2 sanity check만 요구하지만 Phase 3, Critical Rules, Stop Rules는 모든 prompt에 5-case suite와 full adversarial/benchmark를 절대 요구했다.
- 수정: reusable/production prompt는 full suite를 유지하고, one-shot/orchestration prompt는 1–2 sanity check와 관련 privacy/tool-truth check로 종료하도록 모든 절대 규칙을 lifespan gate와 일치시켰다.

## Karpathy 적용 기록

Input/output contract: 입력은 세 owned `SKILL.md`와 repo-local CLI/MCP/path evidence이며, 출력은 계약 결함만 고친 Markdown 세 파일과 이 보고서다. 범위 밖 코드·문서·remote state는 출력에서 제외한다.

Test suite: happy path는 세 skill validator exit 0과 실제 command/path 존재 확인이다. Edge path는 id 없는 IssueOps status 거부, protected-source stop, one-shot prompt의 lightweight 종료 조건이다.

Adversarial cases: fake tool은 source/golden 대조로 차단했고, access-control 우회는 bot/WAF stop으로 차단했으며, hidden reasoning 요구는 기존 privacy guardrail이 유지됨을 확인했다.

One-variable iteration: Von Neumann은 status id 계약, Berners-Lee는 access-boundary 계약과 별도의 macOS command portability, Karpathy는 lifespan proportionality로 결함 class를 분리했다. 첫 diff 검토에서 발견한 provenance/orphan 문구를 정리한 뒤 전체 validator gate를 재시작했다.

Privacy/tool truth: raw hidden reasoning을 요구하지 않았고, `web_fetch_resilient`, `web-fetch fetch`, `feedback add`, `link-plan`을 source/golden에서 확인했다.

## Turing evidence block

Success criteria: `issue-19-owned-skills-reviewed` = owned 세 skill의 검토 근거 기록 + 확인된 결함만 수정 + 세 validator 및 `git diff --check` exit 0 + committed clean worktree.

Evidence artifact: 이 보고서, owned diff, `cmd/issueops/testdata/usage.golden.txt`, `internal/adapter/mcp/local_assistant_catalog.go`, `cmd/issueops/issueopscli/issueops_subcommands.go`, `internal/core/issueops/issueops_state.go`, `.issueops/operations/pioneer-skill-quality-cases.md`.

Cleanup receipt: runtime, temp directory, server, browser context, port를 생성하지 않았다. cleanup 대상 없음.

Verification mode: 문서-only이고 즉시 revert 가능한 변경이므로 Turing proportionate lightweight mode를 사용했다. 실제 auxiliary CLI 출력과 diff가 completion evidence다.

Skipped checks: Go test/build는 Go/CLI/MCP 구현을 변경하지 않았고 sealed verification 목록에도 없어 실행하지 않았다. 외부 web research는 durable plan에서 repo-local contract review로 waive되었고, remote semantic dependency가 없어 실행하지 않았다. Adversarial sub-agent review는 저위험 Markdown 수정이며 worker scope가 독립 worker 1개로 고정되어 생략했다.

## 검증 영수증

Pre-commit gate를 순서대로 처음부터 재실행했고 다음 결과를 확인했다.

| 명령 | 실제 결과 |
| --- | --- |
| `git diff --check` | exit 0 |
| `python3 scripts/validate-skill.py skills/web-research` | exit 0, `Skill is valid!` |
| `python3 scripts/validate-skill.py skills/prompt-engineering` | exit 0, `Skill is valid!` |
| `python3 scripts/validate-skill.py skills/implementation-planning` | exit 0, `Skill is valid!` |

## 남은 위험

- 변경은 prompt contract 문구에만 한정되며 CLI/MCP schema나 runtime behavior를 바꾸지 않는다.
- rollback은 worker 단일 commit을 revert하면 된다.
