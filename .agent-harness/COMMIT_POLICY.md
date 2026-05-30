---
name: COMMIT_POLICY.md
description: Commit message format, scope, and decision-record rules.
---

# 커밋 메시지 정책: Conventional + Lore Hybrid

`agent-harness`는 개발자와 AI agent가 모두 활용하기 쉽도록 **Conventional Commit subject + Lore body**를 사용한다.

---

## 1. 기본 형식

```text
<type>(<scope>)!?: <summary>

Lore:
- Intent: <이 커밋이 달성하는 목적>
- Why: <맥락/이유. 티켓·요청·문제 원인 포함 가능>
- Changes:
  - <주요 변경 1>
  - <주요 변경 2>
- Verify: <실행한 검증 또는 Not-tested: 이유>
- Risk: <남은 위험/롤백 주의. 없으면 Low>
```

Subject는 Conventional Commit으로 사람이 `git log --oneline`에서 빠르게 읽게 하고, body의 `Lore:` 블록은 AI agent가 변경 맥락·검증·위험을 구조적으로 회수할 수 있게 한다.

---

## 2. Subject 규칙

- 형식: `<type>(<scope>)!?: <summary>`
- summary는 imperative mood의 짧은 영어 문장으로 작성한다.
- 가능한 72자 이내를 유지한다.
- type은 아래 기본값을 우선한다.

| type | 용도 |
|------|------|
| `feat` | 사용자/agent-visible 기능 추가 |
| `fix` | 버그 수정 |
| `docs` | 문서만 변경 |
| `refactor` | 동작 변화 없는 구조 개선 |
| `test` | 테스트 추가/수정 |
| `chore` | 빌드, 설정, 유지보수 |
| `ci` | CI/CD 변경 |
| `perf` | 성능 개선 |
| `style` | formatting/style-only 변경 |
| `revert` | 이전 커밋 되돌림 |

예시:

```text
docs(skill): define hybrid commit policy
feat(cli): add workspace inspection command
fix(worker): prevent stale socket reuse
```

---

## 3. Lore body 규칙

- non-trivial commit은 `Lore:` 블록을 포함한다.
- trivial docs/typo commit도 가능하면 `Intent`와 `Verify`만이라도 남긴다.
- `Changes`는 diff 전체가 아니라 의사결정에 필요한 요약만 남긴다.
- `Verify`에는 실제 실행한 명령을 적는다. 실행하지 않았으면 `Not-tested: <reason>`을 적는다.
- `Risk`에는 migration, compatibility, secret, generated artifact, incomplete validation 같은 후속 주의사항을 적는다.
- secret, token, private URL, 고객 데이터 원문은 Lore body에 쓰지 않는다.

간단한 커밋 예시:

```text
docs(skill): add atomic commit workflow

Lore:
- Intent: Codify a safe commit-and-push workflow shared by Codex and Claude.
- Why: Avoid broad staging and preserve unrelated user changes.
- Changes:
  - Add atomic-commit-push skill instructions.
  - Add read-only git preflight helper.
- Verify: quick_validate.py skills/atomic-commit-push
- Risk: Low; documentation and read-only helper only.
```

---

## 4. Atomic commit과의 관계

- 하나의 commit은 하나의 intent만 가져야 한다.
- Lore의 `Intent`가 둘 이상이면 commit을 나눌 신호다.
- subject의 scope와 Lore의 Changes가 맞지 않으면 staging 범위를 다시 확인한다.
- generated file, docs, tests가 source 변경과 직접 결합된 경우에만 같은 commit에 포함한다.
