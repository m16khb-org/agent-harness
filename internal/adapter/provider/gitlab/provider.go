package gitlab

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"agent-harness/cmd/harness/issueopscli/remoteparse"
	"agent-harness/internal/adapter/provider/issuebody"
	"agent-harness/internal/port"
)

// Provider adapts GitLab via the `glab` CLI.
type Provider struct{}

func NewProvider() Provider { return Provider{} }

func (Provider) Name() string { return "gitlab" }

func (Provider) CreateIssue(req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("issue title is required")
	}
	args := []string{"issue", "create", "--title", title}
	body := strings.TrimSpace(req.Body)
	if body != "" {
		args = append(args, "--description", body)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}
	for _, assignee := range req.Assignees {
		args = append(args, "--assignee", assignee)
	}
	cmdStr := "glab " + strings.Join(args, " ")
	if !req.Confirm {
		return port.IssueProviderCreateIssueResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: %s", cmdStr),
		}, nil
	}
	return runGlabJSON(args, req.Repo, "issue")
}

func (Provider) CreatePullRequest(req port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("MR title is required")
	}
	head := strings.TrimSpace(req.HeadBranch)
	base := strings.TrimSpace(req.BaseBranch)
	if head == "" || base == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("source and target branches are required")
	}
	args := []string{"mr", "create", "--title", title, "--source-branch", head, "--target-branch", base}
	body := strings.TrimSpace(req.Body)
	if body != "" {
		args = append(args, "--description", body)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}
	for _, assignee := range req.Assignees {
		args = append(args, "--assignee", assignee)
	}
	cmdStr := "glab " + strings.Join(args, " ")
	if !req.Confirm {
		return port.IssueProviderCreatePullRequestResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: %s", cmdStr),
		}, nil
	}
	return runGlabMRJSON(args, req.Repo)
}

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
		preview := fmt.Sprintf("[dry-run] would execute: glab api graphql --hostname %s resolve Task workItemTypeId for %s; resolve labelIds and assigneeIds; workItemCreate Task for parentIid=%s; workItemHierarchyAddChildrenItems; verify children, labels, assignees: labels=%s assignees=%s",
			hostname, projectPath, parentIID, strings.Join(req.Labels, ","), strings.Join(req.Assignees, ","))
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
	childURL = firstNonEmpty(verifiedChild.WebURL, childURL)
	labels := gitlabLabelTitles(verifiedChild.Labels.Nodes)
	assignees := gitlabAssigneeUsernames(verifiedChild.Assignees.Nodes)
	if missing := missingStrings(req.Labels, labels); len(missing) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab child work item missing labels: %s", strings.Join(missing, ", ")))
	}
	if missing := missingStrings(req.Assignees, assignees); len(missing) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "gitlab"}, gitlabCreatedChildError(childURL, fmt.Errorf("gitlab child work item missing assignees: %s", strings.Join(missing, ", ")))
	}
	return port.IssueProviderCreateChildResult{
		OK:                true,
		Provider:          "gitlab",
		ChildURL:          childURL,
		ChildNumber:       firstNonEmpty(verifiedChild.IID, child.IID),
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
		return port.IssueProviderCloseChildResult{OK: true, Provider: "gitlab", ChildURL: strings.TrimSpace(req.ChildURL), Preview: preview}, nil
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
		ChildURL:          firstNonEmpty(verified.WebURL, child.WebURL, req.ChildURL),
		HierarchyVerified: true,
		Closed:            true,
		AlreadyClosed:     alreadyClosed,
		State:             verified.State,
	}, nil
}

