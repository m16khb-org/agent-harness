# Single-pass self-verification contract

[← TESTING.md](../TESTING.md) owns the test-strategy index and minimum
completion gate. This document defines the document-stage verification battery,
self-verify QA gate, all-or-nothing single-run contract, standalone verification
policy, and web-fetch live parity.

## 문서 단계 검증

문서만 변경해도 최소한 다음을 확인한다.

```bash
find . -maxdepth 3 -type f | sort
find .issueops -maxdepth 1 -type f -name '*.md' | sort
grep -R "외부 Go 하네스\|Go\|MCP\|Codex\|Claude" -n AGENTS.md CLAUDE.md .issueops
python3 scripts/validate-skill.py skills/atomic-commit-push
go test ./... -count=1
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
go build -o bin/issueops ./cmd/issueops
./scripts/install-native.sh
./bin/issueops bootstrap --dry-run
./bin/issueops install --json
./bin/issueops install --dry-run --json
./bin/issueops inspect --json
./bin/issueops docs --json
./bin/issueops guard check --staged --json
printf '{"cwd":"%s","source":"compact"}' "$PWD" | ./bin/issueops hook session-start --host claude
./bin/issueops policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/issueops policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
tmp_state="$(mktemp -d)"
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops doctor --repo . --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state write --key smoke --value "ok" --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state read --key smoke --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state list --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state prune --max-age 720h --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state doctor --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state maintain --json
ISSUEOPS_DAEMON_DIR="$tmp_state/daemon" ./bin/issueops daemon status --json
ISSUEOPS_DAEMON_DIR="$tmp_state/daemon" ./bin/issueops daemon start --json
ISSUEOPS_DAEMON_DIR="$tmp_state/daemon" ./bin/issueops daemon stop --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-smoke --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify history --prefix self-verify --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify history --prefix self-verify --retention-limit 1 --prune-retention --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify compare --baseline-key self-verify-smoke --candidate-key self-verify-smoke --json
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops self-verify promote --from-key self-verify-smoke --baseline-key self-verify-baseline --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
./bin/issueops self-augment --cycles=1 --target-score=95 --json
./bin/issueops self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/issueops self-augment lesson --candidate reflexion-state-memory --lesson "test lesson" --next-action "test next action" --state-key self-augment-lesson-test --json
./bin/issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
grep -R "Conventional Commit\|Lore:" -n AGENTS.md .issueops/COMMIT_POLICY.md skills/atomic-commit-push/SKILL.md
```

IssueOps benchmark fixtures must stay repo-agnostic. They score portable workflow evidence rather than one target repository's domain facts: domain invariants vs exact/equivalent mechanisms, API-doc gate evidence, live runtime evidence matrices, review-feedback accountability, and completion hygiene. A deterministic benchmark passes only when `average_score == 100`, `minimum_score == 100`, and `critical_failure_count == 0`.

추가 확인:

Native integration smoke:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
codex mcp get issueops
claude mcp list | grep issueops
```

- `AGENTS.md`와 `CLAUDE.md`가 같은 source of truth를 가리키는가
- `.issueops/`의 링크가 실제 파일과 맞는가
- plugin vs worker 결정과 Go 선택이 여러 문서에서 충돌하지 않는가
- shared skill 원본(`skills/*`)과 user-level host 연결(`~/.codex/skills/*`, `~/.claude/skills/*`)이 drift 없이 같은 대상을 가리키는가
- 커밋 정책이 `AGENTS.md`, `.issueops/COMMIT_POLICY.md`, `atomic-commit-push` skill에서 충돌하지 않는가

## 자기 검증 QA gate

`issueops self-verify`에는 테스트와 QA gate가 포함된다. QA gate는 루프 문서, `GENIUS_THINK.md`, shared skill metadata, native integration 설치 상태, redaction audit, bounded stdout/stderr metadata, Mermaid 문서 lint를 확인하고, 모든 목표 점수가 95점을 초과해야 종료할 수 있다. Mermaid lint는 `GENIUS_THINK.md`의 따옴표/`<br/>` 규칙을 기준으로 문서 다이어그램의 파싱 오류를 조기에 방지한다.

## 부분 검증 상태 금지 (all-or-nothing verification)

다단계 검증 시나리오에서 한 단계라도 실패하면 이전 통과를 재사용하지 말고 1단계부터 전체를 다시 실행한다.

완료 보고의 evidence는 마지막으로 "전 단계 통과"한 단일 run에서만 가져온다. 서로 다른 run의 부분 통과를 조합하지 않는다.

재실행 비용이 커도 부분 통과를 "검증됨"으로 기록하지 않는다. 비용이 문제면 시나리오를 더 작은 독립 시나리오로 나눈다.

## Standalone Verification Policy

Agent-harness tests must verify harness core and native integration contracts without requiring external toolchains, external accounts, or companion MCP servers. External companion tools are not prerequisites for install/update/self-verify readiness.

Standard deterministic self-verify commands pin `--llm-eval=false`, so ambient `ISSUEOPS_SELF_VERIFY_LLM_EVAL=gate` cannot turn the project gate into the prompt-only diagnostic path. `advisory` or `gate` currently render a read-only evaluator prompt without sending a Z.AI request; without an ingested external verdict, `gate` is expected to remain non-passing. After an interrupted or prompt-only attempt, record the explicit override and rerun the verification sequence from the first gate.

Keep fixtures for external-tool data as plain local input and verify only the harness boundary that consumes it. Do not add tests that clone, install, patch, or register external tools during normal verification.

## Web-Fetch Live Parity

The default web-fetch battery is deterministic and must not require network access. Opt-in live parity uses `ISSUEOPS_WEBFETCH_LIVE=1` with `testdata/webfetch/live/public-fixtures.json`; follow `.issueops/operations/web-fetch-live-parity.md` before interpreting live results or comparing them with a generic baseline executable.
