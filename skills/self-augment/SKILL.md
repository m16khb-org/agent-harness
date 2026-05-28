---
name: self-augment
description: "Run the 자가 증강 루프 for agent-harness or another repository: use GENIUS_THINK.md, repo evidence, and research-backed agent improvement patterns to choose necessary feature, performance, quality, or documentation improvements, implement one safe high-value diff, and verify it with the 자기 검증 루프. Use when the user asks for self-augmentation, autonomous improvement, repo enhancement, 95점 gate loops, or to decide and execute the next valuable improvement."
---

# 자가 증강 루프

## 목표

레포를 실제로 더 좋아지게 만드는 개선을 스스로 후보화, 선택, 구현, 검증한다. 단순 분석 보고서나 테스트 실행만으로는 완료가 아니다.

## 필수 구분

- **자기 검증 루프**: 서비스/하네스가 의도대로 동작하는지 테스트와 QA를 포함해 확인한다. 기본 명령은 `./bin/harness self-verify --iterations=10 --target-score=95 --json`이다.
- **자가 증강 루프**: 필요한 기능 추가, 성능 개선, 품질/문서 개선 중 하나를 직접 구현하고 자기 검증 루프로 검증한다.

## 종료 조건

종료하려면 아래 목표가 모두 `target_score`를 초과해야 한다. 기본 target은 95점이며, 95점 이하이면 계속 개선하거나 실패 원인을 보고한다.

1. 개선 목표 선별: repo evidence와 `GENIUS_THINK.md`를 사용해 10개 이상 후보를 만들고, 가치/실현 가능성/위험을 점수화한다.
2. 개선 구현: 선택 후보가 실제 코드/문서/스킬 diff로 반영된다. cosmetic-only 변경은 제외한다.
3. 검증·QA: 타깃 테스트, QA 단계, `self-verify`가 통과한다.
4. 학습 기록: 결정, 실패 교훈, 다음 개선 후보를 state/docs 중 적절한 곳에 남긴다.

## Workflow

1. **Baseline**
   - Read nearest `AGENTS.md`/`CLAUDE.md`, `GENIUS_THINK.md`, and `agent_docs/SELF_AUGMENTATION.md` when present.
   - Run or inspect `./bin/harness self-augment --json` for the current candidate curriculum.
   - Use `./bin/harness self-augment --save-state --state-key self-augment-latest --json` when the selected plan should become durable memory for the next cycle.
   - Use `./bin/harness self-augment lesson --lesson "..." --next-action "..." --json` to store reusable Reflexion lessons.
   - Run a baseline 자기 검증 루프 when feasible; otherwise capture why it cannot run.

2. **Candidate curriculum**
   - Generate at least 10 concrete improvement candidates.
   - Use at least two `GENIUS_THINK.md` formulas, preferring 문제 재정의, 혁신적 솔루션, 사고의 진화, 복잡성 해결.
   - Score each candidate by impact, feasibility, novelty, risk, verification cost, and user value.
   - Treat candidates marked `already_satisfied` by the planner as audit history only; select the highest-value `open` candidate so repeated 자가 증강 cycles keep moving to the next necessary improvement.
   - Prefer high-value, low-risk, reversible improvements over broad rewrites.

3. **Select and implement**
   - Choose one candidate whose expected score can exceed 95 after implementation.
   - Make small, reviewable diffs. Do not add dependencies unless the user explicitly asked or evidence shows they are necessary.
   - Preserve host-neutral core boundaries: shared behavior in Go core/ports, host-specific details in Codex/Claude adapters.

4. **Feedback and retry**
   - Convert every failing test, QA issue, or design concern into a short Reflexion-style lesson.
   - Apply the lesson and retry until the goal scores exceed 95 or a hard blocker remains.

5. **Verify**
   - Run targeted tests for the changed behavior.
   - Run `go test ./... -count=1`, relevant golden tests, risk-tier QA checks (`go vet ./...` / `go test -race ./... -count=1` when Go risk is present), skill validation, and build checks as applicable.
   - Finish with `./bin/harness self-verify --iterations=10 --target-score=95 --json` when practical.

6. **Capture**
   - Store durable lessons only when reusable: `harness state`, `agent_docs/`; prefer `self-augment --save-state` for the selected candidate curriculum and `self-augment lesson` for reusable failure/QA/design lessons.
   - Final report includes selected candidate, implemented diff, goal scores, verification evidence, and remaining risk.
