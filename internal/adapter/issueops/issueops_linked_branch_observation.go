package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

// githubLinkedBranchListQuery는 이슈의 linked branch를 읽는다. branch prepare의
// readback 질의와 같은 필드를 요청해 두 관측이 같은 모양이 되게 한다.
//
// `first:20`은 분류기가 잘림을 감지할 수 있게 totalCount와 함께 쓰인다 —
// 잘린 페이지는 부재의 증거가 아니므로 조용히 넘기지 않고 ambiguous가 된다.
const githubLinkedBranchListQuery = "query($owner:String!,$repo:String!,$number:Int!)" +
	"{repository(owner:$owner,name:$repo){issue(number:$number)" +
	"{linkedBranches(first:20){totalCount nodes{id ref{name target{oid}}}}}}}"

// githubDeleteLinkedBranchMutation은 노드 id 하나만 지운다.
const githubDeleteLinkedBranchMutation = "mutation($id:ID!){deleteLinkedBranch(input:{linkedBranchId:$id}){clientMutationId}}"

// ObserveGitHubLinkedBranches는 이슈의 linked-branch 목록을 읽어 분류기 입력을
// 만든다. 나머지 필드(요청 브랜치·봉인 base·원격 OID)는 호출부가 record와 git
// 관측에서 채운다 — 이 함수는 provider가 말하는 것만 담는다.
func ObserveGitHubLinkedBranches(runner ProviderCLIRunner) func(context.Context, string) (linkedbranch.Observation, error) {
	return func(ctx context.Context, issueURL string) (linkedbranch.Observation, error) {
		owner, repo, number, err := githubIssueSelector(issueURL)
		if err != nil {
			return linkedbranch.Observation{}, err
		}
		raw, err := runner(ctx, "gh", "api", "graphql",
			"-F", "owner="+owner, "-F", "repo="+repo, "-F", "number="+number,
			"-f", "query="+githubLinkedBranchListQuery)
		if err != nil {
			return linkedbranch.Observation{}, err
		}
		var payload struct {
			Data struct {
				Repository struct {
					Issue struct {
						LinkedBranches struct {
							TotalCount int `json:"totalCount"`
							Nodes      []struct {
								ID  string `json:"id"`
								Ref *struct {
									Name   string `json:"name"`
									Target *struct {
										OID string `json:"oid"`
									} `json:"target"`
								} `json:"ref"`
							} `json:"nodes"`
						} `json:"linkedBranches"`
					} `json:"issue"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return linkedbranch.Observation{}, fmt.Errorf("linked branch readback is not parseable: %w", err)
		}
		linked := payload.Data.Repository.Issue.LinkedBranches
		observation := linkedbranch.Observation{TotalCount: linked.TotalCount}
		for _, node := range linked.Nodes {
			// ref가 null이면 이름도 OID도 없다. 그 부재가 곧 고아의 표식이므로
			// 빈 값으로 두고 분류기가 판단하게 한다.
			observed := linkedbranch.Node{ID: node.ID}
			if node.Ref != nil {
				observed.RefName = node.Ref.Name
				if node.Ref.Target != nil {
					observed.RefOID = node.Ref.Target.OID
				}
			}
			observation.Nodes = append(observation.Nodes, observed)
		}
		return observation, nil
	}
}

// DeleteGitHubLinkedBranch는 확정된 노드 하나를 지운다. 브랜치 이름을 받지
// 않는 것이 의도다 — 이름으로 지우는 표면이 있으면 ref 있는 링크도 지울 수
// 있게 되고, 그 경로는 이 이슈가 막으려는 것이다.
func DeleteGitHubLinkedBranch(runner ProviderCLIRunner) func(context.Context, string, string) error {
	return func(ctx context.Context, _, nodeID string) error {
		if strings.TrimSpace(nodeID) == "" {
			return fmt.Errorf("linked branch deletion requires an exact node id")
		}
		_, err := runner(ctx, "gh", "api", "graphql", "-F", "id="+nodeID, "-f", "query="+githubDeleteLinkedBranchMutation)
		return err
	}
}

// githubIssueSelector는 이슈 URL에서 owner, repo, number를 뽑는다. 형태가
// 어긋나면 추측하지 않고 실패한다 — 잘못된 좌표로 남의 이슈를 읽을 수 있다.
func githubIssueSelector(issueURL string) (string, string, string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(issueURL), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 {
		return "", "", "", fmt.Errorf("issue url does not name an owner, repo, and number: %q", issueURL)
	}
	owner, repo, number := parts[len(parts)-4], parts[len(parts)-3], parts[len(parts)-1]
	if parts[len(parts)-2] != "issues" || owner == "" || repo == "" || number == "" {
		return "", "", "", fmt.Errorf("issue url is not an issue path: %q", issueURL)
	}
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return "", "", "", fmt.Errorf("issue url does not end in an issue number: %q", issueURL)
		}
	}
	return owner, repo, number, nil
}

// ProviderCLIRunner는 provider CLI 한 번의 실행이다. 주입점으로 두어 테스트가
// 실제 네트워크 없이 응답 모양을 고정할 수 있게 한다.
type ProviderCLIRunner func(ctx context.Context, name string, args ...string) (string, error)

// LiveProviderCLI는 provider CLI를 비대화로 실행한다. git 경로와 같은 규율을
// 따른다 — 프롬프트가 뜨면 호출한 세션이 그대로 붙잡히기 때문이다.
func LiveProviderCLI(ctx context.Context, name string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, providerCLITimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GIT_TERMINAL_PROMPT=0", "GH_PAGER=cat", "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		// stderr를 오류에 실어야 GraphQL 거부 사유가 사용자에게 보인다.
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

const providerCLITimeout = 60 * time.Second
