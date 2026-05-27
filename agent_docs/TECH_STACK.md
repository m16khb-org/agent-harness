# 기술 스택

현재 저장소는 Go module과 `cmd/harness` 기반 CLI/MCP MVP가 구현된 상태다. 아래는 계속 유지할 기술 기준이다.

---

## 1. 언어 선택

| 후보 | 장점 | 단점 | 판단 |
|------|------|------|------|
| Go | 단일 바이너리 배포, 빠른 컴파일, goroutine 기반 동시성, CLI/daemon/MCP 구현 생산성, 현재 로컬 `go1.26.3` 확인 | Rust보다 메모리 안전성의 정적 보장이 약함 | **채택** |
| Rust | 강한 메모리 안전성, 고성능, 단일 바이너리 | 러닝커브와 구현 속도 비용, 현재 로컬 toolchain 확인 안 됨 | 추후 sandbox/security critical component에만 재검토 |

결론: 개인 하네스는 빠른 반복과 host integration 생산성이 중요하므로 **Go**로 시작한다. untrusted code sandbox나 고위험 parser가 필요해지면 해당 component만 Rust를 재검토한다.

---

## 2. 런타임 / 패키지 관리

| 항목 | 기준 |
|------|------|
| 언어 | Go |
| 로컬 확인 toolchain | `go version go1.26.3 darwin/arm64` |
| 패키지 관리 | Go modules |
| 기본 바이너리 | `bin/harness` (`cmd/harness` source) |
| 실행 모드 | CLI one-shot, MCP stdio proxy, user-level daemon, future local job worker |
| 설정 prefix | `HARNESS_` |

---

## 3. 예상 라이브러리 후보

최종 dependency는 구현 시 검증 후 확정한다.

| 영역 | 후보 | 비고 |
|------|------|------|
| CLI | 표준 `flag` 또는 `spf13/cobra` | MVP는 표준 library 우선, command가 늘면 Cobra 검토 |
| Config | `gopkg.in/yaml.v3` 또는 JSON/TOML | 초기에는 JSON/YAML 중 하나만 선택 |
| Logging | 표준 `log/slog` | secret redaction wrapper 필요 |
| MCP | 안정적인 Go MCP SDK 또는 직접 JSON-RPC 최소 구현 | SDK 선택 전 schema 안정성 확인 |
| IPC | Unix socket | 현재 MCP proxy daemon은 Unix socket 사용. localhost HTTP는 future worker 필요 시 검토 |
| State | 표준 library JSON 파일 저장 | `HARNESS_STATE_DIR` 또는 `~/.local/state/agent-harness/` |
| Testing | 표준 `testing`, golden file, `httptest` | 외부 agent host 없이 core contract 검증 |

---

## 4. 예정 명령어

현재 검증/빌드:

```bash
go test ./... -count=1
go build -o bin/harness ./cmd/harness
./bin/harness inspect --json
./bin/harness docs --json
./bin/harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/harness daemon status --json
./bin/harness llm-wiki inventory --json
./bin/harness llm-wiki session-context --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/harness self-verify history --prefix self-verify --json
./bin/harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
./bin/harness self-augment --cycles=1 --target-score=95 --json
./scripts/install-native.sh
./bin/harness install-native --json
./bin/harness install-native --dry-run --json
```

예정 사용 예:

```bash
harness inspect --json
harness docs --json
harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
harness state write --key checkpoint --input checkpoint.json --json
harness state read --key checkpoint --json
harness state list --json
harness state prune --max-age 720h --json
harness state prune --max-age 720h --confirm --json
harness state doctor --json
harness state migrate --json
harness state migrate --confirm --json
harness daemon start --json
harness daemon status --json
harness daemon stop --json
harness llm-wiki inventory --json
harness llm-wiki session-context --json
harness llm-wiki search --query "llm wiki" --json
harness llm-wiki read --page llm-wiki-pattern --json
harness llm-wiki capture --title "Reusable finding" --content "..." --json
harness mcp
harness self-verify --iterations=10 --seed=100 --target-score=95
harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
harness self-verify history --prefix self-verify --json
harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
harness self-augment --cycles=1 --target-score=95 --json
harness worker start
```

---

## 5. 주요 설정/상태 위치

| 종류 | 위치 |
|------|------|
| 루트 규칙 | `AGENTS.md`, `CLAUDE.md` |
| 에이전트 문서 | `agent_docs/` |
| 사용자 설정 | `~/.config/agent-harness/config.yaml` |
| 사용자 state/log | OS별 state dir 또는 `~/.local/state/agent-harness/` |
| workspace cache | `.harness/` |
| daemon socket/pid/log | `~/.local/state/agent-harness/daemon/` 또는 `HARNESS_DAEMON_DIR` |
| LLM Wiki vault | `~/workspace/knowledge-base/llm-wiki` 또는 `LLM_WIKI_ROOT` |
| Codex 템플릿 | `configs/codex/` |
| Claude 템플릿 | `configs/claude/` |

---

## Native integration 확인

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.codex/skills/llm-wiki/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/llm-wiki/SKILL.md
grep -q "session-start-llm-wiki.sh" ~/.claude/settings.json
codex mcp get agent_harness
claude mcp list | grep agent-harness
```
