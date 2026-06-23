# IssueOps Issue and PR/MR Guidelines

IssueOps가 생성하는 이슈와 PR/MR 본문은 반드시 한글로 작성한다. 명령어, 코드 식별자, 파일 경로, 외부 문서명은 원문을 유지할 수 있다.

## Source References

- GitHub Docs: issue and pull request templates standardize the information contributors provide.
  - https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/about-issue-and-pull-request-templates
- GitHub Docs: issue forms support required fields, labels, and assignees; pull requests use markdown templates instead.
  - https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms
  - https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/creating-a-pull-request-template-for-your-repository
- GitLab Docs: description templates and quick actions standardize issue/MR content; linked issues and child items should use native provider relationships.
  - https://docs.gitlab.com/user/project/description_templates/
  - https://docs.gitlab.com/api/issue_links/
  - https://docs.gitlab.com/user/work_items/child_items/
- GitHub Docs: pull requests propose and collaborate on branch changes; PRs should include a title and description, and can link to issues.
  - https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request
- Kubernetes Contributors: small, reviewable PRs are preferred over unreviewable monoliths.
  - https://www.kubernetes.dev/docs/guide/pull-requests/
- Kubernetes Contributors: unrelated fixes or generic cleanup should be split into separate PRs.
  - https://www.kubernetes.dev/docs/guide/pull-requests/#open-a-different-pull-request-for-fixes-and-generic-features
- Kubernetes Contributors: tests should be adequate; if a change touches code, test coverage usually matters.
  - https://www.kubernetes.dev/docs/guide/pull-requests/#test
- Kubernetes Contributors: generated or large automated changes should explain how they were generated and how reviewers can reason about them.
  - https://www.kubernetes.dev/docs/guide/pull-requests/#large-or-automatic-edits
- React contribution guide: bug-fix PRs can be submitted directly, but filing an issue that details what is being fixed is still recommended.
  - https://gaearon.github.io/react/contributing/how-to-contribute.html

## Issue Draft Gate

An IssueOps issue draft should include:

- 문제: 사용자가 해결하려는 문제와 왜 지금 필요한지.
- 현재 근거: 실제 파일, 명령 결과, 로그, 사용자 피드백, 또는 확인된 제약.
- 관련 이슈/라벨 판단: selected/rejected related issues and labels, threshold, and manual override reason if any.
- 완료 기준: 검증 가능한 acceptance criteria.
- 비목표: 이번 작업에서 하지 않을 것.
- 구현 범위: 포함할 변경과 분리해야 할 관련 없는 변경.
- 검증: 실행할 테스트, 빌드, 스모크 체크, 품질 게이트.
- 위험과 트레이드오프: 남는 리스크, 대안, 선택 이유.
- 피드백 기록: 사용자 피드백과 요구사항 변경 이력.
- 가이드라인 참조: 이 문서 경로 또는 source reference를 명시한다.
- 이모지 절제: 의미를 돕는 소수의 이모지는 허용하지만 장식적이거나 과도한 이모지는 쓰지 않는다.

Bug issues additionally include 재현 절차, 기대 동작, 실제 동작, 환경, and 로그/출력. Child task issues use the narrower child contract: 부모 이슈, 작업 목표, 완료 기준, 비목표, 검증, 부모 브랜치 병합 조건, child-only cleanup 규칙.

## PR/MR Draft Gate

An IssueOps PR/MR draft should include:

- 의도: 이 변경이 해결하는 문제.
- 이슈 링크: 관련 issue URL 또는 `Closes/Fixes` 정책에 맞는 참조.
- 변경사항: 리뷰 가능한 단위의 변경 요약.
- 검증: 실제 실행한 테스트, 빌드, 스모크 체크와 결과.
- 리뷰어 초점: reviewer가 집중해야 할 계약, 경계, 회귀 위험.
- 위험/rollback: reviewer가 집중해야 할 리스크, 남은 한계, 되돌림 방법.
- 사용자 영향/릴리즈 노트: 사용자에게 보이는 영향과 release-note 필요 여부.
- 문서/마이그레이션: 문서, schema, 설정, migration 필요 여부.
- 범위 관리: 관련 없는 수정이 섞이지 않았다는 근거 또는 분리 계획.
- 워크트리 정리: 격리 worktree 상태와 cleanup 선택지.
- 자동화/AI 개입 근거: 생성형 AI, 대량 변경, 자동화가 개입한 경우 생성 방식과 재현 방법.
- 가이드라인 참조: 이 문서 경로 또는 source reference를 명시한다.
- 이모지 절제: 의미를 돕는 소수의 이모지는 허용하지만 장식적이거나 과도한 이모지는 쓰지 않는다.
- 다이어그램: 복잡한 흐름, 상태 전이, 아키텍처 경계가 reviewer 이해를 실제로 줄일 때만 Mermaid 등 텍스트 기반 다이어그램을 포함한다. 필요 없는 다이어그램을 형식적으로 넣지 않는다.

## Template Rendering Contract

Use the shared renderer for remote artifacts when possible:

```bash
agent-harness issueops remote render-template --kind issue --template implementation_task --provider github --title "$TITLE" --field problem="$PROBLEM" --json
agent-harness issueops remote create-issue --id "$ISSUEOPS_ID" --template implementation_task --field problem="$PROBLEM" --label "$LABEL" --assignee "$ASSIGNEE" --json
agent-harness issueops remote create-pr --id "$ISSUEOPS_ID" --template pull_request --label "$LABEL" --assignee "$ASSIGNEE" --json
```

`--body` and `--body-file` are mutually exclusive. When `--template` is used with an explicit body, the body is validated against the same canonical section policy. Confirmed remote writes fail closed on critical validation failures, missing label, missing assignee, Korean artifact failure, and PR/MR target branch mismatch.

`--field` accepts canonical template keys and the aliases documented in `skills/issueops/references/remote-issue.md` (`goal`, `logs_output`, PR/MR `risks`, `documentation`, `scope`, `cleanup`, and `automation`).

## Benchmark Quality Gate

The deterministic benchmark fails when:

- issue draft or PR/MR draft is primarily English instead of Korean,
- issue draft omits core sections from this guideline,
- PR/MR draft omits core sections from this guideline,
- artifact bundle does not reference this guideline,
- issue draft or PR/MR draft uses excessive emoji decoration,
- PR/MR evidence suggests a broad or unrelated diff without explaining split decisions,
- verification evidence is absent or only aspirational.

The deterministic benchmark treats diagrams as conditional reviewer support, not a universal hard requirement. Complex PR/MR drafts should include a diagram only when it materially reduces review effort. Gratuitous diagrams are a quality failure.
