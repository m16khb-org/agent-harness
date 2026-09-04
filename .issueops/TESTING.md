---
name: TESTING.md
description: Verification standards, test practices, and required checks.
---

# 테스트 컨벤션

이 문서는 `issueops`의 문서·코드 변경 검증 규칙의 정규 인덱스다.
보편적 요약과 최소 완료 기준은 이 인덱스가 소유하고, 절차·근거·명령은
주제별 모듈이 소유한다.

## 모듈

| 주제 | 정규 소유 모듈 |
|---|---|
| 단위·통합·fixture·golden·contract 표준, Go 기본 검증, lifecycle state | [testing/unit-and-contract.md](testing/unit-and-contract.md) |
| race·process·lock·nondeterminism 규칙 | [testing/concurrency-and-race.md](testing/concurrency-and-race.md) |
| CLI·MCP·Codex·Claude·Omo host parity, GitLab snapshot, cross-host conformance | [testing/cli-mcp-and-hosts.md](testing/cli-mcp-and-hosts.md) |
| single-pass self-verification 계약, 문서 단계 검증 battery | [testing/self-verification.md](testing/self-verification.md) |
| OpenAPI 정적·agent review 게이트, prompt contract | [testing/api-documentation.md](testing/api-documentation.md) |
| IssueOps v1 execution vertical·Orca 검증 계약 | [testing/issueops-execution.md](testing/issueops-execution.md) |

각 모듈은 이 인덱스로 다시 링크한다. 한 주제의 정규 소유자는 하나고, 다른
모듈은 링크만 둔다.

## 최소 완료 기준

문서만 변경한 경우에도 최소한 빌드·docs·inspect·self-verify 기본 게이트를
확인한다. 전체 battery와 단계별 명령은
[self-verification.md](testing/self-verification.md)가 소유한다.

```bash
go test ./... -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops docs --json
./bin/issueops inspect --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

Go 코드 변경의 기본 검증(`gofmt -l`, `go test -race ./...`, `go vet ./...`, architecture
ratchet, operational-health 위임, golden 갱신 조건)은
[unit-and-contract.md](testing/unit-and-contract.md)가 소유하고, IssueOps
execution vertical 변경의 focused package set은
[issueops-execution.md](testing/issueops-execution.md)가 소유한다.

## 부분 검증 상태 금지

다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지
않고 첫 게이트부터 전체 재실행한다. 완료 보고의 evidence는 단일 "전 단계
통과" run에서 나온 것이어야 하며, 서로 다른 run의 부분 통과를 조합하지
않는다. 재실행 비용이 큰 경우에도 부분 통과 상태를 "검증됨"으로 승격하지
않는다. 단일-run 계약의 전문은
[self-verification.md](testing/self-verification.md)가 소유한다.

## 완료 보고 기준

완료 보고에는 다음을 포함한다.

- 실제 실행한 검증 명령과 결과
- 실패가 있었다면 실패 원인과 수정/미해결 상태
- 실행하지 않은 검증과 이유(`Not-tested`)
- 변경 파일 요약과 남은 위험

## 이 인덱스 유지

- 보편적 요약·탐색·최소 완료 기준만 이 인덱스에서 바꾼다.
- 절차·명령·근거는 해당 주제 모듈에서 바꾼다.
- 새 규칙은 먼저 정규 소유 모듈을 정하고, 중복 요약을 다른 모듈에 두지 않는다.
- 모듈과 인덱스 양쪽 링크를 항상 같이 검증한다.
- 라인 예산은 검색 경계다. 한 모듈이 250 줄을 넘으면 주제별로 분리하고, 임의
  part 번호로 자르지 않는다.
