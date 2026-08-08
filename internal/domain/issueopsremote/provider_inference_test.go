package remote

import (
	"strings"
	"testing"
)

// TestProviderFromRemoteURLsResolvesASingleProvider는 #300의 bootstrap 순환을
// 끊는 판별을 고정한다. record가 침묵할 때 저장소 remote는 이미 답을 안다.
func TestProviderFromRemoteURLsResolvesASingleProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		urls []string
		want string
	}{
		{"https github", []string{"https://github.com/acme/repo.git"}, "github"},
		{"scp github", []string{"git@github.com:acme/repo.git"}, "github"},
		{"ssh github", []string{"ssh://git@github.com/acme/repo.git"}, "github"},
		{"github enterprise", []string{"https://github.enterprise.example/acme/repo.git"}, "github"},
		{"https gitlab", []string{"https://gitlab.com/acme/repo.git"}, "gitlab"},
		{"scp gitlab", []string{"git@gitlab.self-hosted.example:acme/repo.git"}, "gitlab"},
		{"동일 provider 다중 remote", []string{
			"https://github.com/acme/repo.git", "git@github.com:fork/repo.git",
		}, "github"},
		{"알 수 없는 remote 혼재", []string{
			"https://github.com/acme/repo.git", "https://example.invalid/mirror.git",
		}, "github"},
		{"host가 아닌 경로의 provider 이름 무시", []string{"https://git.example/acme/github.com.git"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProviderFromRemoteURLs(tc.urls)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("판별 불가여야 한다: got=%q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ProviderFromRemoteURLs(%v) = (%q, %v), want %q", tc.urls, got, err, tc.want)
			}
		})
	}
}

// TestProviderFromRemoteURLsFailsSafeOnAmbiguity는 추측하지 않음을 고정한다.
// 잘못된 provider로 원격 mutation을 보내는 것이 실패보다 나쁘다.
func TestProviderFromRemoteURLsFailsSafeOnAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		name        string
		urls        []string
		wantMessage string
	}{
		{"remote 없음", nil, "no GitHub or GitLab remote"},
		{"빈 URL", []string{"", "   "}, "no GitHub or GitLab remote"},
		{"알 수 없는 host만", []string{"https://example.invalid/acme/repo.git"}, "no GitHub or GitLab remote"},
		{"두 provider 혼재", []string{
			"https://github.com/acme/repo.git", "https://gitlab.com/acme/repo.git",
		}, "more than one provider"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProviderFromRemoteURLs(tc.urls)
			if err == nil {
				t.Fatalf("모호한 입력은 typed 오류여야 한다: got=%q", got)
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("오류가 사유를 밝혀야 한다: %v", err)
			}
			if !strings.Contains(err.Error(), "--provider github|gitlab") {
				t.Fatalf("오류가 복구 명령을 안내해야 한다: %v", err)
			}
		})
	}
}
