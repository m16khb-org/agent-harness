package core

import "strings"

// docMetaDescriptions is the canonical, name-keyed metadata for standard project
// docs: a fixed one-line description of WHAT CATEGORY of information each doc
// holds (not a summary of its current content). Same doc name => same
// description in every repo, and it stays fixed across bootstrap and
// bootstrap --sync. It is rendered as SKILL.md-style YAML frontmatter at the top
// of each doc so both humans and the project-doc catalog read the same source.
var docMetaDescriptions = map[string]string{
	"ARCHITECTURE.md":   "시스템의 구조, 컴포넌트 경계, 책임 분담을 담는다. 무엇이 어디에 속하고 왜 그렇게 나뉘는지 알 수 있다.",
	"ADR.md":            "프로젝트의 구조적 결정과 그 근거를 담는다. 어떤 선택을 왜 했고 무엇을 기각했는지 알 수 있다.",
	"CONSTITUTION.md":   "문서 우선순위와 안전·정확성 같은 최상위 원칙을 담는다. 지시가 충돌할 때 무엇을 따라야 하는지 알 수 있다.",
	"CONVENTIONS.md":    "코드 구현 규약과 패키지·레이어 경계를 담는다. 코드를 어떤 구조·명명·패턴으로 작성해야 하는지 알 수 있다.",
	"TECH_STACK.md":     "선택한 언어·런타임·도구와 그 이유를 담는다. 어떤 기술을 쓰고 왜 골랐는지 알 수 있다.",
	"TESTING.md":        "검증 기준과 테스트 작성·실행 규칙을 담는다. 변경을 어떻게 검증하고 무엇을 통과시켜야 하는지 알 수 있다.",
	"COMMIT_POLICY.md":  "커밋 메시지 규칙과 형식을 담는다. 커밋을 어떤 형식·단위로 작성해야 하는지 알 수 있다.",
	"CAUTIONS.md":       "반복되는 실수와 운영상 주의점을 담는다. 과거에 무엇이 잘못됐고 어떻게 피하는지 알 수 있다.",
	"OPERATIONS.md":     "설치·실행·운영 절차를 담는다. 하네스와 도구를 어떻게 설치·동기화·실행하는지 알 수 있다.",
	"OPEN_API_SPEC.md":  "엔드포인트·DTO·OpenAPI 문서화 게이트 규칙을 담는다. API 변경 시 무엇을 문서화하고 어떻게 검사하는지 알 수 있다.",
	"AGENT_WORKFLOW.md": "에이전트의 시작·작업·검증·완료 흐름을 담는다. 작업을 어떤 단계로 진행하고 마무리하는지 알 수 있다.",
}

// DocMetaDescription returns the canonical metadata description for a standard
// project doc filename, and whether one exists.
func DocMetaDescription(name string) (string, bool) {
	desc, ok := docMetaDescriptions[name]
	return desc, ok
}

// parseDocFrontmatter extracts a leading SKILL.md-style frontmatter block
// (--- ... ---) from the very top of content. It returns the name and
// description fields, the body after the block, and whether a block was present.
func parseDocFrontmatter(content string) (name, description, body string, ok bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", content, false
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return "", "", content, false
	}
	for _, line := range lines[1:closeIdx] {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.TrimSpace(value)
		case "description":
			description = strings.TrimSpace(value)
		}
	}
	body = strings.TrimLeft(strings.Join(lines[closeIdx+1:], "\n"), "\n")
	return name, description, body, true
}

// renderDocMetaFrontmatter renders the canonical frontmatter block for a doc.
func renderDocMetaFrontmatter(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n"
}

// ensureDocMetaFrontmatter guarantees content begins with the canonical meta
// frontmatter for the given doc name while preserving the existing body. An
// existing frontmatter block is replaced; otherwise one is prepended. Content is
// returned unchanged when the doc has no canonical metadata. The operation is
// idempotent: applying it twice yields identical output.
func ensureDocMetaFrontmatter(name, content string) string {
	desc, ok := DocMetaDescription(name)
	if !ok {
		return content
	}
	_, _, body, hadFrontmatter := parseDocFrontmatter(content)
	if !hadFrontmatter {
		body = content
	}
	block := renderDocMetaFrontmatter(name, desc)
	if strings.TrimSpace(body) == "" {
		return block
	}
	return block + "\n" + strings.TrimLeft(body, "\n")
}
