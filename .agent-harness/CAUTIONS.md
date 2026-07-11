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

## 12. 외부 도구 의존 재도입 금지

`agent-harness` 설치, 업데이트, self-verify, IssueOps readiness gate는 외부 도구 없이 재현 가능해야 한다. 외부 도구는 사용자가 별도로 설치한 경우에만 일반 파일/명령/MCP 경계에서 참고한다.

주의:
- 하네스 설치 경로에서 외부 도구를 clone/install/register/patch 하지 않는다.
- 외부 도구가 없거나 깨졌다는 이유로 core contract를 약화하거나 readiness gate를 통과시켜서는 안 된다.
- 외부 plugin cache를 하네스가 수정하는 shim을 추가하지 않는다. 문제는 해당 도구의 설치/문서/사용 경로에서 해결한다.
- 외부 도구의 vault, memory store, graph index, query-pack, lifecycle hook 의미를 agent-harness core에 복제하지 않는다.

draft-wiki는 별도 staging/export area다. `.agent-harness/draft-wiki/**`에는 사용자가 검토할 후보 Markdown만 둔다. `agent-harness project draft-wiki promote --confirm`은 승인된 draft를 repo-local `exported/` 디렉토리로 이동하고 `export.log`를 append할 뿐, 외부 wiki ingest/lint/index/query-pack을 완료한 것으로 보고하지 않는다.

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
- Codex 0.142.5 rejects PreToolUse `hookSpecificOutput.permissionDecision="ask"` with `unsupported permissionDecision:ask`. Codex ask-style gates must fall back to a normal `decision="block"` response; hosts with native ask support can keep `permissionDecision="ask"`.
- Codex `hook returned invalid <event> JSON output` means hook stdout looked like JSON but failed strict serde parsing (`deny_unknown_fields`; unknown top-level field, unknown enum value, or truncated/multi-object stdout). It is NOT a generic failure label — 원인 후보를 그 세 가지로 좁히고, 용의 훅 stdout을 `... | cat | wc -c` 파이프 하류에서 재현해 확인한다. Node/Bun 기반 외부 훅은 stdout이 파이프일 때 큰 출력을 512B에서 자르고 종료할 수 있다(2026-07-08 incident 참조). agent-harness Go 훅은 동기 write라 이 문제가 없다.
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
- External code-intelligence tools are the user's own installs; the harness no longer prepares their indexes or validates their `projectPath`. When using one in a worktree session, root every call at the expected worktree yourself.
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
- When readiness reports `intent_contract`, `plan_prep_*`, `branch_prepare`, `worktree_tools_*`, `compatibility_review`, `backward_compatibility`, `side_effects`, `rollback_plan`, `compatibility_verification`, `compatibility_blockers`, `compatibility_approval`, `execution_decision`, `design_review`, `plan_path`, `ai_slop_clean`, or `contract_feedback_issue_update`, run the owning `issueops` command in the main agent loop and retry readiness. Do not add a hook-side workaround.
- Sub-agent use is not a free speedup. It can preserve the main context, give a fresh reviewer, isolate tools, or parallelize independent research, but it also reduces mid-run steering/visibility and adds latency/token/coordination cost. Record those tradeoffs plus the net-positive rationale with `issueops execution decide`; if that rationale is weak, main-agent direct execution remains the default.

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
- Give the full 10-iteration self-verify a generous timeout (>=180s); the 10 seeded deterministic iterations exceed the quick-mode budget.
- `HARNESS_SELF_VERIFY_LLM_EVAL=gate` is a valid ambient runtime configuration, but the current self-verify implementation only renders the read-only evaluator prompt. It sends no Z.AI request and ingests no external verdict, so `gate` intentionally returns a non-passing `llm_eval` result. Do not diagnose that result as environment drift or claim an external judgment occurred. Repository completion gates must use explicit `--llm-eval=false`, record the override, and restart from the first gate after any interrupted or prompt-only run.
- Handoff focused tests must use `./cmd/harness/hookcli/hookinput`; the plausible-looking `./internal/core/hookinput` path does not exist and causes a command-spec failure after other packages have already started. Pin the full focused command in `.agent-harness/TESTING.md` and restart the sequence rather than reusing partial results.
- Do not validate the GJC TypeScript hook with a literal `--host gjc` grep. The shim constructs argv as adjacent array elements, so that shell string is absent even when host forwarding is correct. Run `bun scripts/smoke-gjc-native-hook.ts "$HOME/.gjc/agent/hooks/agent-harness.ts"` and require behavior-level host/session/cwd/block JSON instead.
- `orca orchestration send --type` rejects values outside `status|dispatch|worker_done|merge_ready|escalation|handoff|decision_gate|heartbeat`. Verify the installed CLI when this enum changes; do not improvise `progress`, `blocked`, or `completed` message types.
- A handoff being `closed` is not sufficient for retry. `closed/accepted` is terminal; only `closed/worker_failed` and `closed/cancelled` may mint a new attempt/epoch. Checking only `StateClosed` reopens an already accepted result and violates the actor transition table.

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
- New installs use only `--relay-next-action-judgement` for that relay path. Do not reintroduce auto-proceed aliases or hook-side scoring paths.
- `stop_hook_active` must not suppress main-agent judgement when the recovery response now includes valid next-action choices. It should suppress only missing-choice recovery loops. Otherwise the agent can present `선택지:` after a block and then silently stop instead of either proceeding or explaining why it needs user confirmation.

## 20. /tmp/agent-harness-* build artifact cleanup

