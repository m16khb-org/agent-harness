---
name: release-reproducibility.md
description: Release checklist and clean-machine install reproducibility smoke.
---

# Release Reproducibility

이 문서는 Phase 7 hardening/release의 재현성 팩이다. 목표는 새 checkout에서 문서만 보고 install path와 기본 CLI workflow를 재현할 수 있게 만드는 것이다.

## Release Checklist

릴리스 전 다음 순서로 실행한다.

```bash
git status --short --branch
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
scripts/release-repro-smoke.sh
./bin/agent-harness self-verify --seed=100 --target-score=95 --json
```

릴리스 산출물 판단:

- `git status --short --branch`가 의도한 변경만 보여야 한다.
- `scripts/release-repro-smoke.sh`가 temp `HOME`, `CODEX_HOME`, `HARNESS_STATE_DIR`, fixture `HARNESS_ROOT` 아래에서 통과해야 한다.
- `install-native --dry-run --project-local --json` 결과는 `ok=true`, `dry_run=true`, `project_local=true`이며 host 목록이 정확히 `[codex, claude]`이고 둘 다 `ok=true`여야 한다.
- dry-run 결과의 `files[].written`과 `links[].created`는 모두 false여야 한다.
- `inspect --json`, `docs --json`, `state migrate --json`가 temp state에서 모두 `ok=true`여야 한다.
- 사용자용 `README.md`에는 `Release User Guide: Install, Update, Rollback` 섹션이 있어야 한다.
- `.agent-harness/operations/release-dogfood-notes.md`에 Codex/Claude MCP 등록과 `inspect/docs/state` dogfood 전사가 있어야 한다.

## Clean-Machine Smoke

기본 실행:

```bash
scripts/release-repro-smoke.sh
```

디버깅용 temp 보존:

```bash
HARNESS_RELEASE_KEEP_TMP=1 scripts/release-repro-smoke.sh
```

이미 빌드된 바이너리 검증:

```bash
HARNESS_RELEASE_SKIP_BUILD=1 AGENT_HARNESS_BIN="$PWD/bin/agent-harness" scripts/release-repro-smoke.sh
```

스크립트는 실제 사용자 홈에 쓰지 않는다. 모든 host install 계획은 temp home/root와 `--dry-run`으로 제한된다. 실제 release installer 변경이 있을 때는 이 스모크가 먼저 깨져야 한다.

## Cross-Platform Build Matrix

릴리스 빌드 smoke는 다음 지원 target을 `CGO_ENABLED=0`으로 cross-build한다.

| GOOS | GOARCH | Status | Rationale |
| --- | --- | --- | --- |
| `darwin` | `arm64` | supported | primary local dogfood target |
| `darwin` | `amd64` | supported | Intel macOS compatibility |
| `linux` | `amd64` | supported | common CI/server target |
| `linux` | `arm64` | supported | ARM Linux server target |
| `windows` | `amd64` | excluded | current daemon process setup uses Unix-oriented `syscall.SysProcAttr.Setsid` |

기본 실행:

```bash
scripts/release-build-matrix.sh
```

산출물 보존:

```bash
HARNESS_RELEASE_OUT_DIR="$PWD/dist" scripts/release-build-matrix.sh
```

target override:

```bash
HARNESS_RELEASE_TARGETS="darwin/arm64 linux/amd64" scripts/release-build-matrix.sh
```

## Release User Guide: Install, Update, Rollback

사용자용 절차는 `README.md`의 같은 이름 섹션을 source of truth로 둔다. 이 운영 문서는 release checklist와 docs index에서 해당 사용자 가이드를 찾을 수 있게 연결한다.

## Distribution Decision Gate

Homebrew 또는 tarball 배포 방식은 아래 조건이 모두 만족된 뒤 결정한다.

- release checklist가 현재 checkout에서 green이다.
- temp clean-machine smoke가 green이다.
- 사용자용 README의 `Release User Guide: Install, Update, Rollback` 섹션에 install/update/rollback 절차가 한 화면 안에 정리돼 있다.
- Codex와 Claude Code에서 같은 `inspect/docs/state` workflow가 성공한다.
- `Release Dogfood Notes`가 현재 checkout 기준으로 갱신돼 있다.

Current decision: prefer tarball/manual archive for the first release. Defer Homebrew until release smoke, build matrix, rollback criteria, and dogfood notes are all green.

Decision record: `.agent-harness/ADR.md` section `2026-06-13 — Distribution decision gate`.

Rollback criteria:

- `inspect --json`, `docs --json`, `state migrate --json`, `scripts/release-repro-smoke.sh`, `scripts/release-build-matrix.sh`, or `agent-harness self-verify --seed=100 --target-score=95 --json` fails on the release checkout.
- Codex or Claude Code cannot complete the same `inspect/docs/state` workflow with the rebuilt binary.
- The release checkout requires manual host-specific repair outside documented install/update steps.

Rollback command path:

```bash
git switch main
git reset --hard <known-good-sha>
agent-harness update
agent-harness inspect --json
```

## Release Dogfood Notes

Current dogfood evidence is tracked in `.agent-harness/operations/release-dogfood-notes.md`.
