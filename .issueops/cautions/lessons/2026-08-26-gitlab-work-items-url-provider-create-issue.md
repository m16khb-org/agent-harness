---
name: cautions/lessons/2026-08-26-gitlab-work-items-url-provider-create-issue.md
description: Dated lesson — GitLab 18.10+ (observed on 19.2.4-ee) renders plain issues as /-/work_items/:iid; the provider parent-issue parser and the create-issue live gate rejected that alias, blocking close-issue, reflect-completion, create-child, and close-child.
---

# 2026-08-26 — GitLab work_items 이슈 URL 별칭을 provider 파서와 create-issue 라이브 게이트가 거부

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: Claude Code session 2026-08-26 — systematic-debugging + TDD on cleanup preview block report
- Summary: GitLab 18.10+(work items list; 관측 19.2.4-ee)는 일반 이슈의 web_url을 /-/work_items/:iid로 돌려주는데 provider 파서 parseGitLabIssueURL과 create-issue 라이브 게이트 fetchGitLabIssueArtifact가 /-/issues/:iid만 이슈로 인정해 close-issue·reflect-completion·create-child·close-child가 레코드 자신의 URL을 "parent_issue_url must be a GitLab issue URL"로 거부했다.
- Context: self-hosted GitLab 19.2.4-ee 대상 repo의 IssueOps cycle io-22a5f93d7fc2(#105) cleanup: MR 머지와 이슈 자동 종료 뒤 `cleanup finish --preview`가 `missing: [completion_reflected]`로 막혔고, 그 게이트를 푸는 `remote reflect-completion`/`remote close-issue`(dry-run)가 provider 검증에서 `parent_issue_url must be a GitLab issue URL`로 실패했다. 레코드 `issue_url`은 link 시 GitLab `web_url`을 그대로 봉인한 `/-/work_items/105`였고, REST `projects/:path/issues/105`는 `type=ISSUE`, `issue_type=issue`를 돌려줬다(`glab issue view`도 같은 `web_url`을 출력). `kind != "issues"` 제약은 2026-06-30 df6c22e8이 "경로 표식 = Issue/Task 판별자(issues=부모, work_items=자식 Task)"로 가정하며 넣었고 `TestParseGitLabIssueURLRejectsNonIssue`가 그 가정을 고정했다. 하네스의 나머지 파서 5곳(`execution_issue_snapshot.go`, `issue_snapshot.go`, `issuepreparation/intent.go`, `issueopsremote/issueops_remote_project.go`, `issueopscompletion/artifact.go`)은 이미 두 별칭을 같은 identity(authority·project·IID)로 취급하고 있었다. 같은 검사가 `create-issue --confirm`의 라이브 게이트 `fetchGitLabIssueArtifactContext`(116ebefe)에도 있어, 원격 이슈가 이미 만들어진 뒤 `IssueCreateIntentVerificationFailed`로 끝날 수 있었다.
- Resolution: `parseGitLabIssueURL`(internal/adapter/provider/gitlab/provider.go)과 `fetchGitLabIssueArtifactContext`(cmd/issueops/issueopscli/remoteverify/issueops_remote_fetch.go)가 kind 검사를 버리고 host + project + IID만 요구한다. 두 별칭 모두 REST `projects/:path/issues/:iid`로 해석하고 Issue/Task 판정은 payload `type`/`issue_type`으로만 한다(`verifyGitLabIssuePayloadIsTask` 선례). 자식 Task 파서 `parseGitLabWorkItemURL`은 GraphQL `workItemCreate`의 `webUrl`이 항상 `work_items`라 그대로 둔다. 잘못된 가정을 고정한 단언 2개는 MR URL 거부로 교체했고, 별칭 회귀 테스트(`TestParseGitLabIssueURLAcceptsWorkItemsAlias`, `TestGitLabCloseIssueAcceptsWorkItemsIssueURL`, `TestGitLabUpdateIssueBodySectionAcceptsWorkItemsIssueURL`, `TestGitLabCreateChildPreviewAcceptsWorkItemsParentURL`, `TestFetchGitLabIssueArtifactAcceptsWorkItemsAlias`)가 RED→GREEN으로 고정한다. 재빌드 뒤 실제 레코드의 `remote close-issue`/`remote reflect-completion` dry-run이 `ok=true`로 `projects/<path>/issues/105` endpoint를 가리켰다.
- Evidence:
  - docs.gitlab.com/user/work_items/ — `work_item_planning_view` 18.7 도입·18.10 GA(플래그 제거); "URLs that contain /epics/:iid or /issues/:iid automatically redirect to /work_items/:iid"
  - glab api projects/<path>/issues/105 --hostname <host> → iid=105 type=ISSUE issue_type=issue web_url=…/-/work_items/105 state=closed; glab api version → 19.2.4-ee; glab issue view 105 --output json → 같은 web_url
  - issueops status --id io-22a5f93d7fc2 --json → issue_url=…/-/work_items/105, issue_create_intent=null, branch_prepare.link_verified=true, remote_artifact.kind=mr
  - git log -S'kind != "issues"' -- internal/adapter/provider/gitlab/provider.go → df6c22e8 (2026-06-30); git log -S'parts.Kind != "issues"' -- cmd/issueops/issueopscli/remoteverify/ → 116ebefe
  - internal/adapter/provider/gitlab/provider.go parseGitLabIssueURL (callers: CreateChild, CloseChild, UpdateIssueBodySection, CloseIssue); cmd/issueops/issueopscli/remoteverify/issueops_remote_fetch.go fetchGitLabIssueArtifactContext (callers: create-issue --confirm verifyLive, create-issue reconcile adopt)
  - go test ./internal/adapter/provider/gitlab ./cmd/issueops/issueopscli/remoteverify -count=1 → 신규 6개 RED(프로덕션 에러 문자열 그대로) → 수정 후 두 패키지 GREEN
  - go build -o bin/issueops ./cmd/issueops; issueops remote close-issue --id io-22a5f93d7fc2 --provider gitlab --json (dry-run) → ok=true preview=glab api projects/<path>/issues/105 …; remote reflect-completion (dry-run) → ok=true 같은 endpoint
- Rule: GitLab 이슈 URL의 경로 표식으로 객체 타입을 판별하지 말고, URL 형태로 원격 객체 타입을 고정하는 테스트는 provider가 실제로 돌려주는 `web_url`을 재현해야 한다. Evergreen 규칙: [issueops-lifecycle.md §31](../issueops-lifecycle.md).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
