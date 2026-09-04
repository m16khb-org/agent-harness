# 480 — 이슈별 산출물을 `.issueops/issues/<번호>/`로 모은다 (v3)

이슈: https://github.com/m16khb-org/issueops/issues/480
브랜치: `480-issue-artifact-folder` (base: `main` `32ae063e`)
v2: design-review `revise`(2026-08-27) 반영 — Orca artifact 이동·도메인 패키지·신규 CLI 제거, 중복 판정을 사이클 단위로 축소, 이동 분류 규칙과 링크 리라이트 추가.
v3: design-review 2차 `revise` 반영 — `issueops status` 필드(T3) 삭제(레코드 영속 계약 오염), 링크 게이트를 번호 플랜 경로로 한정, docs/superpowers 링크 수정을 T5 범위에 명시.

## 목표

이슈 번호 하나로 플랜·게이트 원장·스펙·리뷰 작업 파일을 한 폴더에서 찾는다. 옛 경로는 읽기 호환으로 남기고, 현재 사이클의 이슈가 새/옛 게이트 경로에 동시에 있으면 fail-closed한다.

## 레이아웃 (AC-01)

```
.issueops/issues/<n>/
  plan.md            추적   기본 플랜 (link-plan 대상). 같은 번호에 플랜이 여럿이면 plan-<slug>.md
  gates.md           추적   gates init/check 원장
  spec.md            추적   선택, superpowers 스펙을 이슈에 붙일 때
  review/            무시   pr-review·review-agent-feedback 작업 파일 (<provider>-<mr번호>/)
```

`<n>`은 linked issue URL의 번호. 번호를 모르면 옛 규칙(`.issueops/tmp/`)을 그대로 쓰고 출력에 남긴다. `.issueops/artifact/`(Orca 봉인 아티팩트: gitignore + 0600 불변 + 레코드 절대 경로)는 **이번 범위 밖**이다 — 봉인 계약 재설계가 필요하므로 후속 이슈로 분리한다.

## 작업 순서

| # | 작업 | 파일 | 검증 |
|---|---|---|---|
| T1 | `internal/adapter/gates/check.go DiscoverGateFiles`: `.issueops/issues/*/gates.md`를 canonical 1순위 후보(이슈 번호 오름차순 → 파일명)로 추가, 기존 `GATES.md`·`.issueops/gates/*.md`·`gates/*.md` 후보 유지 | check.go, check_test.go | `go test ./internal/adapter/gates` |
| T2 | `internal/adapter/issueops/gatesgate`: 레코드의 linked issue 번호 `n`(`issueopsremote.IssueNumber`)에 대해 `issues/<n>/gates.md`와 `.issueops/gates/{issue-<n>*,<n>-*}.md`가 동시에 있으면 `duplicate_issue_artifact:<n>` missing. 번호가 없으면 검사하지 않음 | gates_gate.go, gates_gate_test.go | `go test ./internal/adapter/issueops/gatesgate` |
| T4 | 문서·스킬: CONVENTIONS(레이아웃 규정 한 번), CAUTIONS:22 정정, issueops SKILL:72 `GATE_FILE=".issueops/issues/<n>/gates.md"`·:248 플랜 경로, implementation-planning 플랜 경로(`.issueops/issues/<n>/plan.md`, 이슈 없으면 `.issueops/plans/<slug>.md` 유지), issueops-cleanup, pr-review `mr_context.py` `--out` 기본값(`issues/<n>/review/<provider>-<mr>` when issue known, else tmp), review-agent-feedback SKILL 출력 경로, `.gitignore`(`.issueops/issues/*/review/`, `.issueops/tmp/`) | 문서/스크립트 | validate-skill, verify-skill-shell, docs checker |
| T5 | 이동 커밋(별도): 분류 규칙 `^(\d+)-` 또는 `^issue-(\d+)`(확장자 무관). 63개 번호 파일(60 이슈) → `issues/<n>/plan.md`; 248·414(.md 2개씩)·46(.json 2개)은 `issues/<n>/plan-<slug>.<ext>`; 25개 무번호 → `issues/_unnumbered/`; `gates/477-cleanup-stops-worktree-processes.md` → `issues/477/gates.md`. 옛 번호 플랜 링크를 새 경로로 리라이트 — `skills/`, `.issueops/`, `docs/superpowers/{plans,specs}` 포함(`rg 'issueops/plans/(\d+-|issue-\d+)' --type md` 0건 게이트; 이슈 없는 플랜의 `.issueops/plans/<slug>.md` fallback 경로는 유지) | `git mv` + 링크 sed | 아래 T5 검증 |
| T6 | 골든 재생성·전체 배터리·self-verify QA gate | testdata | AGENTS.md §9 |

T1→T2, T4 독립, T5는 T1 이후, T6 마지막.

T5 검증은 설치 바이너리가 아니라 `go build -o bin/issueops ./cmd/issueops` 후 `./bin/issueops`로 한다: 이동 전 `gates check --json`의 파일·게이트 집합과 이동 후 집합이 경로만 다르고 게이트 ID·상태가 같아야 하며, `issueops status --id io-69c796434060 --json`의 readiness가 변하지 않아야 한다.

## 비목표

`issueops status`/레코드에 파생 필드 추가(status는 레코드 자체를 반환하므로 영속 계약 오염). `.issueops/artifact/` 봉인 아티팩트 이동(후속 이슈), docs/superpowers 이동, 게이트 판정 규칙 변경, 원격 이슈 본문 구조 변경, 레거시 경로 제거, 신규 CLI 명령.

## 롤백

코드 커밋 revert 시 `issues/*/gates.md`는 읽히지 않지만 옛 후보는 그대로다. 이동 커밋만 revert해도 T1이 남아 있으면 옛 경로가 여전히 읽히므로(후보 목록 유지) 어느 한쪽만 되돌려도 게이트가 사라지지 않는다.

## 후속 이슈 기록

- `gatesgate.withGatesGate`(`gates_gate.go:88-97`)는 발견된 모든 원장을 현재 사이클 readiness에 합산한다. `issues/*/gates.md`가 누적되면 다른 이슈의 미완 원장이 내 PR을 막을 수 있다. 판정 규칙 변경은 이번 비목표이므로 후속 이슈로 남긴다(현재는 477 원장 1개, 8/8 완료).
