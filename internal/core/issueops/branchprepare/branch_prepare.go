package branchprepare

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
)

type Store struct {
	Read             func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite       func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	ValidateIssueURL func(issueURL string) error
	// UmbrellaForChildIssue는 이 이슈를 자식으로 링크한 우산 사이클을 돌려준다.
	// 자식은 우산 브랜치에서 분기해 우산 브랜치로 합류해야 하므로 base_branch를
	// 그 브랜치와 대조하는 데 쓴다(#129).
	//
	// nil이거나 부모를 찾지 못하면 검증을 건너뛴다. 자식이 아니거나 우산이 이미
	// 정리된 경우이며, 근거를 잃은 검증이 일상 사이클을 막아서는 안 된다.
	UmbrellaForChildIssue func(repo, childIssueURL string) (model.IssueOpsRecord, bool)
}

func Prepare(store Store, stateRoot, id string, req model.IssueOpsBranchPrepareRequest) (model.IssueOpsRecord, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	issueURL := strings.TrimSpace(req.IssueURL)
	if issueURL == "" {
		record, err := store.Read(stateRoot, id)
		if err != nil {
			return record, err
		}
		issueURL = strings.TrimSpace(record.IssueURL)
	}
	if err := store.ValidateIssueURL(issueURL); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if provider == "" {
		provider = remote.ProviderFromURL(issueURL)
	}
	if provider != "github" && provider != "gitlab" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("provider must be github or gitlab")
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("branch is required")
	}
	if err := ValidateBranch(branch); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	baseBranch := strings.TrimSpace(req.BaseBranch)
	if baseBranch == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_branch is required")
	}
	if issueNumber := remote.IssueNumber(issueURL); issueNumber != "" {
		if !strings.HasPrefix(branch, issueNumber+"-") {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("issueops branch for issue %s must start with %s-; for example %s-fix-login", issueNumber, issueNumber, issueNumber)
		}
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("issue must be linked before branch prepare")
	}
	if strings.TrimSpace(record.Branch) == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("issueops record must be started with branch before branch prepare")
	}
	if record.Branch != branch {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("branch does not match IssueOps record branch")
	}
	if record.IssueURL != issueURL {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("issue_url does not match linked IssueOps issue")
	}
	if reason := umbrellaBaseBranchMismatch(store, record, branch, baseBranch); reason != "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("%s", reason)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.BranchPrepare = &model.IssueOpsBranchPrepare{
		Provider:        provider,
		IssueURL:        issueURL,
		Branch:          branch,
		BaseBranch:      baseBranch,
		BaseSHA:         strings.TrimSpace(req.BaseSHA),
		RemoteBranchURL: strings.TrimSpace(req.RemoteBranchURL),
		LinkVerified:    req.LinkVerified,
		Steps:           Steps(provider, issueURL, branch, baseBranch),
		CreatedAt:       now,
	}
	return store.TouchWrite(stateRoot, record)
}

// umbrellaBaseBranchMismatch는 자식 사이클의 base_branch가 우산 브랜치를
// 가리키지 않는 이유를 돌려준다. 빈 문자열이면 통과다.
//
// 자식은 우산 브랜치에서 분기해 우산 브랜치로 합류하고, 우산이 하나의 PR로
// 자기 소스 브랜치에 합류한다. 그래야 자식들이 서로 독립적으로 병렬 진행되고
// 통합 리뷰 경계가 우산 PR 하나로 명확해진다(#129).
func umbrellaBaseBranchMismatch(store Store, record model.IssueOpsRecord, branch, baseBranch string) string {
	if store.UmbrellaForChildIssue == nil {
		return ""
	}
	umbrella, ok := store.UmbrellaForChildIssue(record.Repo, record.IssueURL)
	if !ok {
		return ""
	}
	expected := strings.TrimSpace(umbrella.Branch)
	// 우산이 자기 브랜치를 아직 갖지 않았다면 대조할 기준이 없다. 그 상태에서
	// 자식이 만들어졌다는 것은 create-child 게이트 이전의 레코드라는 뜻이므로
	// 소급 차단하지 않는다.
	if expected == "" || expected == strings.TrimSpace(baseBranch) {
		return ""
	}
	return fmt.Sprintf("자식 작업 %s는 우산 사이클 %s의 브랜치 %s에서 분기해 그 브랜치로 합류해야 한다; "+
		"base_branch %q 대신 %s로 다시 준비하라",
		strings.TrimSpace(branch), umbrella.ID, expected, strings.TrimSpace(baseBranch), expected)
}

func ValidateBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	if strings.ContainsAny(branch, " \t\r\n") || strings.HasPrefix(branch, "/") || strings.Contains(branch, "..") {
		return fmt.Errorf("issueops branch contains invalid characters: %s", branch)
	}
	issueNumber, slug, ok := strings.Cut(branch, "-")
	if !ok || strings.TrimSpace(slug) == "" || !isDecimalString(issueNumber) {
		return fmt.Errorf("issueops branch must start with the issue number followed by a hyphen; use names like 2387-fix-grpc-ai-dmm-tag-replication-lag or 2388-fanza-delete-404-stale-registered")
	}
	return nil
}

func isDecimalString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func Steps(provider, issueURL, branch, baseBranch string) []model.IssueOpsBranchPrepareStep {
	switch provider {
	case "gitlab":
		return []model.IssueOpsBranchPrepareStep{
			{
				Order:    1,
				Strategy: "mcp",
				Tool:     "mcp__glab.glab_api",
				ToolArguments: map[string]any{
					"endpoint": "projects/:fullpath/repository/branches",
					"method":   "POST",
					"field":    []string{"branch=" + branch, "ref=" + baseBranch},
				},
				Description: "Create the issue-prefixed branch through the GitLab MCP authenticated API tool.",
			},
			{
				Order:       2,
				Strategy:    "fallback_api",
				Command:     []string{"glab", "api", "projects/:fullpath/repository/branches", "-X", "POST", "-f", "branch=" + branch, "-f", "ref=" + baseBranch},
				Description: "Fallback to the GitLab API through glab when the MCP tool is unavailable or fails.",
			},
			{
				Order:       3,
				Strategy:    "fail",
				Description: "Stop the IssueOps branch preparation if neither provider-linked creation path succeeds.",
			},
		}
	case "github":
		return []model.IssueOpsBranchPrepareStep{
			{
				Order:       1,
				Strategy:    "mcp_unavailable",
				Description: "No GitHub MCP branch-creation tool is currently exposed in this harness session; do not silently create a local branch.",
			},
			{
				Order:    2,
				Strategy: "fallback_api",
				Command:  []string{"gh", "issue", "develop", issueURL, "--base", baseBranch, "--name", branch},
				// Orca 모드에서는 이 단계를 execution prepare **이후**로 미룬다.
				// `createLinkedBranch`는 `oid`에서 새 브랜치를 만들므로 이름이 원격에
				// 이미 있으면 실패하지만, 로컬에만 있으면 성공한다(#163 실측). Orca는
				// 로컬 워크트리와 로컬 브랜치만 만들고 push하지 않으므로, 먼저
				// 실행하면 Orca가 이름 충돌로 막히고(#149·#152·#154) 나중에 실행하면
				// linked branch 추적을 그대로 얻는다.
				Description: "Create a GitHub linked development branch through gh issue develop. " +
					"For Orca mode run this after `issueops execution prepare` instead: Orca creates the local branch " +
					"without pushing, so the name is still free on the remote and the link still attaches.",
			},
			{
				Order:       3,
				Strategy:    "fail",
				Description: "Stop the IssueOps branch preparation if the linked development branch cannot be created.",
			},
		}
	default:
		return nil
	}
}
