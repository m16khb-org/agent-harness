package artifacttemplate

import (
	"fmt"
	"sort"
	"strings"
)

type IssueOpsArtifactKind string

const (
	IssueOpsArtifactIssue IssueOpsArtifactKind = "issue"
	IssueOpsArtifactChild IssueOpsArtifactKind = "child"
	IssueOpsArtifactPR    IssueOpsArtifactKind = "pr"
)

type IssueOpsTemplateKind string

const (
	IssueOpsTemplateBug                IssueOpsTemplateKind = "bug"
	IssueOpsTemplateFeature            IssueOpsTemplateKind = "feature"
	IssueOpsTemplateProposal           IssueOpsTemplateKind = "proposal"
	IssueOpsTemplateImplementationTask IssueOpsTemplateKind = "implementation_task"
	IssueOpsTemplateChildTask          IssueOpsTemplateKind = "child_task"
	IssueOpsTemplatePullRequest        IssueOpsTemplateKind = "pull_request"
)

type IssueOpsTemplateInput struct {
	Kind         IssueOpsArtifactKind `json:"kind"`
	Template     IssueOpsTemplateKind `json:"template"`
	Provider     string               `json:"provider,omitempty"`
	Title        string               `json:"title"`
	Body         string               `json:"body,omitempty"`
	Fields       map[string]string    `json:"fields,omitempty"`
	ScoreSummary string               `json:"score_summary,omitempty"`
}

type IssueOpsTemplateValidation struct {
	OK                    bool     `json:"ok"`
	Critical              []string `json:"critical"`
	Warnings              []string `json:"warnings"`
	MissingRequiredFields []string `json:"missing_required_fields"`
}

type IssueOpsTemplateResult struct {
	OK                    bool                       `json:"ok"`
	Kind                  IssueOpsArtifactKind       `json:"kind"`
	Template              IssueOpsTemplateKind       `json:"template"`
	Provider              string                     `json:"provider,omitempty"`
	Title                 string                     `json:"title"`
	Body                  string                     `json:"body"`
	Warnings              []string                   `json:"warnings"`
	MissingRequiredFields []string                   `json:"missing_required_fields"`
	Validation            IssueOpsTemplateValidation `json:"validation"`
}

