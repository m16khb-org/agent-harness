# 8: validation smoke의 truncate-후-JSON-파싱 결함 수정

이슈: https://github.com/m16khb-org/issueops/issues/8
근거: SV-P fresh-context 측정 진단(재현 2회), 스코어카드 [높음] 후속 항목.

## Root Cause (확정)

`validation_smoke.go:12`의 32KB 예산으로 `commandstep.Run`이 stdout을 tail-truncate(마커 prepend,
`commandstep/result.go:75-95`)하는데, `validateInspectWithDeps:31`·`validateDocsIndexWithDeps:58`이
그 캡처본을 그대로 `json.Unmarshal` → docs 인덱스 38KB 성장으로 main의 95-게이트 결정론적 실패.

## Fix (단일 파일, <30줄)

- [ ] 1. `commandOutputBudgetBytes` 32KB → 4MB (JSON smoke는 전체 출력 필요; 단발 캡처라 메모리 무해)
- [ ] 2. 두 `*WithDeps` 함수에 `step.StdoutTruncated` 가드: Unmarshal 전 명확한 에러
      (`"<label>: stdout truncated (original N bytes > budget); cannot parse JSON — raise the smoke output budget"`)
- [ ] 3. **TDD**: fake `validationCommandRunner`(기존 주입 패턴)로 RED —
      (a) truncated stdout → 현재는 invalid-character, 수정 후 명확한 truncation 에러
      (b) 35KB급 유효 JSON 비-truncate 통과(예산 상향 동작은 self-verify 게이트가 최종 검증)

## Acceptance (이슈와 동일)

- fake-runner 단위 테스트 그린; `go test ./... -count=1` 무회귀.
- `self-verify --seed=100 --target-score=95` main에서 통과(게이트 복구 — 핵심).

## Non-goals

commandstep truncation 정책 변경, docs 인덱스 축소.
