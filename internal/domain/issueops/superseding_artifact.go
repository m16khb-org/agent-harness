package issueops

import (
	"fmt"
	"net/url"
	"strings"
)

// ArtifactObservation은 cleanup 판정에 쓰이는 원격 artifact의 관측값이다.
// provider readback이 채우며, 관측하지 못한 값은 빈 채로 남아 fail-closed의
// 근거가 된다.
type ArtifactObservation struct {
	// URL은 canonical artifact URL이다.
	URL string
	// Provider는 "github" 또는 "gitlab"이다.
	Provider string
	// Merged는 provider가 보고한 머지 여부다.
	Merged bool
	// State는 provider의 원문 상태("MERGED", "CLOSED", "OPEN" 등)다. 진단에 쓴다.
	State string
	// Body는 supersede 관계를 읽는 원문이다.
	Body string
}

// ValidateSupersedingArtifact는 replacement artifact가 original을 대체했다고
// 인정할 수 있는지 판정한다.
//
// 왜 필요한가: 완료된 record가 가리키는 원래 PR이 closed-unmerged이고, 후속
// PR이 그 변경을 명시적으로 포함해 머지된 경우가 있다. 그러면 `cleanup finish`는
// "원래 artifact가 merged가 아니다"로 거부하고, `cleanup abandon`은 "phase가
// done"이라 거부해 정식 정리 경로가 사라진다(#283). 2026-08-08 dogfood에서
// lifecycle 3건이 실제로 이 dead-end에 걸렸다.
//
// 판정은 fail-closed다. 세 조건이 모두 관측돼야 인정한다.
//   - replacement가 provider에서 merged로 확인된다
//   - original과 같은 provider·repo다 (cross-repo 대체를 인정하면 임의의 머지된
//     PR로 아무 record나 지울 수 있다)
//   - replacement 본문이 original을 명시적으로 supersede한다
func ValidateSupersedingArtifact(original, replacement ArtifactObservation) error {
	if strings.TrimSpace(replacement.URL) == "" {
		return fmt.Errorf("superseding artifact URL is required")
	}
	if strings.TrimSpace(original.URL) == "" {
		return fmt.Errorf("original artifact URL is unknown; cannot verify a supersede relation")
	}
	if sameArtifactURL(original.URL, replacement.URL) {
		return fmt.Errorf("superseding artifact must differ from the original artifact")
	}
	if !replacement.Merged {
		state := strings.TrimSpace(replacement.State)
		if state == "" {
			state = "unknown"
		}
		return fmt.Errorf("superseding artifact %s is not merged (state %s)", replacement.URL, state)
	}
	originalProject, ok := artifactProjectKey(original.URL)
	if !ok {
		return fmt.Errorf("original artifact URL %q is not a canonical artifact URL", original.URL)
	}
	replacementProject, ok := artifactProjectKey(replacement.URL)
	if !ok {
		return fmt.Errorf("superseding artifact URL %q is not a canonical artifact URL", replacement.URL)
	}
	if originalProject != replacementProject {
		return fmt.Errorf("superseding artifact %s belongs to %s, not the original project %s",
			replacement.URL, replacementProject, originalProject)
	}
	if !bodyDeclaresSupersede(replacement.Body, original.URL) {
		return fmt.Errorf("superseding artifact %s does not declare that it supersedes %s", replacement.URL, original.URL)
	}
	return nil
}

// bodyDeclaresSupersede는 replacement 본문이 original을 명시적으로 대체한다고
// 밝히는지 본다.
//
// 단순히 URL이 언급되기만 해서는 안 된다 — 관련 이슈 나열이나 인용에도 URL은
// 등장한다. supersede 어휘와 대상 참조가 **같은 줄에** 있어야 인정한다.
func bodyDeclaresSupersede(body, originalURL string) bool {
	number, hasNumber := artifactNumberReference(originalURL)
	for _, line := range strings.Split(body, "\n") {
		lowered := strings.ToLower(line)
		if !strings.Contains(lowered, "supersede") && !strings.Contains(lowered, "replaces") {
			continue
		}
		if strings.Contains(line, originalURL) {
			return true
		}
		if hasNumber && strings.Contains(line, number) {
			return true
		}
	}
	return false
}

// artifactNumberReference는 canonical artifact URL에서 `#<number>` 형태의
// 짧은 참조를 만든다. 본문은 보통 그 형태로 대상을 가리킨다.
func artifactNumberReference(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return "", false
	}
	number := segments[len(segments)-1]
	if number == "" || strings.ContainsAny(number, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return "", false
	}
	return "#" + number, true
}

// artifactProjectKey는 artifact URL에서 `host/owner/repo` 권한을 뽑는다.
// GitLab의 `/-/merge_requests/<iid>`처럼 구분자가 있는 경로도 다룬다.
func artifactProjectKey(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return "", false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	project := []string{}
	for _, segment := range segments {
		if segment == "-" || segment == "issues" || segment == "pull" || segment == "merge_requests" {
			break
		}
		project = append(project, segment)
	}
	if len(project) < 2 {
		return "", false
	}
	return strings.ToLower(parsed.Host + "/" + strings.Join(project, "/")), true
}

func sameArtifactURL(left, right string) bool {
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(left), "/"),
		strings.TrimRight(strings.TrimSpace(right), "/"))
}
