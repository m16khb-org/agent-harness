# 이슈 #107 — Codex 검토 반영: 문서 경계 정정 + cleanup usage finish 누락

이슈: https://github.com/m16khb/agent-harness/issues/107

## 변경 (brooks devil's-advocate revise 반영)

1. `.agent-harness/CONVENTIONS.md` reducer contract 절 — **3단 분류로 재기술**(strict만 빼는 정정은 거짓을 거짓으로 교체):
   - **record-only 순수**: `IssueOpsProblemReadiness`/`IssueOpsPlanReadiness`/`IssueOpsGrillReadiness`만(실측: record 필드만 읽음).
   - **FS 존재검사 수행**: `IssueOpsCompatibilityReviewReadiness`/`IssueOpsImplementationReadiness`/`IssueOpsAISlopCleanReadiness`(+비-strict `IssueOpsPRReadiness`) — `os.Stat`/`EvalSymlinks` 경유(`readinesspaths.go:22-25,34-36,58-66`).
   - **git·network 수행**: `IssueOpsStrictPRReadiness` — rev-parse·branch·status·`git fetch`·rev-list 직접 실행(`issueops_pr_readiness_strict.go:23-46`).
   - stale 라인 앵커 전면 재실측 교체: `validateIssueOpsPhaseTransition`=`issueops_phase.go:75`, `time.Now()`=:142, `issueOpsCurrentHead`/`ChangeFingerprint`=:148-149, `touchAndWriteIssueOps`=:72.
   - 두 번째 bullet의 "git/FS read는 전이 함수 밖 wrapper 소유" 주장 정정: PR 진입 시 validation이 strict 게이트를 직접 호출하므로 경계 서술을 실측에 맞게 완화.
2. `feedback_cleanup.go` usage에 `cleanup finish` 라인 추가 — **canonical 문구를 그대로 복사**(`issueops_cli_support.go:65`/`adapter/cli/usage.go`와 byte 일치, 세 번째 변종 금지).
3. 재발 방지 테스트(issueopscli 패키지, 기존 `captureStdoutForContract` 재사용): canonical usage(`issueOpsUsageText`)의 `issueops cleanup <sub>` 라인 집합 ↔ `runIssueOps(["cleanup","--help"])` 출력의 하위 usage 라인 집합 **양방향 비교** + 각 항목이 sentinel("unknown issueops cleanup subcommand") 없이 디스패치되는 행위 검증. 하드코딩 목록 테스트 금지(#93 계보의 파생 원칙).

## TDD 순서

1. RED: 양방향 테스트 작성 → 하위 usage의 finish 부재 검출로 실패.
2. GREEN: usage 라인 추가(canonical 복사).
3. 문서 3단 분류 재기술(테스트 무관), 회귀: issueopscli·전체 green.

## 비범위

- readiness 게이트들의 실제 순수화 리팩터(CONVENTIONS §28), typed 원격 브랜치 삭제(#99).

## 역할 분담

- 계획·리뷰: Fable 5. 구현안: Opus 5 서브에이전트(holder 적용).
