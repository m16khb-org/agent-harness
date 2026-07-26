# #184 구현 계획 — CLI usage/flag parity

이슈: https://github.com/m16khb/agent-harness/issues/184

## 고칠 네 건

| # | 위치 | 문제 |
|---|---|---|
| ① | `issueops_cli_support.go` `link-plan` 줄 | 필수 actor 플래그 4종을 표시하지 않는다. `link-worktree`도 같다 |
| ② | 같은 파일 `verify-artifact` 줄 | `ACTOR_FLAGS`(7종)라고 쓰지만 flag 등록·가드 spec은 4종만 받는다. 서브커맨드 자신의 usage는 옳다 |
| ③ | 같은 파일 execution 목록 | `execution switch-mode`가 없다 |
| ④ | 같은 파일 전체 | `ACTOR_FLAGS`를 정의하지 않고 쓴다. legend는 `executioncmd`에만 있다 |

②의 근거: `remotecmd/remote.go`의 `verify-artifact` FlagSet은 `--host`·`--session-id`·`--agent-id`·
`--cwd`만 등록하고, `commandparse/issueops.go`의 같은 케이스도 4종만 등록한다. `--session-pid`를
주면 `flag provided but not defined`이고, 가드는 미분류로 거부한다.

## 재발 방지 — parity 테스트

③이 그 필요의 증거다. `#181`이 `ARCHITECTURE.md`·`CAUTIONS.md`의 열거를 갱신했는데 CLI usage는
남았다. 명령 추가 시 갱신할 곳이 여러 군데인 구조가 원인이다.

테스트 설계:

1. `issueOpsUsageText()`에서 줄을 읽어 명령 경로와 `--flag` 토큰, `ACTOR_FLAGS` 사용 여부를 추출한다.
2. 각 명령의 핸들러를 `--help`로 호출한다. `parseIssueOpsFlags`가 `flag.ErrHelp`를 받아 FlagSet이
   등록 플래그를 stderr에 인쇄하므로, `internal/testsupport`의 `CaptureStderr`로 수집한다.
3. 세 가지를 검사한다.
   - usage가 언급한 플래그가 모두 등록돼 있다(유령 플래그 없음).
   - `ACTOR_FLAGS`를 쓴 명령은 7종을 모두 등록한다.
   - 등록된 actor 플래그가 usage에서 빠지지 않았다.

플래그 토큰 추출만 하고 문장 구조는 검사하지 않는다 — 형식 변경에 취약해지지 않게.

## 왜 핸들러 호출인가

정적 파싱은 flag 등록이 함수 안에 있고 `addIssueOpsActorFlags` 같은 헬퍼를 거치므로 놓친다.
`--help` 호출은 실제 FlagSet을 그대로 본다.

## 검증

```
go test ./cmd/harness/issueopscli/... -count=1
go test ./... -count=1
```

## 비범위

- 명령별 actor 플래그 집합 통일. 왜 `verify-artifact`는 4종이고 `execution *`는 7종인지는 각 명령의
  계약 문제다. 이 이슈는 usage가 실제 계약을 정확히 말하게 하는 것만 다룬다.
- usage 문자열을 flag 등록에서 자동 생성하는 것. 명령마다 형식이 다르고 선택적 조합 표현을
  자동화하기 어렵다.
