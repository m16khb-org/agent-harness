package remoteartifact

import remoteartifactcontract "agent-harness/internal/contract/remoteartifact"

import "strings"

// IssueEditTargetFromCommand는 이슈 본문을 바꾸는 편집 명령의 대상 식별자를
// 돌려준다. 번호와 URL 두 형태가 그대로 나오며, 봉인 여부 판정은 durable
// 레코드를 소유한 lifecycle 계층이 한다 — 이 패키지는 상태 저장소를 알지 못한다.
//
// 대상을 해석할 수 없으면 false다. 호출자는 이를 통과로 다뤄야 한다: 봉인 보호가
// 목적이므로 미지의 명령 형태를 막으면 봉인과 무관한 일상 작업이 깨진다.
func IssueEditTargetFromCommand(tool, command, repo string) (string, bool) {
	if !remoteArtifactGateAppliesToTool(tool) {
		return "", false
	}
	artifact, ok := parseGHRemoteArtifactCommand(command, repo)
	if !ok || artifact.kind != "issue" {
		return "", false
	}
	if artifact.action != "edit" && artifact.action != "update" {
		return "", false
	}
	target := strings.TrimSpace(artifact.target)
	if target == "" {
		return "", false
	}
	return target, true
}

func PullRequestBranchInfoFromCommand(tool, command, repo string) (remoteartifactcontract.PullRequestBranchInfo, bool) {
	if !remoteArtifactGateAppliesToTool(tool) {
		return remoteartifactcontract.PullRequestBranchInfo{}, false
	}
	artifact, ok := parseGHRemoteArtifactCommand(command, repo)
	if !ok || artifact.action != "create" {
		return remoteartifactcontract.PullRequestBranchInfo{}, false
	}
	if artifact.kind != "pr" && artifact.kind != "mr" {
		return remoteartifactcontract.PullRequestBranchInfo{}, false
	}
	return remoteartifactcontract.PullRequestBranchInfo{
		Provider:   strings.TrimSpace(artifact.provider),
		Kind:       strings.TrimSpace(artifact.kind),
		HeadBranch: strings.TrimSpace(artifact.headBranch),
		BaseBranch: strings.TrimSpace(artifact.baseBranch),
	}, true
}
