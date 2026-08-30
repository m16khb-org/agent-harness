package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"agent-harness/internal/adapter/provider/providerutil"
	"agent-harness/internal/domain/remoteparse"
	"agent-harness/internal/port"
)

// GitLab descriptions can approach 1 MiB; GraphQL and REST envelopes add
// metadata around them. Keep a finite process bound without rejecting payloads
// that the provider itself accepts.
const gitLabCommandOutputLimit = 2 * 1024 * 1024

func (Provider) CreateChild(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("child title is required")
	}
	hostname, projectPath, parentIID, err := parseGitLabIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, err
	}
	if !req.Confirm {
		args := []string{"api", "graphql", "--hostname", hostname, "--field", "project=" + projectPath, "--field", "parent_iid=" + parentIID, "--field", "title=" + title, "--field", "description=" + strings.TrimSpace(req.Body)}
		for _, label := range req.Labels {
			args = append(args, "--field", "label="+label)
		}
		for _, assignee := range req.Assignees {
			args = append(args, "--field", "assignee="+assignee)
		}
		preview := providerutil.DryRunPreview("glab", args...) + "; resolve Task workItem type; workItemCreate; workItemHierarchyAddChildrenItems; verify children, labels, assignees"
		return port.IssueProviderCreateChildResult{OK: true, Provider: "gitlab", Preview: preview}, nil
	}
	taskTypeID, err := resolveGitLabTaskTypeID(req.Repo, hostname, projectPath)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, err
	}
	labelIDs, err := resolveGitLabLabelIDs(req.Repo, hostname, projectPath, req.Labels)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, err
	}
	assigneeIDs, err := resolveGitLabAssigneeIDs(req.Repo, hostname, req.Assignees)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, err
	}
	createQuery := buildGitLabWorkItemCreateQuery(projectPath, title, strings.TrimSpace(req.Body), taskTypeID, labelIDs, assigneeIDs)
	create, err := runGlabGraphQL[gitlabWorkItemCreateResponse](req.Repo, hostname, createQuery, map[string]string{
		"operation": "workItemCreate",
	})
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, err
	}
	if len(create.Data.WorkItemCreate.Errors) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("gitlab workItemCreate failed: %s", strings.Join(create.Data.WorkItemCreate.Errors, ", "))
	}
	child := create.Data.WorkItemCreate.WorkItem
	if strings.TrimSpace(child.ID) == "" {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("gitlab workItemCreate did not return child work item id")
	}
	childURL := strings.TrimSpace(child.WebURL)
	parent, err := runGlabGraphQL[gitlabParentIssueResponse](req.Repo, hostname, gitlabParentIssueQuery, map[string]string{
		"projectPath": projectPath,
		"parentIid":   parentIID,
	})
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, err)
	}
	parentID := strings.TrimSpace(parent.Data.Project.Issue.ID)
	if parentID == "" {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab parent issue lookup did not return work item id"))
	}
	attach, err := runGlabGraphQL[gitlabHierarchyAddResponse](req.Repo, hostname, gitlabHierarchyAddQuery, map[string]string{
		"parentId": parentID,
		"childId":  child.ID,
	})
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, err)
	}
	if len(attach.Data.WorkItemHierarchyAddChildrenItems.Errors) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab hierarchy attach failed: %s", strings.Join(attach.Data.WorkItemHierarchyAddChildrenItems.Errors, ", ")))
	}
	children, err := runGlabGraphQL[gitlabHierarchyChildrenResponse](req.Repo, hostname, gitlabHierarchyChildrenQuery, map[string]string{
		"parentId": parentID,
	})
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, err)
	}
	if !gitlabChildrenContain(children.Data.WorkItem.Widgets, child.ID, child.IID) {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab child hierarchy verification failed"))
	}
	verify, err := runGlabGraphQL[gitlabChildVerifyResponse](req.Repo, hostname, gitlabChildVerifyQuery, map[string]string{
		"childId":     child.ID,
		"childVerify": "true",
	})
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, err)
	}
	verifiedChild := verify.Data.WorkItem
	childURL = providerutil.FirstNonEmpty(verifiedChild.WebURL, childURL)
	labels := gitlabWorkItemLabelTitles(verifiedChild.Widgets)
	assignees := gitlabWorkItemAssigneeUsernames(verifiedChild.Widgets)
	if missing := providerutil.MissingStrings(req.Labels, labels); len(missing) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab child work item missing labels: %s", strings.Join(missing, ", ")))
	}
	if missing := providerutil.MissingStrings(req.Assignees, assignees); len(missing) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab child work item missing assignees: %s", strings.Join(missing, ", ")))
	}
	return port.IssueProviderCreateChildResult{
		OK:                true,
		Provider:          "gitlab",
		ChildURL:          childURL,
		ChildNumber:       providerutil.FirstNonEmpty(verifiedChild.IID, child.IID),
		HierarchyVerified: true,
		Labels:            labels,
		Assignees:         assignees,
	}, nil
}

