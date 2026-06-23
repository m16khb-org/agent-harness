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
- **S2 (known limitation, accepted)**: state store는 per-key flock RMW(`StateUpdate`/`writeStateRecord`는 temp+rename으로 단일 key 원자성)만 보장하고 **cross-key atomic transaction은 없다**. 현재 2개 이상 key를 한 단위로 commit해야 하는 caller가 없어 의도된 한계다(journal/composite-record 미도입). 그런 invariant가 생기면 그때 도입한다.

---

## 7. MCP schema drift

CLI와 MCP가 서로 다른 응답 의미를 갖기 시작하면 host별 동작이 갈라진다.

주의:
- 새 CLI state command를 추가하면 MCP tool, response contract golden, usage golden을 함께 갱신한다.
- MCP tool이 직접 외부 provider를 호출하지 못하는 경우에도 agent-facing contract에 호출 순서와 실패 조건을 명시한다.
- CLI JSON과 MCP response는 같은 core DTO를 공유한다.
- schema 변경은 golden test와 migration note를 남긴다.
- tool 이름과 field 이름은 안정적으로 유지한다.

---

## 8. IssueOps local-only branch 착각

IssueOps 이슈 브랜치를 `git worktree add -b`로 바로 만들면 GitHub/GitLab 이슈 화면에 branch 생성 기록이 남지 않는다.

주의:
- 이슈 기반 worktree를 만들기 전에 provider-linked branch를 먼저 생성한다.
- IssueOps branch는 GitLab Development 섹션에 자동 연결되도록 issue/task number와 hyphen으로 시작한다. 예: `2386-remove-dmm-ranking-ranktype`, `2387-fix-grpc-ai-dmm-tag-replication-lag`.
- `feature/`, `hotfix/` 같은 branch prefix를 이슈 번호 앞에 붙이면 GitLab native branch linking이 동작하지 않는다.
- GitHub는 linked development branch 생성을 위해 `gh issue develop` 또는 노출된 GitHub MCP linked-branch tool을 사용한다.
- IssueOps branch prepare contract는 MCP 먼저, provider API/CLI fallback, 둘 다 실패하면 중단 순서다.

## 9. Shared skill drift

Codex용 skill과 Claude용 skill을 복사본으로 따로 두면 금방 내용이 갈라진다.

주의:
- `skills/<name>`을 원본으로 둔다.
- 기본 설치는 `~/.codex/skills/<name>`과 `~/.claude/skills/<name>`만 중앙 원본으로 연결한다.
- `.claude/skills/<name>` 같은 repo-local 연결은 적용 대상 repo에 커밋될 수 있으므로 명시적 project-local 모드에서만 만든다.
- 스킬 수정 후 user-level host 경로가 같은 원본을 가리키는지 확인한다.

---

## 10. 자기 검증/자가 증강 drift

자기 검증 루프가 실제 native integration과 QA gate를 검증하지 않으면 문서만 통과하는 가짜 안정성이 생긴다. 자가 증강 루프가 실제 diff를 만들지 않으면 단순 분석 루프로 퇴화한다.

주의:
- 새 CLI/MCP/native skill 기능은 `agent-harness self-verify`의 테스트 또는 QA 단계에 smoke/fuzz evidence label로 승격한다.
- 반복 횟수 10회 하한을 임의로 낮추지 않는다.
- temp git repo 외 실제 사용자 repo에서 commit/push를 수행하지 않는다.
- 교정 후보의 `VerifyWith`는 모델 자기비판이 아니라 외부 검증 메커니즘을 명시해야 한다(`VerificationKind`로 분류, `qualitycatalog.VerifyWithGrounded`가 강제). 불변식 전문은 `CONVENTIONS.md` §9 "self-augment/self-verify 교정 가드레일". intrinsic self-correction은 외부 신호 없이 추론을 악화시킨다(Huang/Kamoi).

---

## 11. 과도한 초기 추상화

