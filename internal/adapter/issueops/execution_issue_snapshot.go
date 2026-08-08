package issueops

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

const executionIssueSnapshotBodyLimit = 1 << 19

type gitLabExecutionIssueIdentity struct {
	authority string
	project   string
	iid       string
}

func executionIssueSnapshotReaderForAction(
	stateRoot string,
	req ExecutionActionRequest,
	fallback ExecutionIssueSnapshotReadFunc,
) (ExecutionIssueSnapshotReadFunc, func() string, error) {
	if req.IssueSnapshot == nil {
		return executionGitLabFallbackSnapshotReader(fallback)
	}
	if err := validateExecutionIssueSnapshotAction(req); err != nil {
		return nil, nil, err
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateExecutionIssueSnapshotRecord(req, record); err != nil {
		return nil, nil, err
	}
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.Provider) != "gitlab" {
		return nil, nil, fmt.Errorf("issue_snapshot provider does not match the linked IssueOps provider")
	}
	if !sameGitLabExecutionIssueIdentity(record.IssueURL, record.BranchPrepare.IssueURL) {
		return nil, nil, fmt.Errorf("linked IssueOps GitLab identity is inconsistent")
	}
	evidence := *req.IssueSnapshot
	if evidence.Provider != "gitlab" {
		return nil, nil, fmt.Errorf("issue_snapshot provider must be gitlab")
	}
	if evidence.Source != "glab_mcp" {
		return nil, nil, fmt.Errorf("issue_snapshot source must be glab_mcp")
	}
	snapshot := port.ExecutionIssueSnapshot{
		URL: evidence.WebURL, Body: evidence.Body, State: evidence.State, Source: evidence.Source,
	}
	if err := validateGitLabExecutionIssueSnapshot(record.IssueURL, snapshot); err != nil {
		return nil, nil, fmt.Errorf("invalid issue_snapshot: %w", err)
	}
	linkedURL := strings.TrimSpace(record.IssueURL)
	reader := func(ctx context.Context, provider string, snapshotReq port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		if err := ctx.Err(); err != nil {
			return port.ExecutionIssueSnapshot{}, err
		}
		if provider != "gitlab" || !samePath(snapshotReq.Repo, record.Repo) || strings.TrimSpace(snapshotReq.URL) != linkedURL {
			return port.ExecutionIssueSnapshot{}, fmt.Errorf("issue_snapshot request does not match the linked IssueOps identity")
		}
		snapshot.URL = linkedURL
		return snapshot, nil
	}
	return reader, func() string { return "glab_mcp" }, nil
}

func executionGitLabFallbackSnapshotReader(
	fallback ExecutionIssueSnapshotReadFunc,
) (ExecutionIssueSnapshotReadFunc, func() string, error) {
	source := ""
	reader := func(ctx context.Context, provider string, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		if fallback == nil {
			err := fmt.Errorf("remote issue snapshot reader is unavailable")
			if provider == "gitlab" {
				return port.ExecutionIssueSnapshot{}, fmt.Errorf("gitlab_issue_snapshot_unavailable: %w", err)
			}
			return port.ExecutionIssueSnapshot{}, err
		}
		snapshot, err := fallback(ctx, provider, req)
		if err != nil {
			if provider == "gitlab" {
				return port.ExecutionIssueSnapshot{}, fmt.Errorf("gitlab_issue_snapshot_unavailable: %w", err)
			}
			return port.ExecutionIssueSnapshot{}, err
		}
		if provider != "gitlab" {
			return snapshot, nil
		}
		if err := validateGitLabExecutionIssueSnapshot(req.URL, snapshot); err != nil {
			return port.ExecutionIssueSnapshot{}, fmt.Errorf("gitlab_issue_snapshot_unavailable: %w", err)
		}
		snapshot.URL = strings.TrimSpace(req.URL)
		snapshot.Source = "glab_cli"
		source = snapshot.Source
		return snapshot, nil
	}
	return reader, func() string { return source }, nil
}

