---
name: gates-ledger
description: Create, check, and report task gate ledgers with the agent-harness gates CLI. Turns acceptance criteria into G-numbered CHECK and EXPECT gates in .agent-harness/issues/<n>/gates.md or .agent-harness/gates/<scope>.md, fills EVIDENCE by running the checks through the command policy, and abandons gates honestly. Use when issueops-plan, issueops-implement, issueops-clean, issueops-verify, or turing needs a gate ledger, or when the user says "게이트 원장", "gates 만들어줘", "수용 기준 체크".
---

# Gates Ledger

이 스킬의 일은 **수용 기준을 실행 가능한 게이트 원장으로 만들고 그 원장을 CLI로
채우는 것**이다. 원장은 "무엇이 됐다고 말할 수 있는가"를 명령과 기대 출력으로
고정한다. 사람이 읽고 판단하는 체크리스트가 아니라 다시 실행할 수 있는 기록이다.

- 계획이 게이트를 만든다: [`issueops-plan`](../issueops-plan/SKILL.md)
- 구현이 EVIDENCE를 채운다: [`issueops-implement`](../issueops-implement/SKILL.md)
- 정리 뒤 재실행: [`issueops-clean`](../issueops-clean/SKILL.md)
- 검증이 읽기 전용으로 재검사한다: [`issueops-verify`](../issueops-verify/SKILL.md)

## 경로 규칙

- provider 이슈 번호가 있으면 `<worktree>/.agent-harness/issues/<n>/gates.md`다.
- 번호가 없으면 `.agent-harness/gates/<scope>.md`다.
- 원장은 워크트리 **안**에 둔다. 밖에 두면 변경 집합에 들어가지 않아 커밋되지 않고,
  다음 세션이 그 원장을 찾지 못한다.
- 같은 번호의 canonical 원장과 legacy 원장이 함께 있으면 pr 진입이
  `duplicate_issue_artifact:<n>`으로 막힌다. 옛 경로의 원장은 옮기고 지운다.

## 만들기

수용 기준 하나가 게이트 하나다. 결과는 관찰 가능한 동작으로 쓰고, 그것을 관찰하는
읽기 전용 명령과 그 출력에 들어 있어야 할 문자열을 함께 적는다.

```bash
agent-harness gates init --file "$WORKTREE/.agent-harness/issues/$ISSUE/gates.md" --scope "$ISSUE" \
  --gate "G1: <관찰 가능한 결과> | CHECK: <read-only 명령> | EXPECT: <출력에 포함될 문자열>" \
  --gate "G2: <결과> | CHECK: <명령> | EXPECT: <문자열>" --json
```

- 결과 문장은 "테스트를 추가한다"가 아니라 "미머지 아티팩트는 닫히고 머지된 것은
  거부된다"처럼 동작으로 쓴다.
- CHECK는 command policy를 지나므로 셸 확장과 파이프 우회를 넣지 않는다.
  `$(...)`, 백틱, `&&`로 이어 붙인 우회는 정책이 거부한다. 한 게이트에 명령 하나다.
- 통과 조건은 EXPECT 문자열 일치와 종료 코드 0 **둘 다**이다. 한쪽만으로는 통과하지
  않는다.
- 자동으로 관찰할 수 없는 결과는 게이트로 쓰지 않는다. 그런 것은 수동 확인 기록으로
  남기고 EVIDENCE에 관측 시각과 관측값을 적는다.

## 검사

```bash
agent-harness gates check --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --write --json
```

- `--write`가 각 게이트의 EVIDENCE를 실제 출력으로 채운다. 이것이 원장을 채우는
  유일한 방법이다.
- 네트워크가 필요한 CHECK는 `--network`를 붙인다. 붙이지 않으면 정책이 막는다.
- 오래 걸리는 CHECK는 `--timeout-seconds N`으로 한도를 올린다.
- 종료 코드 1은 미충족 게이트가 남았다는 뜻이다. 실패가 아니라 아직 아니라는 뜻이므로
  원장을 고치지 말고 구현을 고친다.

## 상태와 보고

```bash
agent-harness gates status --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --json
agent-harness gates report --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE"
```

`--file`·`--cwd`·`--workspace-root`는 세 명령에서 같은 값을 쓴다. 다르면 다른 원장을
보게 되고, 통과했다고 보고한 원장과 게이트가 읽는 원장이 달라진다.

## 포기

실행할 수 없다고 판정한 게이트는 정직하게 닫는다.

```bash
agent-harness gates abandon --gate G3 --reason "<왜 이 게이트를 실행할 수 없는가>" --file "$LEDGER" --json
```

EXPECT를 느슨하게 고쳐 통과시키지 않는다. 그렇게 하면 원장은 통과하고 결과는
검증되지 않은 채 남는다. 포기 사유는 다음 사람이 같은 판단을 다시 할 수 있게 쓴다.

## IssueOps와의 관계

- strict pr-readiness는 자기 이슈 번호의 원장과 익명 원장만 판정하고, 다른 번호의
  원장은 warning으로 건너뛴다.
- 미충족 게이트가 남으면 `gates_incomplete:<file>`로 pr 진입이 막힌다. 원장이 아예
  없으면 이 요구는 추가되지 않는다.
- 단계별 소유: 3단계 계획이 원장을 만들고, 4단계 구현이 `--write`로 EVIDENCE를 채우며,
  5단계 정리 뒤 다시 `--write`로 재실행하고, 7단계 검증이 읽기 전용으로 재검사한다.
- 원장 파일은 변경 집합에 포함되므로 `ai-slop-clean record` 이후에 원장을 고치면
  fingerprint가 바뀌어 `ai_slop_clean_stale`이 된다. 원장 갱신은 봉인 전에 끝낸다.

## 나쁜 예

- EVIDENCE를 손으로 적는다. 명령을 실행하지 않고 채운 증거는 증거가 아니다.
- 실패한 CHECK를 EXPECT 완화로 통과시킨다.
- 원장을 source checkout이나 워크트리 밖에 만든다. 커밋되지 않고 다음 세션이 못 찾는다.
- 같은 이슈 번호로 canonical과 legacy 원장을 둘 다 남긴다. pr 진입이 막힌다.
- CHECK에 `$(...)`나 파이프 체인을 넣는다. 정책이 거부하고, 통과해도 무엇을 관찰했는지
  다음 사람이 재현할 수 없다.
- 게이트를 "구현한다"처럼 작업으로 쓴다. 작업은 끝났는지 관찰할 수 없다.

## 검증

- `agent-harness gates status --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --json`으로
  미충족 게이트가 0인지 확인한다.
- `agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json`의 `missing`에
  `gates_incomplete:`로 시작하는 키가 없는지 확인한다.
- 원장 파일이 `git -C "$WORKTREE" status --porcelain`에 커밋 대상으로 보이는지 확인한다.
