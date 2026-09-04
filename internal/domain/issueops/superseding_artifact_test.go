package issueops

import (
	"strings"
	"testing"
)

func originalArtifact() ArtifactObservation {
	return ArtifactObservation{
		URL:      "https://github.com/m16khb/issueops/pull/241",
		Provider: "github",
		State:    "CLOSED",
	}
}

// TestValidateSupersedingArtifactAcceptsDeclaredMergedReplacement는 #283이
// 요구하는 인정 조건을 고정한다. 세 조건이 모두 관측돼야 한다.
func TestValidateSupersedingArtifactAcceptsDeclaredMergedReplacement(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"URL 참조", "Supersedes https://github.com/m16khb/issueops/pull/241 and #243"},
		{"번호 참조", "Supersedes #243 and #241"},
		{"replaces 어휘", "This replaces #241 with a corrected boundary"},
		{"대소문자 무시", "SUPERSEDES #241"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replacement := ArtifactObservation{
				URL: "https://github.com/m16khb/issueops/pull/245", Provider: "github",
				Merged: true, State: "MERGED", Body: tc.body,
			}
			if err := ValidateSupersedingArtifact(originalArtifact(), replacement); err != nil {
				t.Fatalf("선언된 merged replacement는 인정돼야 한다: %v", err)
			}
		})
	}
}

// TestValidateSupersedingArtifactFailsClosed는 이 경로가 "아무 머지된 PR로
// 아무 record나 지우는" 문을 열지 않음을 고정한다.
func TestValidateSupersedingArtifactFailsClosed(t *testing.T) {
	base := ArtifactObservation{
		URL: "https://github.com/m16khb/issueops/pull/245", Provider: "github",
		Merged: true, State: "MERGED", Body: "Supersedes #241",
	}
	for _, tc := range []struct {
		name        string
		mutate      func(*ArtifactObservation)
		wantMessage string
	}{
		{"머지되지 않음", func(a *ArtifactObservation) { a.Merged, a.State = false, "OPEN" }, "is not merged"},
		{"상태 미관측", func(a *ArtifactObservation) { a.Merged, a.State = false, "" }, "state unknown"},
		{"cross-repo", func(a *ArtifactObservation) {
			a.URL = "https://github.com/other/repo/pull/245"
		}, "not the original project"},
		{"cross-host", func(a *ArtifactObservation) {
			a.URL = "https://github.enterprise/m16khb/issueops/pull/245"
		}, "not the original project"},
		{"supersede 선언 없음", func(a *ArtifactObservation) { a.Body = "Fixes a boundary bug" }, "does not declare"},
		{"URL만 언급", func(a *ArtifactObservation) {
			a.Body = "Related: https://github.com/m16khb/issueops/pull/241"
		}, "does not declare"},
		{"다른 줄의 supersede", func(a *ArtifactObservation) {
			a.Body = "Supersedes an earlier attempt\n\nRelated: #241"
		}, "does not declare"},
		{"자기 자신", func(a *ArtifactObservation) {
			a.URL = "https://github.com/m16khb/issueops/pull/241"
		}, "must differ"},
		{"URL 없음", func(a *ArtifactObservation) { a.URL = "" }, "URL is required"},
		{"canonical 아님", func(a *ArtifactObservation) { a.URL = "not-a-url" }, "canonical artifact URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replacement := base
			tc.mutate(&replacement)
			err := ValidateSupersedingArtifact(originalArtifact(), replacement)
			if err == nil {
				t.Fatal("fail-closed여야 한다")
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("오류가 사유를 밝혀야 한다: got %v, want contains %q", err, tc.wantMessage)
			}
		})
	}
}

// TestValidateSupersedingArtifactRequiresAKnownOriginal은 원본을 모르면
// supersede 관계 자체를 검증할 수 없음을 고정한다.
func TestValidateSupersedingArtifactRequiresAKnownOriginal(t *testing.T) {
	replacement := ArtifactObservation{
		URL:    "https://github.com/m16khb/issueops/pull/245",
		Merged: true, State: "MERGED", Body: "Supersedes #241",
	}
	err := ValidateSupersedingArtifact(ArtifactObservation{}, replacement)
	if err == nil || !strings.Contains(err.Error(), "original artifact URL is unknown") {
		t.Fatalf("원본 미상은 거부돼야 한다: %v", err)
	}
}
