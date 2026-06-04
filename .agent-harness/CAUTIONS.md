---
name: CAUTIONS.md
description: Recurring mistakes, operational cautions, and avoidance guidance.
---

# 주의사항 모음

`agent-harness`에서 반복적으로 실수하기 쉬운 설계·운영 주의사항을 모은다.

---

## 1. Host-specific lock-in

Codex plugin 또는 Claude Code hook에 핵심 로직을 넣으면 다른 host에서 같은 동작을 재사용할 수 없다.

주의:
- core behavior는 Go core에 둔다.
- plugin/skill/slash command/hook은 CLI/MCP 호출 wrapper로 제한한다.
- host adapter가 늘어날수록 contract test로 결과 동일성을 확인한다.

---

## 2. Plugin-only 착각

plugin 방식은 설치 UX에는 좋지만, Codex와 Claude Code가 같은 plugin runtime을 공유하지 않는다.

주의:
- plugin은 배포/발견/문서화 layer로 본다.
- 장기 상태, command policy, audit log는 외부 core/worker가 담당한다.

---

## 3. 위험한 shell 실행

에이전트 하네스에서 shell runner는 가장 위험한 기능이다.

주의:
- argv 실행을 기본으로 하고 shell string 실행은 예외로 둔다.
- cwd, timeout, env, write/network 허용 여부를 명시한다.
- stdout/stderr는 redaction 후 저장/반환한다.
- workspace root 밖 파일 접근을 기본 거부한다. `cwd`뿐 아니라 path-like argv(`../`, `/abs/path`, `--flag=/abs/path`, `~/path`, symlink escape)도 경계 검사를 통과해야 한다.

---

## 4. Secret leakage

agent prompt, logs, MCP responses, test failures에 secret이 쉽게 섞일 수 있다.

주의:
- token/key/password-like pattern은 adapter 경계에서 마스킹한다.
- fixture secret은 실제 값을 쓰지 않는다.
- command echo와 verbose log를 기본 비활성화한다.

---

## 5. Worker lifecycle 문제

persistent worker는 편하지만 stale lock, orphan process, socket 권한, 오래된 binary 문제가 생긴다.

주의:
- MVP에서는 CLI/MCP one-shot을 먼저 안정화한다.
- worker 도입 시 health check, version handshake, graceful shutdown, stale lock cleanup을 구현한다.
- socket path와 permission을 문서화하고 테스트한다.

---

## 6. State 위치 혼동

프로젝트 지식과 런타임 state가 섞이면 repo가 오염되고 secret이 커밋될 수 있다.

주의:
- 추적할 지식은 `.agent-harness/`에 둔다.
- cache/log/runtime state는 user state dir 또는 ignored `.harness/`에 둔다.
- `.harness/`를 도입하면 `.gitignore`에 추가한다.

---

## 7. MCP schema drift

CLI와 MCP가 서로 다른 응답 의미를 갖기 시작하면 host별 동작이 갈라진다.

주의:
- CLI JSON과 MCP response는 같은 core DTO를 공유한다.
- schema 변경은 golden test와 migration note를 남긴다.
- tool 이름과 field 이름은 안정적으로 유지한다.

---

## 8. Shared skill drift

Codex용 skill과 Claude용 skill을 복사본으로 따로 두면 금방 내용이 갈라진다.

주의:
- `skills/<name>`을 원본으로 둔다.
- 기본 설치는 `~/.codex/skills/<name>`과 `~/.claude/skills/<name>`만 중앙 원본으로 연결한다.
- `.claude/skills/<name>` 같은 repo-local 연결은 적용 대상 repo에 커밋될 수 있으므로 명시적 project-local 모드에서만 만든다.
- 스킬 수정 후 user-level host 경로가 같은 원본을 가리키는지 확인한다.

---

## 9. 자기 검증/자가 증강 drift

자기 검증 루프가 실제 native integration과 QA gate를 검증하지 않으면 문서만 통과하는 가짜 안정성이 생긴다. 자가 증강 루프가 실제 diff를 만들지 않으면 단순 분석 루프로 퇴화한다.

주의:
- 새 CLI/MCP/native skill 기능은 `agent-harness self-verify`의 테스트 또는 QA 단계에 smoke/fuzz evidence label로 승격한다.
- 반복 횟수 10회 하한을 임의로 낮추지 않는다.
- temp git repo 외 실제 사용자 repo에서 commit/push를 수행하지 않는다.