Manual builds, smoke tests, and ad-hoc verification runs can leave stale binaries and log files under `/tmp/agent-harness-*`. Self-verify temp directories are properly cleaned (`t.TempDir()`), but one-off commands like `go build -o /tmp/agent-harness-test ./cmd/harness` and output captures (`... >/tmp/agent-harness-*.txt`) are manual artifacts that accumulate.

주의:
- Harness Go code never writes to `/tmp/agent-harness-*` — these are always manual developer artifacts.
- Self-verify temp directories (`/tmp/agent-harness-self-verify-*`, `/tmp/ahd-*`) are cleaned on normal completion paths, but SIGKILL or host crash can still leave them behind. No automatic `/tmp` reaper is implemented; cleanup remains explicit hygiene.
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

## 24. IssueOps 도메인 어휘를 CLI 서브커맨드로 착각

에이전트가 `issueops grill`, `issueops domain`, `issueops split` 같은 존재하지 않는 서브커맨드를 반복적으로 호출한다. 원인은 IssueOps skill 산문이 grill/domain/split 같은 선명한 도메인 명사(phase 이름, 결정 동사, ledger artifact)를 1급 개념으로 쓰는 반면 CLI는 `phase`, `remote create-child`, `link-related` 같은 제네릭 동사를 쓰는 **명명 불일치**다.

주의:
- `issueops <domain-word>`를 추측으로 호출하지 말 것. 먼저 `issueops --help`로 실제 서브커맨드 레지스트리를 확인한다.
- `grill`/`problem`/`implement`/`ai-slop-clean`/`feedback`/`pr`은 lifecycle phase이며 `issueops phase --id ID --to <phase>`로 진입한다.
- `split`은 breakdown 결정(no-split 기본)이지 명령어가 아니다. child 생성은 `issueops remote create-child`, 기존 child 연결은 `issueops link-related --type splits-from`.
- `domain`(review), `compatibility`(review), `design`(review), `intent`(record)는 ledger artifact이며 각각 `issueops domain-review record`, `issueops compatibility review`, `issueops design review`, `issueops intent record`가 실제 명령이다.
- CLI는 잘못된 서브커맨드에 대해 did-you-mean 힌트를 출력한다(`issueops.go`의 `suggestIssueOpsSubcommand`). 매핑 전체는 `skills/issueops/SKILL.md`의 "Concept → Command Map" 섹션을 참조.

---

## 25. gh/glab create 출력은 항상 bare URL — JSON/Number 파싱 추가 금지

`gh issue/pr create`와 `glab issue/mr create`는 어떤 create 경로에서도 `--json` 플래그를 넘기지 않는다. 따라서 stdout은 항상 bare 아티팩트 URL 한 줄이다. 과거에 `parseGhOutput`/`parseGlabOutput`에 JSON-first 분기(`ghResult.Number`/`glabResult.IID` 채우기)를 뒀지만 도달 불가능한 죽은 코드였고 port 결과의 `Number`는 항상 ""였다(9d6be19에서 제거).

주의:
- create-output 파서는 URL을 trim해서 반환만 한다. JSON 디코딩/Number 추출 분기를 되살리지 말 것.
- 아티팩트 번호가 필요하면 URL에서 파싱한다(예: `parseGitHubIssueURL`), create 출력 JSON을 기대하지 않는다.
- provider가 JSON을 내는 경로는 `gh api`/`glab api graphql`(별도 `runGhAPIJSON`/`runGlabGraphQL`)뿐이며 create 파서와 무관하다.

---

## 26. `ValidateArtifactURL`은 verify-artifact(pr/mr) 전용 — issue 케이스 추가 금지

`remote.ValidateArtifactURL`의 유일한 prod 호출자는 `artifactverify.verificationFromRequest`이고, 이 함수는 호출 전에 `kind != pr/mr`을 하드 거부한다(이슈는 사이클의 remote-artifact가 아니다 — 사이클의 RemoteArtifact는 PR/MR이다). `create-issue --confirm`의 라이브 검증 게이트는 이 계층을 **거치지 않고** `VerifyRemoteArtifactLive` → `fetchGitHubIssueArtifact`/`fetchGitLabIssueArtifact`로 직행하며, fetcher가 자체 URL 파싱(GitLab은 `SplitGitLabIssuePath`+kind 체크, GitHub은 `gh issue view`)을 한다.

주의:
- `ValidateArtifactURL`/`verificationFromRequest`에 `github:issue`/`gitlab:issue` 분기를 추가하면 죽은 코드가 된다(116ebef 리뷰에서 지적·제거).
- 새 아티팩트 종류의 라이브 검증을 배선할 때는 실제 도달 경로(`VerifyRemoteArtifactLive` switch + fetcher)를 확장하고, "이미 라우팅된다"는 주석은 도달 경로를 실증한 뒤에만 쓴다.
- 게이트 배선은 prod에서 CLI `issueOpsRemoteDeps`(`VerifyLive`)와 MCP `harnessapp/mcp_facade`(`VerifyIssueOpsRemoteArtifactLive`)가 주입한다. 미배선 기본값은 "dependency is not configured"를 반환하므로 게이트가 실제로 살아있는지 이 배선을 확인한다.

---

## 27. `.agent-harness/*.md` 편집은 response-contract 골든을 드리프트시킨다

`cmd/harness/testdata/response_contracts.golden.json`은 `.agent-harness/*.md` 문서의 `docs_index`(byte 수 + heading + title)를 캡처한다. 문서 본문을 고치거나 heading을 바꾸면 `TestResponseContractsGolden`이 실패한다. 문서 커밋이 골든을 재생성하지 않으면 pre-existing red로 남아 무관한 변경이 오인 reject된다.

