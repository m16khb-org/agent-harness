# #51 P0 안전·치명 결함 실행 계획

## 범위

GitHub #51의 세 경계만 수정한다: Engelbart 추적 fixture의 완전 합성화, Shannon shell 측정의 crash/거짓 0 제거, Torvalds bisect 및 `git clean -fd` 안전 계약 보정. Git history rewrite, force-push, P1 이후 항목, reference 대규모 이동은 수행하지 않는다.

## 구현 순서

1. 현재 fixture·shell block·Torvalds 문구를 파일:줄 근거로 inventory하고 각 결함을 재현한다.
   - verify: 민감 문자열 hit, Shannon empty/no-match/space/alias matrix, bisect quoting 및 clean confirmation의 RED 증거를 기록한다.
2. Engelbart fixture 6종을 의미상 동등한 합성 데이터로 최소 교체한다.
   - verify: `python3 scripts/engelbart_skill_contract_test.py`, `python3 scripts/engelbart_quality_rubric.py`, 대상 민감 문자열 zero-hit.
3. Shannon shell 측정 블록을 bounded argv/quoting 계약에 맞게 수정한다.
   - verify: 빈 diff, 무매치, 공백 경로, ugrep alias matrix가 crash 없이 정확한 값을 반환한다.
4. Torvalds bisect는 실행 스크립트 경계를 사용하고 clean은 dry-run → 목록 → stash 대안 → 명시 확인 순서로 보정한다.
   - verify: 깨지는 인용 재현이 GREEN으로 전환되고 `git clean -fd`가 사전 확인 없이 실행되지 않는다.
5. 변경한 skill validation과 관련 계약 테스트를 실행하고 AI slop을 제거한다.
   - verify: focused tests, `python3 scripts/validate-skill.py` 대상 스킬, diff check, worker 보고서가 모두 통과한다.

## 파일 소유권과 중단 조건

- 이 child는 Engelbart P0 fixture, Shannon P0 shell block, `skills/torvalds/references/bisect-protocol.md` 및 Torvalds clean 안전 문구만 소유한다.
- `skills/torvalds/references/rebase-protocol.md`는 #52 소유이므로 수정하지 않는다.
- 민감 원문을 새 파일·로그·보고서에 복사하지 않는다.
- 범위 충돌, history rewrite 필요, destructive command 필요, 모델/usage 변경 요구가 발생하면 즉시 coordinator에 보고하고 중단한다.

## 완료 증거

수정 전 RED, 수정 후 GREEN, 변경 경로, 최종 HEAD, contract/rubric/skill validation 결과, 임시 artifact cleanup receipt를 IssueOps handoff 결과에 포함한다.
