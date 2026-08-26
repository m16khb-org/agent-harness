---
name: cautions/lessons/2026-08-26-ci-gofmt-gate-local-battery-drift.md
description: Dated lesson — CI failed at the gofmt gate on eight consecutive main pushes while the local battery, a working-tree-dependent golden, and a symlink-rejecting skill validator hid three independent breakages.
---

# 2026-08-26 — CI gofmt gate drifted from the local battery; golden captured a dirty working tree

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: agent-harness/issueops/skills 품질 감사 (2026-08-26)
- Summary: `.github/workflows/ci.yml`은 첫 단계에서 `gofmt -l $(git ls-files '*.go')`를
  실행하지만 `AGENTS.md §9`와 `self-verify` 배터리에는 gofmt가 없었다. 2026-08-20~26 사이
  커밋된 10개 파일이 gofmt 미적용 상태였고, `main`의 마지막 8개 push는 모두 21~29초 만에 CI
  failure로 끝났다. CI가 첫 게이트에서 끊겨 그 뒤의 `go test`는 실행되지 않았고, 그 뒤에는
  독립적인 두 실패가 더 숨어 있었다.
- Context:
  - `TestResponseContractsGolden`: `self-augment`의 `implementation_delta` goal은 설계상 실제
    harness working tree의 `git status --porcelain`을 관찰한다(`augmentplan/plan_evidence.go`).
    golden이 dirty tree에서 `-update`되어 `score=100/passed=true/"non-empty"` 관측값을 그대로
    저장했고, clean checkout(CI, 새 세션)에서는 반드시 실패했다.
  - `TestAllSkillShellFencesValidate`: `.gitignore`는 `skills/kody-review-feedback` 같은 로컬
    외부 스킬 심링크를 허용하고 `inspect`/installer(`ListSkillNames`)는 심링크 항목을 무시하지만,
    `scripts/verify-skill-shell.py`는 `skills/` 아래 모든 심링크를 `symlink-not-allowed`로
    거부해 같은 저장소가 머신에 따라 통과/실패했다.
  - 세 결함 모두 "로컬에서 통과했다"는 보고와 양립했다. 로컬 배터리가 CI와 다른 게이트
    집합을 쓰고, 검증 결과가 환경 상태(working tree, 로컬 심링크)에 따라 달라졌기 때문이다.
- Resolution:
  - `self-verify`에 무조건 실행되는 `gofmt` 단계(`cmd/harness/validationcli/goformat`)를 추가해
    CI Format check와 같은 파일 집합·같은 판정 조건을 로컬에서 재현한다. `AGENTS.md §9`와
    `testing/unit-and-contract.md`의 기본 검증에도 `gofmt -l $(git ls-files '*.go')`를 넣었다.
  - golden test는 `implementation_delta` goal의 evidence/score/passed를 `$WORKING_TREE_OBSERVATION`
    placeholder로 정규화한다(#109 `docs_count` 정규화와 같은 구조적 제거). goal 로직 자체는
    `augmentplan/plan_evidence_test.go`가 clean/dirty 임시 저장소로 검증한다.
  - `verify-skill-shell.py`는 스캔 루트 직하에서 `SKILL.md`를 가진 디렉터리로 해석되는 심링크만
    "로컬 외부 스킬 링크"로 보고 건너뛴다. 중첩 심링크(`references -> ...`)와 SKILL.md가 없는
    링크는 여전히 위반이다.
- Evidence:
  - `gh run list --limit 8` → 8건 모두 `failure`, 21~29초; `gh run view <id> --log-failed` →
    `These files are not gofmt-clean:` 뒤 `Process completed with exit code 1`.
  - 로컬 `gofmt -l $(git ls-files '*.go')` → 10개 파일. `go test ./... -count=1` →
    `FAIL agent-harness/cmd/harness/harnessapp`(golden `$.cli.self_augment_plan.goals[1]`),
    `FAIL agent-harness/internal/adapter/skillcontract`(`symlink-not-allowed`).
  - `./bin/agent-harness quality inspect --json` → `ok:false`, `warnings:["coverage: exit status 1"]`
    (같은 테스트 실패가 coverage 수집을 막아 품질 게이트까지 `block`으로 보였다).
- Rule: 새 CI 게이트를 추가하면 같은 명령을 `AGENTS.md §9`, `TESTING.md` 기본 검증, 그리고
  가능하면 `self-verify` 단계에도 같은 판정 조건으로 넣는다. golden이나 검증기가 환경 관측값을
  읽는다면 그 값을 placeholder로 정규화하거나 fixture로 고정하고, 절대 실제 tree 상태를
  기록하지 않는다. 완료 보고 전에는 `gh run list`로 CI 상태를 한 번 확인한다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