func validateExecutionIssueSnapshotAction(req ExecutionActionRequest) error {
	switch req.Action {
	case ExecutionActionPrepare, ExecutionActionClaim:
		return nil
	case ExecutionActionReconcile:
		if req.Confirm && !req.Preview {
			return nil
		}
	case ExecutionActionReplace:
		if req.ReplaceAction == ExecutionReplaceFinalize || req.ReplaceAction == ExecutionReplaceReseed {
			return nil
		}
	}
	return fmt.Errorf("issue_snapshot is not supported for execution action %q", req.Action)
}

func validateExecutionIssueSnapshotRecord(req ExecutionActionRequest, record issueops.IssueOpsRecord) error {
	if req.Action != ExecutionActionReconcile {
		return nil
	}
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca ||
		record.Execution.Pending == nil || record.Execution.Pending.Kind != "worktree_create" {
		return fmt.Errorf("issue_snapshot is supported for execution reconcile only when confirm resumes a pending worktree_create intent")
	}
	return nil
}

func validateGitLabExecutionIssueSnapshot(linkedURL string, snapshot port.ExecutionIssueSnapshot) error {
	if !sameGitLabExecutionIssueIdentity(linkedURL, snapshot.URL) {
		return fmt.Errorf("GitLab issue snapshot URL does not match the linked issue")
	}
	if strings.TrimSpace(snapshot.Body) == "" || len([]byte(snapshot.Body)) > executionIssueSnapshotBodyLimit {
		return fmt.Errorf("GitLab issue snapshot body must be non-empty and at most %d bytes", executionIssueSnapshotBodyLimit)
	}
	if snapshot.State != "opened" && snapshot.State != "closed" {
		return fmt.Errorf("GitLab issue snapshot state must be opened or closed")
	}
	return nil
}

func sameGitLabExecutionIssueIdentity(left, right string) bool {
	leftIdentity, leftErr := parseGitLabExecutionIssueIdentity(left)
	rightIdentity, rightErr := parseGitLabExecutionIssueIdentity(right)
	return leftErr == nil && rightErr == nil &&
		leftIdentity.authority == rightIdentity.authority &&
		leftIdentity.project == rightIdentity.project &&
		leftIdentity.iid == rightIdentity.iid
}

func parseGitLabExecutionIssueIdentity(raw string) (gitLabExecutionIssueIdentity, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed != raw || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL must be canonical")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" ||
		parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL must be canonical HTTPS without userinfo, query, or fragment")
	}
	path := parsed.EscapedPath()
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL path is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 5 || parts[len(parts)-3] != "-" ||
		(parts[len(parts)-2] != "issues" && parts[len(parts)-2] != "work_items") {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL must end with /-/issues/:iid or /-/work_items/:iid")
	}
	projectParts := parts[:len(parts)-3]
	for _, part := range projectParts {
		decoded, decodeErr := url.PathUnescape(part)
		if decodeErr != nil || part == "" || decoded == "." || decoded == ".." ||
			strings.ContainsAny(decoded, `/\`) || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
			return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue project path is invalid")
		}
	}
	iid := parts[len(parts)-1]
	number, err := strconv.Atoi(iid)
	if err != nil || number <= 0 || strconv.Itoa(number) != iid {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue IID must be a canonical positive integer")
	}
	return gitLabExecutionIssueIdentity{
		authority: strings.ToLower(parsed.Host),
		project:   strings.Join(projectParts, "/"),
		iid:       iid,
	}, nil
}

func withExecutionIssueSnapshotSource(result any, source string) any {
	if source == "" {
		return result
	}
	switch typed := result.(type) {
	case ExecutionPrepareResult:
		typed.IssueSnapshotSource = source
		return typed
	case ExecutionResult:
		typed.IssueSnapshotSource = source
		return typed
	case ExecutionReplaceResult:
		typed.IssueSnapshotSource = source
		return typed
	case ExecutionReconcileResult:
		typed.IssueSnapshotSource = source
		return typed
	default:
		return result
	}
}