주의:
- `.agent-harness/*.md`를 편집하면 `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update`로 골든을 재생성하고, diff가 `docs_index`(bytes/headings/title)만인지 확인한다(tool schema/response 계약 변화가 섞이면 안 된다).
- 골든 재생성은 같은 문서 편집 커밋에 포함하거나 바로 뒤의 `chore(contract)` 커밋으로 남겨 red를 남기지 않는다.
- 골든이 이미 red라면 무관한 변경 탓으로 오인하기 전에 clean HEAD에서 재현해 pre-existing 드리프트인지 먼저 확인한다.

## 28. 새 fail-closed readiness 게이트는 모든 전진 테스트/공유 픽스처로 파급된다

IssueOps에 새 implement-entry(또는 임의 phase) fail-closed 게이트를 추가하면, 그 phase로 사이클을 전진시키거나 readiness를 단언하는 **모든 테스트가 깨진다**. 게이트 코드만 고치고 인접 패키지를 안 돌리면 CI에서 뒤늦게 터진다. 실제로 devil's-advocate 게이트 추가 시 `internal/core/issueops`뿐 아니라 `cmd/harness/issueopscli`, `cmd/harness/mcpcli/issueops`, `cmd/harness/harnessapp`(response 골든 스냅샷), `internal/core/lifecycle`, `cmd/harness/hookcli`, `internal/adapter/mcp`(catalog count)까지 픽스처를 손봐야 했다.

주의:
- 게이트를 추가하면 `IssueOpsImplementationReadiness`(또는 대상 readiness)를 단언하거나 그 phase로 `AdvanceIssueOpsPhase`하는 테스트를 **넓게 grep**한다: `grep -rn 'IssueOpsImplementationReadiness\|to.*implement\|ai-slop-clean' --include='*_test.go'`.
- 새 아티팩트를 **공유 픽스처 헬퍼**(예: `recordIssueOpsCompatibilityReviewForTest`, lifecycle/hook의 implement-ready seeder)에 seed하면 다수 테스트가 한 번에 통과한다. 직접 필드-set하는 readiness 단언 테스트는 개별 수정한다.
- MCP 도구를 추가하면 catalog count 테스트(`IssueOpsBasicTools`/`IssueOpsLifecycleTools`의 exhaustive `wantNames`)와 `mcp_tools.golden.json`을 함께 갱신한다.
- 게이트가 derived phase-ledger에 나타나면 `response_contracts.golden.json` 스냅샷도 드리프트한다(§27). 스냅샷이 그 phase로 전진하면 전제조건을 실제로 충족(fake CLI 포함)시켜야 한다.
- 증분 검증만 믿지 말고 커밋 전 `go test ./...` 전체를 한 번 돌려 미검출 패키지 파급을 잡는다.

## 29. Slack List create payload와 readback schema는 다를 수 있다

`slackLists.items.create` 입력 schema와 `slackLists.items.list` readback schema를 같은 것으로 가정하면 live List write가 실패한다. 2026-07-01 Engelbart live E2E에서 link 컬럼을 readback 모양인 `originalUrl`로 보냈더니 `missing required field: original_url`로 실패했다. 같은 link는 readback에서는 `originalUrl`로 보이지만 create payload는 `original_url`을 요구한다.

주의:
- Slack List link 컬럼 생성 payload는 `{"link":[{"original_url":"https://..."}]}`를 사용한다. readback의 `originalUrl`을 그대로 create payload로 재사용하지 않는다.
- 회의록 List의 `이름` 컬럼은 날짜를 포함하지 않는 topic-prefix 제목을 쓴다. 날짜는 `회의일` 컬럼에 따로 들어간다. 예: `[AI] Vertex BYOK 비용 비교 회의`, `[배포] TC NCP 마이그레이션 및 플랫폼 정책 회의`.
- Canvas 제목 규칙(`YYYY-MM-DD [Topic] Title`)과 List row 제목 규칙(`[Topic] Title`)을 혼동하지 않는다.
- Raw Slack Web API `canvases.create/edit`의 `document_content` 지원 목록은 `quote block`은 포함하지만 `callout`은 명시하지 않는다. Connector의 Canvas-flavored markdown은 callout을 문법으로 받을 수 있어도, raw Web API 경로에서 `::: {.callout}`이 일반 문단으로 보이면 quote block(`> ...`) 또는 검증된 connector 경로로 대체한다.
- 팀 공용 Slack List 테스트 write는 삭제/재생성해도 알림이나 흔적이 남을 수 있다. 테스트 항목은 `[TEST] ...`처럼 명확히 표시하고, 사용자 승인 없이 rename/delete/recreate 하지 않는다.
- Slack List schema regression test는 아직 구현하지 않았다. Live Slack 쓰기 부작용 때문에 자동 테스트로 옮기기 전에는 connector fixture 또는 승인된 isolated test List가 필요하다.

## 30. IssueOps worktree 세션의 source-checkout mirror edit 오인

IssueOps worktree 세션에서도 host cwd, MCP root, file-edit tool root가 원본 source checkout에 남아 있으면 worktree에 존재하는 같은 상대경로 파일을 source checkout에서 수정하는 실수가 발생한다. 이 경우 단순 branch gate만으로는 부족하다. 같은 파일이 worktree에도 있으면 source checkout edit는 대개 target drift다.

