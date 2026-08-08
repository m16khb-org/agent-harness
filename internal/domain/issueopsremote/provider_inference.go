package remote

import (
	"fmt"
	"sort"
	"strings"
)

// ProviderFromRemoteURLs는 저장소 remote URL 집합에서 VCS provider를 단일
// 판별한다.
//
// 왜 필요한가: `remote create-issue`는 최초 원격 이슈를 만드는 명령인데
// provider 판별이 record의 `issue_url`에만 의존했다. 이슈를 만들기 전에는
// 그 값이 없는 것이 정상이므로 bootstrap 순환이 생긴다(#300). record가
// 침묵할 때 저장소 remote는 이미 답을 알고 있다.
//
// 판별은 fail-safe다. remote가 없거나, 알 수 없는 host만 있거나, 서로 다른
// provider를 가리키는 remote가 섞여 있으면 추측하지 않고 typed 오류를 낸다 —
// 잘못된 provider로 원격 mutation을 보내는 것이 실패보다 나쁘다.
func ProviderFromRemoteURLs(urls []string) (string, error) {
	seen := map[string]bool{}
	for _, raw := range urls {
		if provider := providerFromRemoteURL(raw); provider != "" {
			seen[provider] = true
		}
	}
	switch len(seen) {
	case 1:
		for provider := range seen {
			return provider, nil
		}
	case 0:
		return "", fmt.Errorf("cannot determine provider: no GitHub or GitLab remote is configured; pass --provider github|gitlab")
	}
	names := make([]string, 0, len(seen))
	for provider := range seen {
		names = append(names, provider)
	}
	sort.Strings(names)
	return "", fmt.Errorf("cannot determine provider: remotes point at more than one provider (%s); pass --provider github|gitlab",
		strings.Join(names, ", "))
}

// providerFromRemoteURL은 하나의 remote URL에서 provider를 읽는다. host
// 성분만 본다 — 경로에 들어간 우연한 문자열을 host로 오인하지 않기 위해서다.
func providerFromRemoteURL(raw string) string {
	host := remoteURLHost(raw)
	switch {
	case host == "":
		return ""
	case host == "github.com" || strings.HasPrefix(host, "github."):
		return "github"
	case host == "gitlab.com" || strings.HasPrefix(host, "gitlab."):
		return "gitlab"
	default:
		return ""
	}
}

// remoteURLHost는 scp 형식(`git@host:owner/repo.git`)과 URL 형식
// (`https://host/owner/repo.git`, `ssh://git@host/owner/repo`)에서 host를 뽑는다.
func remoteURLHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "://"); index >= 0 {
		value = value[index+3:]
	} else if at := strings.Index(value, "@"); at >= 0 && !strings.Contains(value[:at], "/") {
		// scp 형식: user@host:path
		value = value[at+1:]
		if colon := strings.Index(value, ":"); colon >= 0 {
			return strings.ToLower(strings.TrimSpace(value[:colon]))
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
	if at := strings.Index(value, "@"); at >= 0 {
		value = value[at+1:]
	}
	if slash := strings.Index(value, "/"); slash >= 0 {
		value = value[:slash]
	}
	if colon := strings.Index(value, ":"); colon >= 0 {
		value = value[:colon]
	}
	return strings.ToLower(strings.TrimSpace(value))
}