처음부터 remote server, distributed queue, plugin marketplace packaging을 만들면 개인 하네스 MVP가 늦어진다.

주의:
- 1단계는 `agent-harness inspect`와 state/checkpoint 같은 작은 기능으로 시작한다.
- 반복 사용으로 필요가 확인된 기능만 worker/plugin layer로 승격한다.

---

## 12. LLM Wiki 재구현 금지

`agent-harness`는 llm-wiki vault, 검색, capture, SessionStart 주입을 직접 구현하지 않는다. LLM Wiki 기능이 필요하면 upstream `m16khb/llm-wiki` CLI/MCP 서버 또는 portable AGENTS.md를 설치해 사용한다. 하네스 MCP/CLI에는 llm-wiki 전용 tool/resource를 다시 추가하지 않는다.

같은 원칙으로 CodeGraph와 claude-mem도 하네스 core에 복제하지 않는다. 이 프로젝트의 철학은 **바퀴를 재발명하지 않는다**이며, `scripts/install-native.sh --with-upstream-tools`는 upstream installer/MCP/plugin 배선을 호출하는 opt-in convenience path일 뿐이다. companion tool이 실패해도 하네스 core contract를 약화하거나 adapter에 임시 구현을 넣지 말고 upstream 설치/문서 경로를 고친다.

예외: Codex native hook validator가 upstream companion plugin의 오래된/Claude 전용 출력 필드만 거부하거나, companion plugin의 lifecycle hook이 Codex critical path에서 병렬 실행 race로 사용자 작업을 막는 경우에는, 설치/업데이트 단계에서 **기능 재구현 없이** 호환성 shim을 적용할 수 있다. 예를 들어 `suppressOutput`처럼 Codex 0.135.0에서 unsupported top-level field로 실패하는 값은 백업 후 제거하되, `hookSpecificOutput`, MCP 등록, worker 시작, context 주입 동작은 유지한다. `llm_wiki_session.py` 같은 companion session hook을 패치할 때도 vault/query/capture 의미를 하네스에 복제하지 말고, atomic file write와 fail-open 같은 host compatibility 경계만 idempotent하게 고친다.

draft-wiki는 이 예외가 아니라 별도 staging area다. `.agent-harness/draft-wiki/**`에는 사용자가 검토할 후보 Markdown만 둔다. `agent-harness project draft-wiki promote --confirm`은 configured `m16khb/llm-wiki` topic의 `raw/<type>/` note와 `log.md` append까지만 수행한다. validate/lint/index/query-pack을 하네스가 대신 완료한 것처럼 보고하지 않는다.

draft-wiki queue는 hook 휴리스틱이 자동 생성하지 않는다. UserPromptSubmit은 메인 에이전트에게 장기 재사용 가치 판단 책임과 명시 queue 명령만 알려주고, 메인 에이전트가 의미 있는 후보라고 판단한 경우에만 `agent-harness project draft-wiki queue --stdin`(heredoc 권장) 또는 `--input`으로 적재한다. `agent-harness worker draft-wiki`가 나중에 `agy -p`를 argv로 호출해 draft를 쓴다. hook stdout에는 host-compatible no-op shape를 유지하고, queue/draft 생성 여부는 명시 queue command, queue file, draft file, worker result로 검증한다.

## 13. Daemon lifecycle drift

`agent-harness mcp`가 daemon을 자동 시작하므로 오래된 binary가 이미 떠 있으면 새 코드 검증과 실제 MCP 동작이 갈라질 수 있다. `agent-harness update`와 `agent-harness bootstrap`은 실행 중인 daemon을 post-install 단계에서 재시작하지만, 수동 `go build`나 `install-native`만 실행한 경우에는 daemon이 그대로 남을 수 있다.

