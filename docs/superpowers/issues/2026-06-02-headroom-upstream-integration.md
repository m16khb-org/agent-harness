# Headroom을 optional upstream companion으로 통합

## 문제

`agent-harness bootstrap --sync`는 LLM Wiki, CodeGraph, claude-mem 같은 upstream companion 도구를 선택적으로 설치/갱신한다. Headroom도 토큰·컨텍스트 최적화 companion 후보지만, LLM 요청 앞단에서 proxy/wrapper로 동작할 수 있어 기본 통합으로 켜면 영향 범위가 크다.

Headroom을 하네스 core나 hook에 넣지 않고, 명시적 upstream sync 경로에서만 설치/갱신되는 opt-in 도구로 정리해야 한다.

## 현재 근거

- `.agent-harness/OPERATIONS.md`는 `bootstrap --sync`를 upstream companion 갱신 표면으로 정의한다.
- `scripts/install-native.sh`는 llm-wiki, CodeGraph, claude-mem 설치/갱신을 담당한다.
- Headroom 문서는 `headroom-ai`, proxy, MCP, agent wrap을 제공하지만 자동 활성화는 별도 판단이 필요하다.

## 완료 기준

- `scripts/install-native.sh --with-upstream-tools --dry-run` 출력에 Headroom 설치/갱신 계획이 포함된다.
- 실제 upstream setup은 기존 명시적 opt-in 경로에서만 Headroom을 설치/갱신한다.
- Headroom 설치는 `pipx install --python python3.13 "headroom-ai[all]"` 및 `pipx upgrade headroom-ai` 계약으로 고정한다.
- installer는 `headroom proxy`, `headroom wrap`, `headroom learn`을 자동 실행하지 않는다.
- 문서에 Headroom을 optional upstream dependency로 추가하고 `HEADROOM_TELEMETRY=off` 실험 가이드를 남긴다.
- 테스트 문서에 Headroom smoke check를 추가한다.
- 기본 `bootstrap`과 repo-local 파일 쓰기 동작은 바뀌지 않는다.

## 비목표

- Headroom을 harness core, MCP schema, lifecycle hook, command policy에 넣지 않는다.
- `headroom learn`을 자동 실행하지 않는다.
- Codex/Claude 요청을 Headroom proxy나 wrapper로 자동 라우팅하지 않는다.
- repo-local Headroom 설정을 만들지 않는다.


## 원격 이슈

https://github.com/m16khb/agent-harness/issues/2

## 검증

- `go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseHeadroom -count=1`
- `./scripts/install-native.sh --with-upstream-tools --dry-run`
- `./bin/agent-harness bootstrap --sync --dry-run`
- `bash -n scripts/install-native.sh`
- `go test ./cmd/harness -run Golden -count=1`
- `go test ./... -count=1`
- `go build -o bin/agent-harness ./cmd/harness`

## 피드백 로그

- 사용자는 `$issueops`로 Headroom 통합을 진행하라고 요청했다.
- 사용자는 IDD 원칙상 구현 전에 이슈를 먼저 정리해야 한다고 지적했다.
- 원격 이슈를 생성하고 IssueOps record에 연결했다.
