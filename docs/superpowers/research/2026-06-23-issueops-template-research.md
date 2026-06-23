# IssueOps Remote Issue/PR/MR Template Research

Date: 2026-06-23

## Goal

Improve IssueOps remote artifact templates without depending on repo-native templates as the only quality gate. The canonical source of truth is the shared renderer/validator in `agent-harness issueops remote`, with GitHub/GitLab native templates kept aligned for humans using the provider UI.

## Provider Findings

| Provider | Adopt | Reason |
| --- | --- | --- |
| GitHub issue and pull request templates | Yes | Templates standardize submitted information, but markdown PR templates cannot enforce required fields. |
| GitHub issue forms | Yes | YAML forms support required fields, labels, and assignees, so repo-native issue forms should mirror canonical sections. |
| GitHub sub-issues | Yes | Current `gh` supports `gh issue create --parent` and `gh issue edit --add-sub-issue`; REST remains the fallback when CLI support or permission is missing. |
| GitLab description templates | Yes | Markdown templates cover issues and MRs and can include quick actions when useful. |
| GitLab issue links | Yes | Related/non-hierarchical issue relationships belong in native linked items rather than a GitLab body section. |
| GitLab child work items | Yes | Large IssueOps breakdown should use provider-native child Task work items instead of body-only task lists. |

Source links:

- https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/about-issue-and-pull-request-templates
- https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms
- https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/creating-a-pull-request-template-for-your-repository
- https://docs.gitlab.com/user/project/description_templates/
- https://docs.gitlab.com/api/issue_links/
- https://docs.gitlab.com/user/work_items/child_items/

## OSS Template Patterns

| Project family | Observed pattern | Decision |
| --- | --- | --- |
| Kubernetes | PR type, issue link, release note, docs, and test focus. | Adopt issue link, verification, reviewer focus, user impact/release note, and docs/migration sections. |
| Flutter | Bug reports emphasize repro steps, expected/actual output, environment, logs, and diagnostic output. | Adopt bug-only reproduction, expected, actual, environment, and logs/output fields. |
| Rust diagnostics | Bug templates ask for code, current output, desired output, and version. | Adopt evidence-first bug sections and avoid vague "does not work" fields. |
| GitLab | Separate operational templates for bug, security, feature flags, and decisions. | Keep issue kinds separate: bug, feature, proposal, implementation task, child task. |
| Uber/Naver/Twitter(X)/Woowa public repos | Mixed quality: some repos have detailed repro/what/why/change/test templates; others have no or thin templates. | Do not depend on repo-native templates as the sole gate; use core renderer/validator for automation. |

## Canonical Issue Sections

- 문제
- 현재 근거
- 관련 이슈/라벨 판단
- 완료 기준
- 비목표
- 구현 범위
- 검증
- 위험과 트레이드오프
- 피드백 기록

Bug additions: 재현 절차, 기대 동작, 실제 동작, 환경, 로그/출력.

Child task sections: 부모 이슈, 작업 목표, 완료 기준, 비목표, 검증, 부모 브랜치 병합 조건, child-only cleanup 규칙.

## Canonical PR/MR Sections

- 의도
- 이슈
- 변경 사항
- 검증
- 리뷰어 초점
- 위험/rollback
- 사용자 영향/릴리즈 노트
- 문서/마이그레이션
- 범위 관리
- 워크트리 정리
- 자동화/AI 개입 근거

## Non-Adopted Patterns

- Body-only GitLab related issue sections: rejected because GitLab has native linked items.
- GitHub-only related issue APIs for non-hierarchical links: rejected because cross-reference body links are the portable native GitHub mechanism for related issues.
- Mandatory diagrams in every issue/PR: rejected because diagrams help only when they reduce review effort.
- Provider UI templates as source of truth: rejected because automation must work in repos with no templates or thin templates.
