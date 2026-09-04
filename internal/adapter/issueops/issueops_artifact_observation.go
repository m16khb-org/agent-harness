package issueops

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	issueopsdomain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

// ObserveRemoteArtifact는 원격 artifact의 현재 상태를 provider CLI로 읽는다.
// replacement 증거 검증의 유일한 관측 경로다(#283).
//
// 읽기 전용이며 mutation 인자를 쓰지 않는다. 관측에 실패하면 오류를 돌려주고,
// 호출부는 그것을 통과 근거로 삼지 않는다.
func ObserveRemoteArtifact(url string) (issueopsdomain.ArtifactObservation, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("artifact URL is required")
	}
	provider, err := remote.ProviderFromRemoteURLs([]string{url})
	if err != nil {
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("artifact URL %q has no recognizable provider host", url)
	}
	switch provider {
	case "github":
		return observeGitHubArtifact(url)
	case "gitlab":
		return observeGitLabArtifact(url)
	default:
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("unsupported provider %q for artifact observation", provider)
	}
}

func observeGitHubArtifact(url string) (issueopsdomain.ArtifactObservation, error) {
	out, err := exec.Command("gh", "pr", "view", url, "--json", "state,mergedAt,body,url").Output()
	if err != nil {
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("gh pr view failed for %s", url)
	}
	var payload struct {
		State    string `json:"state"`
		MergedAt string `json:"mergedAt"`
		Body     string `json:"body"`
		URL      string `json:"url"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("gh pr view returned malformed JSON for %s", url)
	}
	return issueopsdomain.ArtifactObservation{
		URL: firstNonEmptyArtifactValue(payload.URL, url), Provider: "github",
		// mergedAt이 채워진 것만 머지로 인정한다. state 문자열은 provider마다
		// 표기가 갈리므로 진단에만 쓴다.
		Merged: strings.TrimSpace(payload.MergedAt) != "",
		State:  strings.TrimSpace(payload.State),
		Body:   payload.Body,
	}, nil
}

func observeGitLabArtifact(url string) (issueopsdomain.ArtifactObservation, error) {
	out, err := exec.Command("glab", "mr", "view", url, "--output", "json").Output()
	if err != nil {
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("glab mr view failed for %s", url)
	}
	var payload struct {
		State       string `json:"state"`
		MergedAt    string `json:"merged_at"`
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return issueopsdomain.ArtifactObservation{}, fmt.Errorf("glab mr view returned malformed JSON for %s", url)
	}
	return issueopsdomain.ArtifactObservation{
		URL: firstNonEmptyArtifactValue(payload.WebURL, url), Provider: "gitlab",
		Merged: strings.TrimSpace(payload.MergedAt) != "" || strings.EqualFold(strings.TrimSpace(payload.State), "merged"),
		State:  strings.TrimSpace(payload.State),
		Body:   payload.Description,
	}, nil
}

func firstNonEmptyArtifactValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
