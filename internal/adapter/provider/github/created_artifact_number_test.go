package github

import "testing"

// TestCreatedArtifactNumberProjectsTheCanonicalURLNumber는 #314를 고정한다.
// GitHub provider는 생성 결과로 URL만 돌려주고 public contract가 선언한
// `number`는 빈 문자열로 남겼다. GitLab provider는 채우므로 두 provider의
// 결과 계약이 어긋나 있었다.
func TestCreatedArtifactNumberProjectsTheCanonicalURLNumber(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		want string
	}{
		{"issue", "https://github.com/acme/repo/issues/312", "312"},
		{"pull request", "https://github.com/acme/repo/pull/313", "313"},
		// Enterprise는 자체 도메인을 쓴다. 호스트를 못박으면 Enterprise 생성
		// 결과의 number가 통째로 비게 된다.
		{"enterprise issue", "https://github.enterprise/acme/repo/issues/15", "15"},
		{"enterprise pull request", "https://github.enterprise/acme/repo/pull/16", "16"},
		{"nested namespace issue", "https://github.com/acme/group/repo/issues/7", ""},
		{"trailing slash", "https://github.com/acme/repo/issues/42/", "42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := createdArtifactNumber(tc.url); got != tc.want {
				t.Fatalf("createdArtifactNumber(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

// TestCreatedArtifactNumberRejectsMalformedURLs는 malformed URL을 성공으로
// 오인하지 않음을 고정한다. 빈 number는 호출부가 fail-closed 판단에 쓴다.
func TestCreatedArtifactNumberRejectsMalformedURLs(t *testing.T) {
	for _, raw := range []string{
		"",
		"   ",
		"https://github.com/acme/repo/issues/",
		"https://github.com/acme/repo/issues/abc",
		"https://github.com/acme/repo/issues/-1",
		"https://gitlab.com/acme/repo/-/issues/1",
		"https://github.com/acme/repo/discussions/9",
		"not a url",
	} {
		if got := createdArtifactNumber(raw); got != "" {
			t.Fatalf("createdArtifactNumber(%q) = %q, want empty", raw, got)
		}
	}
}
