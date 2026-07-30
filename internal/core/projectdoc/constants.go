package projectdoc

const ProjectDocsDir = ".agent-harness"

var requiredProjectDocNames = []string{
	"ARCHITECTURE.md",
	"CAUTIONS.md",
	"COMMIT_POLICY.md",
	"CONSTITUTION.md",
	"CONVENTIONS.md",
	"TECH_STACK.md",
	"TESTING.md",
	"OPEN_API_SPEC.md",
	"ADR.md",
	"OPERATIONS.md",
	"AGENT_WORKFLOW.md",
}

var optionalProjectDocNames = []string{"VCS.md"}

const AgentsStartMarker = "<!-- AGENT_HARNESS:START -->"

const AgentsEndMarker = "<!-- AGENT_HARNESS:END -->"

const BehavioralGuidelines = `# AGENTS.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
~~~text
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
~~~

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
`

const SolidDesignPatternGuidance = `## SOLID / Design Pattern guidance

Apply SOLID, YAGNI, and KISS together. SOLID does not mean adding interfaces and layers by default; it means clarifying responsibility and dependency direction only where a real axis of change exists. Use a design pattern only when the name explains the problem and reduces maintenance cost.

### Good cases

- Apply existing patterns such as Adapter, Strategy, Factory, or Repository consistently to the same kind of problem.
- Put interfaces/ports at boundaries with real substitutability, such as external hosts, SDKs, filesystems, processes, or networks.
- Use dependency inversion where there are multiple implementations or a test double is needed.
- When introducing a pattern, record the problem, chosen pattern, rejected simpler alternatives, and cost in ADR.md.

### Bad cases

- Creating an interface, factory, registry, or plugin layer for a single use site.
- Adding abstraction or configurability based only on hypothetical future extension.
- Expanding a simple function call into a class/object graph just to match a pattern name.
- Duplicating core policy in host adapters or per-host implementations in the name of SOLID.

### Rules

- Start with the simplest implementation; introduce patterns only after a real variation point is confirmed.
- Add a new abstraction only when there are at least two use sites, a clear test boundary, or an external technology boundary.
- If a pattern turns a 50-line solution into a 200-line structure, revert and simplify.
`

func ProjectDocNames() []string {
	return append([]string(nil), requiredProjectDocNames...)
}

func OptionalProjectDocNames() []string {
	return append([]string(nil), optionalProjectDocNames...)
}

func AllowedProjectDocNames() []string {
	out := ProjectDocNames()
	return append(out, optionalProjectDocNames...)
}