func Render(input IssueOpsTemplateInput) IssueOpsTemplateResult {
	input = normalizeInput(input)
	originalBodyEmpty := strings.TrimSpace(input.Body) == ""
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = renderBody(input)
	}
	input.Body = body
	validation := Validate(input)
	if originalBodyEmpty {
		validation.MissingRequiredFields = uniqueSorted(missingFields(IssueOpsTemplateInput{
			Kind:     input.Kind,
			Template: input.Template,
			Provider: input.Provider,
			Title:    input.Title,
			Fields:   input.Fields,
		}))
		if len(validation.MissingRequiredFields) > 0 && !containsString(validation.Critical, "missing_required_fields") {
			validation.Critical = uniqueSorted(append(validation.Critical, "missing_required_fields"))
		}
		validation.OK = len(validation.Critical) == 0
	}
	return IssueOpsTemplateResult{
		OK:                    validation.OK,
		Kind:                  input.Kind,
		Template:              input.Template,
		Provider:              input.Provider,
		Title:                 strings.TrimSpace(input.Title),
		Body:                  body,
		Warnings:              validation.Warnings,
		MissingRequiredFields: validation.MissingRequiredFields,
		Validation:            validation,
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func Validate(input IssueOpsTemplateInput) IssueOpsTemplateValidation {
	input = normalizeInput(input)
	v := IssueOpsTemplateValidation{OK: true, Critical: []string{}, Warnings: []string{}, MissingRequiredFields: []string{}}
	if strings.TrimSpace(input.Title) == "" {
		v.MissingRequiredFields = append(v.MissingRequiredFields, "title")
	}
	v.MissingRequiredFields = append(v.MissingRequiredFields, missingFields(input)...)
	body := strings.TrimSpace(input.Body)
	if strings.Contains(strings.ToLower(body), "## plan link") || strings.Contains(strings.ToLower(body), "## plan") {
		v.Critical = append(v.Critical, "plan_link_section_forbidden")
	}
	if strings.EqualFold(input.Provider, "gitlab") && strings.Contains(strings.ToLower(body), "## related issues") {
		v.Critical = append(v.Critical, "gitlab_related_issues_body_section_forbidden")
	}
	if body != "" && !containsHangul(body) {
		v.Critical = append(v.Critical, "korean_body_required")
	}
	if len(v.MissingRequiredFields) > 0 {
		v.Critical = append(v.Critical, "missing_required_fields")
	}
	if input.Provider == "" {
		v.Warnings = append(v.Warnings, "provider_not_set")
	}
	v.MissingRequiredFields = uniqueSorted(v.MissingRequiredFields)
	v.Critical = uniqueSorted(v.Critical)
	v.Warnings = uniqueSorted(v.Warnings)
	v.OK = len(v.Critical) == 0
	return v
}

func normalizeInput(input IssueOpsTemplateInput) IssueOpsTemplateInput {
	input.Kind = IssueOpsArtifactKind(strings.ToLower(strings.TrimSpace(string(input.Kind))))
	input.Template = IssueOpsTemplateKind(strings.ToLower(strings.TrimSpace(string(input.Template))))
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if input.Fields == nil {
		input.Fields = map[string]string{}
	}
	input.Fields = normalizeFields(input.Kind, input.Fields)
	if input.Kind == "" {
		input.Kind = IssueOpsArtifactIssue
	}
	if input.Template == "" {
		switch input.Kind {
		case IssueOpsArtifactChild:
			input.Template = IssueOpsTemplateChildTask
		case IssueOpsArtifactPR:
			input.Template = IssueOpsTemplatePullRequest
		default:
			input.Template = IssueOpsTemplateFeature
		}
	}
	return input
}

func normalizeFields(kind IssueOpsArtifactKind, fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(fields))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		normalized := normalizeFieldKey(key)
		if canonical, ok := fieldAliases[normalized]; ok {
			normalized = canonical
		}
		if kind == IssueOpsArtifactPR {
			if canonical, ok := prFieldAliases[normalized]; ok {
				normalized = canonical
			}
		}
		if strings.TrimSpace(out[normalized]) != "" && strings.TrimSpace(fields[key]) == "" {
			continue
		}
		out[normalized] = fields[key]
	}
	return out
}

func normalizeFieldKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	return key
}

var fieldAliases = map[string]string{
	"goal":         "task_goal",
	"logs_output":  "logs",
	"parent_merge": "merge_condition",
}

var prFieldAliases = map[string]string{
	"automation":    "automation_evidence",
	"cleanup":       "worktree_cleanup",
	"docs":          "docs_migration",
	"document":      "docs_migration",
	"documents":     "docs_migration",
	"documentation": "docs_migration",
	"risk":          "risk_rollback",
	"risks":         "risk_rollback",
	"rollback":      "risk_rollback",
	"scope":         "scope_management",
}

func renderBody(input IssueOpsTemplateInput) string {
	switch input.Kind {
	case IssueOpsArtifactChild:
		return renderSections([]section{
			{"부모 이슈", field(input, "parent_issue")},
			{"작업 목표", field(input, "task_goal")},
			{"완료 기준", field(input, "acceptance")},
			{"비목표", field(input, "non_goals")},
			{"검증", field(input, "verification")},
			{"부모 브랜치 병합 조건", field(input, "merge_condition")},
			{"child-only cleanup 규칙", field(input, "cleanup")},
		})
	case IssueOpsArtifactPR:
		return renderSections([]section{
			{"의도", field(input, "intent")},
			{"이슈", field(input, "issue")},
			{"변경 사항", field(input, "changes")},
			{"검증", field(input, "verification")},
			{"리뷰어 초점", field(input, "reviewer_focus")},
			{"위험/rollback", field(input, "risk_rollback")},
			{"사용자 영향/릴리즈 노트", field(input, "user_impact")},
			{"문서/마이그레이션", field(input, "docs_migration")},
			{"범위 관리", field(input, "scope_management")},
			{"워크트리 정리", field(input, "worktree_cleanup")},
			{"자동화/AI 개입 근거", field(input, "automation_evidence")},
		})
	default:
		sections := []section{
			{"문제", field(input, "problem")},
			{"현재 근거", field(input, "current_evidence")},
		}
		if input.Template == IssueOpsTemplateBug {
			sections = append(sections,
				section{"재현 절차", field(input, "reproduction_steps")},
				section{"기대 동작", field(input, "expected_behavior")},
				section{"실제 동작", field(input, "actual_behavior")},
				section{"환경", field(input, "environment")},
				section{"로그/출력", field(input, "logs")},
			)
		}
		sections = append(sections,
			section{"관련 이슈/라벨 판단", scoreSummary(input)},
			section{"완료 기준", field(input, "acceptance_criteria")},
			section{"비목표", field(input, "non_goals")},
			section{"구현 범위", field(input, "implementation_scope")},
			section{"검증", field(input, "verification")},
			section{"위험과 트레이드오프", field(input, "risks")},
			section{"피드백 기록", field(input, "feedback_log")},
		)
		return renderSections(sections)
	}
}

