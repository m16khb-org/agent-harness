# 프로젝트 헌법

이 문서는 `agent-harness`에서 코드·문서·설정 변경 시 따라야 할 상위 원칙이다.
구체적인 구현 규칙은 `agent_docs/CONVENTIONS.md`, 테스트 규칙은 `agent_docs/TESTING.md`, 반복 실수 방지는
`agent_docs/CAUTIONS.md`, 기술 스택과 명령어는 `agent_docs/TECH_STACK.md`를 따른다.

---

## 제0장: 문서 우선순위

충돌 시 우선순위는 다음과 같다.

1. 현재 사용자 지시와 작업 범위
2. 가장 가까운 범위의 `AGENTS.md` 또는 `CLAUDE.md`
3. 루트 `AGENTS.md`
4. 이 문서의 안전·정확성·설계 원칙
5. `agent_docs/CAUTIONS.md`의 장애/회귀 방지 규칙
6. `agent_docs/CONVENTIONS.md`의 구현 컨벤션
7. `agent_docs/TESTING.md`의 테스트 작성·검증 규칙
8. `agent_docs/TECH_STACK.md`의 기술 스택·명령어 설명
9. `agent_docs/ARCHITECTURE.md`, `agent_docs/IMPLEMENTATION_PLAN.md`, README, 과거 계획 문서

문서와 현재 코드/설정이 어긋나면 현재 코드와 설정 파일을 source of truth로 확인하고, 작업 범위에 포함되면 문서를 함께 최신화한다.

---

## 제1장: 언어 규칙

- 사용자 응답과 프로젝트 문서는 한글을 기본으로 작성한다.
- 코드 식별자, 패키지명, 파일명, CLI command 이름은 영어를 사용한다.
- Agent, Harness, Worker, Plugin, MCP, CLI, Adapter 같은 기술 용어는 영어 그대로 사용할 수 있다.

---

## 제2장: 안전 / 정확성 우선

원칙 간 충돌 시 다음 순서를 따른다.

1. **안전**: secret hygiene, 파일 접근 경계, 명령 실행 정책, 로그 노출 방지
2. **정확성**: Codex와 Claude Code에서 같은 입력이 같은 결과를 내는가
3. **가독성**: 새 에이전트나 사람이 구조를 빠르게 이해할 수 있는가
4. **성능**: 실제 병목이 확인된 부분만 최적화

이 저장소의 안전 불변식:

- 핵심 정책은 host adapter가 아니라 Go core에 둔다.
- shell/process 실행은 명시적 cwd, timeout, env 정책, audit log 없이 추가하지 않는다.
- workspace root 밖 파일 접근은 기본 거부하고, 허용이 필요하면 정책으로 드러낸다.
- secret 원문은 로그, 문서, 테스트 assertion, MCP/CLI 응답에 남기지 않는다.
- worker daemon은 권한이 제한된 local IPC를 사용하고, stale lock/orphan process 복구 방안을 갖춘다.
- Codex plugin이나 Claude hook은 core 정책을 우회하지 않는다.

---

## 제3장: 아키텍처 원칙

이 프로젝트는 Clean/Hexagonal Architecture를 단순화해서 따른다.

```text
Codex adapter      ┐
Claude adapter     ├─> CLI / MCP / Worker API ─> core usecase ─> ports
Human CLI          ┘                                      ├─> fs/git/process adapter
                                                          ├─> state/log adapter
                                                          └─> host config adapter
```

- `internal/core`는 host와 무관한 usecase와 policy를 둔다.
- `internal/port`는 core가 의존하는 interface와 DTO를 둔다.
- `internal/adapter/*`는 CLI, MCP, worker, filesystem, process, git 같은 외부 기술 구현을 둔다.
- Codex/Claude별 설정은 adapter/template이며 core behavior를 복제하지 않는다.
- persistent worker는 CLI/MCP 계약이 안정된 뒤 도입한다.

---

## 제4장: 설계 3원칙

### KISS

- MVP는 CLI one-shot으로 시작한다.
- daemon, plugin packaging, multi-agent scheduling은 필요성이 검증된 뒤 확장한다.

### DRY

- Codex용 코드와 Claude용 코드를 각각 구현하지 않는다.
- 공통 동작은 core에 한 번만 두고, host adapter는 요청/응답 변환만 담당한다.

### YAGNI

- 처음부터 원격 서버, 분산 queue, 복잡한 권한 모델을 만들지 않는다.
- 개인 로컬 하네스에 필요한 최소 범위에서 시작하고, 실제 사용 불편이 생긴 지점을 확장한다.

---

## 제5장: 검증 원칙

- 완료 주장은 fresh verification 이후에만 한다.
- 문서 변경은 파일 존재, 링크/경로, grep 기반 결정 사항 확인으로 검증한다.
- Go 코드 변경은 영향 범위에 맞게 `go test ./... -count=1`, `go test -race ./...`, `go build ./cmd/harness`를 실행한다.
- CLI/MCP contract 변경은 JSON schema/golden test와 실제 command smoke test를 남긴다.
- worker 변경은 timeout, cancellation, stale lock, concurrent request 테스트를 포함한다.
- 실행하지 못한 검증은 완료 보고에 이유와 대체 확인을 명시한다.
