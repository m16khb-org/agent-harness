package artifacttemplate

import (
	"strings"
	"testing"
)

func TestRenderIssueTemplateUsesCanonicalKoreanSectionsAndProviderRules(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateFeature,
		Provider: "gitlab",
		Title:    "원격 템플릿 계약 강화",
		Fields: map[string]string{
			"problem":              "원격 이슈 본문 품질이 명령마다 달라진다.",
			"current_evidence":     "create-issue와 create-pr이 임의 body만 받는다.",
			"acceptance_criteria":  "템플릿 렌더링과 검증이 CLI/MCP에서 같은 결과를 낸다.",
			"non_goals":            "원격 provider adapter에 정책을 넣지 않는다.",
			"implementation_scope": "core renderer, CLI, MCP schema를 갱신한다.",
			"verification":         "go test ./internal/core/issueops/...",
			"risks":                "golden contract drift",
			"feedback_log":         "초기 구현 계획 반영",
			"score_summary":        "선택 라벨: enhancement, 거절 라벨: documentation, threshold 0.70",
		},
	})

	if !result.OK {
		t.Fatalf("render should pass with complete fields: %+v", result)
	}
	for _, want := range []string{"## 문제", "## 현재 근거", "## 관련 이슈/라벨 판단", "## 완료 기준", "## 비목표", "## 구현 범위", "## 검증", "## 위험과 트레이드오프", "## 피드백 기록"} {
		if !strings.Contains(result.Body, want) {
			t.Fatalf("rendered body missing section %q:\n%s", want, result.Body)
		}
	}
	if strings.Contains(result.Body, "## Related Issues") {
		t.Fatalf("GitLab issue body must not use GitHub-style Related Issues section:\n%s", result.Body)
	}
}

func TestRenderBugTemplateReportsMissingRequiredFields(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateBug,
		Provider: "github",
		Title:    "로그인 실패",
		Fields: map[string]string{
			"problem": "로그인이 실패한다.",
		},
	})

	if result.OK {
		t.Fatalf("bug template with missing fields must not be OK: %+v", result)
	}
	for _, want := range []string{"current_evidence", "acceptance_criteria", "reproduction_steps", "expected_behavior", "actual_behavior", "environment", "logs"} {
		if !contains(result.MissingRequiredFields, want) {
			t.Fatalf("missing required fields %v did not include %q", result.MissingRequiredFields, want)
		}
	}
}

func TestValidateRejectsPlanLinkAndGitLabRelatedIssuesSection(t *testing.T) {
	validation := Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateFeature,
		Provider: "gitlab",
		Title:    "본문 검증",
		Body:     "## 문제\n\n본문\n\n## Related Issues\n\n#1\n\n## Plan Link\n\n/path/to/plan.md",
	})

	if validation.OK {
		t.Fatalf("validation must fail for forbidden sections: %+v", validation)
	}
	if !contains(validation.Critical, "plan_link_section_forbidden") {
		t.Fatalf("validation criticals missing plan link finding: %+v", validation)
	}
	if !contains(validation.Critical, "gitlab_related_issues_body_section_forbidden") {
		t.Fatalf("validation criticals missing gitlab related finding: %+v", validation)
	}
}

func TestRenderChildAndPRTemplatesIncludeContractSections(t *testing.T) {
	child := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactChild,
		Template: IssueOpsTemplateChildTask,
		Provider: "github",
		Title:    "하위 작업",
		Fields: map[string]string{
			"parent_issue":    "https://github.com/acme/repo/issues/1",
			"task_goal":       "템플릿 렌더러 구현",
			"acceptance":      "렌더러 테스트 통과",
			"non_goals":       "provider 정책 복제 제외",
			"verification":    "go test ./internal/core/issueops/...",
			"merge_condition": "부모 브랜치에 병합된 뒤 close-children 실행",
			"cleanup":         "child-only cleanup 규칙 유지",
		},
	})
	pr := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "gitlab",
		Title:    "MR",
		Fields: map[string]string{
			"intent":              "원격 템플릿 계약을 고정한다.",
			"issue":               "https://gitlab.example/acme/repo/-/issues/1",
			"changes":             "core renderer와 CLI/MCP를 추가한다.",
			"verification":        "go test ./...",
			"reviewer_focus":      "template validation boundary",
			"risk_rollback":       "기능 flag 없이 dry-run 기본 유지",
			"user_impact":         "원격 artifact 품질 일관성 개선",
			"docs_migration":      "IssueOps 문서 갱신",
			"scope_management":    "provider adapter는 thin 유지",
			"worktree_cleanup":    "cleanup status 확인",
			"automation_evidence": "AI 생성 본문은 renderer 결과로 검증",
		},
	})

	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"child", child.Body, []string{"## 부모 이슈", "## 작업 목표", "## 부모 브랜치 병합 조건", "## child-only cleanup 규칙"}},
		{"pr", pr.Body, []string{"## 의도", "## 이슈", "## 리뷰어 초점", "## 위험/rollback", "## 자동화/AI 개입 근거"}},
	} {
		for _, want := range tc.want {
			if !strings.Contains(tc.body, want) {
				t.Fatalf("%s body missing %q:\n%s", tc.name, want, tc.body)
			}
		}
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
