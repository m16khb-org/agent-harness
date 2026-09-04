---
name: 2026-08-28-gitlab-workitem-labels-assignees-selection-create-child-glab
description: Caution record for a solved false case or recurring risk.
---

# GitLab WorkItem 직속 labels/assignees selection이 create-child를 막았고 가짜 glab 스텁이 그 오류를 감췄다

- Date: 2026-08-28
- Kind: `caution`
- Source: Claude Code session 2026-08-28 — systematic-debugging + TDD on issueops remote create-child failure
- Summary: GitLab WorkItem은 labels/assignees를 widgets(WorkItemWidgetLabels/WorkItemWidgetAssignees)로만 노출하는데 provider의 workItemCreate 반환 selection과 childVerify 쿼리가 둘을 직속 필드로 select해 `issueops remote create-child`가 GraphQL validation에서 거절됐고, 가짜 glab 스텁이 잘못된 응답 모양을 그대로 되돌려줘 테스트는 계속 초록불이었다.
- Context: 사용자가 사내 GitLab 19.2.4-ee에서 `issueops remote create-child`를 실행하자 `glab graphql failed: Field 'labels' doesn't exist on type 'WorkItem'` / `Field 'assignees' doesn't exist on type 'WorkItem'`로 실패했고, 스킬이 허용하는 escape hatch로 GraphQL workItemCreate + workItemUpdate를 직접 호출해 자식을 만든 뒤 link-child로 등록했다. 라이브 인트로스펙션 결과 WorkItem 필드 목록에는 labels/assignees가 없고 WorkItemWidgetLabels.labels(LabelConnection) / WorkItemWidgetAssignees.assignees(UserCoreConnection)로만 존재한다. 이는 버전 특성이 아니라 work item의 표준 모델이라 해당 쿼리는 어떤 GitLab에서도 통과하지 못한다. 깨진 곳은 두 군데였고 입력 쪽(labelsWidget.labelIds, assigneesWidget.assigneeIds)은 정상이며 반환 selection만 잘못됐다. GraphQL은 실행 전에 문서 전체를 검증하므로 실제로 먼저 터진 것은 검증 쿼리가 아니라 workItemCreate mutation이고, 그래서 원격에 반쪽짜리 work item은 남지 않았다. 이 결함이 살아남은 이유는 provider_test.go의 가짜 glab 스텁이 쿼리 문자열을 case 매칭만 하고 스키마를 검증하지 않은 채 `"labels":{"nodes":[...]},"assignees":{"nodes":[...]}`를 workItem 직속으로 되돌려줬기 때문이다.
- Resolution: internal/adapter/provider/gitlab/provider.go의 gitlabChildVerifyQuery와 buildGitLabWorkItemCreateQuery 반환 selection을 `widgets { type ... on WorkItemWidgetLabels { labels { nodes { title } } } ... on WorkItemWidgetAssignees { assignees { nodes { username } } } }`로 교체하고, gitlabWorkItem에서 Labels/Assignees 직속 필드를 제거해 Widgets로 옮긴 뒤 gitlabWorkItemLabelTitles/gitlabWorkItemAssigneeUsernames가 위젯을 순회해 추출한다. hierarchy children 노드 파싱(id/iid/webUrl/state)과 close 계열 쿼리는 이미 유효해 그대로 뒀다. 재발 방지로 writeFakeGlab이 모든 가짜 glab 스크립트에 스키마 가드를 주입한다 — WorkItem 선택에 직속 labels/assignees가 오면 실제 GitLab과 같은 메시지로 exit 2 하므로, 앞으로 이 파일에 추가되는 work item 쿼리에도 자동 적용된다. 스텁 응답도 실제 관측 모양(무관 위젯이 다수 섞이고 labels/assignees가 비어 있는 위젯들)으로 바꿨다.
- Evidence:
  - glab api graphql --hostname gitlab.example.com 인트로스펙션 → WorkItem 필드에 labels/assignees 없음; WorkItemWidgetLabels: allowsScopedLabels, labels, type / WorkItemWidgetAssignees: allowsMultipleAssignees, assignees, canInviteMembers, type; labels→LabelConnection→Label.title, assignees→UserCoreConnection→UserCore.username
  - glab api version --hostname gitlab.example.com → 19.2.4-ee
  - 수정된 childVerify 쿼리를 실제 서버에 실행 → validation 오류 없이 통과(gid://gitlab/WorkItem/999999999는 권한/부재 오류, 실제 work item은 widgets 배열로 ASSIGNEES/LABELS 반환)
  - RED: writeFakeGlab 스키마 가드 + 위젯 모양 스텁 적용 후 go test ./internal/adapter/provider/gitlab/... → TestGitLabCreateChildConfirmCreatesAttachesAndVerifies와 TestGitLabCreateChildFailureAfterCreateIncludesChildURL이 사용자가 겪은 것과 동일한 `Field 'labels' doesn't exist on type 'WorkItem'`로 실패
  - GREEN: 위젯 기반 수정 후 go test ./internal/adapter/provider/gitlab/... 통과
  - gofmt -l internal cmd 무출력, go vet ./... 깨끗, go build ./... OK, go test ./... -count=1 실패 0건
