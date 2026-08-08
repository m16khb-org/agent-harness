package branchprepare

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/issueopsremote"
)

type Store struct {
	Read             func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite       func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	ValidateIssueURL func(issueURL string) error
	// ResolveBaseCommit은 요청된 base_sha가 이 레포에서 실제 commit으로
	// 해석되는지 확인하고 provider 명령에 쓸 canonical OID를 돌려준다.
	ResolveBaseCommit func(repo, revision string) (string, error)
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
	parentWorktree := strings.TrimSpace(req.ParentWorktree)
	if parentWorktree != "" {
		if !filepath.IsAbs(parentWorktree) {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("parent_worktree must be an absolute path")
		}
		parentWorktree = filepath.Clean(parentWorktree)
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
	baseSHA := strings.TrimSpace(req.BaseSHA)
	if baseSHA != "" {
		if store.ResolveBaseCommit == nil {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_sha validation is unavailable")
		}
		resolved, resolveErr := store.ResolveBaseCommit(record.Repo, baseSHA)
		if resolveErr != nil {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_sha %q does not resolve to a local commit: %w", baseSHA, resolveErr)
		}
		baseSHA = strings.TrimSpace(resolved)
		if baseSHA == "" {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_sha %q resolved to an empty commit OID", strings.TrimSpace(req.BaseSHA))
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.BranchPrepare = &model.IssueOpsBranchPrepare{
		Provider:        provider,
		IssueURL:        issueURL,
		Branch:          branch,
		BaseBranch:      baseBranch,
		BaseSHA:         baseSHA,
		ParentWorktree:  parentWorktree,
		RemoteBranchURL: strings.TrimSpace(req.RemoteBranchURL),
		LinkVerified:    req.LinkVerified,
		Steps:           Steps(provider, issueURL, branch, baseBranch, baseSHA),
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

// Steps는 provider별 브랜치 생성 안내를 만든다.
//
// baseSHA는 두 provider 모두에서 새 브랜치의 base를 못박는 데 쓴다. 비어 있으면
// base 브랜치 이름으로 떨어지고, 그 경우 provider가 **그 시점** 브랜치 HEAD를
// 쓰므로 orca가 봉인한 base와 갈릴 수 있다(#176 GitHub, #180 GitLab).
func Steps(provider, issueURL, branch, baseBranch, baseSHA string) []model.IssueOpsBranchPrepareStep {
	switch provider {
	case "gitlab":
		return gitlabSteps(branch, baseBranch, baseSHA)
	case "github":
		return githubSteps(issueURL, branch, baseBranch, baseSHA)
	default:
		return nil
	}
}

const gitlabBranchEndpoint = "projects/:fullpath/repository/branches"

// gitlabSteps는 issue-prefixed 브랜치 생성 안내를 만든다.
//
// 세 가지가 상류 소스로 확정됐다(#180). `glab`이 로컬에 설치돼 있지 않은 것은 그
// 계약을 확인할 수 없다는 뜻이 아니다 — gitlab-org/cli는 공개 저장소다.
//
// ① `ref`는 커밋 SHA를 받는다. GitLab의 Create repository branch API가 그것을
// 명시한다: "ref — Branch name or commit SHA to create the branch from." 따라서
// baseSHA가 있으면 그것을 넘겨 base 갈림을 원리적으로 없앤다. `#176`이 GitHub에서
// `createLinkedBranch`의 `oid`로 한 것과 같은 조치다.
//
// ② `glab_api`의 input schema는 중첩이다. internal/commands/mcp/serve/server.go의
// buildToolFromCommand가 모든 도구에 같은 네 키만 만든다:
//
//	inputSchema := map[string]any{"type": "object", "properties": map[string]any{
//	    argsParam: ..., flagsParam: ..., limitParam: ..., offsetParam: ...}}
//
// 즉 `args`(문자열 배열), `flags`(객체), `limit`, `offset`뿐이다. endpoint는 위치
// 인자라 `args`에 들어가고 나머지는 `flags` 안에 들어간다. `flags`의 키는
// strings.ReplaceAll(flag.Name, "-", "_")로 만들어진다.
//
// ③ `field`와 `raw-field`는 별개 플래그다. internal/commands/api/api.go:
//
//	fl.StringArrayVarP(&opts.magicFields, "field", "F", nil, "Add a parameter of inferred type. ...")
//	fl.StringArrayVarP(&opts.rawFields, "raw-field", "f", nil, "Add a string parameter.")
//
// 브랜치 이름과 ref는 문자열이므로 추론형 `field`가 아니라 `raw_field`가 맞고, CLI
// 폴백의 `-f`와 같은 플래그를 가리킨다.
//
// 도구 이름은 `glab_api`다. `glab` MCP는 opt-in이라 어노테이션 없는 명령은 등록되지
// 않지만(server.go의 mcpannotations.HasAnnotation 검사), `glab api`는
// `mcp:destructive`를 달고 있어 통과하고 이름은 "glab_" + 명령 경로가 된다.
func gitlabSteps(branch, baseBranch, baseSHA string) []model.IssueOpsBranchPrepareStep {
	ref := baseSHA
	base := "the sealed base " + baseSHA + "."
	if ref == "" {
		ref = baseBranch
		base = "branch " + baseBranch + ", whose current HEAD GitLab resolves at call time. " +
			"That can diverge from the base this cycle sealed; pass --base-sha to pin the exact commit (#180)."
	}
	rawFields := []string{"branch=" + branch, "ref=" + ref}

	return []model.IssueOpsBranchPrepareStep{
		{
			Order:    1,
			Strategy: "mcp",
			Tool:     "mcp__glab.glab_api",
			ToolArguments: map[string]any{
				"args": []string{gitlabBranchEndpoint},
				"flags": map[string]any{
					"method":    "POST",
					"raw_field": rawFields,
				},
			},
			Description: "Create the issue-prefixed branch through the GitLab MCP authenticated API tool. " +
				"The branch starts from " + base +
				" glab_api takes the endpoint as a positional `args` entry and everything else inside " +
				"`flags`; `raw_field` is glab's --raw-field string parameter, not the inferred-type --field.",
		},
		{
			Order:    2,
			Strategy: "fallback_api",
			Command: []string{"glab", "api", gitlabBranchEndpoint, "-X", "POST",
				"-f", rawFields[0], "-f", rawFields[1]},
			Description: "Fallback to the GitLab API through glab when the MCP tool is unavailable or fails. " +
				"-f is --raw-field, the same flag the MCP step names as raw_field.",
		},
		{
			Order:       3,
			Strategy:    "fail",
			Description: "Stop the IssueOps branch preparation if neither provider-linked creation path succeeds.",
		},
	}
}

// githubSteps는 linked branch 생성 안내를 만든다.
//
// `gh issue develop --base <branch>`는 GitHub이 **그 시점** 브랜치 HEAD를 조회해
// `CreateLinkedBranchInput.oid`로 쓴다. Orca는 봉인된 base SHA에서 로컬 브랜치를
// 만들므로, 링크를 붙이는 사이 base 브랜치가 진행하면 두 base가 갈리고 push가
// non-fast-forward로 거부된다. 그때 봉인 가드가 merge를, 안전 훅이 force push를,
// `sync-base`가 completion 이전 실행을 막으므로 발행 경로가 사라진다(#176, #147 실측).
//
// `oid`는 그 mutation의 필수 필드다(GraphQL 인트로스펙션 확인). `gh issue develop`이
// 그것을 숨기고 브랜치 HEAD로 채우는 것뿐이므로, 봉인 SHA를 직접 넘기면 갈림이
// 원리적으로 생기지 않는다 — 현재 base HEAD가 아닌 임의 SHA로 링크 브랜치가
// 만들어지는 것을 실측했다.
//
// node ID 조회를 별도 단계로 두는 이유는 가드다. 한 명령에 셸 치환을 넣으면
// 정적으로 분류할 수 없고 이 저장소는 그런 명령을 거부한다. `fallback_api`는
// 지금도 사람이나 에이전트가 실행하는 안내이므로 값을 옮기는 것이 그 주체의 일이다.
func githubSteps(issueURL, branch, baseBranch, baseSHA string) []model.IssueOpsBranchPrepareStep {
	// Orca 모드에서는 링크 단계를 execution prepare **이후**로 미룬다.
	// `createLinkedBranch`는 이름이 원격에 이미 있으면 실패하지만 로컬에만 있으면
	// 성공한다(#163 실측). Orca는 로컬 워크트리와 로컬 브랜치만 만들고 push하지
	// 않으므로, 먼저 실행하면 Orca가 이름 충돌로 막히고(#149·#152·#154) 나중에
	// 실행하면 linked branch 추적을 그대로 얻는다.
	const orcaOrder = "For Orca mode run this after `issueops execution prepare` instead: Orca creates the local branch " +
		"without pushing, so the name is still free on the remote and the link still attaches."
	steps := []model.IssueOpsBranchPrepareStep{{
		Order:       1,
		Strategy:    "mcp_unavailable",
		Description: "No GitHub MCP branch-creation tool is currently exposed in this harness session; do not silently create a local branch.",
	}}
	if baseSHA == "" {
		// base SHA 없이는 oid를 못박을 수 없다. 종전 경로로 떨어지되 그 사실을
		// 밝힌다 — 조용히 브랜치 이름을 쓰면 #147의 base 갈림이 재발한다.
		steps = append(steps, model.IssueOpsBranchPrepareStep{
			Order:    2,
			Strategy: "fallback_api",
			Command:  []string{"gh", "issue", "develop", issueURL, "--base", baseBranch, "--name", branch},
			Description: "Create a GitHub linked development branch through gh issue develop. " +
				"Recorded base_sha is empty, so the linked branch takes whatever " + baseBranch +
				" points at when this runs — it can diverge from a sealed Orca base (#176). " + orcaOrder,
		})
		steps = append(steps, githubLinkedBranchReadbackSteps(issueURL, branch, "", 3)...)
		return append(steps, model.IssueOpsBranchPrepareStep{
			Order:       5,
			Strategy:    "fail",
			Description: "Stop the IssueOps branch preparation if the linked development branch cannot be created or the readback does not match.",
		})
	}
	steps = append(steps,
		model.IssueOpsBranchPrepareStep{
			Order:    2,
			Strategy: "resolve_issue_node",
			Command:  []string{"gh", "api", githubIssueAPIPath(issueURL), "--jq", ".node_id"},
			Description: "Read the issue's GraphQL node id. The next step needs it as createLinkedBranch's issueId; " +
				"substitute the printed value literally instead of piping, because a command with shell substitution " +
				"cannot be statically classified by the lifecycle guard.",
		},
		model.IssueOpsBranchPrepareStep{
			Order:    3,
			Strategy: "fallback_api",
			Command: []string{"gh", "api", "graphql", "-f", "query=" + githubCreateLinkedBranchQuery,
				"-F", "issueId=<node-id-from-previous-step>", "-F", "oid=" + baseSHA, "-F", "name=" + branch},
			Description: "Create the linked development branch pinned to the sealed base " + baseSHA + ". " +
				"gh issue develop cannot do this: its --base flag takes a branch name and GitHub resolves that " +
				"branch's current HEAD, which diverges from the sealed base once " + baseBranch + " moves (#176). " +
				"Single-quote the query argument in the shell — its GraphQL variables ($issueId, $oid, $name) are read " +
				"as shell parameter expansion otherwise and the lifecycle guard rejects the command. " + orcaOrder,
		},
	)
	steps = append(steps, githubLinkedBranchReadbackSteps(issueURL, branch, baseSHA, 4)...)
	return append(steps, model.IssueOpsBranchPrepareStep{
		Order:       6,
		Strategy:    "fail",
		Description: "Stop the IssueOps branch preparation if the linked development branch cannot be created or the readback does not match.",
	})
}

// githubLinkedBranchReadbackSteps는 생성 직후의 검증 단계를 만든다.
//
// 왜 필요한가: `createLinkedBranch`가 GraphQL 오류 없이 응답하고도 실제 ref를
// 만들지 않는 부분 성공이 실측됐다 — 응답의 `linkedBranch`가 null이고 issue에는
// `ref: null`인 LinkedBranch 레코드만 남았다(#306). 생성 단계만 안내하고 끝나면
// 그 상태가 성공으로 통과하고, 같은 이름으로 재개할 수도 없게 된다.
//
// 검증은 두 축이다. issue 쪽 `linkedBranches`와 원격 `refs/heads/<branch>`가
// 서로를 확인한다 — 한쪽만 보면 provider 레코드와 실제 ref의 불일치를 놓친다.
func githubLinkedBranchReadbackSteps(issueURL, branch, baseSHA string, firstOrder int) []model.IssueOpsBranchPrepareStep {
	expectation := "Confirm that the node's `ref` is not null and that `ref.name` is exactly " + branch + "."
	if baseSHA != "" {
		expectation += " Confirm that `ref.target.oid` is exactly the sealed base " + baseSHA + "."
	}
	return []model.IssueOpsBranchPrepareStep{
		{
			Order:    firstOrder,
			Strategy: "verify_linked_branch",
			Command: []string{"gh", "api", "graphql", "-f",
				"query=" + githubLinkedBranchReadbackQuery(issueURL)},
			Description: "Read the issue's linked branches back. " + expectation +
				" A null `ref` is a partial success, not a failure to retry: the mutation already created a " +
				"LinkedBranch record, so running it again does not fix the state and may add a second orphan. " +
				"Report the observation instead.",
		},
		{
			Order:    firstOrder + 1,
			Strategy: "verify_linked_branch",
			Command:  []string{"git", "ls-remote", "--heads", "origin", "refs/heads/" + branch},
			Description: "Read the remote ref back. It must print exactly one line for refs/heads/" + branch +
				" and its OID must match what the previous step reported. An empty result with a non-null " +
				"LinkedBranch record is the same partial success; do not retry the mutation.",
		},
	}
}

// githubLinkedBranchReadbackQuery는 issue의 linked branch를 다시 읽는 질의다.
// 생성 질의와 같은 필드를 요청해 응답을 그대로 대조할 수 있게 한다.
func githubLinkedBranchReadbackQuery(issueURL string) string {
	owner, repo, number := githubIssueCoordinates(issueURL)
	return "{repository(owner:\"" + owner + "\",name:\"" + repo + "\")" +
		"{issue(number:" + number + "){linkedBranches(first:10){totalCount nodes{id ref{name target{oid}}}}}}}"
}

// githubIssueCoordinates는 이슈 URL에서 owner, repo, number를 뽑는다.
func githubIssueCoordinates(issueURL string) (string, string, string) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(issueURL), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 {
		return "OWNER", "REPO", "NUMBER"
	}
	return parts[len(parts)-4], parts[len(parts)-3], parts[len(parts)-1]
}

const githubCreateLinkedBranchQuery = "mutation($issueId:ID!,$oid:GitObjectID!,$name:String!)" +
	"{createLinkedBranch(input:{issueId:$issueId,oid:$oid,name:$name}){linkedBranch{ref{name target{oid}}}}}"

// githubIssueAPIPath는 이슈 URL을 REST 경로로 바꾼다. node id를 그 경로에서 읽는다.
func githubIssueAPIPath(issueURL string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(issueURL), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 {
		return "repos/OWNER/REPO/issues/NUMBER"
	}
	number := parts[len(parts)-1]
	repo := parts[len(parts)-3]
	owner := parts[len(parts)-4]
	return "repos/" + owner + "/" + repo + "/issues/" + number
}