func (Provider) CloseChild(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	hostname, projectPath, parentIID, err := parseGitLabIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, err
	}
	childHostname, childProjectPath, childIID, err := parseGitLabWorkItemURL(req.ChildURL)
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, err
	}
	if childHostname != hostname || childProjectPath != projectPath {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("child issue url must match linked parent issue project")
	}
	if !req.Confirm {
		preview := fmt.Sprintf("[dry-run] would execute: glab api graphql --hostname %s parentIid=%s children verify work_items/%s; workItemUpdate stateEvent: CLOSE; childCloseVerify",
			hostname, parentIID, childIID)
		result := port.IssueProviderCloseChildResult{OK: true, Provider: "gitlab", ChildURL: strings.TrimSpace(req.ChildURL), Preview: preview}
		// GitHub 쪽과 같은 계약이다: preview가 자식의 현재 원격 상태를 함께
		// 관측한다. cleanup close-children이 부모 머지 증거 없이 정리해도
		// 되는지 판정하는 근거다(#129). 읽기 전용이고 best-effort이므로 glab
		// 부재나 조회 실패는 상태 미상으로 남기고 성공을 돌려준다.
		if child, ok := observeGitLabChild(req.Repo, hostname, projectPath, parentIID, childIID); ok {
			result.HierarchyVerified = true
			result.State = child.State
			result.AlreadyClosed = strings.EqualFold(child.State, "CLOSED")
			result.ChildURL = providerutil.FirstNonEmpty(child.WebURL, result.ChildURL)
		}
		return result, nil
	}
	parent, err := runGlabGraphQL[gitlabParentIssueResponse](req.Repo, hostname, gitlabParentIssueQuery, map[string]string{
		"projectPath": projectPath,
		"parentIid":   parentIID,
	})
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, err
	}
	parentID := strings.TrimSpace(parent.Data.Project.Issue.ID)
	if parentID == "" {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("gitlab parent issue lookup did not return work item id")
	}
	children, err := runGlabGraphQL[gitlabHierarchyChildrenResponse](req.Repo, hostname, gitlabHierarchyChildrenQuery, map[string]string{
		"parentId": parentID,
	})
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, err
	}
	child := gitlabChildByIID(children.Data.WorkItem.Widgets, childIID)
	if strings.TrimSpace(child.ID) == "" {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("gitlab child hierarchy verification failed")
	}
	alreadyClosed := strings.EqualFold(child.State, "CLOSED")
	if !alreadyClosed {
		closeResp, err := runGlabGraphQL[gitlabWorkItemUpdateResponse](req.Repo, hostname, gitlabWorkItemCloseMutation, map[string]string{
			"childId": child.ID,
		})
		if err != nil {
			return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, err
		}
		if len(closeResp.Data.WorkItemUpdate.Errors) > 0 {
			return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("gitlab workItemUpdate failed: %s", strings.Join(closeResp.Data.WorkItemUpdate.Errors, ", "))
		}
	}
	verify, err := runGlabGraphQL[gitlabChildCloseVerifyResponse](req.Repo, hostname, gitlabChildCloseVerifyQuery, map[string]string{
		"childId":          child.ID,
		"childCloseVerify": "true",
	})
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, err
	}
	verified := verify.Data.WorkItem
	if !strings.EqualFold(verified.State, "CLOSED") {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "gitlab"}, fmt.Errorf("gitlab child work item close verification failed: state=%s", verified.State)
	}
	return port.IssueProviderCloseChildResult{
		OK:                true,
		Provider:          "gitlab",
		ChildURL:          providerutil.FirstNonEmpty(verified.WebURL, child.WebURL, req.ChildURL),
		HierarchyVerified: true,
		Closed:            true,
		AlreadyClosed:     alreadyClosed,
		State:             verified.State,
	}, nil
}