주의:
- 수동 설치/빌드 후 MCP smoke 전에는 필요하면 `agent-harness daemon stop --json`으로 기존 daemon을 내린다.
- 테스트는 `HARNESS_DAEMON_DIR=$(mktemp -d)/daemon`으로 실제 user daemon과 분리한다.
- daemon socket/pid/log는 user state dir에 두고 repo나 wiki vault에 쓰지 않는다.
- **D2 (NFS caveat, accepted)**: daemon single-instance locking은 `daemonlock/lock.go`의 `O_EXCL` create + stale(30s)/PID-liveness 감지로 막는다. lock 파일은 startup handoff 후 child가 삭제하므로(transient) flock fallback은 부적합하다(flock은 inode에 묶여 삭제 시 깨짐). `O_EXCL`은 NFS/FUSE에서 원자성이 보장되지 않으니 **daemon state는 로컬 FS에 둔다**; 네트워크 마운트 home에서는 이론상 두 daemon이 뜰 수 있으나 두 번째는 동일 unix socket bind에서 실패한다.

## 14. Codex vs Claude Code hook rendering drift

Codex and Claude Code accept similar UserPromptSubmit JSON, but they do not render it the same way.

주의:
- Codex shows `hookSpecificOutput.additionalContext` in the TUI `hook context:` row, so anything injected for the model is also visible to the user and may be newline-collapsed.
- Claude Code can use `systemMessage` as the user-visible channel while keeping `additionalContext` as model-facing context.
- Do not assume a hook field is hidden just because another host hides it. Verify the installed host runtime/schema before changing hook output.
- For Codex, keep the project-doc catalog in `additionalContext` because the agent needs it, but avoid route/action/profile/pending-upkeep status prose there.
- Keep project-doc frontmatter descriptions concise English metadata; `project bootstrap` and `project bootstrap --sync` use this canonical metadata, so verbose descriptions multiply across every target repo.

## 15. IssueOps worktree edits must be hook-guarded

IssueOps worktree isolation cannot rely on the model remembering `pwd` or shell `workdir`. Some edit tools can apply relative paths from a different checkout than the shell command just verified.

