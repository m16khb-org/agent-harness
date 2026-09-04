package remoteverify

import (
	"context"
	"fmt"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopscore "issueops/internal/contract/issueops"
	policydomain "issueops/internal/domain/policy"
)

const remoteArtifactVerifyTimeout = 30 * time.Second

type liveRemoteArtifact struct {
	URL       string
	Labels    []string
	Assignees []string
	Merged    bool
	// HeadRefName/HeadRefOID는 PR/MR의 source 브랜치와 그 tip이다. cleanup
	// remote-branch가 "이 artifact가 정말 이 브랜치의 것인가"와 "머지 이후
	// push된 커밋이 없는가"를 판정하는 유일한 근거다(#116 게이트 ⑨·⑩).
	HeadRefName string
	HeadRefOID  string
	// BaseRefName은 PR/MR의 target 브랜치다. cleanup finish의 base drift
	// 게이트가 머지 관측과 같은 시점의 base를 요구한다.
	BaseRefName string
}

func VerifyRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return VerifyRemoteArtifactLiveContext(context.Background(), req)
}

func VerifyRemoteArtifactLiveContext(ctx context.Context, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, remoteArtifactVerifyTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	var artifact liveRemoteArtifact
	var err error
	switch provider + ":" + kind {
	case "github:issue":
		artifact, err = fetchGitHubIssueArtifactContext(ctx, strings.TrimSpace(req.URL))
	case "gitlab:issue":
		artifact, err = fetchGitLabIssueArtifactContext(ctx, strings.TrimSpace(req.URL))
	case "github:pr":
		artifact, err = fetchGitHubPullRequestArtifactContext(ctx, strings.TrimSpace(req.URL))
	case "gitlab:mr":
		artifact, err = fetchGitLabMergeRequestArtifactContext(ctx, strings.TrimSpace(req.URL))
	default:
		return fmt.Errorf("unsupported remote artifact verification: %s:%s", provider, kind)
	}
	if err != nil {
		return err
	}
	if strings.TrimSuffix(strings.TrimSpace(artifact.URL), "/") != strings.TrimSuffix(strings.TrimSpace(req.URL), "/") {
		return fmt.Errorf(
			"verified remote artifact URL %q does not match requested URL",
			policydomain.BoundedDiagnostic(artifact.URL, maxRemoteVerifyDiagnosticBytes),
		)
	}
	if err := requireRemoteValues("label", req.Labels, artifact.Labels); err != nil {
		return err
	}
	if err := requireRemoteValues("assignee", req.Assignees, artifact.Assignees); err != nil {
		return err
	}
	return nil
}

func VerifyRemoteArtifactMergedLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) error {
	_, err := VerifyRemoteArtifactMergedHeadLive(artifact)
	return err
}

// ObserveRemoteArtifactMergedLive는 artifact의 현재 병합 여부만 관측한다.
//
// VerifyRemoteArtifactMergedLive는 "조회 실패"와 "조회했더니 미병합"을 모두
// error로 뭉갠다. 머지를 요구하는 게이트(finish, remote-branch)에는 그 구분이
// 필요 없지만, 미병합을 요구하는 게이트(cleanup abandon)는 둘을 반드시 구분해야
// 한다. 조회 실패를 미병합으로 읽으면 원격이 잠시 불통일 때 병합된 사이클이
// abandon으로 지워질 수 있다(#342).
//
// 따라서 여기서는 관측에 성공한 경우에만 (merged, nil)을 돌려주고, 조회가
// 실패하면 병합 여부를 미상으로 두고 error를 그대로 올린다.
func ObserveRemoteArtifactMergedLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (bool, error) {
	live, err := fetchRemoteArtifactLive(artifact)
	if err != nil {
		return false, err
	}
	return live.Merged, nil
}

// ObserveRemoteArtifactTargetLive는 artifact의 현재 target(base) 브랜치만
// 관측한다. 머지 여부와 무관하게 provider가 지금 보여주는 값이며, branch
// retarget이 요청된 base를 대조하는 유일한 근거다.
func ObserveRemoteArtifactTargetLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (string, error) {
	live, err := fetchRemoteArtifactLive(artifact)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(live.BaseRefName), nil
}

func fetchRemoteArtifactLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (liveRemoteArtifact, error) {
	provider := strings.ToLower(strings.TrimSpace(artifact.Provider))
	kind := strings.ToLower(strings.TrimSpace(artifact.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	switch provider + ":" + kind {
	case "github:pr":
		return fetchGitHubPullRequestArtifact(strings.TrimSpace(artifact.URL))
	case "gitlab:mr":
		return fetchGitLabMergeRequestArtifact(strings.TrimSpace(artifact.URL))
	default:
		return liveRemoteArtifact{},
			fmt.Errorf("unsupported remote artifact for merge verification: %s:%s", provider, kind)
	}
}

// VerifyRemoteArtifactMergedHeadLive는 머지 검증과 head ref 관측을 한 번의
// readback으로 수행한다. 두 값이 다른 시점의 관측이면 cleanup remote-branch의
// OID CAS가 무의미해지므로 분리된 조회 표면을 두지 않는다(#116).
func VerifyRemoteArtifactMergedHeadLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscore.CleanupRemoteBranchArtifactHead, error) {
	live, err := fetchRemoteArtifactLive(artifact)
	if err != nil {
		return issueopscore.CleanupRemoteBranchArtifactHead{}, err
	}
	if !live.Merged {
		return issueopscore.CleanupRemoteBranchArtifactHead{},
			fmt.Errorf("remote artifact is not verified merged: %s", artifact.URL)
	}
	return issueopscore.CleanupRemoteBranchArtifactHead{
		HeadRefName: strings.TrimSpace(live.HeadRefName),
		HeadRefOID:  strings.TrimSpace(live.HeadRefOID),
		BaseRefName: strings.TrimSpace(live.BaseRefName),
	}, nil
}
