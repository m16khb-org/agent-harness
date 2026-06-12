package hookprompt

import "strings"

type HookRoutingRule struct {
	Tool            string
	Reason          string
	Priority        string
	LowerKeywords   []string
	PromptKeywords  []string
	RequireAgyOptIn bool
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
		Reason:         "For VCS remote work, use an authenticated CLI token first; if the token is unavailable or the CLI returns auth/permission errors, use the configured MCP fallback. Do not print tokens.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"github", "gitlab", "gh ", "glab", "pull request", "merge request", " pr", " mr", "issue"},
		PromptKeywords: []string{"PR", "MR", "이슈", "깃허브", "깃랩", "머지리퀘스트", "풀리퀘스트"},
	},
	{
		Tool:           "project_docs_record",
		Reason:         "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"architecture", "architect", "refactor", "design", "decision", "alternative"},
		PromptKeywords: []string{"아키텍처", "리팩터", "결정", "대안", "설계"},
	},
	{
		Tool:           "project_docs_record",
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
		Tool:           "project_docs_read/project_docs_update",
		Reason:         "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "project_docs_read/project_docs_update",
		Reason:         "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.",
		Priority:       PriorityAction,
		LowerKeywords:  []string{".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs", "project rules"},
		PromptKeywords: []string{"문서", "컨벤션", "최신화", "프로젝트 규칙"},
	},
	{
		Tool:          "CodeGraph",
		Reason:        "Secondary hint: use CodeGraph for repo-local symbol, call graph, impact, or trace questions; keep rg for exact strings, env keys, errors, and filenames.",
		Priority:      PrioritySecondary,
		LowerKeywords: []string{"codegraph", "symbol", "call graph", "impact", "trace", "caller", "callee"},
	},
	{
		Tool:          "LLM Wiki",
		Reason:        "Secondary hint: consider upstream LLM Wiki for explicit wiki, research, knowledge-base, query, or compile workflows.",
		Priority:      PrioritySecondary,
		LowerKeywords: []string{"llm-wiki", "wiki", "knowledge base", "research", "compile"},
	},
	{
		Tool:           "claude-mem",
		Reason:         "Secondary hint: consider claude-mem for previous-session memory or repeated-work questions.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"claude-mem", "agentmemory", "agent-memory", "memory", "previous session", "last time", "already solve", "already solved"},
		PromptKeywords: []string{"전에", "지난번", "이미 해결"},
	},
	{
		Tool:            "agy -p",
		Reason:          "Secondary hint: consider agy -p for foreground second-pass LLM review or background synthesis when extra model judgment is useful.",
		Priority:        PrioritySecondary,
		LowerKeywords:   []string{"review", "analyze", "analysis", "critique", "second opinion", "plan", "research"},
		PromptKeywords:  []string{"검토", "리뷰", "분석", "비평", "계획", "리서치", "조사"},
		RequireAgyOptIn: true,
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
		Reason:         "Secondary hint: use the codd skill for schema/index/query design — normalization, write-penalty trade-offs, query plans.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"schema", "index design", "query plan", "normalization", "slow query", "n+1"},
		PromptKeywords: []string{"스키마", "인덱스", "쿼리", "정규화"},
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
		Tool:           "shannon",
		Reason:         "Secondary hint: use the shannon skill for quantitative code-quality measurement — SNR/entropy/redundancy before and after cleanup.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"snr", "code quality measure", "quality baseline", "slop"},
		PromptKeywords: []string{"품질 측정", "슬롭", "품질 비교"},
	},
	{
		Tool:           "karpathy",
		Reason:         "Secondary hint: use the karpathy skill for prompt design and optimization — contracts, eval cases, adversarial tests.",
		Priority:       PrioritySecondary,
		LowerKeywords:  []string{"prompt engineering", "optimize this prompt", "system prompt", "prompt injection"},
		PromptKeywords: []string{"프롬프트"},
	},
}

var RoutingRules = hookRoutingRules

func (rule HookRoutingRule) matches(prompt, lower string, enableAgyHints bool) bool {
	if rule.RequireAgyOptIn && !enableAgyHints {
		return false
	}
	return containsAnySlice(lower, rule.LowerKeywords) || containsAnySlice(prompt, rule.PromptKeywords)
}

func (rule HookRoutingRule) Matches(prompt, lower string, enableAgyHints bool) bool {
	return rule.matches(prompt, lower, enableAgyHints)
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