type gitlabWorkItem struct {
	ID      string                 `json:"id"`
	IID     string                 `json:"iid"`
	WebURL  string                 `json:"webUrl"`
	State   string                 `json:"state"`
	Widgets []gitlabWorkItemWidget `json:"widgets"`
}

type gitlabLabelNodes struct {
	Nodes []gitlabLabel `json:"nodes"`
}

type gitlabLabel struct {
	Title string `json:"title"`
}

type gitlabAssigneeNodes struct {
	Nodes []gitlabAssignee `json:"nodes"`
}

type gitlabAssignee struct {
	Username string `json:"username"`
}

type gitlabWorkItemCreateResponse struct {
	Data struct {
		WorkItemCreate struct {
			WorkItem gitlabWorkItem `json:"workItem"`
			Errors   []string       `json:"errors"`
		} `json:"workItemCreate"`
	} `json:"data"`
}

type gitlabTaskTypeResponse struct {
	Data struct {
		Namespace struct {
			WorkItemTypes struct {
				Nodes []gitlabWorkItemType `json:"nodes"`
			} `json:"workItemTypes"`
		} `json:"namespace"`
	} `json:"data"`
}

type gitlabWorkItemType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type gitlabLabelLookupResponse struct {
	Data struct {
		Project struct {
			Labels struct {
				Nodes []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"project"`
	} `json:"data"`
}

type gitlabUserLookupResponse struct {
	Data struct {
		User struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
	} `json:"data"`
}

type gitlabParentIssueResponse struct {
	Data struct {
		Project struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"project"`
	} `json:"data"`
}

type gitlabHierarchyAddResponse struct {
	Data struct {
		WorkItemHierarchyAddChildrenItems struct {
			Errors []string `json:"errors"`
		} `json:"workItemHierarchyAddChildrenItems"`
	} `json:"data"`
}

type gitlabHierarchyChildrenResponse struct {
	Data struct {
		WorkItem struct {
			Widgets []gitlabWorkItemWidget `json:"widgets"`
		} `json:"workItem"`
	} `json:"data"`
}

// gitlabWorkItemWidget carries the widget selections this provider reads.
// GitLab keeps hierarchy, labels, and assignees on separate widget types, so a
// single struct with empty siblings is how one selection set decodes.
type gitlabWorkItemWidget struct {
	Type     string `json:"type"`
	Children struct {
		Nodes []gitlabWorkItem `json:"nodes"`
	} `json:"children"`
	Labels    gitlabLabelNodes    `json:"labels"`
	Assignees gitlabAssigneeNodes `json:"assignees"`
}

type gitlabChildVerifyResponse struct {
	Data struct {
		WorkItem gitlabWorkItem `json:"workItem"`
	} `json:"data"`
}

type gitlabWorkItemUpdateResponse struct {
	Data struct {
		WorkItemUpdate struct {
			WorkItem gitlabWorkItem `json:"workItem"`
			Errors   []string       `json:"errors"`
		} `json:"workItemUpdate"`
	} `json:"data"`
}

type gitlabChildCloseVerifyResponse struct {
	Data struct {
		WorkItem gitlabWorkItem `json:"workItem"`
	} `json:"data"`
}

const gitlabTaskTypeQuery = `query taskType($namespacePath: ID!) { namespace(fullPath: $namespacePath) { workItemTypes(name: TASK, onlyAvailable: true, first: 1) { nodes { id name } } } }`
const gitlabLabelLookupQuery = `query labelLookup($projectPath: ID!, $title: String!) { project(fullPath: $projectPath) { labels(title: $title, searchIn: [TITLE], includeAncestorGroups: true, first: 1) { nodes { id title } } } }`
const gitlabUserLookupQuery = `query userLookup($username: String!) { user(username: $username) { id username } }`
const gitlabParentIssueQuery = `query parentIid($projectPath: ID!, $parentIid: String!) { project(fullPath: $projectPath) { issue(iid: $parentIid) { id } } }`
const gitlabHierarchyAddQuery = `mutation workItemHierarchyAddChildrenItems($parentId: WorkItemID!, $childId: WorkItemID!) { workItemHierarchyAddChildrenItems(input: { id: $parentId, childrenIds: [$childId] }) { workItem { id } errors } }`
const gitlabHierarchyChildrenQuery = `query children($parentId: WorkItemID!) { workItem(id: $parentId) { widgets { type ... on WorkItemWidgetHierarchy { children { nodes { id iid webUrl state } } } } } }`
const gitlabChildVerifyQuery = `query childVerify($childId: WorkItemID!) { workItem(id: $childId) { iid webUrl widgets { type ... on WorkItemWidgetLabels { labels { nodes { title } } } ... on WorkItemWidgetAssignees { assignees { nodes { username } } } } } }`
const gitlabWorkItemCloseMutation = `mutation workItemUpdate($childId: WorkItemID!) { workItemUpdate(input: { id: $childId, stateEvent: CLOSE }) { workItem { id iid webUrl state } errors } }`
const gitlabChildCloseVerifyQuery = `query childCloseVerify($childId: WorkItemID!) { workItem(id: $childId) { id iid webUrl state } }`

func runGlabJSON(args []string, repo string, kind string) (port.IssueProviderCreateIssueResult, error) {
	return runGlabJSONContext(context.Background(), args, repo, kind)
}

func runGlabJSONContext(ctx context.Context, args []string, repo string, kind string) (port.IssueProviderCreateIssueResult, error) {
	out, invoked, err := providerutil.RunBoundedMutationContext(ctx, repo, "glab", args...)
	url, number := parseGlabOutput(string(out))
	result := port.IssueProviderCreateIssueResult{OK: err == nil, URL: url, Number: number}
	if err == nil {
		if strings.TrimSpace(url) == "" {
			return port.IssueProviderCreateIssueResult{OK: false}, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("glab %s creation returned no canonical URL; do not retry", kind)}
		}
		return result, nil
	}
	if !invoked {
		cause := err
		if strings.Contains(err.Error(), "executable file not found") {
			cause = fmt.Errorf("glab CLI is not installed; install it from https://gitlab.com/gitlab-org/cli")
		}
		return port.IssueProviderCreateIssueResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: cause}
	}
	diagnostic := strings.TrimPrefix(providerutil.BoundedDiagnostic(err.Error(), 384), "command failed after start: ")
	if strings.TrimSpace(url) != "" {
		return result, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("glab %s create failed: %s; outcome unknown with a canonical URL returned separately; do not retry", kind, diagnostic)}
	}
	return port.IssueProviderCreateIssueResult{OK: false}, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("glab %s create failed: %s; outcome unknown; do not retry", kind, diagnostic)}
}