주의:
- `issueops resume` 또는 `issueops worktree prepare-tools`가 출력하는 `export HARNESS_EXPECTED_WORKTREE=<worktree>`를 세션 환경에 반영하고, 편집 전 cwd와 절대경로가 worktree를 가리키는지 확인한다.
- Worktree 세션에서는 file tool에 worktree 절대경로를 넘기고, shell은 `git -C "$HARNESS_EXPECTED_WORKTREE"` 또는 `rg "$pattern" "$HARNESS_EXPECTED_WORKTREE"`처럼 명시 root로 실행한다.
- Guard는 source checkout의 모든 edit를 막지 않는다. §21의 multi-path deadlock 방지 때문에 non-cycle branch에서 source checkout에 새 파일을 만드는 정상 작업은 허용되어야 한다.
- 방어층은 세 겹이다: PostToolUse source-checkout warning, PreToolUse mirror-file `ask`, SessionStart/UserPrompt worktree reminder. Host가 `ask`를 지원하지 않으면 Codex처럼 `block`으로 degrade될 수 있다.
- 의도적으로 source checkout에서 작업해야 하면 worktree 절대경로로 바꾸거나, 해당 IssueOps cycle을 `issueops force-release --id <id> --reason <why>`로 해제한 뒤 진행한다.

## Incident Archive

Dated incident notes are preserved in `.agent-harness/archive/cautions-incidents.md`. Keep this file focused on evergreen hazards and move one-off history there.

## 2026-07-02 — Re-verify stale memory observations against HEAD before planning gaps

