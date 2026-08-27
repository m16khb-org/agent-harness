# Single-pass self-verification contract

[← TESTING.md](../TESTING.md) owns the test-strategy index and minimum
completion gate. This module owns the document-stage verification battery, the
self-verify QA gate, the all-or-nothing single-run contract, the standalone
verification policy, and web-fetch live parity.

## 현재 문서 단계 검증

문서만 변경한 경우에도 최소한 다음을 확인한다.

```bash
find . -maxdepth 3 -type f | sort
find .agent-harness -maxdepth 1 -type f -name '*.md' | sort
grep -R "외부 Go 하네스\|Go\|MCP\|Codex\|Claude" -n AGENTS.md CLAUDE.md .agent-harness
python3 scripts/validate-skill.py skills/atomic-commit-push
go test ./... -count=1
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
go build -o bin/agent-harness ./cmd/harness
./scripts/install-native.sh
./bin/agent-harness bootstrap --dry-run
./bin/agent-harness install --json
./bin/agent-harness install --dry-run --json
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness guard check --staged --json
printf '{"cwd":"%s","source":"compact"}' "$PWD" | ./bin/agent-harness hook session-start --host claude
./bin/agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
tmp_state="$(mktemp -d)"
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness doctor --repo . --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state write --key smoke --value "ok" --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state read --key smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state list --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state prune --max-age 720h --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state doctor --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state maintain --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon status --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon start --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon stop --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify history --prefix self-verify --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify history --prefix self-verify --retention-limit 1 --prune-retention --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify compare --baseline-key self-verify-smoke --candidate-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify promote --from-key self-verify-smoke --baseline-key self-verify-baseline --json
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/agent-harness self-augment lesson --candidate reflexion-state-memory --lesson "test lesson" --next-action "test next action" --state-key self-augment-lesson-test --json
./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json
grep -R "Conventional Commit\|Lore:" -n AGENTS.md .agent-harness/COMMIT_POLICY.md skills/atomic-commit-push/SKILL.md
```

IssueOps benchmark fixtures must stay repo-agnostic. They should score portable workflow evidence rather than one target repository's domain facts: domain invariants vs exact/equivalent mechanisms, API-doc gate evidence, live runtime evidence matrices, review-feedback accountability, and completion hygiene. A passing deterministic benchmark means `average_score == 100`, `minimum_score == 100`, and `critical_failure_count == 0`.

확인할 것:

Native integration smoke:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
codex mcp get agent_harness
claude mcp list | grep agent_harness
```

- `AGENTS.md`와 `CLAUDE.md`가 같은 source of truth를 가리키는가
- `.agent-harness/`의 링크가 실제 파일과 맞는가
- plugin vs worker 결정과 Go 선택이 여러 문서에서 충돌하지 않는가
- shared skill 원본(`skills/*`)과 user-level host 연결(`~/.codex/skills/*`, `~/.claude/skills/*`)이 drift 없이 같은 대상을 가리키는가
- 커밋 정책이 `AGENTS.md`, `.agent-harness/COMMIT_POLICY.md`, `atomic-commit-push` skill에서 충돌하지 않는가

## 자기 검증 QA gate

`agent-harness self-verify`는 테스트 실행뿐 아니라 QA gate를 포함한다. QA gate는 루프 문서, `GENIUS_THINK.md`, shared skill metadata, native integration 설치 상태, redaction audit, bounded stdout/stderr metadata, Mermaid 문서 lint를 확인하며, 모든 목표 점수가 95점을 초과해야 종료 가능하다. Mermaid lint는 `GENIUS_THINK.md`의 따옴표/`<br/>` 규칙을 기준으로 문서 다이어그램 파싱 오류를 조기에 막는다.

## 부분 검증 상태 금지 (all-or-nothing verification)

다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 1단계부터 전체 재실행한다.

완료 보고의 evidence는 마지막 "전 단계 통과" 단일 run에서 나온 것이어야 하며, 서로 다른 run의 부분 통과를 조합하지 않는다.

재실행 비용이 큰 경우에도 부분 통과 상태를 "검증됨"으로 승격하지 않는다. 비용이 문제면 검증 시나리오를 더 작은 독립 시나리오로 분리한다.

## Standalone Verification Policy

Agent-harness tests must verify harness core and native integration contracts without requiring external toolchains, external accounts, or companion MCP servers. External companion tools are not prerequisites for install/update/self-verify readiness.

The standard deterministic self-verify commands pin `--llm-eval=false` so an intentional ambient `HARNESS_SELF_VERIFY_LLM_EVAL=gate` cannot turn the project gate into the prompt-only diagnostic path. Enabling `advisory` or `gate` currently renders a read-only evaluator prompt and sends no Z.AI request; `gate` is therefore expected to remain non-passing without an ingested external verdict. Record the explicit override and rerun the verification sequence from its first gate after any interrupted or prompt-only attempt.

If a test fixture models data produced by an external tool, keep it as plain local input and verify only the harness boundary that consumes that input. Do not add tests that clone, install, patch, or register external tools as part of normal verification.

## Web-Fetch Live Parity

The default web-fetch battery is deterministic and must not require network access. Opt-in live parity uses `HARNESS_WEBFETCH_LIVE=1` and the fixture file at `testdata/webfetch/live/public-fixtures.json`; follow `.agent-harness/operations/web-fetch-live-parity.md` before interpreting live results or comparing against a generic baseline executable.
