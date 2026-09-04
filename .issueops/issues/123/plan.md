# 123 sealed packet 수명주기 결함 3건 구현 계획

- 이슈: https://github.com/m16khb-org/issueops/issues/123
- 부모 백로그: https://github.com/m16khb-org/issueops/issues/99
- IssueOps: io-893c0bc9d01c / direct / generation 1
- 브랜치: 123-sealed-packet-lifecycle (base main, base head fa1b5cc3f68cddcbffd7d11868e023f6dbfa710b)

## 문제 요약

세 결함이 맞물려 봉인 계약의 목적(owner가 읽는 스코프가 표류·조작되지 않았음을 보증)을 generation 2부터 무력화한다.

1. reseed(lease 소스 332-359행)는 generation을 올리고 claim token만 재발급한다. packet 재봉인 호출이 없어 generation 2 owner는 `lease_generation: 1`과 구 token 경로가 박힌 낡은 packet을 읽는다. 봉인 이후 이슈 본문이 정당하게 개정되면 재봉인 수단이 없어 claim이 영구 거부된다.
2. 초기 claim 검증(owner context 소스 92-121행)은 요청 generation이 1이 아니면 검증 함수를 반환하지 않는다. 따라서 1의 불일치가 무음으로 통과한다.
3. drift 오류가 expected와 observed digest를 담지 않아 owner가 원인을 실측할 수 없다.

## 결함 1과 2 설계: 한 원자 단위

결함 1만 고치면 구멍이 남고, 결함 2만 고치면 generation 2 사이클의 claim이 전부 영구 차단된다. 따라서 같은 변경에 넣는다.

### 재봉인

reseed 케이스에서 orca 모드일 때:

1. `lease.Generation++`과 새 claim token 생성을 먼저 반영한 레코드를 만든다(packet의 `lease_generation`과 `claim_token_file`이 새 값을 담아야 한다).
2. 그 레코드로 원격 이슈를 다시 읽는다. 읽기 실패는 통과가 아니라 거부다.
3. 스테이징 materialize와 owner artifacts 빌더를 그대로 재사용해 packet과 prompt를 재봉인한다. 새 봉인 로직을 작성하지 않는다.
4. 결과에 재봉인된 packet 경로와 digest를 노출해 owner가 claim 명령에 쓸 값을 얻게 한다.

owner host·model·effort는 이미 orca binding에 보관되어 있으므로 요청에 새 플래그를 추가하지 않는다.

이슈 읽기 함수는 replace 의존성에 추가하고, CLI가 prepare·claim에서 이미 쓰는 같은 구현을 주입한다.

### 세대 검증

claim 검증의 generation 1 하드코딩을 요청 generation과의 일치 요구로 바꾼다. 검증 범위가 초기 claim을 넘어서므로 함수명에서 initial을 뗀다(호출부 1곳 동반 변경).

### RED 테스트

- reseed 후 packet의 `lease_generation`이 2이고 `claim_token_file`이 새 경로인지 → 현재는 1과 구 경로로 실패
- generation 2 claim이 낡은 packet으로 거부되는지 → 현재는 검증 스킵으로 통과해 실패
- 이슈 본문 개정 후 reseed를 거치면 claim이 다시 가능한지 → 현재는 packet이 구 digest라 실패
- direct 모드 reseed는 재봉인 대상이 아님을 구분

## 결함 3 설계: drift 진단

두 오류 문자열에 expected와 observed digest를 병기한다. digest는 secret이 아니며, 이 병기로 해시 도구를 관측 allowlist에 넣을 필요가 사라진다.

### RED 테스트

- 이슈 본문 drift 오류에 두 digest가 포함되는지
- packet digest 불일치 오류에 두 digest가 포함되는지

## 수용 기준 매핑

| AC | 검증 |
| --- | --- |
| AC-01 reseed 재봉인 | reseed 후 packet 재생성 테스트 |
| AC-02 packet 정체 일치 | `lease_generation`과 `claim_token_file` 검증 테스트 |
| AC-03 세대 검증 | generation 2 낡은 packet 거부 테스트 |
| AC-04 영구 거부 해소 | 본문 개정 후 reseed 회복 테스트 |
| AC-05 drift 진단 | 오류 문자열 digest 병기 테스트 |
| AC-06 RED 선행 | 각 테스트를 구현 전 실행해 실패 확인 |

## 검증 명령

```bash
go -C /Users/m16khb/Workspace/issueops.worktrees/123-sealed-packet-lifecycle test /Users/m16khb/Workspace/issueops.worktrees/123-sealed-packet-lifecycle/internal/core/issueops
go -C /Users/m16khb/Workspace/issueops.worktrees/123-sealed-packet-lifecycle test /Users/m16khb/Workspace/issueops.worktrees/123-sealed-packet-lifecycle/...
go -C /Users/m16khb/Workspace/issueops.worktrees/123-sealed-packet-lifecycle test /Users/m16khb/Workspace/issueops.worktrees/123-sealed-packet-lifecycle/cmd/issueops/contractgolden
```

## 비범위

- 봉인된 이슈의 본문 편집을 훅이 선제 차단하는 것. 훅 계층 변경이므로 독립 이슈로 분리한다.
- raw 해시 도구를 관측 allowlist에 편입하는 것. 결함 3의 typed 진단으로 필요성이 사라진다.
- core 배선이 없는 dead adapter 코드 삭제.