- Kind: `caution`
- Source: article-insights improvement plan brooks review
- Summary: A claude-mem observation (Jun 11, #14024) claiming self-verify promote skips snapshot validation was used as a plan gap, but commit 09fcb7c had already implemented the fail-closed gate; the stale claim survived into plan v1 and was only caught by adversarial review.
- Context: 2026-07-02 article-insights improvement plan v1 proposed P0-3 "promote snapshot validation" based on a remembered observation. Brooks devil's-advocate review falsified it: cmd/harness/selfworkflow/stateio/self_verify_promote_core.go:35-44 already checks snapshot.OK && Summary.TerminationEligible and refuses promotion without --allow-failed-source (commit 09fcb7c).
- Resolution: Before promoting any memory/observation-sourced claim into a plan gap or work item, re-verify it against HEAD with direct evidence (git log on the file, read the current implementation). Treat dated observations as hypotheses, not facts; record the verification command alongside the gap.
- Evidence:
  - git log --oneline -- cmd/harness/selfworkflow/stateio/self_verify_promote_core.go → 09fcb7c
  - cmd/harness/selfworkflow/stateio/self_verify_promote_core.go:35-44

## 2026-07-07 — IssueOps orchestration locks, additive fields, and worker leases

- Kind: `caution`
- Source: codex Task 15 project docs update from codex-orchestration implementation plan
- Summary: IssueOps child/workpool orchestration must preserve single-entity lock boundaries, additive mixed-binary compatibility, and timestamp lease heartbeat semantics.
- Context: Delegated child cycles and workpool state add orchestration records that can be touched by multiple sessions. The existing store is per-key/per-entity atomic and host-neutral, not a cross-entity transaction manager or process supervisor.
- Resolution: Use only one entity lock at a time and never call a same-entity with*Lock helper from inside another same-entity lock callback. Keep new orchestration fields additive with omitempty and schema_version=1 until an explicit migration is needed; verify the active binary, docs, CLI/MCP schema, and daemon readback before trusting mixed-version state. Treat pool liveness as LeaseExpiresAt plus heartbeat, not PID ownership: a worker whose lease is expired or lost must stop and let another claim proceed.
- Evidence:
  - .agent-harness/ARCHITECTURE.md actor model and workpool namespace
  - .agent-harness/AGENT_WORKFLOW.md pool worker loop and heartbeat contract
  - .agent-harness/CAUTIONS.md existing state and worktree lock cautions
- Alternatives / rejected options:
  - Nested parent/child/pool locks — rejected because lock ordering is hard to prove and same-entity re-entry can self-deadlock.
  - Schema-version bump for additive fields — rejected until destructive migration or incompatible read semantics are required.
  - PID-based worker ownership — rejected because agents may run across host sessions and compaction; timestamp leases are portable and recoverable.

## 2026-07-10 — 로컬 MCP 게이트웨이 FD 고갈이 모든 loopback MCP 연결을 즉시 리셋시킨다

- Kind: `caution`
- Source: glab-api-servers/glab-cloud-platform 동시 실패 진단 (mcp-proxy 로그 + lsof 실측)
- Summary: launchd가 띄운 loopback MCP 게이트웨이(예: mcp-proxy)가 stateful streamable-HTTP 모드에서 세션을 회수하지 않으면(관측: 생성 1,373 vs DELETE 26) 클라이언트 재시도 폭풍(정점 분당 318 세션)이 FD를 소진시킨다. launchd 기본 soft limit은 256이라 11분 만에 EMFILE(`socket.accept() ... Too many open files`)에 도달했고, 이후 포트는 LISTEN 상태지만 모든 연결이 즉시 ECONNRESET된다. Claude Code에서는 "△ tools fetch failed" 또는 "✘ failed"로 보인다.
- Context: initialize POST는 200으로 성공하면서 SSE GET이 실패하는 구간에서는 클라이언트가 핸드셰이크 전체를 재시도하며 매 시도가 새 stateful 세션을 서버에 남긴다 — 실패가 실패를 증폭시키는 구조다. 게이트웨이 하나가 여러 named server(glab, codegraph)를 서빙하므로 장애는 특정 서버가 아니라 게이트웨이 전체 단위로 발생한다.
- Triage: (1) `lsof -nP -iTCP:<port> -sTCP:LISTEN`으로 리스너 생존 확인, (2) curl로 MCP initialize POST — 즉시 리셋이면 FD 고갈 의심, (3) `lsof -p <pid> | wc -l`을 limit과 비교, (4) 게이트웨이 stderr 로그에서 `Errno 24` 확인. `agent-harness doctor`의 `mcp_gateway` 체크가 (2)(3)을 자동화한다 — 도달 불능이면 `mcp_gateway_unreachable`, FD 512 이상이면 `mcp_gateway_fd_pressure` warning.
- Resolution: 게이트웨이 재시작으로 즉시 복구. 재발 방지는 ① streamable-HTTP를 stateless 모드로 운영(세션 누적 원천 차단, 서버→클라이언트 알림이 필요 없는 도구에 적합), ② launchd plist에 `SoftResourceLimits:NumberOfFiles` 상향(기본 256은 재시도 폭풍 아래에서 수 분 버퍼에 불과). limit 변경은 `launchctl kickstart`가 아니라 bootout→bootstrap이어야 반영된다.

## 2026-07-09 — macOS 파이프 KVA 고갈이 stdout-capture CLI 테스트를 무기한 블록시킨다

- Kind: `caution`
- Source: b9e293c 감사 중 workpoolcli/loopcli 테스트 600s 행 진단 (goroutine dump + 파이프 용량 실측)
- Summary: 시스템 전체 파이프 fd가 폭증하면(관측: 14,402개, codex 호스트 1개가 3,112개) xnu의 전역 파이프 버퍼 풀이 고갈되어 **새 파이프가 512바이트 최소 버퍼로 강등**된다(정상 16,384 — 100/100 실측). 이때 "쓰기 완료 후 읽기" 방식의 stdout 캡처 테스트 헬퍼는 512B를 넘는 JSON 출력(예: loop record-attempt 579B, workpool claim 결과)에서 write가 영구 블록되어 go test 타임아웃 FAIL이 된다. 코드 회귀처럼 보이지만 머신 상태 문제다(부모 커밋에서도 동일 재현).
- Context: 증상은 간헐적이다 — KVA 압력이 변동하며 새 파이프가 16K↔512B를 오간다. 타임아웃/중단된 `go test` 실행을 `pkill -f 'go test'`로 죽이면 `.test` 바이너리가 고아로 살아남아 파이프 압력을 가중시킨다(양성 피드백). 6ee897d가 harnessapp response-contract 캡처는 동시 reader로 고쳤지만, `cmd/harness/workpoolcli`·`cmd/harness/loopcli`의 capture 헬퍼는 아직 write-then-read 패턴이다.
- Triage: (1) `ps -axo pid,etime,ppid,command | rg '\.test'`로 고아 테스트 바이너리 확인·제거, (2) `lsof -n | rg -c PIPE`로 총량과 `awk '{print $1,$2}' | sort | uniq -c | sort -rn`으로 최다 점유 프로세스 확인, (3) 신규 파이프에 nonblocking write를 가득 채우는 프로브로 실효 버퍼 크기 측정 — 512B면 KVA 고갈 확정.
- Resolution: 재발 방지는 완료됐다. stdout/stderr 캡처 테스트는 `internal/testsupport.CaptureStdout`, `CaptureStdoutAndError`, `CaptureStderrAndError`를 사용한다. 이 헬퍼들은 fn 실행 전에 reader goroutine을 시작하므로 파이프 버퍼 크기에 의존하지 않는다. `agent-harness doctor --json`은 `pipe_capacity_bytes`와 `pipe_capacity` 체크를 노출하고 8192B 미만이면 `pipe_capacity_degraded` warning을 낸다. 근본 완화는 여전히 파이프를 누수하는 장수 host 프로세스 재시작이다. `agent-harness mcp cleanup --apply`는 부모가 죽은 고아 프록시만 정리하므로 살아 있는 host의 누수에는 효과가 없다. `go test`를 죽일 때는 `pkill -f 'go test'`가 아니라 `.test` 바이너리까지 함께 정리한다.

## 2026-07-09 — Workpool pilot gates require updated binaries

- Kind: `caution`
- Source: loop/article gaps 1-2-3 implementation
- Summary: `pilot_required` is an additive field, so a stale binary can ignore it and claim non-pilot work.
- Resolution: Update the shared daemon and local CLI together before trusting pilot-first fan-out. Verify with `agent-harness workpool status --pool ID --json` and `agent-harness contract check --json` when mixed sessions may be active.

## 2026-07-07 — SQLite sqlstore span 규율: 중첩 금지, per-root 직렬화, fresh start

- Kind: `caution`
- Source: JSON+flock → sqlite 전면 전환 세션 (사용자 결정: 전체 일괄 전환 + fresh start)
- Summary: 모든 상태 저장/락은 `internal/core/sqlstore`를 통해야 하며, span은 state root 단위로 직렬화되고 절대 중첩할 수 없다. 레거시 JSON/lock 파일은 무시된다(마이그레이션 없음).
- Context: 5개 with*Lock 계열(issueops, session, state, workpool, worker)이 전부 sqlstore span으로 이동했다. flock 시절엔 per-entity 락이라 서로 다른 entity 락 중첩이 기술적으로 가능했지만, sqlite span은 per-root라 같은 root의 어떤 span 안에서든 다른 span 진입은 in-process mutex self-deadlock이다.
- Resolution: with*Lock 콜백 안에서 같은 state root의 다른 with*Lock/WithKeyLock/withSessionLock 계열 호출 금지 — multi-entity 작업은 순차 single-span 단계 + read-repair로 유지한다. 새 저장 표면은 파일 I/O가 아니라 sqlstore bucket(Get/Put/List/Delete)으로 추가한다. `harness.db`/`harness.lock.db`와 그 sidecar(-wal/-shm/-journal)는 삭제하지 않는다(락 db는 flock inode 규칙의 후계자). 테스트 픽스처는 raw 파일 쓰기 대신 `sqlstore.Open(dir).Put(bucket, id, raw)`로 심는다. 레거시 `<key>.json`/`.lock`/`.state-lock` 파일은 fresh start 정책상 읽지도 지우지도 않는다(doctor는 무시).
- Evidence:
  - internal/core/sqlstore/sqlstore.go WithSpan 주석과 cross-handle 직렬화 테스트
  - internal/core/issueops/issueops_lock.go, internal/core/state/state_lock.go의 span 계약 주석
  - .agent-harness/ADR.md "State storage moves from JSON files + flock to SQLite"
- Alternatives / rejected options:
  - per-entity sqlite 락 db — 거부: 파일 수가 flock 시절로 회귀하고 span 규율 단순성이 사라진다.
  - 레거시 JSON 자동 마이그레이션 — 거부(사용자 결정): fresh start; 필요 시 수동 재생성이 더 단순하다.

## 2026-07-08 — Codex hook "invalid JSON output"의 원인은 동거 훅의 파이프 truncation이었다

- Kind: `caution`
- Source: Codex PreToolUse `hook returned invalid pre-tool-use JSON output` 실패 진단 세션
- Summary: 사용자에게 반복 표시된 PreToolUse/SessionStart "invalid JSON output" 실패의 원인은 agent-harness가 아니라, 같은 이벤트에 등록된 claude-mem codex 훅이 stdout이 파이프일 때 JSON을 512바이트에서 잘라 내보낸 것(+ 별건으로 `status` unknown top-level field). agent-harness 훅은 전 경로에서 스키마 유효했다.
- Context: Codex 0.142.5는 hook stdout을 `deny_unknown_fields` serde wire로 파싱하며, `{`로 시작하는 파싱 불가 stdout에만 정확히 이 오류를 낸다. claude-mem worker(node/bun)는 stdout이 파이프면 큰 출력(예: SessionStart context ~19KB, 관측 기록 많은 파일의 file-context)에서 flush 전에 종료해 정확히 512B만 전달했다. 파일 리다이렉트(동기)에서는 전체가 나오기 때문에 단독 실행 재현으로는 잡히지 않았고, `... | cat` 파이프 하류 비교로 확정했다. 계측 시에도 `node ... | tee`처럼 stdout을 다시 파이프로 만들면 잘림이 재유발되는 관찰자 효과가 있다.
- Resolution: (1) 진단 시 "invalid JSON output"은 unknown field / unknown enum / truncated-or-multi-object stdout 세 갈래로 좁힌다. (2) 같은 이벤트에 등록된 모든 훅을 용의선상에 두고(`~/.codex/hooks.json` + `[hooks.state]`의 plugin hooks), 각 훅 stdout을 파이프 하류(`| cat | wc -c`)에서 검증한다. (3) node 기반 훅 명령은 `_O=$(mktemp); ... > "$_O" 2>/dev/null || true; cat "$_O"; rm -f "$_O"` 패턴으로 stdout을 동기 기록 후 전달한다(claude-mem codex-hooks.json 5개 명령에 적용, 백업 `.harness.bak-20260708`). (4) 훅 명령 문자열이 바뀌면 codex trust 해시가 무효화되어 훅이 조용히 스킵되므로, 검증 전 TUI에서 재신뢰가 필요하고 "실패 0"이 "안 돌아서 0"인지 구분해야 한다.
- Evidence:
  - 동일 명령: 파일 리다이렉트 19,489B vs 파이프 512B(`Unterminated string`) 재현
  - openai/codex rust-v0.142.5 `hooks/src/engine/output_parser.rs` parse_json + `events/pre_tool_use.rs`의 오류 문자열 분기
  - 패치 후 `codex exec` e2e: SessionStart 4/4, PreToolUse 2/2 등 전 훅 Failed 0
- Alternatives / rejected options:
  - claude-mem minified worker 코드 직접 수정 — 거부: 업데이트로 유실되고 검증 부담이 큼; 명령 레벨 버퍼링이 더 단순.
  - 훅 stdout 무음화 — context 계열은 모델 컨텍스트 주입이 목적이라 무음화 불가(스키마 위반인 worker-start만 무음 처리).

## 26. SQLite WAL 고수위 및 사이드카 권한

sqlite 전환 후 WAL 파일이 checkpoint 후에도 truncate되지 않고 고수위로 유지되는 현상(M1), 사이드카 파일이 0600이 아닌 권한으로 생성되는 현상(M2)이 관측되었다.

주의:
- `sqlstore.Maintain`이 `PRAGMA wal_checkpoint(TRUNCATE)` + 사이드카 0600 재보증을 수행한다. `state maintain` CLI 또는 session-start hook의 `MaybeMaintainStateStores(24h)`가 자동 호출한다.
- 세션 바인딩은 done/absent 사이클에도 잔존한다. `issueops cleanup stale --apply`로만 정리되며, dry-run(`--apply` 없음)은 보고만 한다.
- VACUUM은 DB가 수십 MB로 성장하기 전에는 비용만 있고 이득이 없어 비범위다(ADR 참조).
- `.last-store-maintain` sentinel은 state root에 생성되며 state doctor가 인식한다. sentinel은 에러 시에도 touch되어 폭주를 방지한다.

## Orca create 호출의 모호한 실패를 재시도하지 말 것

Orca worktree/terminal/task create 또는 dispatch는 프로세스 timeout/error가 mutation 부재를 뜻하지 않는다. 호출 전 IssueOps `pending_operation`을 durable하게 기록하고, 호출 뒤 실패하면 `recovery_required`로 멈춘다. 같은 create를 자동 재시도하거나 inline fallback을 시작하면 중복 worker가 생길 수 있다.

- `issueops handoff recover --action reconcile`은 persisted baseline/marker 대비 정확히 하나의 후보만 받아들인다. 후보가 0개거나 여러 개면 fail closed 상태를 유지한다.
- Orca 1.4.134의 terminal create 응답에서 `ptyId`는 선택적이다. adapter가 이를 필수로 거부하지 말고, core가 create 전 baseline과 create 후 terminal list를 비교해 exact worktree의 connected/writable PTY delta가 정확히 하나인지 검증한다. create가 PTY ID를 돌려준 경우에는 그 delta와 일치해야 한다.
- `auto` fallback은 read-only readiness probe가 mutation 전에 실패한 경우에만 허용한다.
- cycle lock 안에서는 record CAS만 수행하고 외부 Orca CLI를 호출하지 않는다.
- fresh Orca terminal의 native hook은 기본 IssueOps state root를 조회한다. custom `HARNESS_STATE_DIR` cycle은 별도 전파 없이는 SessionStart/PreToolUse에서 보이지 않으므로 hook을 우회하거나 성공으로 간주하지 않는다. 안전한 custom state-root 전파는 issue #17 범위이며, V1 live hook 증거는 기본 state에서 수집한다.
- Codex 0.144.1 공식 `rust-v0.144.1`(44918ea)은 session setup에서 hook을 초기화하지만 `refresh_runtime_config`가 hook을 다시 build/store하는 경로도 제공하고, `pre_tool_use.rs`는 현재 session id를 payload에 넣는다. 관측된 live worker에서는 install-native가 `~/.codex/hooks.json`을 교체해도 active session command가 갱신되지 않았다. 따라서 파일 readback만으로 runtime 적용을 주장하지 말고 current-session live probe를 권위로 삼는다. installer의 `--host codex`는 유지하고, retained command 호환은 payload host와 CLI `--host`가 모두 비었을 때만 Codex로 정규화한다. 이 경우에도 exact nonempty session, canonical cwd/repo, persisted fence, in-tree target 검사를 모두 유지하며 명시 host는 절대 덮어쓰지 않는다. binary 재설치 후 같은 worker에 허용하는 mutation 재시도는 정확히 한 번뿐이다.
- Codex 0.144.1 PreToolUse payload는 repo 밖 rollout을 가리키는 top-level `transcript_path`를 항상 포함하고 subagent에서는 `agent_transcript_path`도 포함할 수 있다. 이를 일반 `*_path` edit target으로 재귀 수집하면 정상 in-tree patch가 외부 target으로 오판되어 block된다. 두 키는 `tool_input` 밖에서만 hook metadata로 제외하고, `tool_input` 안의 동일 키·file path·patch target은 계속 검사한다. 라이브 재시도 전 probe는 transcript metadata까지 포함한 full payload여야 하며, 이를 생략한 synthetic allow 결과만으로 성공을 주장하지 않는다.
- Live worker의 첫 `handoff finish`는 shell-quoted `--verification` 값 안의 세미콜론 때문에 false block됐다. raw `strings.ContainsAny(";&|\\n\\r")`는 quote 안의 evidence data와 실제 compound operator를 구분하지 못한다. lifecycle guard는 quote-aware scan으로 quote 밖의 `;`, `&`, `|`, CR/LF만 차단하고, quoted punctuation은 그대로 허용해야 한다. 빈 native agent id는 이 장애의 원인이 아니지만 `SplitCommandTokens`가 quoted empty argument를 버리므로, `--agent-id ''`를 렌더하지 말고 flag 자체를 생략한다.
- supervised source checkout은 observation-only다. `git status/diff/log/show/rev-parse/ls-files`와 `rg` 같은 비실행 관찰만 허용하고, test/build/format/install/generate는 claim된 worker root에서만 실행한다. 테스트 초기화와 fixture도 파일·프로세스·네트워크를 바꿀 수 있어 read-only로 분류하지 않는다.
- supervised readiness를 통과시키려고 현재 cycle과 무관한 legacy plan을 link하지 않는다. `coordinator_preparing`에서 current issue/cycle intent와 acceptance criteria, exact branch/path/base, exact bounded worker scope, claim/finish/accept, verification, cleanup을 담은 Markdown만 plan-only coordinator commit으로 보존하고, clean exact branch에서 `link-plan`이 그 commit을 새 attempt base head로 고정한 뒤 dispatch한다. Report-only는 해당 disposable cycle이 그렇게 선언했을 때만 적용하며 production implementation worker까지 일반화하지 않는다.
- zsh의 unquoted word-leading `=git`은 command-path expansion이고 `=(...)`는 프로세스를 실행해 임시 경로를 만든다. active command/process substitution, parameter/tilde, brace/glob pathname expansion, `eval`/`source`를 supervised shell에서 차단하고 quoted/escaped literal만 데이터로 취급한다.
- Turing report는 worker root 기준 canonical relative path만 저장한다. accept 전에 leaf를 `Lstat`하여 symlink를 거부한 뒤 실제 regular file, clean worktree, committed diff를 검증한다. `EvalSymlinks` 후 `Stat`만 하면 in-root symlink가 증거 파일로 가장될 수 있다.
- publish 검증에서 `test "$(git rev-parse ...)"`처럼 command substitution을 쓰지 않는다. `git rev-parse --verify refs/heads/<branch>`를 standalone observation으로 실행해 stdout이 accepted FinalHead와 exact한지 확인한 뒤 별도 exact branch push와 explicit draft head/base create를 실행한다.
- freeform durable evidence는 opaque `Authorization: Bearer <value>`와 `api_key=<value>`를 각각 독립적으로 redaction한다. Failure.Message는 optional이지만 값이 있으면 bounded/redacted여야 하고, bounded string-list validator도 raw secret을 직접 거부해야 한다.
- coordinator plan file edit 권한은 target path만으로 정하지 않는다. hook request의 CWD와 repo identity가 둘 다 exact `record.Repo` source coordinator root여야 한다. feature-worktree/claimed-worker session이 child plan에 직접 쓰거나 bare PTY에 mutation을 주입하면 target-side hook surface를 우회할 수 있으므로 차단한다.
- raw Orca terminal steering은 claimed worker와 non-source session에서 금지한다. 설치 help의 `send/stop/create/switch/focus/close/rename/split` 및 write/input/type/paste control alias는 모두 mutation/control로 취급하고 `list/show/read/wait`만 observation으로 둔다. 유일한 예외는 이미 `claimed`인 handoff에 exact source coordinator root가 uniquely matching persisted worker terminal handle로 `orca terminal send --terminal <handle> --text '# agent-harness guidance: <single-line-literal>' --enter --json`을 보내는 경우다. Decoded guidance의 ASCII C0/DEL은 backspace·tab·ESC로 comment marker를 지우거나 PTY를 제어할 수 있으므로 차단한다. Preparation/dispatch는 `issueops handoff start`를 사용하며 target hook가 injected shell을 막아줄 것이라고 가정하지 않는다. payload는 한 argv로 전달하거나 POSIX single-quote encoder를 정확히 한 번 적용하고 JSON double-quote·shell/JS template interpolation을 중첩하지 않는다.
- `closed/worker_failed` 또는 `closed/cancelled` 뒤 source coordinator는 persisted mailbox handle에 대한 exact `orca terminal close --terminal <handle> --json`만 선택적 cleanup 시도로 실행할 수 있다. active/accepted state, worker/non-source session, 다른 handle, extra flag, `stop/create/send`는 계속 차단한다. 이 예외는 historical mailbox identity를 일반 live-control 권한으로 승격하지 않으며, close 성공 자체도 spawned PTY 전체 정리 증거가 아니다. exact worktree removal 뒤 terminal inventory로 각 handle/PTY의 absent 또는 disconnected 상태를 다시 확인한다.
- Codex 0.144.1 supervised startup에서 hook definition이 modified/untrusted이면 fresh terminal은 `Hooks need review`에서 SessionStart 전에 멈출 수 있다. #16은 Codex app-server/fingerprint를 자동 구현하지 않는다. exact worker cwd의 public `hooks/list`를 read-only로 확인해 warnings/errors empty, required SessionStart/PreToolUse enabled, 모든 untrusted/modified entry가 exact generated agent-harness command임을 증명한 뒤에만 per-attempt `--allow-codex-hook-trust-bypass`를 쓴다. 첫 unattested preview 뒤 review, attested second no-confirm preview, reviewed context hash 기록, `--confirm`만 추가한 identical mutation 순서를 지킨다. 이 additive bool은 legacy retry를 깨지 않도록 ContextVersion 1이며 retry 때 false로 reset된다. 자동 trust lifecycle은 issue #17이다.
- `orca orchestration send --type` prefilter는 direct `orca orchestration send`의 explicit type만 검사한다. 8개 installed value 밖의 unique type 또는 duplicate type은 record selection 전에 차단하고 enum을 안내한다. type 생략/valid value는 새 권한을 부여하지 않고 기존 policy로 그대로 넘긴다.
- `orca orchestration check`는 기본이 unread이고 `--all --inject`는 더 많은 history를 주입할 수 있다. repeat-prevention PreToolUse guard는 direct check의 any explicit `--inject`(equals/reordered 포함)를 record lookup 전에 차단한다. 먼저 `orca orchestration check --all --json`으로 read-only inventory를 보고 exact current task id, dispatch id, sequence를 고른다. live terminal handle을 historical mailbox identity로 간주하지 않으며 urgent worker correction은 uniquely persisted handle의 literal-safe source-coordinator guidance만 사용한다. 자동 handle/mailbox 동기화는 issue #17이다.
- 별도의 legacy-JSON workpool reminder defect는 이 handoff 변경과 무관한 follow-up이다. SQLite workpool 상태를 읽지 못하는 기존 reminder를 이 branch에서 함께 고치거나 handoff recovery와 결합하지 않는다.

## IssueOps ownership 필드를 root schema bump 없이 추가하지 말 것

`execution_handoff`처럼 mutation authority를 소유하는 필드를 기존 root schema에 additive `omitempty`로만 추가하면, 그 필드를 모르는 이전 binary가 unknown JSON을 버린 뒤 같은 schema로 rewrite할 수 있다. IssueOps root는 v2이며 missing/zero/v1 row를 보존해 upgrade하지만, v1 binary는 v2를 future schema로 byte-equivalent reject해야 한다.

- 새 ownership/security 필드는 root schema compatibility를 명시적으로 검토하고 legacy decoder-writer rejection fixture를 둔다.
- future schema hook scan은 row 전체를 해석하지 않고 bounded repo/worker identity와 invalid marker만 유지해 mutation을 fail-closed한다.
- CLI, daemon, Codex, Claude, GJC installed binary가 같은 schema를 읽는지 migration smoke로 확인하기 전에는 mixed-version handoff를 시작하지 않는다.