type gitlabWorkItem struct {
	ID        string              `json:"id"`
	IID       string              `json:"iid"`
	WebURL    string              `json:"webUrl"`
	State     string              `json:"state"`
	Labels    gitlabLabelNodes    `json:"labels"`
	Assignees gitlabAssigneeNodes `json:"assignees"`
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

type gitlabWorkItemWidget struct {
	Type     string `json:"type"`
	Children struct {
		Nodes []gitlabWorkItem `json:"nodes"`
	} `json:"children"`
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
const gitlabChildVerifyQuery = `query childVerify($childId: WorkItemID!) { workItem(id: $childId) { iid webUrl labels { nodes { title } } assignees { nodes { username } } } }`
const gitlabWorkItemCloseMutation = `mutation workItemUpdate($childId: WorkItemID!) { workItemUpdate(input: { id: $childId, stateEvent: CLOSE }) { workItem { id iid webUrl state } errors } }`
const gitlabChildCloseVerifyQuery = `query childCloseVerify($childId: WorkItemID!) { workItem(id: $childId) { id iid webUrl state } }`

func runGlabJSON(args []string, repo string, kind string) (port.IssueProviderCreateIssueResult, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return port.IssueProviderCreateIssueResult{OK: false},
			fmt.Errorf("glab CLI is not installed; install it from https://gitlab.com/gitlab-org/cli")
	}
	cmd := exec.Command("glab", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return port.IssueProviderCreateIssueResult{OK: false},
			fmt.Errorf("glab %s create failed: %s", kind, stderr)
	}
	url, number := parseGlabOutput(string(out))
	return port.IssueProviderCreateIssueResult{OK: true, URL: url, Number: number}, nil
}

func runGlabMRJSON(args []string, repo string) (port.IssueProviderCreatePullRequestResult, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false},
			fmt.Errorf("glab CLI is not installed")
	}
	cmd := exec.Command("glab", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return port.IssueProviderCreatePullRequestResult{OK: false},
			fmt.Errorf("glab mr create failed: %s", stderr)
	}
	url, number := parseGlabOutput(string(out))
	return port.IssueProviderCreatePullRequestResult{OK: true, URL: url, Number: number}, nil
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
		`mutation workItemCreate { workItemCreate(input: { namespacePath: %s, title: %s, workItemTypeId: %s, descriptionWidget: { description: %s }, labelsWidget: { labelIds: %s }, assigneesWidget: { assigneeIds: %s } }) { workItem { id iid webUrl labels { nodes { title } } assignees { nodes { username } } } errors } }`,
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
	cmd := exec.Command("glab", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return zero, fmt.Errorf("glab graphql failed: %s", stderr)
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

func parseGitLabIssueURL(raw string) (hostname, projectPath, iid string, err error) {
	host, project, issueIID, kind, perr := splitGitLabIssueURL(raw)
	if perr != nil || kind != "issues" {
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

func missingStrings(want, got []string) []string {
	gotSet := make(map[string]bool, len(got))
	for _, value := range got {
		gotSet[strings.TrimSpace(value)] = true
	}
	var missing []string
	for _, value := range want {
		value = strings.TrimSpace(value)
		if value != "" && !gotSet[value] {
			missing = append(missing, value)
		}
	}
	return missing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (Provider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	hostname, projectPath, iid, err := parseGitLabIssueURL(req.IssueURL)
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	section := issuebody.RenderDevilsAdvocateSection(req.Findings, time.Now().UTC().Format(time.RFC3339))
	endpoint := "projects/" + url.PathEscape(projectPath) + "/issues/" + iid
	if !req.Confirm {
		return port.IssueProviderUpdateIssueBodySectionResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: glab api %s --hostname %s; then --method PUT -f description=<merged devil's-advocate section>", endpoint, hostname),
		}, nil
	}
	current, err := runGlabAPI(req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	var payload struct {
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(current, &payload); err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, fmt.Errorf("parse glab issue description: %w", err)
	}
	merged := issuebody.MergeIssueBodySection(payload.Description, section)
	if _, err := runGlabAPI(req.Repo, hostname, endpoint, "--method", "PUT", "-f", "description="+merged); err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	return port.IssueProviderUpdateIssueBodySectionResult{OK: true, URL: firstNonEmpty(payload.WebURL, req.IssueURL), Updated: true}, nil
}

// runGlabAPI runs a REST `glab api <endpoint> --hostname <host> [extra...]` call,
// mirroring the hostname/order shape the verify layer uses for issue reads.
func runGlabAPI(repo, hostname, endpoint string, extra ...string) ([]byte, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return nil, fmt.Errorf("glab CLI is not installed; install it from https://gitlab.com/gitlab-org/cli")
	}
	cmdArgs := []string{"api", endpoint}
	if strings.TrimSpace(hostname) != "" {
		cmdArgs = append(cmdArgs, "--hostname", strings.TrimSpace(hostname))
	}
	cmdArgs = append(cmdArgs, extra...)
	cmd := exec.Command("glab", cmdArgs...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("glab api failed: %s", stderr)
	}
	return out, nil
}
