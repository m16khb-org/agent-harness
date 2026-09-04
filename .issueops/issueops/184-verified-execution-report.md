# #184 검증 보고서 — CLI usage/flag parity

이슈: https://github.com/m16khb-org/issueops/issues/184

## 이슈에 적은 네 건보다 넓었다

RED를 쓰자 **여섯 종류**가 나왔다. 이슈 본문은 손으로 찾은 네 건이었고, 기계적 검사는 그 밖을
보여줬다.

| # | 결함 | 이슈에 있었나 |
|---|---|---|
| ① | `phase` usage가 `[--force]`를 광고하지만 FlagSet에 없다 | **없었다** |
| ② | actor를 요구하는 **24개** 명령이 usage에서 그것을 숨긴다 | `link-plan`·`link-worktree`만 알고 있었다 |
| ③ | `verify-artifact`가 7종 축약을 쓰면서 4종만 받는다 | 있었다 |
| ④ | 두 usage 텍스트에 `execution switch-mode`가 없다 | 있었다 |
| ⑤ | `ACTOR_FLAGS`를 정의하지 않고 쓴다 | 있었다 |
| ⑥ | adapter usage에 `execution whoami`가 없다 | **없었다** |

①이 특히 실질적이다. `--force`는 게이트를 우회하는 것처럼 읽히는데 그런 플래그는 존재하지 않는다.
운영자가 그것을 쓰면 `flag provided but not defined`를 만난다.

## 두 축약으로 나눴다

actor 플래그 집합이 실제로 두 종류다. 하나의 이름으로 둘을 가리키던 것이 ③의 원인이었다.

```
RECORD_ACTOR_FLAGS: --host codex|claude --session-id ID [--agent-id ID] --cwd PATH
ACTOR_FLAGS: --host ... --session-pid PID --session-started-at RFC3339 --session-executable PATH --cwd PATH
```

durable record mutation은 앞의 4종을, execution lease 전이와 generation-fenced 발행은 live session
process까지 검증하므로 뒤의 7종을 받는다. legend를 두 usage 텍스트 안에 넣어 같은 출력에서 확장을
읽을 수 있게 했다.

## parity 테스트가 재발을 막는다

`issueops_usage_flag_parity_test.go`. usage 텍스트에서 명령 경로와 플래그 토큰을 뽑고, 그 명령을
`--help`로 호출해 FlagSet이 인쇄하는 등록 플래그와 비교한다.

정적 파싱을 쓰지 않은 이유는 flag 등록이 함수 안에 있고 `addIssueOpsActorFlags` 같은 헬퍼를
거치기 때문이다. `--help`는 실제 FlagSet을 그대로 보여준다.

검사 다섯 가지:

1. usage가 언급한 플래그가 모두 등록돼 있다(유령 플래그 없음).
2. 축약을 쓴 명령이 그 축약의 플래그를 모두 등록한다.
3. actor를 요구하는 명령이 usage에서 그것을 밝힌다.
4. 축약을 쓰는 usage 출력이 같은 출력 안에서 그것을 정의한다.
5. execution 하위 명령이 두 usage 텍스트에 모두 있다.

⑤가 별도 검사인 이유는 execution 하위 명령이 sub-subcommand라 기존 dispatch registry 검사가 덮지
않는다는 것이다. `switch-mode`(`#167`)가 그 구멍으로 살아남았고, `#181`이 문서 열거를 고쳤을 때도
CLI는 남았다.

## RED가 잡은 테스트 자신의 결함

첫 RED 실행에서 `cycles: 2 (scanned 2 records...)`와 `preview: 0 done cycles selected` 같은 **실제
명령 출력**이 섞였다. 테스트가 명령을 실행해 버린 것이다.

원인은 공유 헬퍼 `usageCommandKey`였다. 플래그로 시작하는 필드에서만 명령 경로를 끊는데,
`list [--repo PATH]`의 `[--repo`는 `[`로 시작해 끊기지 않았다. 명령 경로가 `list [--repo`가 되고
그것을 인자로 넘기자 `--help`가 위치 인자 뒤로 밀려 파싱되지 않았다.

`[`와 `(`도 경로의 끝으로 처리하도록 고쳤다. 기존 두 parity 테스트도 같은 헬퍼를 쓰므로 함께
정확해졌다.

## 관측할 수 없는 것을 숨기지 않았다

세 명령(`design review`·`regress`·`feedback resolve`)은 FlagSet 대신 직접 만든 usage 문자열을
인쇄해 등록 플래그를 수집할 수 없다. 그 목록을 `t.Logf`로 남긴다 — 조용히 건너뛰면 "전부 검사했다"로
읽힌다.

## 정정한 판단

처음에 `artifact stage`에도 `RECORD_ACTOR_FLAGS`를 붙였다. **틀렸다** — 그 명령은 actor 플래그를
등록하지 않는다. `execution prepare` 이전 단계라 lease가 없고, 그래서 holder 검증이 없는 것이
일관된 계약이다. 되돌렸다.

가드 spec(`commandparse/issueops.go`)은 `artifact stage`에 actor 플래그를 등록하지만, 같은 파일의
`cleanup remote-branch` 주석이 밝힌 대로 spec-only 등록은 가드 통과 권한이 아니라 관례 parity
목적이다. CLI가 진실이다.

## 검증

RED가 여섯 종류를 실증했다.

```
--- FAIL: TestUsageDeclaredFlagsAreRegistered
    usage for "phase" names flags that are not registered: --force
--- FAIL: TestActorFlagShorthandMatchesRegisteredFlags
    usage for "remote verify-artifact" uses ACTOR_FLAGS but these are not registered:
    --session-pid, --session-started-at, --session-executable
--- FAIL: TestCommandsRequiringActorDiscloseItInUsage   (24 commands)
--- FAIL: TestUsageTextsDefineActorFlagShorthand        (both texts)
--- FAIL: TestExecutionSubcommandsAppearInUsageTexts
    issueOpsUsageText omits execution subcommands: switch-mode
    adapter usage omits execution subcommands: whoami, switch-mode
```

GREEN 후 `go test ./... -count=1` 전 패키지 PASS. `usage.golden.txt`를 의도된 usage 변경으로
갱신했다.

## 남는 것

- **명령 목록이 두 곳에 중복된다.** `issueOpsUsageText()`와 adapter usage. parity 테스트가 재발을
  잡아주지만 갱신 부담 자체는 남는다. 목록 단일화는 usage를 코드에서 생성하는 큰 변경이라 별건이다.
- **세 명령의 flag 등록을 관측할 수 없다.** 직접 만든 usage 문자열을 FlagSet 기반으로 바꾸면
  덮이지만, 그 세 명령의 안내문은 게이트 요구사항을 설명하는 긴 텍스트라 단순 치환이 아니다.
