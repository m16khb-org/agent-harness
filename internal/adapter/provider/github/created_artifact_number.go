package github

import (
	"net/url"
	"strconv"
	"strings"
)

// createdArtifactNumber는 canonical GitHub issue/PR URL에서 artifact number를
// 뽑는다. 판정하지 못하면 빈 문자열을 돌려주고, 호출부가 그것을 fail-closed
// 근거로 쓴다(#314).
//
// 호스트는 검사하지 않는다 — GitHub Enterprise는 자체 도메인을 쓰므로
// `github.com`으로 못박으면 Enterprise 생성 결과의 number가 통째로 비게 된다.
// 대신 **경로 모양**을 정확히 요구한다: path 세그먼트가 정확히
// `<owner>/<repo>/(issues|pull)/<number>` 네 개여야 한다. GitLab의
// `/-/issues/<iid>`는 세그먼트 수와 위치가 달라 자연히 배제되고, nested
// namespace나 discussions 같은 다른 경로도 마찬가지다.
//
// 잘못된 number를 채우는 것은 비워 두는 것보다 나쁘므로 추측하지 않는다.
func createdArtifactNumber(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 4 {
		return ""
	}
	owner, repo, kind, number := segments[0], segments[1], segments[2], segments[3]
	if owner == "" || repo == "" || (kind != "issues" && kind != "pull") {
		return ""
	}
	if value, err := strconv.Atoi(number); err != nil || value <= 0 {
		return ""
	}
	return number
}