주의:
- IssueOps sessions set `HARNESS_SOURCE_CHECKOUT` and `HARNESS_EXPECTED_WORKTREE` before implementation.
- Installed Codex and Claude PreToolUse hooks include `--enforce-worktree` and block mutating tool events outside the expected worktree when that env is set.
- Installed Codex and Claude PreToolUse hooks include `--enforce-gitops-kubectl`; direct mutating `kubectl` commands such as `apply`, `delete`, `patch`, or `rollout restart` must be represented as manifest changes in git and applied through the repo's GitOps path. Live-access commands such as `kubectl exec` and `kubectl port-forward` ask for explicit user confirmation because they expose live workloads or local ports. Read-only commands and dry-runs such as `kubectl get`, `logs`, `diff`, and `apply --dry-run=server` remain allowed.
- Installed Codex and Claude PreToolUse hooks include `--enforce-staged-checks`; broad Biome lint/format commands such as `biome check apps libs` or package scripts that expand to broad `apps libs` checks ask for explicit user confirmation. Prefer staged or changed-file commands such as `biome check --staged`, `biome format --staged`, lint-staged, or explicit changed file lists so existing repo debt does not become the current diff's failure.
- Manual edit rules still require absolute paths rooted at the expected worktree and status checks for both source checkout and worktree, because host hook coverage can differ by runtime.
- If a source checkout receives implementation edits by mistake, stop, move only your own changes into the IssueOps worktree, and verify the source checkout is clean before continuing.
- Late-promotion gap: when work starts under `/goal`, ralph, or ad-hoc edits and is only promoted to IssueOps at the issue/PR phase, the worktree-first trigger ("`$issueops` explicitly invoked") never fires, so implementation can land in the source checkout on a feature branch without an isolated worktree. Observed 2026-06-03: gap-closure work was committed directly on `feat/...` in the main checkout; main ref stayed clean but the isolation contract was skipped. Mitigation: when a cycle is promoted to IssueOps, move remaining work into an isolated worktree or explicitly record the deviation. Implemented (issue #25) and hardened 2026-06-05: the `--enforce-worktree` PreToolUse guard judges by the current work's own cycle. IssueOps cycle ids are deterministic per (repo, branch) and `issueops start` resumes instead of duplicating, so cycles cannot accumulate as stale duplicates. The guard reads the current branch from `.git/HEAD` and loads only that branch's cycle (`ActiveIssueOpsCycleForBranch`). In `implement`/`ai-slop-clean`/`feedback`/`pr`, a missing `worktree_path` is now fail-closed for mutating source/worktree targets: create the sibling worktree and run `issueops link-worktree` before editing. The guard also blocks `git checkout -b`/`git switch -c` of a known IssueOps branch in the source checkout, while allowing `git worktree add ../<repo>.worktrees/...` as the preparation step. Legacy timestamp-id records and other branches' cycles have different ids and are never read, so they cannot cause a false linked-worktree lock; marking the cycle `done` releases the source checkout.
- Tool-root drift: IssueOps may run implementation in a sibling worktree while a host session and some MCP servers remain rooted at the original source checkout. In `api-servers`, verified 2026-06-05, Claude `.mcp.json` was gitignored and its `codegraph` server had `--path /Users/habin/workspace/api-servers`, while Codex global `codegraph` was `codegraph serve --mcp` without `--path`. Do not solve this by asking the user to restart Claude Code in the worktree; preserve the current session and enforce per-call evidence instead.
- CodeGraph is salvageable in worktree sessions because the MCP tools expose `projectPath` and the CLI exposes `--path`. IssueOps worktree preparation should ensure a CodeGraph index exists for the exact worktree, and the PreToolUse worktree guard should block CodeGraph calls unless `projectPath` equals `HARNESS_EXPECTED_WORKTREE`.
- In IssueOps worktrees, prefer native absolute-path file tools, `git -C "$HARNESS_EXPECTED_WORKTREE"`, `rg` rooted at the expected worktree, and worktree-local tests for correctness. Do not let a CodeGraph/Serena-first project rule override direct worktree evidence. Treat filesystem/Serena MCP tools as blocked or advisory unless their root is proven to be the expected worktree in the current session.

## 16. IssueOps decision replies must have numbered choices

When the user must choose a route, cleanup action, feedback response, or next phase, free-form prose is too easy to miss. Prompt discipline alone is insufficient.

주의:
- Installed Codex and Claude Stop hooks include `--enforce-numbered-next-actions`; when the host exposes `last_assistant_message` or a transcript path, missing `1.`, `2.`, and `3.` choices are blocked.
- The Stop hook should explain the missing choices to the agent and instruct the agent to present context-specific next actions; it should not synthesize fixed choices itself.
- Keep the three choices concrete: recommended proceed, narrower/lower-risk alternative, and pause/defer.
- If the host does not expose the final assistant message to Stop hook input, the guard must no-op and record diagnostics rather than guessing.

## 16.1 IssueOps hooks must not become workflow workers

IssueOps state is durable because `agent-harness issueops ...` commands record intent, issue links, branch preparation, worktree paths, tool preparation, design review, plan links, ai-slop-clean evidence, feedback joins, PR/MR readiness, and cleanup status. Moving any of that work into lifecycle hooks would make progress depend on host-specific event timing and incomplete hook payloads.

주의:
- Hooks may block fast, deterministic, inspectable violations only: wrong worktree target, Korean remote artifact failure, invalid VCS issue-linking body metadata, missing PR/MR target branch, missing labels/assignee, staged-check/live-command confirmation, or missing numbered next-action choices.
- Hooks must not create or edit issues, mutate files, run tests, wait for background jobs, prepare branches or worktrees, create PRs/MRs, reply to reviews, merge, or delete branches/worktrees.
- When readiness reports `intent_contract`, `plan_prep_*`, `branch_prepare`, `worktree_tools_*`, `design_review`, `plan_path`, `ai_slop_clean`, or `contract_feedback_issue_update`, run the owning `issueops` command in the main agent loop and retry readiness. Do not add a hook-side workaround.

## 17. MCP tool-use risks

- Broad tool descriptions make agents over-call tools or pass wrong arguments.
- Always injecting all project documents at session start wastes context and can hide task-specific evidence.
- Writable tools need explicit write semantics; prefer dry-run or append-only behavior.
- Tool output is evidence, not proof: verify file existence, warnings, and command/test results before claiming completion.

## 18. Audit harness flags must match the CLI contract

A stability-audit failure is not automatically a harness defect; the audit framework itself can call the CLI with invalid flags.

주의:
- `self-verify --iterations=N` requires `--full`; without it the CLI exits fast with "--iterations requires --full". Observed 2026-06-03: `e2e_stability_audit.py` invoked `self-verify --iterations=10` without `--full`, so the audit reported a false self-verify failure in ~150ms while a direct quick run passed 22/22.
- When an audit step fails suspiciously fast, reproduce the exact invocation directly and compare against the documented commands in `.agent-harness/OPERATIONS.md` / root `AGENTS.md` before concluding the harness is unstable.
- Give the full 10-iteration self-verify a generous timeout (>=180s); the final LLM gate plus 10 seeded iterations exceed the quick-mode budget.

## 19. Verify git identity before contributor-sensitive pushes

GitHub contributor attribution follows commit author/committer email, not just the displayed author name.

주의:
- Before committing or pushing contributor-sensitive history, run `git config --show-origin --get-regexp '^user\.'` and `git var GIT_AUTHOR_IDENT && git var GIT_COMMITTER_IDENT`.
- In this repo, `m16khb@bubbletap.com` maps to the unwanted `habinkim-bubbletap` contributor. Use `m16khb@gmail.com` or `43867832+m16khb@users.noreply.github.com` instead.
- If a tool may bypass repo-local config, set `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, and `GIT_COMMITTER_EMAIL` explicitly for the commit command.
- After push, verify `git log --all --format='%an <%ae> %cn <%ce>' | rg 'bubbletap'` is empty and check GitHub contributors when attribution matters.

## 20. Stop hook output: `continue:false` hard-stops; use `decision:block` + `reason` to continue in-turn

A Stop hook that wants the agent to *recover and keep going* (for example, to present the missing numbered choices) must NOT set `continue:false`. Doing so halts the agent and surfaces the reason to the user, instead of letting the agent act on it in-turn.

주의:
- Verified against host binaries. Claude `2.1.162` embedded hook docs: `continue` — "Set to `false` to block/stop (default: true)", `stopReason` — "Message shown when `continue` is false". Codex `0.137.0` `stop.command.output` schema: `continue` (default true), `decision` = `BlockDecisionWire(["block"])`, `reason` with the note "Claude requires `reason` when `decision` is `block`". Both hosts mirror the same schema.
- `continue:false` is a hard stop and takes precedence over `decision`. To drive an IN-TURN continuation, return `decision:"block"` + `reason` and leave `continue:true` (or omit it). `runHookStop`'s next-action judgement relay branch already does this; the `--enforce-numbered-next-actions` block branch wrongly sent `continue:false`, so the agent "just stopped" and the user had to prompt it manually (observed 2026-06-04, fixed in `cmd/harness/hook_user_prompt.go`).
- When the block branch uses `continue:true`, guard it with `stop_hook_active`: hosts set that flag true on a Stop that is itself a continuation of a prior stop-hook block. Allow the stop (no-op `{}` output) while it is true so a non-complying agent cannot loop forever.
- Stop hooks accept only the stop-control schema (`continue`/`decision`/`reason`/`stopReason`/`systemMessage`/`suppressOutput`). Injecting `hookSpecificOutput.additionalContext` on Stop makes Codex report "invalid stop hook JSON output"; use a no-op `{}` payload when not blocking.
- The ONLY Stop-hook output reliably surfaced to the user is `decision:"block"` + `reason` — it renders as "Stop hook feedback" AND re-invokes the agent in-turn. Two channels were observed to produce NO visible notice (2026-06-04): a non-blocking `{"systemMessage": ...}` (turn allowed to end), AND `{"continue": false, "stopReason": ...}` — despite the doc claiming "systemMessage — display to the user (all hooks)" and "stopReason — shown when continue is false". Do not rely on either to notify the user from a Stop hook.
- Claude Code labels a successful Stop `decision:"block"` relay as `hook_blocking_error` in the transcript attachment and can surface it as `stop-hook-error` in stream/UI output. Do not treat that label alone as an agent-harness hook process failure. Check the hook command exit code/stderr plus the follow-up `stop_hook_summary`: an intended next-action relay has `preventedContinuation:false`, `level:"suggestion"`, and an empty failure stderr even though the display name says "error". Treat it as a real failure only when the process failed, stderr names a schema/runtime error, or continuation was actually prevented.
- Consequence: you cannot both stop-and-wait AND show the user a message via raw Stop output — the only visible channel (`decision:"block"`) continues the agent. So when a Stop hook reaches a recoverable review point, return `decision:"block"` + a `reason` that instructs the agent to act on the observed facts. The follow-up Stop carries `stop_hook_active=true`; missing-choice recovery still no-ops on that follow-up to avoid loops.
- So the Stop outcomes are: (1) recover/continue in-turn → `decision:"block"` + `reason`; (2) next-action judgement relay → `decision:"block"` + observed facts for the main agent; (3) silent no-op → `{}`. `continue:false` is a hard stop that suppresses the visible feedback, so avoid it for notifications.
- The Stop hook should only treat numbered lines inside an explicit `선택지:`/`Options:`/`Next actions:` section as next-action choices. Explanatory numbered lists can contain words like `추천` and `자동진행` and must not be parsed as next-action choices.
- The Stop hook is not a judge, scorer, classifier, or safety gate. It must not claim "자동진행 후보", calculate scores/thresholds/confidence, classify destructive/safe/reversible/eligible choices, or decide whether the action should run. Its job is only to say that a next-action judgement point was reached and relay inspectable facts such as choice count, recommendation count, and recommended text. The main agent owns safety, reversibility, user-intent alignment, and proceed-or-ask judgement from current context, and must state that judgement in the recovery response: either why it is auto-proceeding now, or why it is not auto-proceeding and needs user confirmation. If it auto-proceeds, the result report still needs a `선택지:` section so the next action boundary remains explicit.
- A main-agent `no-auto-proceed` judgement is sticky. If the agent says it will not auto-proceed at a Stop-hook next-action boundary, an automated `/goal`/goal-continuation prompt must not immediately reinterpret the active objective as permission to resume the same action. Resume only after an explicit user choice or a new user instruction. Observed 2026-06-06: the agent said it would stop for diff review, then a goal continuation message arrived and the agent resumed implementation, contradicting the prior judgement.
- A `no-auto-proceed` Stop-hook recovery response should be allowed to stop without adding a new `선택지:` block. Repeating the choices in that response creates a fresh next-action judgement point and can produce the exact "recommend -> no-auto-proceed -> recommend" loop. The missing-choice guard should require choices for ordinary final responses, but no-op when the final response explicitly says `자동진행하지...` / `no-auto-proceed`.
- UserPromptSubmit must not clear `stop-next-action-relay.json` for automated goal-continuation or Stop-feedback prompts. The relay file is the duplicate-suppression guard for repeated `선택지:` responses after a no-auto-proceed judgement; clear it only for an explicit next-action instruction or real progress such as PostToolUse. Observed 2026-06-10: a no-auto-proceed response with choices could be relayed repeatedly when an automated continuation cleared the relay before another Stop.
- New installs should use `--relay-next-action-judgement` for that relay path. The older `--auto-proceed-next-actions` flag is a deprecated alias kept only so existing user hook configs do not break; do not document it as the primary behavior switch.
- `stop_hook_active` must not suppress main-agent judgement when the recovery response now includes valid next-action choices. It should suppress only missing-choice recovery loops. Otherwise the agent can present `선택지:` after a block and then silently stop instead of either proceeding or explaining why it needs user confirmation.

## 20. /tmp/agent-harness-* build artifact cleanup

Manual builds, smoke tests, and ad-hoc verification runs can leave stale binaries and log files under `/tmp/agent-harness-*`. Self-verify temp directories are properly cleaned (`t.TempDir()`), but one-off commands like `go build -o /tmp/agent-harness-test ./cmd/harness` and output captures (`... >/tmp/agent-harness-*.txt`) are manual artifacts that accumulate.

주의:
- Harness Go code never writes to `/tmp/agent-harness-*` — these are always manual developer artifacts.
- Self-verify temp directories (`/tmp/agent-harness-self-verify-*`, `/tmp/ahd-*`) are properly cleaned after each run.
- To clean up stale manual artifacts: `rm -f /tmp/agent-harness-*`. Add this to a periodic workspace hygiene routine.
- CI and automated scripts should prefer `mktemp -d` or Go `t.TempDir()` / `os.MkdirTemp` over hardcoded `/tmp/` paths.
- Build scripts (`scripts/install-native.sh`) build to `$ROOT/bin/agent-harness`, not `/tmp`.

## 21. Worktree guard has multiple block paths; stale worktrees need consistent liveness

A single PreToolUse worktree-guard "fix" is rarely enough: the guard blocks through more than one path, and a record keyed only on (repo, branch) is treated as an active cycle even when its worktree was deleted. Verified 2026-06-09 via multi-session QA.

주의:
- Fixing only `CycleForBranch` did not clear a deleted-worktree deadlock — the block merely moved to `noCycleIssueOpsBranchNeedsWorktree`. Trace every block path (current-branch cycle, no-cycle issue-branch needs-worktree, linked-worktree cycles, MCP root guard) before claiming a deadlock is resolved. Add a reproduction test per path.
- Liveness must be consistent across all readers. `CycleForBranch` (guard primary), `noCycleIssueOpsBranchNeedsWorktree`, and `LinkedWorktreeCyclesForRepo` must agree on "worktree present-but-deleted = inactive"; an empty `worktree_path` is a distinct not-yet-linked state and must NOT be treated as deleted.
- A stale-cycle block message must name a working escape. `issueops phase --to done` and `--to done --force` both require `pr` phase, so "mark the stale cycle done" was a dead end; only `issueops force-release --id <id> --reason <why>` releases a non-pr cycle. Keep the guard message and the actual escape command in sync (pin it with a test).
- Resetting a stale cycle on restart must exclude phases whose work product is external. `Start` reset is gated to `implement`/`ai-slop-clean`/`feedback` (worktree IS the work product); a `pr`-phase cycle with a deleted worktree resumes instead, because its issue/PR/remote-artifact linkage lives remotely and would be destroyed by a blank reset. Always preserve recovery anchors (`issue_url`/`issue_links`) and stamp audit fields (`stale_reset_at`/`stale_reset_prior_phase`).
- Stale cleanup belongs off the hot path. The `issueops cleanup stale` CLI / `issueops_cleanup_stale` MCP tool may consult git/remote for multi-signal classification (confirmed-stale / likely-done / needs-review); do NOT add git/remote calls to the PreToolUse guard itself. `needs-review` (age-only) is never auto-released; only `confirmed-stale` and `likely-done` are releasable under `--apply`.
- "First matching" cycle must be deterministic AND must not name an unrelated escape. `LinkedWorktreeCyclesForRepo` returned records in `os.ReadDir` order, and IDs are `sha256(repo\x00branch)` hashes, so `linkedRecs[0]` was arbitrary w.r.t. the cycle the user is editing. With ≥2 parallel worktree cycles, a block message that singled out `linkedRecs[0]` for `force-release` (a) pointed at a possibly-live, unrelated cycle and (b) would not unblock the edit while the other worktrees remain — the same non-working-escape trap as the `--to done` dead end. Fix: sort `LinkedWorktreeCyclesForRepo` deterministically (branch, then ID) and have the multi-cycle block message enumerate every worktree holder rather than one arbitrary cycle (`linkedWorktreeCyclesBlockReason`). Verified 2026-06-09.
- Destructive cleanup (`--apply` force-release) must re-read+re-classify immediately before the write. `ScanStaleIssueOpsCycles` snapshots cycles via `NonDoneCyclesForRepo` then probes by bare `os.Stat`; a worktree briefly missing (unmount/NFS/in-flight `git worktree` recreate) or a cycle advanced by a parallel session between snapshot and release would otherwise be clobbered to `done` (TOCTOU). The fix re-reads the fresh record and re-runs `stalescan.Classify` before `ForceReleaseIssueOps`; this NARROWS but does not close the window.
- Known residual gaps (multi-session QA round 2 — resolved): the `(repo,branch)` JSON store now uses per-id advisory flock (`withIssueOpsLock`) so concurrent `start`/`link`/`set-phase`/`force-release` on the same id serialize across processes; hot-path worktree validity now uses `os.Lstat` on `.git` (file OR directory) via `worktreeGitTracked`, so `git worktree prune`d directories and leftover non-git dirs are correctly excluded; and stale-reset/force-release now stamp `OrphanWorktreePath` for the off-hot-path stale-scan reaper to clean via `git worktree prune` and `git worktree remove --force`.

## 22. Stability audit smoke tests must track current host/MCP contracts

The stability audit can false-fail when its smoke assumptions lag the harness contract.

주의:
- `agent-harness mcp` defaults to the daemon-backed proxy. Newline-delimited JSON-RPC smoke tests that expect direct stdout responses must set `HARNESS_MCP_DIRECT=1`, matching `validationcli/mcpsmoke`. Otherwise stdout can be empty while the proxy path exits successfully, producing a false `mcp_ids=[]` failure.
- UserPromptSubmit currently injects compact per-turn `[agent-harness]` bullet context. The audit must reject old/noisy catalog injection markers such as `Required project docs` or `필수 프롬프트 주입중`, but must not fail merely because compact context contains newlines.
- `self-verify promote --confirm` refuses failed snapshots by default. Validation fixtures that intentionally promote a non-termination-eligible snapshot for state-roundtrip coverage must pass `--allow-failed-source`; production baseline promotion should not use that override.
- Pin these assumptions in `skills/stability-audit/scripts/e2e_stability_audit_test.py` and `cmd/harness/validationcli/stateroundtrip` tests before changing the audit script.

## 23. Skill validation must not depend on host-managed system skill copies

Codex and Claude system skills can be re-materialized by the host and can depend
on optional host-side Python packages. Do not use
`~/.codex/skills/.system/.../quick_validate.py` or a marketplace plugin copy as
the required agent-harness skill validation gate.

주의:
- Use `python3 scripts/validate-skill.py skills/<skill-name>` for
  agent-harness skill metadata validation.
- Treat upstream `openai/codex` `quick_validate.py` fixes as quality pointers,
  not agent-harness completion dependencies.
- If a host-managed validator fails because PyYAML is unavailable, that may be
  valid upstream evidence, but it must not block local agent-harness
  verification when the repo-owned validator passes.

## Incident Archive

Dated incident notes are preserved in `.agent-harness/archive/cautions-incidents.md`. Keep this file focused on evergreen hazards and move one-off history there.
