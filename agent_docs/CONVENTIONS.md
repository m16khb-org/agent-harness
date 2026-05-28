# 구현 컨벤션

현재 `agent-harness`는 초기 문서 단계다. Go 코드가 추가되면 이 문서를 우선 적용한다.

---

## 1. 네이밍 / 구조

- 프로젝트 이름: `agent-harness`
- CLI 바이너리 이름: `harness`
- Go module 이름은 repo 원격이 확정되면 `agent-harness`를 현재 로컬 module 이름으로 사용한다.
- 파일명은 snake_case를 사용한다.
- Go 패키지명은 짧은 소문자 단어를 사용한다.
- 테스트 파일은 대상 파일 가까이에 `*_test.go`로 둔다.

현재/예정 구조:

```text
cmd/harness/main.go
internal/core/docs.go
internal/core/inspect.go
internal/core/policy.go
internal/core/preflight.go
internal/core/state.go
cmd/harness/testdata/*.golden.*
internal/port/
internal/adapter/cli/
internal/adapter/mcp/
internal/adapter/codex/
internal/adapter/claude/
internal/adapter/installutil/
internal/adapter/worker/
internal/adapter/fs/
configs/codex/
configs/claude/
skills/
agent_docs/
```

---

## 2. 레이어 경계

| 레이어 | 책임 | 의존 가능 | 금지 |
|--------|------|-----------|------|
| `core` | workspace/docs/state/policy/preflight/inspect usecase. 현재 host-neutral core 로직이 여기 있다 | `port`, 표준 라이브러리 | Codex/Claude SDK, CLI flag, MCP transport 직접 의존 |
| `port` | interface, DTO, error contract | 표준 라이브러리 | adapter concrete type 의존 |
| `adapter/cli` | flag/stdout/stderr/exit code | `core`, CLI library | 정책 복제 |
| `adapter/mcp` | MCP tool schema/transport | `core`, MCP library | CLI와 다른 의미의 응답 |
| `adapter/codex` | Codex user skill/MCP 설치 구현 | `core`, `port`, 표준 라이브러리 | 적용 대상 repo 파일 쓰기 |
| `adapter/claude` | Claude user skill/hook/MCP 설치 구현 | `core`, `port`, 표준 라이브러리 | 기본 설치에서 `.claude/skills` 같은 repo-local 파일 쓰기 |
| `adapter/worker` | local daemon, job lifecycle, IPC | `core`, stdlib/net | command policy 우회 |
| `adapter/fs` | filesystem/git/process 구현 | `port`, os/exec 등 | root 밖 접근을 암묵 허용 |

---

## 3. CLI 컨벤션

- subcommand는 동사 중심으로 짧게 둔다: `inspect`, `state`, `run`, `mcp`, `worker`.
- 사람이 읽는 기본 출력과 agent가 파싱하는 `--json` 출력을 구분한다.
- `--json` 출력 field는 snake_case를 사용한다.
- 실패 시 stderr에는 짧은 설명, JSON 모드에는 machine-readable error code를 포함한다.
- exit code는 의미 있게 유지한다.
  - `0`: 성공
  - `1`: 일반 실패
  - `2`: 잘못된 사용법/flag
  - `3`: policy 거부
  - `4`: workspace/config 문제

---

## 4. MCP 컨벤션

- MCP tool 이름은 `harness.<verb>` 형태를 사용한다.
- CLI와 MCP는 같은 core request/response DTO를 공유한다.
- tool response에는 불필요한 대용량 파일 내용을 싣지 않는다. 요약, 경로, hash, line range를 우선한다.
- command 실행 tool은 policy 결과와 audit log id를 함께 반환한다.
- schema 변경은 golden test와 문서 업데이트를 동반한다.
- CLI/MCP contract golden은 `cmd/harness/testdata/`에 둔다. 의도된 schema 변경일 때만 `go test ./cmd/harness -run Golden -update -count=1`로 갱신한다.
- 실제 JSON response golden은 dynamic field(timestamp, temp path, audit id)를 normalize해서 host/session 차이로 인한 drift를 막는다.
- response golden 범위는 state/policy뿐 아니라 docs/inspect/preflight처럼 agent가 자주 의존하는 읽기 표면을 우선 포함한다.