func runGlabMRJSON(ctx context.Context, args []string, repo string) (port.IssueProviderCreatePullRequestResult, error) {
	out, invoked, err := providerutil.RunBoundedMutationContext(ctx, repo, "glab", args...)
	url, number := parseGlabOutput(string(out))
	result := port.IssueProviderCreatePullRequestResult{OK: err == nil, URL: url, Number: number}
	if err == nil {
		return result, nil
	}
	if !invoked {
		return port.IssueProviderCreatePullRequestResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: err}
	}
	if validCanonicalGitLabMergeRequestURL(url) {
		return result, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("GitLab MR creation outcome unknown with a canonical URL returned separately; do not retry: %s", providerutil.BoundedDiagnostic(err.Error(), 384))}
	}
	return port.IssueProviderCreatePullRequestResult{OK: false}, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("GitLab MR creation outcome unknown; do not retry: %s", providerutil.BoundedDiagnostic(err.Error(), 384))}
}

// parseGlabOutput extracts the created artifact's web URL by scanning glab
// output for the first https line. No create call passes a JSON flag, so glab
// issue/mr create always emits a bare URL and the IID never appears in the
// output; number is therefore always empty and kept only to mirror the port
// result's (url, number) shape.
func parseGlabOutput(out string) (url string, number string) {
	out = strings.TrimSpace(out)
	if out == "" {
		return "", ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			return line, ""
		}
	}
	return "", ""
}