type section struct {
	title string
	body  string
}

func renderSections(sections []section) string {
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(section.title)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(section.body))
	}
	return strings.TrimSpace(b.String())
}

func field(input IssueOpsTemplateInput, key string) string {
	return strings.TrimSpace(input.Fields[key])
}

func scoreSummary(input IssueOpsTemplateInput) string {
	if s := strings.TrimSpace(input.ScoreSummary); s != "" {
		return s
	}
	if s := field(input, "score_summary"); s != "" {
		return s
	}
	return "선택/거절한 관련 이슈와 라벨, threshold, 수동 override 여부를 기록한다."
}

func missingFields(input IssueOpsTemplateInput) []string {
	required := requiredFields(input)
	missing := []string{}
	for _, key := range required {
		if strings.TrimSpace(input.Fields[key]) == "" && !fieldSatisfiedByBody(input.Body, key) {
			missing = append(missing, key)
		}
	}
	return missing
}

func requiredFields(input IssueOpsTemplateInput) []string {
	switch input.Kind {
	case IssueOpsArtifactChild:
		return []string{"parent_issue", "task_goal", "acceptance", "non_goals", "verification", "merge_condition", "cleanup"}
	case IssueOpsArtifactPR:
		return []string{"intent", "issue", "changes", "verification", "reviewer_focus", "risk_rollback", "user_impact", "docs_migration", "scope_management", "worktree_cleanup", "automation_evidence"}
	default:
		fields := []string{"problem", "current_evidence", "acceptance_criteria", "non_goals", "implementation_scope", "verification", "risks", "feedback_log"}
		if input.Template == IssueOpsTemplateBug {
			fields = append(fields, "reproduction_steps", "expected_behavior", "actual_behavior", "environment", "logs")
		}
		return fields
	}
}

func fieldSatisfiedByBody(body, key string) bool {
	if strings.TrimSpace(body) == "" {
		return false
	}
	heading := map[string][]string{
		"problem":              {"## 문제", "## Problem"},
		"current_evidence":     {"## 현재 근거", "## Current Evidence"},
		"acceptance_criteria":  {"## 완료 기준", "## Acceptance Criteria"},
		"non_goals":            {"## 비목표", "## Non-goals"},
		"implementation_scope": {"## 구현 범위"},
		"verification":         {"## 검증", "## Verification"},
		"risks":                {"## 위험", "## Risks", "## 위험과 트레이드오프"},
		"feedback_log":         {"## 피드백 기록", "## Feedback Log"},
	}
	for _, want := range heading[key] {
		if strings.Contains(body, want) {
			return true
		}
	}
	return false
}

func containsHangul(s string) bool {
	for _, r := range s {
		if r >= '가' && r <= '힣' {
			return true
		}
	}
	return false
}

func uniqueSorted(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func ParseFieldAssignments(values []string) (map[string]string, error) {
	fields := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("field must be key=value: %s", value)
		}
		fields[key] = strings.TrimSpace(val)
	}
	return fields, nil
}