---

## 10. 과도한 초기 추상화

처음부터 remote server, distributed queue, plugin marketplace packaging을 만들면 개인 하네스 MVP가 늦어진다.

주의:
- 1단계는 `agent-harness inspect`와 state/checkpoint 같은 작은 기능으로 시작한다.
- 반복 사용으로 필요가 확인된 기능만 worker/plugin layer로 승격한다.

---

## 11. LLM Wiki 재구현 금지

`agent-harness`는 llm-wiki vault, 검색, capture, SessionStart 주입을 직접 구현하지 않는다. LLM Wiki 기능이 필요하면 upstream `nvk/llm-wiki` plugin/portable AGENTS.md를 설치해 사용한다. 하네스 MCP/CLI에는 llm-wiki 전용 tool/resource를 다시 추가하지 않는다.

같은 원칙으로 CodeGraph와 claude-mem도 하네스 core에 복제하지 않는다. 이 프로젝트의 철학은 **바퀴를 재발명하지 않는다**이며, `scripts/install-native.sh --with-upstream-tools`는 upstream installer/plugin을 호출하는 opt-in convenience path일 뿐이다. companion tool이 실패해도 하네스 core contract를 약화하거나 adapter에 임시 구현을 넣지 말고 upstream 설치/문서 경로를 고친다.

예외: Codex native hook validator가 upstream companion plugin의 오래된/Claude 전용 출력 필드만 거부하는 경우에는, 설치/업데이트 단계에서 **기능 재구현 없이** 호환성 shim을 적용할 수 있다. 예를 들어 `suppressOutput`처럼 Codex 0.135.0에서 unsupported top-level field로 실패하는 값은 백업 후 제거하되, `hookSpecificOutput`, MCP 등록, worker 시작, context 주입 동작은 유지한다.

draft-wiki는 이 예외가 아니라 별도 staging area다. `.agent-harness/draft-wiki/**`에는 사용자가 검토할 후보 Markdown만 둔다. `agent-harness project draft-wiki promote --confirm`은 configured `nvk/llm-wiki` topic의 `raw/<type>/` note와 `log.md` append까지만 수행한다. index/query/compile을 하네스가 대신 완료한 것처럼 보고하지 않는다.

PostToolUse hook 기반 draft-wiki 자동화는 hook에서 `agy`를 실행하지 않는다. hook은 bounded/redacted queue record만 user state에 append하고, `agent-harness worker draft-wiki`가 나중에 `agy -p`를 argv로 호출해 draft를 쓴다. hook stdout에는 host-compatible no-op shape를 유지하고, queue/draft 생성 여부는 raw `--json`, queue file, draft file, worker result로 검증한다.

## 12. Daemon lifecycle drift

`agent-harness mcp`가 daemon을 자동 시작하므로 오래된 binary가 이미 떠 있으면 새 코드 검증과 실제 MCP 동작이 갈라질 수 있다. `agent-harness update`와 `agent-harness bootstrap`은 실행 중인 daemon을 post-install 단계에서 재시작하지만, 수동 `go build`나 `install-native`만 실행한 경우에는 daemon이 그대로 남을 수 있다.

주의:
- 수동 설치/빌드 후 MCP smoke 전에는 필요하면 `agent-harness daemon stop --json`으로 기존 daemon을 내린다.
- 테스트는 `HARNESS_DAEMON_DIR=$(mktemp -d)/daemon`으로 실제 user daemon과 분리한다.
- daemon socket/pid/log는 user state dir에 두고 repo나 wiki vault에 쓰지 않는다.

## 13. Codex vs Claude Code hook rendering drift

Codex and Claude Code accept similar UserPromptSubmit JSON, but they do not render it the same way.

주의:
- Codex shows `hookSpecificOutput.additionalContext` in the TUI `hook context:` row, so anything injected for the model is also visible to the user and may be newline-collapsed.
- Claude Code can use `systemMessage` as the user-visible channel while keeping `additionalContext` as model-facing context.
- Do not assume a hook field is hidden just because another host hides it. Verify the installed host runtime/schema before changing hook output.
- For Codex, keep the project-doc catalog in `additionalContext` because the agent needs it, but avoid route/action/profile/pending-upkeep status prose there.
- Keep project-doc frontmatter descriptions concise English metadata; `project bootstrap` and `project bootstrap --sync` use this canonical metadata, so verbose descriptions multiply across every target repo.