func resolveGitLabTaskTypeID(repo, hostname, projectPath string) (string, error) {
	result, err := runGlabGraphQL[gitlabTaskTypeResponse](repo, hostname, gitlabTaskTypeQuery, map[string]string{
		"namespacePath": projectPath,
	})
	if err != nil {
		return "", err
	}
	for _, node := range result.Data.Namespace.WorkItemTypes.Nodes {
		if strings.EqualFold(node.Name, "Task") && strings.TrimSpace(node.ID) != "" {
			return node.ID, nil
		}
	}
	return "", fmt.Errorf("gitlab Task work item type is not available for namespace %s", projectPath)
}

func resolveGitLabLabelIDs(repo, hostname, projectPath string, labels []string) ([]string, error) {
	ids := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		result, err := runGlabGraphQL[gitlabLabelLookupResponse](repo, hostname, gitlabLabelLookupQuery, map[string]string{
			"projectPath": projectPath,
			"title":       label,
		})
		if err != nil {
			return nil, err
		}
		var id string
		for _, node := range result.Data.Project.Labels.Nodes {
			if node.Title == label && strings.TrimSpace(node.ID) != "" {
				id = node.ID
				break
			}
		}
		if id == "" {
			return nil, fmt.Errorf("gitlab label %q was not found", label)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func resolveGitLabAssigneeIDs(repo, hostname string, assignees []string) ([]string, error) {
	ids := make([]string, 0, len(assignees))
	for _, assignee := range assignees {
		assignee = strings.TrimPrefix(strings.TrimSpace(assignee), "@")
		if assignee == "" {
			continue
		}
		result, err := runGlabGraphQL[gitlabUserLookupResponse](repo, hostname, gitlabUserLookupQuery, map[string]string{
			"username": assignee,
		})
		if err != nil {
			return nil, err
		}
		if result.Data.User.Username != assignee || strings.TrimSpace(result.Data.User.ID) == "" {
			return nil, fmt.Errorf("gitlab user %q was not found", assignee)
		}
		ids = append(ids, result.Data.User.ID)
	}
	return ids, nil
}

func buildGitLabWorkItemCreateQuery(projectPath, title, description, taskTypeID string, labelIDs, assigneeIDs []string) string {
	return fmt.Sprintf(
		`mutation workItemCreate { workItemCreate(input: { namespacePath: %s, title: %s, workItemTypeId: %s, descriptionWidget: { description: %s }, labelsWidget: { labelIds: %s }, assigneesWidget: { assigneeIds: %s } }) { workItem { id iid webUrl widgets { type ... on WorkItemWidgetLabels { labels { nodes { title } } } ... on WorkItemWidgetAssignees { assignees { nodes { username } } } } } errors } }`,
		graphqlString(projectPath),
		graphqlString(title),
		graphqlString(taskTypeID),
		graphqlString(description),
		graphqlStringList(labelIDs),
		graphqlStringList(assigneeIDs),
	)
}

func graphqlString(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func graphqlStringList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			quoted = append(quoted, graphqlString(value))
		}
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func runGlabGraphQL[T any](repo, hostname, query string, fields map[string]string) (T, error) {
	return runGlabGraphQLContext[T](context.Background(), repo, hostname, query, fields)
}

func runGlabGraphQLContext[T any](ctx context.Context, repo, hostname, query string, fields map[string]string) (T, error) {
	var zero T
	if _, err := exec.LookPath("glab"); err != nil {
		return zero, fmt.Errorf("glab CLI is not installed; install it from https://gitlab.com/gitlab-org/cli")
	}
	args := []string{"api", "graphql"}
	if strings.TrimSpace(hostname) != "" {
		args = append(args, "--hostname", strings.TrimSpace(hostname))
	}
	args = append(args, "-f", "query="+query)
	for key, value := range fields {
		args = append(args, "-f", key+"="+value)
	}
	out, _, err := providerutil.RunBoundedMutationWithOutputLimitContext(ctx, repo, "glab", gitLabCommandOutputLimit, args...)
	if err != nil {
		return zero, fmt.Errorf("glab graphql failed: %w", err)
	}
	var envelope struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(out, &envelope); err == nil && len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, graphqlErr := range envelope.Errors {
			messages = append(messages, strings.TrimSpace(graphqlErr.Message))
		}
		return zero, fmt.Errorf("glab graphql failed: %s", strings.Join(messages, "; "))
	}
	if err := json.Unmarshal(out, &zero); err != nil {
		return zero, fmt.Errorf("parse glab graphql response: %w", err)
	}
	return zero, nil
}

func gitlabCreatedChildError(childURL string, err error) error {
	childURL = strings.TrimSpace(childURL)
	if childURL == "" {
		return err
	}
	return fmt.Errorf("created child %s but follow-up failed: %w", childURL, err)
}

// parseGitLabIssueURL decodes the parent/primary issue URL. GitLab 18.10+ (work
// items list, GA; observed on 19.2.4-ee) redirects /-/issues/:iid to
// /-/work_items/:iid and returns that web_url for plain issues, and the record
// seals that value, so the path marker cannot discriminate Issue from Task:
// identity is host + project + IID and both aliases resolve on the
// projects/:path/issues/:iid REST endpoint (2026-08-26 lesson).
func parseGitLabIssueURL(raw string) (hostname, projectPath, iid string, err error) {
	host, project, issueIID, _, perr := splitGitLabIssueURL(raw)
	if perr != nil {
		return "", "", "", fmt.Errorf("parent_issue_url must be a GitLab issue URL")
	}
	return host, project, issueIID, nil
}

func parseGitLabWorkItemURL(raw string) (hostname, projectPath, iid string, err error) {
	host, project, itemIID, kind, perr := splitGitLabIssueURL(raw)
	if perr != nil || kind != "work_items" {
		return "", "", "", fmt.Errorf("child_url must be a GitLab work item URL")
	}
	return host, project, itemIID, nil
}

// splitGitLabIssueURL structurally decodes a GitLab issue or work item URL using
// net/url plus the shared remoteparse path splitter (keyed on the /-/issues/ and
// /-/work_items/ markers). This accepts self-hosted instances on custom domains
// that do not contain the literal substring "gitlab", matching the structural
// detection already used by the verify layer (remoteverify.VerifyGitLabIssueLive).
func splitGitLabIssueURL(raw string) (hostname, projectPath, iid, kind string, err error) {
	trimmed := strings.TrimSpace(raw)
	parsed, perr := url.Parse(trimmed)
	if perr != nil {
		return "", "", "", "", perr
	}
	// A scheme-less URL (e.g. "gitlab.example.com/group/proj/-/issues/1") parses
	// with an empty Host and the authority folded into Path; re-parse with an
	// https scheme so the host is recovered, matching what the old substring
	// scanner tolerated.
	if parsed.Hostname() == "" && !strings.Contains(trimmed, "://") {
		if reparsed, rerr := url.Parse("https://" + trimmed); rerr == nil {
			parsed = reparsed
		}
	}
	hostname = parsed.Hostname()
	parts := remoteparse.SplitGitLabIssuePath(parsed.EscapedPath())
	if hostname == "" || parts.Project == "" || parts.IID == "" {
		return "", "", "", "", fmt.Errorf("not a GitLab issue or work item URL")
	}
	return hostname, parts.Project, parts.IID, parts.Kind, nil
}

func gitlabChildrenContain(widgets []gitlabWorkItemWidget, id, iid string) bool {
	for _, widget := range widgets {
		if widget.Type != "" && !strings.EqualFold(widget.Type, "HIERARCHY") {
			continue
		}
		for _, child := range widget.Children.Nodes {
			if id != "" && child.ID == id {
				return true
			}
			if iid != "" && child.IID == iid {
				return true
			}
		}
	}
	return false
}

// observeGitLabChild는 부모 work item 계층에서 자식의 현재 상태를 읽는다.
// 확인된 계층 소속일 때만 true다. 조회 실패는 오류가 아니라 미관측이다 —
// preview에서만 쓰이며 호출자가 미상을 통과 근거로 인정하지 않는다.
func observeGitLabChild(repo, hostname, projectPath, parentIID, childIID string) (gitlabWorkItem, bool) {
	parent, err := runGlabGraphQL[gitlabParentIssueResponse](repo, hostname, gitlabParentIssueQuery, map[string]string{
		"projectPath": projectPath,
		"parentIid":   parentIID,
	})
	if err != nil {
		return gitlabWorkItem{}, false
	}
	parentID := strings.TrimSpace(parent.Data.Project.Issue.ID)
	if parentID == "" {
		return gitlabWorkItem{}, false
	}
	children, err := runGlabGraphQL[gitlabHierarchyChildrenResponse](repo, hostname, gitlabHierarchyChildrenQuery, map[string]string{
		"parentId": parentID,
	})
	if err != nil {
		return gitlabWorkItem{}, false
	}
	child := gitlabChildByIID(children.Data.WorkItem.Widgets, childIID)
	if strings.TrimSpace(child.ID) == "" {
		return gitlabWorkItem{}, false
	}
	return child, true
}

func gitlabChildByIID(widgets []gitlabWorkItemWidget, iid string) gitlabWorkItem {
	for _, widget := range widgets {
		if widget.Type != "" && !strings.EqualFold(widget.Type, "HIERARCHY") {
			continue
		}
		for _, child := range widget.Children.Nodes {
			if iid != "" && child.IID == iid {
				return child
			}
		}
	}
	return gitlabWorkItem{}
}

func gitlabWorkItemLabelTitles(widgets []gitlabWorkItemWidget) []string {
	out := make([]string, 0)
	for _, widget := range widgets {
		out = append(out, gitlabLabelTitles(widget.Labels.Nodes)...)
	}
	return out
}

func gitlabWorkItemAssigneeUsernames(widgets []gitlabWorkItemWidget) []string {
	out := make([]string, 0)
	for _, widget := range widgets {
		out = append(out, gitlabAssigneeUsernames(widget.Assignees.Nodes)...)
	}
	return out
}

func gitlabLabelTitles(labels []gitlabLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.TrimSpace(label.Title) != "" {
			out = append(out, label.Title)
		}
	}
	return out
}

func gitlabAssigneeUsernames(assignees []gitlabAssignee) []string {
	out := make([]string, 0, len(assignees))
	for _, assignee := range assignees {
		if strings.TrimSpace(assignee.Username) != "" {
			out = append(out, assignee.Username)
		}
	}
	return out
}

// gitLabIssueBodyLimit keeps merged descriptions under GitLab's ~1MiB
// description ceiling. The section budget is computed against the merged body.
const gitLabIssueBodyLimit = 900000