---

## 5. Worker 컨벤션

- worker는 로컬 전용으로 시작한다. 원격 API는 별도 요구가 생기기 전까지 만들지 않는다.
- Unix socket 또는 localhost binding을 사용하고, 권한을 제한한다.
- job은 idempotency key, timeout, cancellation을 갖는다.
- worker 시작/종료는 stale lock과 orphan process를 처리한다.
- 장기 작업 상태는 user state dir에 저장하고, repo에 secret/state 원문을 쓰지 않는다.

---

## 6. Config / env

- env prefix는 `HARNESS_`를 사용한다.
- config 파일 기본 후보는 `~/.config/agent-harness/config.yaml`이다.
- workspace별 설정이 필요하면 `.harness/config.yaml`을 사용하되, secret 원문 저장은 금지한다.
- config 우선순위는 `flag → env → workspace config → user config → default` 순서로 둔다.

---

## 7. Logging / secret hygiene

- 구조화 로그를 사용한다.
- secret으로 보이는 값은 adapter 경계에서 redaction한다.
- command stdout/stderr를 그대로 저장하기 전에 redaction filter를 거친다.
- 로그에는 workspace, command id, duration, exit code를 남기되 token/key 원문은 남기지 않는다.

---

## 8. Dependency 원칙

- 새 dependency는 표준 라이브러리로 명확히 부족할 때만 추가한다.
- CLI/MCP library는 기능보다 안정성과 유지보수성을 우선한다.
- dependency 추가 시 `go.mod`, license, 보안 위험을 확인한다.
- 생성물은 직접 수정하지 않는다. 생성 스크립트와 source를 고친 뒤 재생성한다.

---

## 9. Shared Skill 컨벤션

- 공용 스킬 원본은 `skills/<skill-name>/`에 둔다.
- Codex/Claude별 user skill 경로는 원본으로 향하는 symlink 또는 installer로 연결한다. 적용 대상 repo의 `.claude/skills`는 기본 설치에서 만들지 않는다.
- skill 이름은 lowercase/digit/hyphen만 사용한다.
- 각 skill은 `SKILL.md`를 반드시 포함하고, Codex UI metadata가 필요하면 `agents/openai.yaml`을 둔다.
- 스킬 안에는 README, 설치 가이드, changelog 같은 보조 문서를 만들지 않는다.
- 검증은 skill-creator의 `quick_validate.py`로 수행한다.

설치 adapter 규칙:

- `internal/core.InstallNative`가 host-neutral 설치 engine이고 `port.HostInstaller`가 SOLID 경계다.
- Codex/Claude adapter는 자기 host의 user/global 설정만 기본으로 쓴다. repo-local `.mcp.json`, `.claude/settings.json`, `.claude/skills`는 `--project-local` 같은 명시적 opt-in 없이는 만들지 않는다.
- symlink는 사용자 홈의 skill 경로에서 중앙 `skills/<name>`을 참조하기 위해서만 기본 사용한다.
- adapter 설치 계약을 바꾸면 `internal/adapter/install_contract_matrix_test.go`와 `internal/adapter/testdata/native_install_contract_matrix.golden.json`을 함께 갱신해 user/global 기본 설치와 explicit project-local opt-in의 차이를 보존한다.

---

## 10. 커밋 메시지 컨벤션

- 커밋 메시지는 `agent_docs/COMMIT_POLICY.md`를 따른다.
- subject는 Conventional Commit 형식(`<type>(<scope>)!?: <summary>`)을 사용한다.
- body에는 AI agent가 맥락을 회수할 수 있도록 `Lore:` 블록을 둔다.
- atomic commit 하나에는 Lore `Intent`도 하나만 있어야 한다.
## API Documentation Gate

Reusable API documentation checks belong in `harness api-doc review`, not in a single application repository or framework-specific hook. Keep the core prompt framework-agnostic: it may mention examples such as NestJS Swagger decorators, Go swaggo annotations, OpenAPI specs, Spring/FastAPI annotations, but must instruct reviewers to follow the target project's own convention and staged diff only. Project-specific strictness should be supplied via `--prompt-file` rather than hardcoded into harness core.
