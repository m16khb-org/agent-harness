package hookprompt

import "strings"

type HookRoutingRule struct {
	Tool            string
	Reason          string
	Priority        string
	LowerKeywords   []string
	PromptKeywords  []string
	RequireLLMOptIn bool
}

var hookRoutingRules = []HookRoutingRule{
	{
		Tool:           "issueops",
		Reason:         "Use the issue-driven workflow for problem intake -> domain grill -> issue -> plan -> TDD/subagents -> ai-slop-clean -> feedback -> PR/MR; hooks must not create issues or PRs.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"issueops", "issue-driven", "feedback loop", "pull request", "merge request"},
		PromptKeywords: []string{"문제 파악", "이슈 기반", "피드백 루프", "PR", "MR", "이슈"},
	},
	{
		Tool:           "vcs_remote_auth",
		Reason:         "For VCS remote work, pick the authenticated surface (CLI token or configured MCP server) whose credentials match the target host/project; on missing token or auth/permission errors switch to the other configured surface. Do not print tokens.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"github", "gitlab", "gh ", "glab", "pull request", "merge request", " pr", " mr", "issue"},
		PromptKeywords: []string{"PR", "MR", "이슈", "깃허브", "깃랩", "머지리퀘스트", "풀리퀘스트"},
	},
	{
		Tool:           "gitlab-usecase",
		Reason:         "Required for GitLab/glab/GitLab MCP/IssueOps remote work; distinguish linked items from child items, verify issue body, labels, assignee, target branch, and review-thread state.",
		Priority:       PriorityRequired,
		LowerKeywords:  []string{"gitlab", "glab", "gitlab mcp", "merge request", "linked item", "linked items", "child item", "child items", "work item", "work items", "kody", "kodus"},
		PromptKeywords: []string{"깃랩", "머지리퀘스트", "링크드", "하위 Task", "하위 태스크", "자식", "상위", "부모", "Kody", "Kodus"},
	},
	{
		Tool:           "aside-cli",
		Reason:         "When the task needs a real browser (page inspection, screenshots, web UI QA, E2E flows), prefer the installed Aside CLI: check availability with `aside --version` and drive pages through aside before reaching for other browser automation; fall back only when Aside cannot handle the surface.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"browser", "playwright", "puppeteer", "screenshot", "web page", "webpage", "headless chrome", "ui test", "e2e test"},
		PromptKeywords: []string{"브라우저", "스크린샷", "웹페이지", "웹 페이지", "화면 확인", "UI 확인", "E2E"},
	},
	{
		Tool:           "project_docs_append",
		Reason:         "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"architecture", "architect", "refactor", "design", "decision", "alternative"},
		PromptKeywords: []string{"아키텍처", "리팩터", "결정", "대안", "설계"},
	},
	{
		Tool:           "project_docs_append",
		Reason:         "When a resolved false case or recurring failure is reusable, consider kind=caution for CAUTIONS.md.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"bug", "fix", "regression", "failure", "false case", "caution"},
		PromptKeywords: []string{"버그", "고쳐", "회귀", "실패", "주의"},
	},
	{
		Tool:           "api_doc_static_check",
		Reason:         "For API/endpoint/DTO/OpenAPI changes, consider deterministic Swagger/OpenAPI gap checks before implementation or review.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "api_doc_review",
		Reason:         "Use agent review to compare business-logic error paths such as 400/401/403/404/409 with the documented API contract.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "project_docs_read/project_docs_revise",
		Reason:         "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "project_docs_read/project_docs_revise",
		Reason:         "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs", "project rules"},
		PromptKeywords: []string{"문서", "컨벤션", "최신화", "프로젝트 규칙"},
	},
	{
		Tool:            "host-agent judgement",
		Reason:          "Secondary hint: render a prompt for foreground second-pass review or background synthesis when extra host-agent judgment is useful.",
		Priority:        PrioritySecondary,
		LowerKeywords:   []string{"review", "analyze", "analysis", "critique", "second opinion", "plan", "research"},
		PromptKeywords:  []string{"검토", "리뷰", "분석", "비평", "계획", "리서치", "조사"},
		RequireLLMOptIn: true,
	},
	// CS pioneer skill routing (issue #10): keyword hints so the matching
	// specialist skill surfaces on ordinary, non-issueops requests too.
	{
		Tool:           "berners-lee",
		Reason:         "Secondary hint: use the berners-lee skill for web research — multi-angle searches, source cross-checking, cited reports.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"web research", "web search", "cite sources", "cross-reference", "competitive analysis", "literature survey"},
		PromptKeywords: []string{"웹에서", "자료 조사", "조사해서", "출처", "검색해서", "웹 검색"},
	},
	{
		Tool:           "hopper",
		Reason:         "Secondary hint: use the hopper skill for systematic debugging — reproduce, isolate, root-cause, verified fix.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"debug", "diagnose", "flaky", "root cause", "why does this fail", "intermittent"},
		PromptKeywords: []string{"디버그", "원인 찾", "왜 실패", "왜 깨지", "간헐적"},
	},
	{
		Tool:           "dijkstra",
		Reason:         "Secondary hint: use the dijkstra skill for algorithmic optimization — profile first, complexity class, scaling evidence.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"optimize", "optimization", "time complexity", "space complexity", "too slow", "performance bottleneck"},
		PromptKeywords: []string{"최적화", "복잡도", "느려", "성능 개선"},
	},
	{
		Tool:           "codd",
		Reason:         "Secondary hint: use the codd skill for schema/index/query design and concurrency — normalization, write-penalty trade-offs, query plans, locks/deadlocks/isolation.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"schema", "index design", "query plan", "normalization", "slow query", "n+1", "deadlock", "lock contention", "isolation level", "transaction"},
		PromptKeywords: []string{"스키마", "인덱스", "쿼리", "정규화", "데드락", "락", "트랜잭션", "격리수준"},
	},
	{
		Tool:           "torvalds",
		Reason:         "Secondary hint: use the torvalds skill for advanced git — rebase, bisect, conflict resolution, history recovery.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"rebase", "bisect", "cherry-pick", "reflog", "merge conflict"},
		PromptKeywords: []string{"리베이스", "충돌 났", "충돌 해결", "히스토리 복구"},
	},
	{
		Tool:           "atomic-commit-push",
		Reason:         "Secondary hint: use the atomic-commit-push skill for safe staged commits and push — exact paths, atomic intents, secret blockers.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"commit and push", "atomic commit", "split commits", "stage and commit"},
		PromptKeywords: []string{"커밋하고", "커밋해", "푸시해", "커밋 분리"},
	},
	{
		Tool:           "von-neumann",
		Reason:         "Secondary hint: use the von-neumann skill for decision-complete planning when scope is broad or multi-module.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"work plan", "implementation plan", "decision-complete", "blueprint"},
		PromptKeywords: []string{"계획 세워", "플랜 짜", "설계해줘", "계획해줘"},
	},
	{
		Tool:           "turing",
		Reason:         "Secondary hint: use the turing skill for evidence-bound execution with measurable criteria and cleanup receipts.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"evidence-bound", "cleanup receipt", "success criteria and evidence"},
		PromptKeywords: []string{"완료 기준별 증거", "증거와 완료 기준", "정리 영수증"},
	},
	{
		Tool:           "boehm",
		Reason:         "Secondary hint: use the boehm skill for risk-driven planning-document, OCR, table, and screenshot analysis.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"kordoc", "ocr uncertainty", "planning document screenshot", "planning document table"},
		PromptKeywords: []string{"기획 문서의 스크린샷", "기획 문서 캡처", "기획 문서 표", "OCR 불확실성"},
	},
	{
		Tool:           "engelbart",
		Reason:         "Secondary hint: use the engelbart skill to turn meeting transcripts into durable team memory and Canvas-ready handoffs.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"meeting transcript", "meeting minutes", "slack canvas handoff"},
		PromptKeywords: []string{"회의 전사본", "회의 녹취", "회의록과 Slack Canvas", "회의록 만들어"},
	},
	{
		Tool:           "shannon",
		Reason:         "Secondary hint: use the shannon skill for quantitative code-quality measurement — SNR/entropy/redundancy before and after cleanup.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"snr", "code quality measure", "quality baseline", "slop"},
		PromptKeywords: []string{"품질 측정", "슬롭", "품질 비교"},
	},
	{
		Tool:           "brooks",
		Reason:         "Secondary hint: dispatch the brooks skill as a SUB-AGENT (sub-agent-only, never inline) for adversarial design/plan review — separate essential from accidental complexity, defend conceptual integrity, expose the second-system effect before code is written.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"devil's advocate", "devils advocate", "over-engineered", "over engineered", "second-system", "conceptual integrity", "stress-test the plan", "review this design before"},
		PromptKeywords: []string{"과설계", "계획 검토", "악마의 변호인", "설계 검토"},
	},
	{
		Tool:           "karpathy",
		Reason:         "Augment prompts with the karpathy skill — prompt quality directly drives output quality. Harden every authored prompt before dispatch: plan-generation, sub-agent dispatch (incl. devil's-advocate reviewers), and reviewer prompts. In IssueOps, run karpathy on the prompt before spawning any sub-agent.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"prompt engineering", "optimize this prompt", "system prompt", "prompt injection", "issueops", "sub-agent", "subagent", "dispatch prompt", "agent prompt", "reviewer prompt", "plan-generation prompt", "implement", "refactor", "analyze", "investigate"},
		PromptKeywords: []string{"프롬프트", "서브에이전트", "프롬프트 증강", "에이전트 프롬프트", "구현해", "만들어줘", "분석해줘", "리팩토링", "개선해", "수정해"},
	},
}

var RoutingRules = hookRoutingRules

func (rule HookRoutingRule) matches(prompt, lower string, enableLLMHints bool) bool {
	if rule.RequireLLMOptIn && !enableLLMHints {
		return false
	}
	return containsAnySlice(lower, rule.LowerKeywords) || containsAnySlice(prompt, rule.PromptKeywords)
}

func (rule HookRoutingRule) Matches(prompt, lower string, enableLLMHints bool) bool {
	return rule.matches(prompt, lower, enableLLMHints)
}

func containsAnySlice(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func ContainsAnySlice(s string, needles []string) bool {
	return containsAnySlice(s, needles)
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func ContainsAny(s string, needles ...string) bool {
	return containsAny(s, needles...)
}
