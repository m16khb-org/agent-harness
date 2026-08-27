# 490 — 재타깃된 stacked PR의 정리 dead-end 해소 (v2)

v2: brooks `revise` 반영 — `--accept-retargeted-base` 승인 플래그와 T3 전체 삭제(자기주장 승인은 진짜 오머지를 통과시키고, 이 저장소의 유일한 우회로 `--superseded-by`는 provider readback으로 검증된다), 슬러그를 `merged_base_remote_unobserved`로 정정(ancestry를 계산하지 않으므로 이름이 사실과 달랐다), `ls-remote --symref origin HEAD`는 명령 정책(`exactReadOnlyGitLSRemote`)이 거부하므로 문서·수동 절차에 적지 않는다.

이슈: https://github.com/m16khb/agent-harness/issues/490
브랜치: `490-retargeted-base-cleanup` (base: `main` `e8923d5b`)

## 결함

`cleanupFinishGates`(`issueops_cleanup_finish.go:238-248`)는 준비 base와 관측 base가 다르면 무조건 `base_branch_drifted`로 거부한다. stacked PR의 부모 브랜치가 머지·삭제되어 PR이 기본 브랜치로 재타깃되는 정상 흐름도 여기 걸리고, `cleanup abandon`은 머지된 artifact를 `remote_artifact_unmerged`로 거부하므로 레코드가 어느 경로로도 정리되지 않는다(실측: `io-71af6dd82f0d`).

## 판정 규칙 (관측 기반)

| 준비 base 원격 존재 | 관측 base | 판정 |
|---|---|---|
| 없음(머지·삭제) | 기본 브랜치와 일치 | 통과 — 정상 재타깃, 근거를 결과에 기록 |
| 있음 | 무엇이든 | `base_branch_drifted` |
| 없음 | 기본 브랜치와 불일치 | `base_branch_drifted` |
| 관측 실패(ls-remote 비정상 종료, symref 빈/이형 출력) | — | `merged_base_remote_unobserved` |

명시 승인 플래그는 두지 않는다. 재현 사례는 자동 규칙만으로 닫히고, 관측값을 되받아 적는 승인은 준비 base가 살아 있는 진짜 오머지까지 통과시킨다. 원격 관측 결과는 기존 규율대로 fingerprint 입력이 아니다(관측 실패가 preview 재발급 루프를 만들지 않도록).

## 작업

| # | 작업 | 파일 | 검증 |
|---|---|---|---|
| T1 | 순수 판정 함수 `classifyMergedBase(preparedBase, observedBase, defaultBranch string, preparedBaseRemotePresent, observed bool) (missing string)` | issueops_cleanup_finish.go, 테스트 | `go test ./internal/adapter/issueops -run MergedBase` |
| T2 | 게이트에서 준비 base 부재와 기본 브랜치를 기존 `deps.Git` ls-remote 패턴으로 관측(`ls-remote --heads origin refs/heads/<base>`, `ls-remote --symref origin HEAD`)해 T1에 넘기고, 결과에 `retargeted_base`(준비/관측/기본/승인) 근거를 남긴다 | 위 파일 | 단위 테스트(fake Git) |
| T3 | 문서: `skills/issueops-cleanup/SKILL.md`에 재타깃 판정 한 문단(수동 확인은 `ls-remote --heads`만 쓰고 `--symref`는 정책이 거부하므로 적지 않는다), `.agent-harness/CAUTIONS.md` 한 줄 | 문서 | validate-skill, docs checker |
| T4 | 실환경(AC-05): 고친 바이너리로 `io-71af6dd82f0d` preview → 통과 확인 → apply로 워크트리·브랜치·레코드 삭제 | 수동 | 원장 G5 |
| T5 | 배터리·골든 | | AGENTS.md §9 |

## 비목표

`cleanup abandon` 머지 판정 변경, PR 재타깃 대행, 원격 관측 표면 신설, contract·CLI 플래그 추가.

## 롤백

한 함수 판정의 revert(계약 변경 없음). revert 후에는 재타깃 레코드가 다시 dead-end가 되므로, 그 전에 남은 레코드를 정리한다.