## 14. IssueOps worktree edits must be hook-guarded

IssueOps worktree isolation cannot rely on the model remembering `pwd` or shell `workdir`. Some edit tools can apply relative paths from a different checkout than the shell command just verified.

주의:
- IssueOps sessions set `HARNESS_SOURCE_CHECKOUT` and `HARNESS_EXPECTED_WORKTREE` before implementation.
- Installed Codex and Claude PreToolUse hooks include `--enforce-worktree` and block mutating tool events outside the expected worktree when that env is set.
- Installed Codex and Claude PreToolUse hooks include `--enforce-gitops-kubectl`; direct mutating `kubectl` commands such as `apply`, `delete`, `patch`, or `rollout restart` must be represented as manifest changes in git and applied through the repo's GitOps path. Live-access commands such as `kubectl exec` and `kubectl port-forward` ask for explicit user confirmation because they expose live workloads or local ports. Read-only commands and dry-runs such as `kubectl get`, `logs`, `diff`, and `apply --dry-run=server` remain allowed.
- Installed Codex and Claude PreToolUse hooks include `--enforce-staged-checks`; broad Biome lint/format commands such as `biome check apps libs` or package scripts that expand to broad `apps libs` checks ask for explicit user confirmation. Prefer staged or changed-file commands such as `biome check --staged`, `biome format --staged`, lint-staged, or explicit changed file lists so existing repo debt does not become the current diff's failure.
- Manual edit rules still require absolute paths rooted at the expected worktree and status checks for both source checkout and worktree, because host hook coverage can differ by runtime.
- If a source checkout receives implementation edits by mistake, stop, move only your own changes into the IssueOps worktree, and verify the source checkout is clean before continuing.
- Late-promotion gap: when work starts under `/goal`, ralph, or ad-hoc edits and is only promoted to IssueOps at the issue/PR phase, the worktree-first trigger ("`$issueops` explicitly invoked") never fires, so implementation can land in the source checkout on a feature branch without an isolated worktree. Observed 2026-06-03: gap-closure work was committed directly on `feat/...` in the main checkout; main ref stayed clean but the isolation contract was skipped. Mitigation: when a cycle is promoted to IssueOps, either move remaining work into an isolated worktree or explicitly record the deviation. Implemented (issue #25): the `--enforce-worktree` PreToolUse guard judges by the current work's own cycle. IssueOps cycle ids are deterministic per (repo, branch) and `issueops start` resumes instead of duplicating, so cycles cannot accumulate as stale duplicates. The guard reads the current branch from `.git/HEAD`, loads only that branch's cycle (`ActiveIssueOpsCycleForBranch`), and when it is active and in a code-editing phase (implement/feedback/pr) blocks mutating edits whose target is not inside a `.worktrees/` path. Legacy timestamp-id records and other branches' cycles have different ids and are never read, so they cannot cause a false lock; marking the cycle `done` releases the source checkout.

## 15. IssueOps decision replies must have numbered choices

When the user must choose a route, cleanup action, feedback response, or next phase, free-form prose is too easy to miss. Prompt discipline alone is insufficient.

주의:
- Installed Codex and Claude Stop hooks include `--enforce-numbered-next-actions`; when the host exposes `last_assistant_message` or a transcript path, missing `1.`, `2.`, and `3.` choices are blocked.
- The Stop hook should explain the missing choices to the agent and instruct the agent to present context-specific next actions; it should not synthesize fixed choices itself.
- Keep the three choices concrete: recommended proceed, narrower/lower-risk alternative, and pause/defer.
- If the host does not expose the final assistant message to Stop hook input, the guard must no-op and record diagnostics rather than guessing.

## 16. MCP tool-use risks

- Broad tool descriptions make agents over-call tools or pass wrong arguments.
- Always injecting all project documents at session start wastes context and can hide task-specific evidence.
- Writable tools need explicit write semantics; prefer dry-run or append-only behavior.
- Tool output is evidence, not proof: verify file existence, warnings, and command/test results before claiming completion.

## 17. Audit harness flags must match the CLI contract

A stability-audit failure is not automatically a harness defect; the audit framework itself can call the CLI with invalid flags.

주의:
- `self-verify --iterations=N` requires `--full`; without it the CLI exits fast with "--iterations requires --full". Observed 2026-06-03: `e2e_stability_audit.py` invoked `self-verify --iterations=10` without `--full`, so the audit reported a false self-verify failure in ~150ms while a direct quick run passed 22/22.
- When an audit step fails suspiciously fast, reproduce the exact invocation directly and compare against the documented commands in `.agent-harness/OPERATIONS.md` / root `AGENTS.md` before concluding the harness is unstable.
- Give the full 10-iteration self-verify a generous timeout (>=180s); the final LLM gate plus 10 seeded iterations exceed the quick-mode budget.

## 18. Verify git identity before contributor-sensitive pushes

GitHub contributor attribution follows commit author/committer email, not just the displayed author name.

주의:
- Before committing or pushing contributor-sensitive history, run `git config --show-origin --get-regexp '^user\.'` and `git var GIT_AUTHOR_IDENT && git var GIT_COMMITTER_IDENT`.
- In this repo, `m16khb@bubbletap.com` maps to the unwanted `habinkim-bubbletap` contributor. Use `m16khb@gmail.com` or `43867832+m16khb@users.noreply.github.com` instead.
- If a tool may bypass repo-local config, set `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, and `GIT_COMMITTER_EMAIL` explicitly for the commit command.
- After push, verify `git log --all --format='%an <%ae> %cn <%ce>' | rg 'bubbletap'` is empty and check GitHub contributors when attribution matters.

## 19. Stop hook output: `continue:false` hard-stops; use `decision:block` + `reason` to continue in-turn

A Stop hook that wants the agent to *recover and keep going* (for example, to present the missing numbered choices) must NOT set `continue:false`. Doing so halts the agent and surfaces the reason to the user, instead of letting the agent act on it in-turn.

주의:
- Verified against host binaries. Claude `2.1.162` embedded hook docs: `continue` — "Set to `false` to block/stop (default: true)", `stopReason` — "Message shown when `continue` is false". Codex `0.137.0` `stop.command.output` schema: `continue` (default true), `decision` = `BlockDecisionWire(["block"])`, `reason` with the note "Claude requires `reason` when `decision` is `block`". Both hosts mirror the same schema.
- `continue:false` is a hard stop and takes precedence over `decision`. To drive an IN-TURN continuation, return `decision:"block"` + `reason` and leave `continue:true` (or omit it). `runHookStop`'s auto-proceed branch already did this; the `--enforce-numbered-next-actions` block branch wrongly sent `continue:false`, so the agent "just stopped" and the user had to prompt it manually (observed 2026-06-04, fixed in `cmd/harness/hook_user_prompt.go`).
- When the block branch uses `continue:true`, guard it with `stop_hook_active`: hosts set that flag true on a Stop that is itself a continuation of a prior stop-hook block. Allow the stop (no-op `{}` output) while it is true so a non-complying agent cannot loop forever.
- Stop hooks accept only the stop-control schema (`continue`/`decision`/`reason`/`stopReason`/`systemMessage`/`suppressOutput`). Injecting `hookSpecificOutput.additionalContext` on Stop makes Codex report "invalid stop hook JSON output"; use a no-op `{}` payload when not blocking.
- A non-blocking Stop hook's `systemMessage` is NOT surfaced to the user. Observed 2026-06-04: returning `{"systemMessage": ...}` with the turn allowed to end (no `decision`/`continue:false`) produced no visible notice — the user saw nothing. Only BLOCKING output reaches the user: `decision:"block"` + `reason` renders as "Stop hook feedback" (and continues the agent in-turn), and `stopReason` renders only when `continue:false`. The doc's "systemMessage — display to the user (all hooks)" overstates Stop behavior.
- Therefore there are THREE distinct Stop outcomes — pick by intent: (1) recover/continue in-turn → `continue:true` + `decision:"block"` + `reason`; (2) STOP the turn AND notify the user (e.g., auto-proceed gate not met, user must choose) → `continue:false` + `stopReason` (the only reliably-rendered stop+notify channel; also set `systemMessage` as a cross-host fallback); (3) silent no-op → `{}`. Implemented in `runHookStop`: the "auto-proceed not engaged" branch uses case (2). So `continue:false` is NOT universally wrong — it is wrong only when you wanted case (1) and right when you want case (2).

## Incident Archive

Dated incident notes are preserved in `.agent-harness/archive/cautions-incidents.md`. Keep this file focused on evergreen hazards and move one-off history there.
