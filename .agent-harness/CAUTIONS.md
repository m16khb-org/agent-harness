---
name: CAUTIONS.md
description: Recurring mistakes, operational cautions, and avoidance guidance.
---

# 주의사항 모음

`agent-harness`에서 반복적으로 실수하기 쉬운 설계·운영 주의사항을 모은다.

Native hook 설치는 lifecycle worktree의 binary를 영구 target으로 쓰거나 실행 중 inode를 제자리 덮어쓰면 안 된다. invoking checkout에서 build하되 Git common-dir의 source `bin/agent-harness`에 staged-file fsync와 atomic rename으로 활성화하고, 이전 target을 캐시한 host session은 재시작한다.

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
- advertised schema가 unknown key를 허용했다면 canonical intent와 달라도 model fault로 분류하지 않는다. `advertised_valid`와 `canonical_valid`를 따로 기록한다.
- `additionalProperties`가 없는 implicit open object를 새로 만들지 않는다. 자유형 map은 schema owner가 `additionalProperties:true`를 명시해야 한다.
- unknown key 삭제, alias 적용, Unicode 수정, string/CSV/bool coercion 같은 silent repair로 malformed call을 성공시키지 않는다.
- raw argument drift는 capture-only probe에서 production handler보다 먼저 관측하고, 동일 signature가 재현되기 전에는 production validator나 tracked regression fixture로 승격하지 않는다.
- `failure_cause`는 typed evidence로만 올리고, evidence가 없거나 상충하면 `unknown`을 유지한다. 기존 반복 패턴 축인 `failure_class`를 덮어쓰지 않는다.

---

## 8. IssueOps local-only branch 착각

IssueOps 이슈 브랜치를 `git worktree add -b`로 바로 만들면 GitHub/GitLab 이슈 화면에 branch 생성 기록이 남지 않는다.

주의:
- 이슈 기반 worktree를 만들기 전에 provider-linked branch를 먼저 생성한다.
- 단, GitHub Orca는 원격 이름 충돌을 피하려고 branch prepare 기록과 Orca
  `execution prepare`를 먼저 수행한 뒤 linked branch를 생성하고
  `--link-verified`로 갱신한다. marker identity는 provider/issue 일치로 봉인하며
  아직 생성할 수 없는 linked branch 확인을 Orca prepare 전제로 삼지 않는다.
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
- macOS actual-socket QA는 Unix-domain socket 경로 길이 제한을 피하도록 `/tmp/ahd-*`처럼 짧은 임시 root를 사용한다. 기본 `t.TempDir()`의 긴 `/var/folders/...` 경로는 구현과 무관한 `bind: invalid argument`를 만들 수 있다.
- QA launcher가 daemon child의 parent라면 `daemon stop`과 parent의 `Wait`를 동시에 진행해 SIGTERM 종료 자식을 즉시 reap한다. unreaped zombie는 `kill(pid, 0)`에 살아 있는 것으로 보여 fail-closed forced-stop 검증을 오탐할 수 있다. 모든 QA는 `defer`/`finally` 정리 후 임시 binary, state root, PID, socket이 0개인지 확인한다.
- daemon socket/pid/log는 user state dir에 두고 repo나 wiki vault에 쓰지 않는다.
- **D2 (NFS caveat, accepted)**: daemon single-instance locking은 `daemonlock/lock.go`의 `O_EXCL` create + stale(30s)/PID-liveness 감지로 막는다. lock 파일은 startup handoff 후 child가 삭제하므로(transient) flock fallback은 부적합하다(flock은 inode에 묶여 삭제 시 깨짐). `O_EXCL`은 NFS/FUSE에서 원자성이 보장되지 않으니 **daemon state는 로컬 FS에 둔다**; 네트워크 마운트 home에서는 이론상 두 daemon이 뜰 수 있으나 두 번째는 동일 unix socket bind에서 실패한다.

## 14. Codex vs Claude Code hook rendering drift

Codex and Claude Code accept similar UserPromptSubmit JSON, but they do not render it the same way.

주의:
- Codex shows `hookSpecificOutput.additionalContext` in the TUI `hook context:` row, so anything injected for the model is also visible to the user and may be newline-collapsed.
- Claude Code can use `systemMessage` as the user-visible channel while keeping `additionalContext` as model-facing context.
- Do not assume a hook field is hidden just because another host hides it. Verify the installed host runtime/schema before changing hook output.
- Codex 0.144.1 rejects PreToolUse `hookSpecificOutput.permissionDecision="ask"` with `unsupported permissionDecision:ask`. Generic Codex ask-style gates fall back to `decision="block"`; kubectl live-access uses the bounded one-shot flow below. Hosts with native ask support keep `permissionDecision="ask"`.
- Codex `hook returned invalid <event> JSON output` means hook stdout looked like JSON but failed strict serde parsing (`deny_unknown_fields`; unknown top-level field, unknown enum value, or truncated/multi-object stdout). It is NOT a generic failure label — 원인 후보를 그 세 가지로 좁히고, 용의 훅 stdout을 `... | cat | wc -c` 파이프 하류에서 재현해 확인한다. Node/Bun 기반 외부 훅은 stdout이 파이프일 때 큰 출력을 512B에서 자르고 종료할 수 있다(2026-07-08 incident 참조). agent-harness Go 훅은 동기 write라 이 문제가 없다.
- For Codex, keep the project-doc catalog in `additionalContext` because the agent needs it, but avoid route/action/profile/pending-upkeep status prose there.
- Keep project-doc frontmatter descriptions concise English metadata; `project bootstrap` and `project bootstrap --sync` use this canonical metadata, so verbose descriptions multiply across every target repo.

## 15. IssueOps worktree edits must be hook-guarded

IssueOps worktree isolation cannot rely on the model remembering `pwd` or shell `workdir`. Some edit tools can apply relative paths from a different checkout than the shell command just verified.

주의:
- IssueOps sessions set `HARNESS_SOURCE_CHECKOUT` and `HARNESS_EXPECTED_WORKTREE` before implementation.
- Installed Codex and Claude PreToolUse hooks include `--enforce-worktree` and block mutating tool events outside the expected worktree when that env is set.
- Installed Codex and Claude PreToolUse hooks include `--enforce-gitops-kubectl`; direct mutating `kubectl` commands such as `apply`, `delete`, `patch`, or `rollout restart` must be represented as manifest changes in git and applied through the repo's GitOps path. Claude keeps native `ask` for `kubectl exec` and `port-forward`. Codex `port-forward` uses the exact-command one-shot token with a 10-minute pending/granted TTL. Codex read-only exec session approval requires explicit kube context and namespace and accepts only the exact DNS (`getent hosts`, `nslookup`, bounded `dig`), `/etc/resolv.conf`, and Linkerd localhost metrics grammars. Entering `승인 AH-XXXXXX` creates a 10-minute activation window; the first allowed diagnostic and each subsequent allowed command refresh a 30-minute idle TTL for the same session, canonical repo, context, and namespace, while target/container may vary. Unsafe or unclassified Codex exec blocks without a token and cannot be approved around. Project-scoped user state uses mode `0600` and stores fingerprints rather than raw commands or cluster identifiers. Expired/lost grants require a new token; do not disable the GitOps gate as routine recovery. Read-only commands and dry-runs such as `kubectl get`, `logs`, `diff`, and `apply --dry-run=server` remain allowed.
- Installed Codex and Claude PreToolUse hooks include `--enforce-staged-checks`; broad Biome lint/format commands such as `biome check apps libs` or package scripts that expand to broad `apps libs` checks ask for explicit user confirmation. Prefer staged or changed-file commands such as `biome check --staged`, `biome format --staged`, lint-staged, or explicit changed file lists so existing repo debt does not become the current diff's failure.
- Manual edit rules still require absolute paths rooted at the expected worktree and status checks for both source checkout and worktree, because host hook coverage can differ by runtime.
- If a source checkout receives implementation edits by mistake, stop, move only your own changes into the IssueOps worktree, and verify the source checkout is clean before continuing.
- Late-promotion gap: when work starts under `/goal`, ralph, or ad-hoc edits and is only promoted to IssueOps at the issue/PR phase, the worktree-first trigger ("`$issueops` explicitly invoked") never fires, so implementation can land in the source checkout on a feature branch without an isolated worktree. Observed 2026-06-03: gap-closure work was committed directly on `feat/...` in the main checkout; main ref stayed clean but the isolation contract was skipped. Mitigation: when a cycle is promoted to IssueOps, move remaining work into an isolated worktree or explicitly record the deviation. Implemented (issue #25) and hardened 2026-06-05: the `--enforce-worktree` PreToolUse guard judges by the current work's own cycle. IssueOps cycle ids are deterministic per (repo, branch) and `issueops start` resumes instead of duplicating, so cycles cannot accumulate as stale duplicates. The guard reads the current branch from `.git/HEAD` and loads only that branch's cycle (`ActiveIssueOpsCycleForBranch`). In `implement`/`ai-slop-clean`/`feedback`/`pr`, a missing `worktree_path` is now fail-closed for mutating source/worktree targets: create the sibling worktree and run `issueops link-worktree` before editing. The guard also blocks `git checkout -b`/`git switch -c` of a known IssueOps branch in the source checkout, while allowing `git worktree add ../<repo>.worktrees/...` as the preparation step. Legacy timestamp-id records and other branches' cycles have different ids and are never read, so they cannot cause a false linked-worktree lock; marking the cycle `done` releases the source checkout.
- Tool-root drift: IssueOps may run implementation in a sibling worktree while a host session and some MCP servers remain rooted at the original source checkout. In `api-servers`, verified 2026-06-05, Claude `.mcp.json` was gitignored and its `codegraph` server had `--path /Users/habin/workspace/api-servers`, while Codex global `codegraph` was `codegraph serve --mcp` without `--path`. Do not solve this by asking the user to restart Claude Code in the worktree; preserve the current session and enforce per-call evidence instead.
- External code-intelligence tools are the user's own installs; the harness no longer prepares their indexes or validates their `projectPath`. When using one in a worktree session, root every call at the expected worktree yourself.
- In IssueOps worktrees, prefer native absolute-path file tools, `git -C "$HARNESS_EXPECTED_WORKTREE"`, `rg` rooted at the expected worktree, and worktree-local tests for correctness. Do not let a CodeGraph/Serena-first project rule override direct worktree evidence. Treat filesystem/Serena MCP tools as blocked or advisory unless their root is proven to be the expected worktree in the current session.
- A merged recordless orphan is not permission to bypass the direct `git worktree remove` guard. Use `issueops cleanup orphan` preview and its exact fingerprinted `--apply --confirm` path only; it fails closed for any record, lifecycle/lease, or Orca authority, never creates a lifecycle, and never deletes the remote branch.

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
- When readiness reports `intent_contract`, `plan_prep_*`, `branch_prepare`, compatibility/design/plan evidence, canonical workspace/lease, `ai_slop_clean`, or contract feedback, run the owning `issueops` command in the main-agent loop and retry readiness. Workspace and write authority belong to `issueops execution prepare/status/claim/release/replace/reconcile/switch-mode/complete`; do not add a hook-side workaround.
- `execution replace --revoke`는 **다른** 홀더를 걷어내는 명령이다. 자기 lease에 실행하면 `revoking` 상태와 살아 있는 홀더가 겹쳐 claim·release·replace가 모두 막힌다(2026-07-26 실측, 세션 재시작으로만 벗어났다). 자기 정리는 `release --generation N`이다. #170이 이 자기-revoke를 거부하도록 고쳤지만, 진단이 막히는 상태를 스스로 만들지 않는 규율이 먼저다.
- `--session-id`는 host가 부여한 세션 식별자이며 조합해 만드는 값이 아니다. 세션을 재시작해도 그 값은 유지되고 PID만 바뀌므로, 재시작 후의 `holder_identity_mismatch`는 홀더가 죽었다는 뜻이 아니라 잘못된 id가 기록됐다는 뜻이다. `execution whoami`로 확인한다.
- Sub-agent use is not a free speedup. It can preserve main context, give a fresh reviewer, isolate tools, or parallelize independent research, but it also reduces mid-run steering/visibility and adds latency/token/coordination cost. Record the documented pattern, tradeoffs, fallback, and net-positive rationale in the plan/evidence; if that rationale is weak, direct main-agent execution remains the default.

## 16.2 선택지는 기존 목표와 execution holder를 대체하지 않는다

숫자 선택 응답을 복원할 때 선택지 label만 새 원문 요청으로 승격하면 이전 턴에서
확정한 실행 주체, workspace, execution mode가 사라질 수 있다. `Orca owner에게
실행을 위임`이 `격리된 구현 worktree에서 실행`으로 축약된 뒤 source/main이 직접
구현한 2026-07-22 incident가 이 경로로 재현됐다.

주의:
- 선택지는 기존 사용자 목표와 명시적 제약을 유지한 채 적용하는 next-action delta다. 선택지 text를 원문 요청의 대체물로 취급하지 않는다.
- 실행 방식이 갈리는 선택지에는 실행 주체, source/canonical worktree 위치, direct/Orca mode, 외부 mutation 경계를 self-contained하게 적는다.
- Active cycle에서는 durable generation/holder/workspace state가 자유형 선택지보다 우선한다. `격리된 worktree`만 보고 실행 주체를 추정하지 않는다.
- Orca launch 뒤 source/main은 owner 완료를 동기 대기하거나 terminal output을 무기한 tail하지 않는다. 한 번의 관측은 30초 이내로 제한하고, 60초 안에 사용자에게 진행 상태를 갱신한다.
- Claim 또는 native process가 멈추면 source가 구현을 대신하지 않는다. `issueops execution status`를 읽고 generation-CAS replacement/reconciliation 판단 지점으로 돌아간다.
- 코드·commit·publish·PR/MR mutation은 active generation holder의 canonical worktree에 남긴다. Source main worktree는 unrelated work에 계속 사용할 수 있다.

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
- `orca orchestration send --type` rejects values outside `status|dispatch|worker_done|merge_ready|escalation|handoff|decision_gate|heartbeat`. Verify the installed CLI when this enum changes; do not improvise `progress`, `blocked`, or `completed` message types.

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
- 모든 `decision:"block"` Stop branch는 `stop_hook_active` continuation과 `자동진행하지 않음` exit를 평가한 뒤에만 재차단할 수 있다. Durable decision state가 그대로라는 이유로 이 guard보다 먼저 무조건 반환하면 같은 hook episode가 영구 재진입한다. 해당 branch 안에서 즉시 `{}`를 반환해야 아래의 독립 relay가 다시 block하지 않는다. 최초 no-auto no-op, ordinary fresh block, relay-enabled choice continuation no-op, 다음 독립 episode 재알림을 한 회귀 테스트로 고정한다.
- Stop hooks accept only the stop-control schema (`continue`/`decision`/`reason`/`stopReason`/`systemMessage`/`suppressOutput`). Injecting `hookSpecificOutput.additionalContext` on Stop makes Codex report "invalid stop hook JSON output"; use a no-op `{}` payload when not blocking.
- The ONLY Stop-hook output reliably surfaced to the user is `decision:"block"` + `reason` — it renders as "Stop hook feedback" AND re-invokes the agent in-turn. Two channels were observed to produce NO visible notice (2026-06-04): a non-blocking `{"systemMessage": ...}` (turn allowed to end), AND `{"continue": false, "stopReason": ...}` — despite the doc claiming "systemMessage — display to the user (all hooks)" and "stopReason — shown when continue is false". Do not rely on either to notify the user from a Stop hook.
- Claude Code labels a successful Stop `decision:"block"` relay as `hook_blocking_error` in the transcript attachment and can surface it as `stop-hook-error` in stream/UI output. Do not treat that label alone as an agent-harness hook process failure. Check the hook command exit code/stderr plus the follow-up `stop_hook_summary`: an intended next-action relay has `preventedContinuation:false`, `level:"suggestion"`, and an empty failure stderr even though the display name says "error". Treat it as a real failure only when the process failed, stderr names a schema/runtime error, or continuation was actually prevented.
- Consequence: you cannot both stop-and-wait AND show the user a message via raw Stop output — the only visible channel (`decision:"block"`) continues the agent. So when a Stop hook reaches a recoverable review point, return `decision:"block"` + a `reason` that instructs the agent to act on the observed facts. The follow-up Stop carries `stop_hook_active=true`; missing-choice recovery still no-ops on that follow-up to avoid loops.
- So the Stop outcomes are: (1) recover/continue in-turn → `decision:"block"` + `reason`; (2) next-action judgement relay → `decision:"block"` + observed facts for the main agent; (3) silent no-op → `{}`. `continue:false` is a hard stop that suppresses the visible feedback, so avoid it for notifications.
- The Stop hook should only treat numbered lines inside an explicit `선택지:`/`Options:`/`Next actions:` section as next-action choices. Explanatory numbered lists can contain words like `추천` and `자동진행` and must not be parsed as next-action choices.
- The Stop hook is not a judge, scorer, classifier, or safety gate. It must not claim "자동진행 후보", calculate scores/thresholds/confidence, classify destructive/safe/reversible/eligible choices, or decide whether the action should run. Its job is only to say that a next-action judgement point was reached and relay inspectable facts such as choice count, recommendation count, and recommended text. The main agent owns safety, reversibility, user-intent alignment, and proceed-or-ask judgement from current context, and must state that judgement in the recovery response: either why it is auto-proceeding now, or why it is not auto-proceeding and needs user confirmation. If it auto-proceeds, the result report still needs a `선택지:` section so the next action boundary remains explicit.
- A main-agent `no-auto-proceed` judgement is sticky. If the agent says it will not auto-proceed at a Stop-hook next-action boundary, an automated `/goal`/goal-continuation prompt must not immediately reinterpret the active objective as permission to resume the same action. Resume only after an explicit user choice or a new user instruction. Observed 2026-06-06: the agent said it would stop for diff review, then a goal continuation message arrived and the agent resumed implementation, contradicting the prior judgement.
- A `no-auto-proceed` Stop-hook recovery response should be allowed to stop without adding a new `선택지:` block. Repeating the choices in that response creates a fresh next-action judgement point and can produce the exact "recommend -> no-auto-proceed -> recommend" loop. The missing-choice guard should require choices for ordinary final responses, but no-op when the final response explicitly says `자동진행하지...` / `no-auto-proceed`.
- 사람의 입력만 기다리는 판단 지점에서는 agent/background wait 도구를 호출하지 않는다. 현재 응답을 끝내고 다음 실제 사용자 turn에서 재개한다. 대기 도구로 turn을 붙잡는 것은 종료 경로가 아니며, Stop 재진입 결함과 결합하면 토큰만 소비하는 무한 대기가 된다.
- supervised IssueOps owner는 worker worktree에서 `scripts/install-native.sh` 또는 `agent-harness install`/`install-native`/`update`/`bootstrap`의 실제 실행으로 사용자 범위 통합을 교체할 수 없다. owner 빌드가 `~/.codex/hooks.json` 등을 가리키게 만들면 source checkout의 수정과 런타임 provenance가 분리되어 이미 고친 Stop 결함도 다시 활성화된다. `--dry-run` 검증은 허용하고 실제 설치·업데이트는 source checkout에서만 수행한다.
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

## 21. Worktree guard and execution liveness must use the same v1 fence

A guard that checks only branch or path can deadlock an absent holder or admit a
stale one. All readers must agree on the exact lifecycle ID, generation, native
process receipt, canonical worktree, and mode-specific resource identity.

주의:
- A block message must name one command that the current state actually allows. Start with `issueops execution status`; use its rendered claim, reconciliation, or replacement command rather than inventing an override.
- A missing directory, old timestamp, stable diff, or absent terminal row is not lease-release evidence. Replacement requires expected generation plus preview inventory and quiescence fingerprints.
- Hooks may read bounded state and reject a mismatched mutation, but must not call Git, provider, or Orca mutators, revoke a lease, or delete a worktree.
- One active execution exists per record, not per repository. Exact-ID selection must not capture an unrelated parallel cycle or make the source main worktree read-only.
- Completion is generation-fenced and requires phase `pr` plus verified remote evidence. Never force a non-PR cycle to `done` as a recovery shortcut.

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

## 23.1 Operational health is diagnosis, not cleanup authority

Cross-system residue must not acquire a second truth source in stale scan, status, or the stability script.

주의:
- `agent-harness doctor` is the sole public operational-health gate. `--preserve-cycle` and `--preserve-terminal` are exact, invocation-only inputs; never persist them as exemptions.
- A generic binding proves neither authority nor liveness. An active generation without a complete native process receipt, canonical worktree, and mode-specific identity is unhealthy even when other resource IDs match.
- Time is diagnostic only. Age never interrupts a holder, deletes a resource, or authorizes replacement; expected generation, exact inventory, and quiescence proof are required.
- Orca absence is optional only when no durable cycle claims Orca resources. Never turn a missing or incomplete Orca inventory into an empty healthy list.
- One-time global cleanup evidence lives in an external `0700` recovery bundle. Git/SQLite copies can be restore-tested, but Orca snapshot is archival-only: the public CLI has no conditional reset/import/restore and a last-moment external actor race remains. Stop on pre-reset digest drift; after reset, continue idempotently from the append-only journal instead of guessing rollback.
- A destructive IssueOps replacement must not rely on a runner-side observation followed by an unfenced write: record and process state can change in that gap. Use expected generation plus inventory/quiescence fingerprint CAS and journal the locked before/after proof.
- Replacement의 최초 `--preview`는 현재 generation을 찾는 읽기이므로 세대를 생략할 수 있지만, 이후 revoke/finalize/reseed는 preview가 돌려준 exact generation CAS를 요구한다. Orca finalize/reseed는 새 token을 만든 뒤 owner packet/prompt 재봉인까지 성공해야 durable claimable 상태를 기록한다. 실패하면 이전 durable 상태를 보존하고 아직 권한 없는 target generation의 token/packet/prompt residue를 정리하며, 다음 재시도도 같은 exact harness-owned 경로를 먼저 회수한다. GitLab MCP snapshot은 이 재봉인이 실제로 읽는 `replace --finalize|--reseed`에만 전달한다.
- A sealed cleanup must also seal its authorities: invoke a bundle-private clean-HEAD executor by hash/VCS revision, override live state/root/daemon/worker environment paths, require singleton equal fetch/push authorities, and push to the sealed explicit URL. Fetch/prune readiness, mutation, and readback must share that URL plus a heads-only refspec, `--no-tags`, and `--no-write-fetch-head`; never reopen mutable `origin` refspec/tag authority. Ignored `bin/agent-harness`, inherited environment, and mutable remote names are not execution evidence.

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
- `issueops execution prepare`가 반환한 canonical path를 `HARNESS_EXPECTED_WORKTREE`에 반영하고, 편집 전 cwd와 절대경로가 그 worktree를 가리키는지 확인한다.
- Worktree 세션에서는 file tool에 worktree 절대경로를 넘기고, shell은 `git -C "$HARNESS_EXPECTED_WORKTREE"` 또는 `rg "$pattern" "$HARNESS_EXPECTED_WORKTREE"`처럼 명시 root로 실행한다.
- Guard는 source checkout의 모든 edit를 막지 않는다. §21의 multi-path deadlock 방지 때문에 non-cycle branch에서 source checkout에 새 파일을 만드는 정상 작업은 허용되어야 한다.
- 방어층은 세 겹이다: PostToolUse source-checkout warning, PreToolUse mirror-file `ask`, SessionStart/UserPrompt worktree reminder. Host가 `ask`를 지원하지 않으면 Codex처럼 `block`으로 degrade될 수 있다.
- 선택된 cycle의 구현은 canonical worktree로 이동한다. Holder 교체가 필요하면 source에서 구현을 계속하지 말고 `issueops execution status`가 안내하는 generation-CAS replacement 절차를 따른다. Source checkout은 unrelated work에 계속 사용할 수 있다.

## Incident Archive

Dated incident notes are preserved in `.agent-harness/archive/cautions-incidents.md`. Keep this file focused on evergreen hazards and move one-off history there.

아래 날짜별 기록에 등장하는 제거된 IssueOps 명령·필드·상태는 사고 당시
증거일 뿐 실행 지시가 아니다. 현재 실행 계약은
`skills/issueops/references/execution.md`와
`.agent-harness/OPERATIONS.md`를 따른다. 역사적 증거는 삭제하거나 현재 표면으로
위장해 고쳐 쓰지 않는다.

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
- Summary: IssueOps child orchestration must preserve single-entity lock boundaries and additive mixed-binary compatibility.
- Context: Delegated child cycles add orchestration records that can be touched by multiple sessions. The existing store is per-key/per-entity atomic and host-neutral, not a cross-entity transaction manager or process supervisor.
- Resolution: Use only one entity lock at a time and never call a same-entity with*Lock helper from inside another same-entity lock callback. At the July 7 baseline the non-ownership orchestration fields remained additive under schema v1; issue #16 introduced schema v3 for supervised ownership/stable terminal identity, v4 for sealed mailbox/completion projection, and now v5 for publish/cleanup authority. Verify the active binary, docs, CLI/MCP schema, and daemon readback before trusting mixed-version state.
- Evidence:
  - .agent-harness/ARCHITECTURE.md actor model
  - .agent-harness/AGENT_WORKFLOW.md child-cycle contract
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
- Source: b9e293c 감사 중 CLI 테스트 600s 행 진단 (goroutine dump + 파이프 용량 실측)
- Summary: 시스템 전체 파이프 fd가 폭증하면(관측: 14,402개, codex 호스트 1개가 3,112개) xnu의 전역 파이프 버퍼 풀이 고갈되어 **새 파이프가 512바이트 최소 버퍼로 강등**된다(정상 16,384 — 100/100 실측). 이때 "쓰기 완료 후 읽기" 방식의 stdout 캡처 테스트 헬퍼는 512B를 넘는 JSON 출력(예: loop record-attempt 579B)에서 write가 영구 블록되어 go test 타임아웃 FAIL이 된다. 코드 회귀처럼 보이지만 머신 상태 문제다(부모 커밋에서도 동일 재현).
- Context: 증상은 간헐적이다 — KVA 압력이 변동하며 새 파이프가 16K↔512B를 오간다. 타임아웃/중단된 `go test` 실행을 `pkill -f 'go test'`로 죽이면 `.test` 바이너리가 고아로 살아남아 파이프 압력을 가중시킨다(양성 피드백). 6ee897d가 harnessapp response-contract 캡처는 동시 reader로 고쳤지만, 일부 CLI capture 헬퍼는 아직 write-then-read 패턴이다.
- Triage: (1) `ps -axo pid,etime,ppid,command | rg '\.test'`로 고아 테스트 바이너리 확인·제거, (2) `lsof -n | rg -c PIPE`로 총량과 `awk '{print $1,$2}' | sort | uniq -c | sort -rn`으로 최다 점유 프로세스 확인, (3) 신규 파이프에 nonblocking write를 가득 채우는 프로브로 실효 버퍼 크기 측정 — 512B면 KVA 고갈 확정.
- Resolution: 재발 방지는 완료됐다. stdout/stderr 캡처 테스트는 `internal/testsupport.CaptureStdout`, `CaptureStdoutAndError`, `CaptureStderrAndError`를 사용한다. 이 헬퍼들은 fn 실행 전에 reader goroutine을 시작하므로 파이프 버퍼 크기에 의존하지 않는다. `agent-harness doctor --json`은 `pipe_capacity_bytes`와 `pipe_capacity` 체크를 노출하고 8192B 미만이면 `pipe_capacity_degraded` warning을 낸다. 근본 완화는 여전히 파이프를 누수하는 장수 host 프로세스 재시작이다. `agent-harness mcp cleanup --apply`는 Darwin에서 검증된 고아 프록시만 정리하므로 살아 있는 host의 누수에는 효과가 없다. `go test`를 죽일 때는 `pkill -f 'go test'`가 아니라 `.test` 바이너리까지 함께 정리한다.

## 2026-07-28 — update의 MCP 수명은 host 소유이며 pending 요청은 자동 재생하지 않는다

- Kind: `caution`
- Source: `ah update` 중 Codex agent_harness MCP 연결 종료 재현과 Claude Code 2.1.220 `mcp list` 번들 확인
- Summary: update는 host가 소유한 stdio MCP 프로세스나 외부 MCP를 열거·종료·접속하지 않는다. agent-harness proxy는 daemon generation 교체 뒤 초기 handshake의 protocol/capability projection이 동일할 때만 세션을 복구한다.
- Resolution: 단일 요청과 구버전 NDJSON batch의 미완료 request ID는 자동 재실행하지 않고 `outcome=unknown`과 reconcile 요구를 반환한다. reconnect는 전체 20초 deadline과 host EOF cancellation을 공유한다. handshake projection이 달라지거나 initialize가 거부되면 proxy를 종료해 host 재연결을 유도한다. 사용하지 않는 SDK logging capability는 광고하지 않는다. GitLab MCP/personal wrapper 동기화는 `scripts/sync-glab-mcp.sh`를 수동 실행할 때만 수행한다.
- Cleanup boundary: `mcp cleanup --apply`는 Darwin에서만 exact current-checkout `agent-harness mcp`, `PPID=1`, verified executable/start time, signal 직전 동일 identity를 모두 만족한 프로세스를 종료한다. Linux 컨테이너 등에서는 `PPID=1`이 살아 있는 host일 수 있으므로 `skip-unsupported-platform`으로 거부한다. live-parent proxy, PID reuse, 다른 checkout, DBHub/Context7/Kordoc/개인 wrapper도 fail-closed로 건너뛴다.

## 2026-07-07 — SQLite sqlstore span 규율: active-root chain, per-root 직렬화, fresh start

- Kind: `caution`
- Source: JSON+flock → sqlite 전면 전환 세션 (사용자 결정: 전체 일괄 전환 + fresh start)
- Summary: 모든 상태 저장/락은 `internal/core/sqlstore`를 통해야 한다. span은 state root 단위로 직렬화되며, 전달된 context의 active-root chain에 이미 있는 root로 재진입하면 `*NestedSpanError`로 즉시 실패한다. 서로 다른 root는 문서화된 비순환 순서에서만 중첩할 수 있다. 레거시 JSON/lock 파일은 무시된다(마이그레이션 없음).
- Context: 4개 with*Lock 계열(issueops, session, state, worker)이 전부 sqlstore span으로 이동했다. 같은 root 재진입과 `A -> B -> A`는 self-deadlock 위험이 있지만, remote-create는 `remote-create-live/<id>` child root에서 main IssueOps root로 이어지는 실제 `A -> B` 순서를 필요로 한다.
- Resolution: 모든 lock helper는 `WithSpan(ctx, fn)`의 `spanCtx`를 내부 호출에 전달한다. 같은 root나 chain cycle은 금지하고, distinct-root 중첩은 호출부에 순서를 명시한다. 현재 허용된 production 순서는 remote-create child root 다음 main IssueOps root다. multi-entity 작업은 가능한 한 순차 single-span 단계 + read-repair로 유지한다. 새 저장 표면은 파일 I/O가 아니라 sqlstore bucket(Get/Put/List/Delete)으로 추가한다. `harness.db`/`harness.lock.db`와 그 sidecar(-wal/-shm/-journal)는 삭제하지 않는다. 테스트 픽스처는 raw 파일 쓰기 대신 `sqlstore.Open(dir).Put(bucket, id, raw)`로 심는다. 레거시 `<key>.json`/`.lock`/`.state-lock` 파일은 fresh start 정책상 읽지도 지우지도 않는다(doctor는 무시).
- Evidence:
  - internal/core/sqlstore/span_context_test.go의 same-root, distinct-root, cycle, cancellation 회귀 테스트
  - internal/core/issueops/issueops_remote_create_claim.go의 child-root-to-main-root context 전달
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
- 정상 `done` 전이는 해당 사이클의 세션 바인딩을 제거한다. 비정상 종료로 남은 done/absent 바인딩만 `issueops cleanup stale --apply`로 정리하며, dry-run(`--apply` 없음)은 보고만 한다.
- VACUUM은 DB가 수십 MB로 성장하기 전에는 비용만 있고 이득이 없어 비범위다(ADR 참조).
- `.last-store-maintain` sentinel은 state root에 생성되며 state doctor가 인식한다. sentinel은 에러 시에도 touch되어 폭주를 방지한다.

## Orca create 호출의 모호한 실패를 재시도하지 말 것

> 이 아래 supervised handoff 사건 항목들의 위협 모델·불변식 근거는 `ARCHITECTURE.md`의 "IssueOps handoff: threat model and invariants" 절을 참조한다. 여기서는 각 사건의 한 줄 교훈만 유지한다.
> 아래의 `issueops handoff ...`, legacy schema, coordinator/worker 명칭은 당시 사건을 식별하는 역사 표기이며 실행 명령이 아니다. 현재 복구·lease 제어는 `issueops execution status|claim|release|replace|reconcile|complete`만 사용한다.

Orca worktree/terminal/task create 또는 dispatch는 프로세스 timeout/error가 mutation 부재를 뜻하지 않는다. 호출 전 IssueOps `pending_operation`을 durable하게 기록하고, 호출 뒤 실패하면 `recovery_required`로 멈춘다. 같은 create를 자동 재시도하거나 inline fallback을 시작하면 중복 worker가 생길 수 있다.

- `issueops handoff recover --action reconcile`은 persisted baseline/marker 대비 정확히 하나의 후보만 받아들인다. 후보가 0개거나 여러 개면 fail closed 상태를 유지한다.
- force-abandon의 exact-candidate narrowing은 안정 ID가 있고 exact-vs-unrelated 분류 필드가 모두 채워진 post-baseline 비일치 행만 무시한다. ID 누락·중복이나 분류 필드 누락 행은 absence 증거가 아니라 ambiguity이므로 `{}` 같은 행을 unrelated로 취급하지 않는다.
- Orca 1.4.134의 terminal create 응답에서 `ptyId`는 선택적이다. adapter가 이를 필수로 거부하지 말고, core가 create 전 baseline과 create 후 terminal list를 비교해 exact worktree의 connected/writable PTY delta가 정확히 하나인지 검증한다. create가 PTY ID를 돌려준 경우에는 그 delta와 일치해야 한다.
- local repository symbols는 `.codegraph/`가 있으면 CodeGraph로 먼저 찾고, 없으면 `rg`와 직접 읽기로만 찾는다. web search는 local symbol discovery의 fallback이 아니다.
- `auto` fallback은 read-only readiness probe가 mutation 전에 실패한 경우에만 허용한다.
- cycle lock 안에서는 record CAS만 수행하고 외부 Orca CLI를 호출하지 않는다.
- durable record equality만으로는 checkout/context/report TOCTOU를 막지 못한다. context persist와 terminal/task/dispatch first-time journal은 lock 안에서 source fingerprint + exact branch/attempt-base HEAD + clean status를 다시 확인하고, claim/acknowledge/complete도 자기 filesystem evidence를 record equality 직후와 write 직전에 다시 검증한다. 단계 사이 drift가 나면 이미 완료된 terminal/task identity는 보존하고 새 pending journal 없이 멈춘다. 외부 호출 전 `started_at`을 post-call completion timestamp로 재사용하지 않는다.
- fresh Orca terminal의 native hook은 기본 IssueOps state root를 조회한다. custom `HARNESS_STATE_DIR` cycle은 별도 전파 없이는 SessionStart/PreToolUse에서 보이지 않으므로 hook을 우회하거나 성공으로 간주하지 않는다. 안전한 custom state-root 전파는 issue #17 범위이며, live hook 증거는 기본 state에서 수집한다.
- Codex 0.144.1 공식 `rust-v0.144.1`(44918ea)은 session setup에서 hook을 초기화하지만 `refresh_runtime_config`가 hook을 다시 build/store하는 경로도 제공하고, `pre_tool_use.rs`는 현재 session id를 payload에 넣는다. 관측된 live worker에서는 install-native가 `~/.codex/hooks.json`을 교체해도 active session command가 갱신되지 않았다. 따라서 파일 readback만으로 runtime 적용을 주장하지 말고 current-session live probe를 권위로 삼는다. installer의 `--host codex`는 유지하고, retained command 호환은 payload host와 CLI `--host`가 모두 비었을 때만 Codex로 정규화한다. 이 경우에도 exact nonempty session, canonical cwd/repo, persisted fence, in-tree target 검사를 모두 유지하며 명시 host는 절대 덮어쓰지 않는다. binary 재설치 후 같은 worker에 허용하는 mutation 재시도는 정확히 한 번뿐이다.
- Codex 0.144.1 PreToolUse payload는 repo 밖 rollout을 가리키는 top-level `transcript_path`를 항상 포함하고 subagent에서는 `agent_transcript_path`도 포함할 수 있다. 이를 일반 `*_path` edit target으로 재귀 수집하면 정상 in-tree patch가 외부 target으로 오판되어 block된다. 두 키는 `tool_input` 밖에서만 hook metadata로 제외하고, `tool_input` 안의 동일 키·file path·patch target은 계속 검사한다. 라이브 재시도 전 probe는 transcript metadata까지 포함한 full payload여야 하며, 이를 생략한 synthetic allow 결과만으로 성공을 주장하지 않는다.
- Completion의 shell-quoted `--verification` 값 안 세미콜론은 evidence data일 수 있다. lifecycle guard는 quote-aware scan으로 quote 밖의 `;`, `&`, `|`, CR/LF만 차단하고 quoted punctuation은 허용한다. `SplitCommandTokens`가 quoted empty argument를 버리므로 `--agent-id ''`를 렌더하지 말고 flag 자체를 생략한다.
- 관찰 명령을 세미콜론, 개행, 성공 조건 `&&`로 묶었다는 이유만으로 `unsafe_mutation` 처리하면 owner가 상태 확인조차 못 한다. 각 조각을 기존 exact read-only parser로 다시 검증하는 시퀀스와 고정된 `.codegraph` 존재/출력 probe만 정적으로 관찰로 인정한다. `&&`는 두 조각이 모두 exact reader일 때만 구분자로 허용하고, `||`·단독 `&`·파이프·리다이렉트·치환·미분류 명령·임의 분기 본문은 계속 fail-closed로 둔다.
- `rg`의 `-A5`/`-B5`/`-C5`/`-m5`는 짧은 숫자 옵션과 값이 한 token에 붙은 정상 관찰 표기다. active lifecycle에서 이를 미분류 mutation으로 막지 말되, 기존에 허용한 네 옵션과 10,000 이하 숫자에만 한정하고 알 수 없는 결합형은 계속 fail-closed로 둔다.
- Turing 증거 JSON의 구문 검증은 `jq empty <literal-json-file>` 한 파일 문법만 exact reader로 인정한다. stdin과 `/dev/stdin`·`/dev/fd/0` 같은 확장자 없는 alias, 여러 파일, 다른 filter, option, module/argument 주입, 리다이렉트는 이 좁은 검증 계약에 필요하지 않으므로 계속 fail-closed로 둔다.
- 프로젝트 문서 전체 읽기에 쓰는 `sed -n '<positive-line>,$p' <literal-file>`은 exact reader로 인정한다. 마지막 줄 표식은 시작점이 양의 정수인 이 범위에서만 허용하고, `$p` 단독·다른 sed 명령·option operand·stdin·리다이렉트·활성 shell expansion·word 시작의 unquoted comment는 계속 fail-closed로 둔다.
- Transferred ownership cycle에 관해서만 source checkout은 observation-only다: `git status/diff/log/show/rev-parse/ls-files`와 `rg` 같은 관찰은 가능하지만 그 cycle의 test/build/format/install/generate는 claimed worker root에서만 실행한다. 이 제약은 source main worktree의 unrelated work를 막지 않는다. 테스트 초기화와 fixture도 파일·프로세스·네트워크를 바꿀 수 있어 read-only로 분류하지 않는다.
- 같은 source checkout에 supervised cycle이 여러 개면 proven observation은 record 선택 전에 분류해야 한다. owner가 필요 없는 hardened `pwd`/`rg`/read-only Git/명시적 read tool까지 먼저 exact record를 고르게 하면 복구 self-lock이 된다. 단, 이 선처리는 source-matching supervised record가 실제로 둘 이상인 경우에만 적용해 일반 linked-worktree MCP guard를 우회하지 않는다. exact lifecycle parser는 문서화된 `agent-harness`, `bin/agent-harness`, `./bin/agent-harness` 표기만 받고 shell control·unknown flag는 계속 거부한다. `handoff start` identity flags를 `handoff recover`에 추정으로 붙이지 말고 실제 subcommand help/spec을 확인한다. 상세 사건 기록은 `ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md`를 참조한다.
- response-contract golden에는 gitignored project-local host 설치 여부를 raw boolean으로 남기지 않는다. `.claude`와 `.codex` skill presence는 같은 placeholder로 정규화한다. 머신 상태 때문에 golden이 실패하면 user artifact를 삭제하거나 update 결과를 그대로 수용하지 말고 실제 diff가 contract인지 environment drift인지 먼저 분리한다.
- bounded concurrency test는 최종 assertion이 요구하는 모든 reserve 상태를 기다려야 한다. channel A의 길이만 보고 channel B의 overflow/classifier goroutine도 준비됐다고 가정하면 third caller가 B를 먼저 차지해 false timeout이 된다. production limit을 느슨하게 고치지 말고 active slot과 bounded overflow reserve를 각각 관찰한 뒤 rejection을 시작한다.
- supervised readiness를 통과시키려고 현재 cycle과 무관한 plan을 link하지 않는다. Current issue/cycle intent와 acceptance criteria, exact branch/path/base, exact bounded worker scope, claim/acknowledge/complete, verification, cleanup을 담은 Markdown만 plan-only source commit으로 보존하고, clean exact branch에서 `link-plan`이 그 commit을 attempt base head로 고정한 뒤 dispatch한다. Report-only는 해당 disposable cycle이 그렇게 선언했을 때만 적용하며 production implementation owner까지 일반화하지 않는다.
- zsh의 unquoted word-leading `=git`은 command-path expansion이고 `=(...)`는 프로세스를 실행해 임시 경로를 만든다. active command/process substitution, parameter/tilde, brace/glob pathname expansion, `eval`/`source`를 supervised shell에서 차단하고 quoted/escaped literal만 데이터로 취급한다.
- zsh의 `status`는 read-only 예약 parameter다. 검증 wrapper에서 `status=$?`를 쓰면 실제 명령이 성공해도 wrapper가 실패한다. exit code를 저장해야 하면 `rc` 또는 `exit_code`를 쓰고, test verdict와 wrapper bookkeeping 오류를 별도로 보고한다. 2026-07-11 incident에서는 `go test ./internal/core/issueops`가 `ok`였고 그 다음 `status=$?` 대입만 실패했다.
- Markdown backtick이 들어간 `rg`/shell search pattern을 double quote로 감싸면 backtick 안의 단어가 command substitution으로 실행된다. pattern 전체를 single-quoted literal로 주거나 shell을 거치지 않는 literal argv로 전달한다. 2026-07-11 incident의 double-quoted backtick-wrapped status 검색은 실제 `status` 명령을 실행해 `command not found`를 냈다.
- Turing report는 worker root 기준 canonical relative path만 저장한다. Complete 전에 leaf를 `Lstat`하여 symlink를 거부한 뒤 실제 regular file, clean worktree, committed diff를 검증한다. `EvalSymlinks` 후 `Stat`만 하면 in-root symlink가 증거 파일로 가장될 수 있다.
- publish 검증에서 `test "$(git rev-parse ...)"`처럼 command substitution을 쓰지 않는다. `git rev-parse --verify refs/heads/<branch>`를 standalone observation으로 실행해 stdout이 completed FinalHead와 exact한지 확인한 뒤 별도 exact branch push와 explicit draft head/base create를 실행한다.
- freeform durable evidence는 opaque `Authorization: Bearer <value>`와 `api_key=<value>`를 각각 독립적으로 redaction한다. Failure.Message는 optional이지만 값이 있으면 bounded/redacted여야 하고, bounded string-list validator도 raw secret을 직접 거부해야 한다.
- coordinator plan file edit 권한은 target path만으로 정하지 않는다. hook request의 CWD와 repo identity가 둘 다 exact `record.Repo` source coordinator root여야 한다. feature-worktree/claimed-worker session이 child plan에 직접 쓰거나 bare PTY에 mutation을 주입하면 target-side hook surface를 우회할 수 있으므로 차단한다.
- raw Orca terminal steering은 claimed worker와 non-source session에서 금지한다. 설치 help의 `send/stop/create/switch/focus/close/rename/split` 및 write/input/type/paste control alias는 모두 mutation/control로 취급하고 `list/show/read/wait`만 observation으로 둔다. 유일한 예외는 이미 `claimed`인 handoff에 exact source coordinator root가 uniquely matching persisted worker terminal handle로 `orca terminal send --terminal <handle> --text '# agent-harness guidance: <single-line-literal>' --enter --json`을 보내는 경우다. Decoded guidance의 ASCII C0/DEL은 backspace·tab·ESC로 comment marker를 지우거나 PTY를 제어할 수 있으므로 차단한다. Preparation/dispatch는 `issueops handoff start`를 사용하며 target hook가 injected shell을 막아줄 것이라고 가정하지 않는다. payload는 한 argv로 전달하거나 POSIX single-quote encoder를 정확히 한 번 적용하고 JSON double-quote·shell/JS template interpolation을 중첩하지 않는다.
- Cleanup에서 terminal close 성공 자체는 spawned PTY 전체 정리 증거가 아니다. Exact worktree removal 뒤 terminal inventory로 각 handle/PTY의 absent 또는 disconnected 상태를 다시 확인한다. Active/cleanup-unapproved state, worker/non-source session, 다른 identity, extra flag, create/send는 계속 차단한다.
- `orca orchestration send --type` prefilter는 direct `orca orchestration send`의 explicit type만 검사한다. 8개 installed value 밖의 unique type 또는 duplicate type은 record selection 전에 차단하고 enum을 안내한다. type 생략/valid value는 새 권한을 부여하지 않고 기존 policy로 그대로 넘긴다.
- `orca orchestration check`는 기본이 unread이고 `--all --inject`는 더 많은 history를 주입할 수 있다. repeat-prevention PreToolUse guard는 direct check의 any explicit `--inject`(equals/reordered 포함)를 record lookup 전에 차단한다. read-only JSON envelope의 message array는 `.result.messages`에 있다. top-level `.messages`를 조회하면 오류 없이 빈 결과가 나올 수 있으므로 absence 증거가 아니다. opaque `msg_*` ID를 lexical order/filter에 쓰지 말고 numeric `sequence`와 exact `taskId`/`dispatchId`, sender/recipient direction을 선택한다. Sequence는 selection evidence일 뿐 lease fence가 아니다. exact executable projection은 `skills/issueops/references/orca-handoff.md`만 원본으로 둔다. live terminal handle을 historical mailbox identity로 간주하지 않으며 urgent worker correction은 uniquely persisted handle의 literal-safe source-coordinator guidance만 사용한다. 자동 handle/mailbox 동기화는 issue #17이다.
- Explicit nonsecret Orca environment-key allowlist: never dump broad ORCA-prefixed env output or use prefix filtering for identity probes. Allow only explicitly named nonsecret keys such as `ORCA_TERMINAL_HANDLE`, `ORCA_TAB_ID`, `ORCA_PANE_KEY`, and `ORCA_WORKTREE_ID`, and never record secret values in tests, docs, logs, or evidence.

## Orca correction과 재개 attestation을 interrupt나 transcript 기억으로 처리하지 말 것

- 두 번의 interrupt-style correction이 active Codex를 종료하고 prompt body를 이어 붙인 사고가 있었다. Additive correction은 normal orchestration status/inbox로 보내고, interrupt는 명시적 cancellation/override에만 쓴다. Interrupt 뒤에는 submission을 먼저 읽어 확인하고 body를 재전송하지 않으며, idle prompt에 그대로 남은 경우 at most one Enter만 보낸다.
- Resume 시 transcript에 남은 task/dispatch/coordinator/worker handle은 authority가 아니다. Injected current preamble만 사용하고, raw exact-worktree terminal inventory와 server-filtered dispatched-task inventory를 먼저 읽은 뒤 current assignee handle 또는 omitted `--from`으로 exact dispatch를 확인한다.
- Evidence read/check는 control operator로 합치지 않는다. Broad inventory를 guessed jq path로 local filtering해 absence를 만들지 말고, `in_progress` 같은 unsupported status, guessed cursor flag, zsh reserved `path` variable을 쓰지 않는다. Bounded raw output와 exact process exit를 각각 확보한다.
- Model 전환과 usage reset은 사용자 승인 없이는 실행하지 않는다. Critical/Important review나 required gate가 남은 상태의 checkpoint `worker_done`은 completion이 아니며 금지한다.
- Startup attestation을 `pwd && git ... && orca ...` 같은 합성 명령으로 만들지 않는다. cwd/root/branch/HEAD/dirty/main-clean, raw terminal inventory, server-filtered dispatched task inventory, exact dispatch를 각각 standalone으로 읽어야 어느 authority read와 exit가 실패했는지 보존된다.
- 긴 suite나 golden이 실패하면 첫 실패의 exact test/byte 차이를 먼저 읽고 수정한다. 원인과 입력이 그대로인 long suite를 반복 실행하는 것은 새 evidence가 아니다.

## Orca supervised dispatch에서 완료 task나 안정된 diff를 writer lease로 오인하지 말 것

- Valid `worker_done`은 해당 dispatch를 끝낸다. Coordinator는 exact worker terminal을 닫고, review edit가 필요하면 새 ready task, fresh dispatch, exact sole-writer attestation으로 다시 시작한다. Completed worker에게 edit 지시를 보내거나 기존 task를 mutation lease처럼 재사용하지 않는다.
- Replacement/dispatch 직전 exact-worktree terminal inventory와 active orchestration task를 함께 확인한다. 다른 connected 또는 writable possible writer나 dispatched task가 하나라도 있으면 중단하고 durable lease recovery를 남긴다. Diff가 오래 안정돼 보이는 것은 ownership evidence가 아니며, original task/writer가 terminal임을 확인하기 전 preserved WIP를 adopt하지 않는다.
- Sole-writer task attestation은 server-filtered `orca orchestration task-list --status dispatched`와 exact `orca orchestration dispatch-show --task <current-task-id> --json`을 사용한다. Broad `task-list --json`을 local `jq`로 거른 출력이 truncated/unparsable이면 task absence가 아니라 ambiguity이며, exact task/dispatch가 증명될 때까지 mutation을 막는다.
- Fresh worker는 login shell에서 실제 host banner가 나타난 뒤 시작한다. Dispatch 직전 exact terminal의 `connected=true`, `writable=true`를 다시 확인하며, `tui-idle` 단일 표본만으로 startup을 attest하지 않는다.
- Authorized terminal send가 interrupt text와 Enter를 전달한 뒤에는 terminal read로 UserPromptSubmit 또는 working state 시작을 확인한다. Instruction이 idle prompt에 남아 있으면 instruction body를 다시 보내지 말고 Enter를 정확히 한 번만 보낸 뒤 다시 읽는다.
- Mailbox message는 numeric `sequence`와 exact `taskId`, `dispatchId`, sender/recipient direction을 모두 맞춰 선택한다. Sequence는 selection evidence일 뿐 lease fence가 아니다.

## Orca runtime rollover에서 presentation title이나 stale relay를 identity로 쓰지 말 것

실제 runtime restart에서 runtime ID, terminal handle/PTY, dynamic terminal title, worktree instance가 다시 발급됐지만 public tab/leaf와 visual-layout의 custom tab title은 유지됐다. 반대로 worktree instance가 같은 값으로 유지되는 restart도 유효할 수 있으므로 old/new equality 자체를 실패로 보지 않는다.

- current-runtime의 complete bounded worktree/terminal inventory가 sealed repo/base/path/branch/HEAD/comment와 stable tab/leaf를 유일하게 증명해도 곧바로 쓰지 않는다. journal snapshot exact equality와 context source, clean exact branch/attempt-base HEAD를 cycle lock 안에서 다시 확인한 뒤에만 runtime, worktree instance, terminal tuple을 한 CAS로 갱신한다. Stable ID가 없는 handoff record는 복구하지 않고 새 ownership cycle로 다시 시작하며 dynamic terminal title은 identity로 쓰지 않는다.
- recovered terminal이 connected 또는 writable이거나 uncommitted WIP가 하나라도 있으면 replacement를 시작하지 않는다. local observation의 대기는 caller-side Ctrl-C 또는 host tool cancellation로 끝내며 target PTY에 control input을 보내지 않는다.
- 현재 설치 환경의 relay pin 이름은 `ORCA_RELAY_DIR`와 `ORCA_RELAY_SOCKET_PATH`다. stale pin으로 handshake가 됐다는 사실은 current runtime/terminal/worktree identity 증거가 아니다.
- terminal-create는 설치 help에서 확인한 fixed built-in agent form 또는 harness가 생성한 fixed host command form만 쓴다. worktree provisioning 뒤 capability가 사라지면 create를 호출하지 않았어도 lease를 `recovery_required`로 보존한다.

## IssueOps ownership 필드를 root schema bump 없이 추가하지 말 것

`execution_handoff`처럼 mutation authority를 소유하는 필드를 기존 root schema에 additive `omitempty`로만 추가하면, 그 필드를 모르는 이전 binary가 unknown JSON을 버린 뒤 같은 schema로 rewrite할 수 있다. 이 사건 당시 legacy root의 schema는 8이었다. 현재 IssueOps는 dedicated v1 namespace의 schema 1만 읽고 쓰며 legacy/future rows를 migration 없이 fail-closed한다. 호환 decoder, re-attestation, background conversion을 추가하지 않는다.

- 새 ownership/security 필드는 root schema compatibility를 명시적으로 검토하고 removed-shape rejection fixture를 둔다.
- future schema hook scan은 row 전체를 해석하지 않고 bounded repo/worker identity와 invalid marker만 유지해 mutation을 fail-closed한다.
- CLI, daemon, Codex, Claude installed binary가 같은 schema를 읽는지 cutover smoke로 확인하기 전에는 mixed-version execution을 시작하지 않는다.

## Orca worker_done에서 live terminal을 sealed mailbox 대신 쓰지 말 것

Orca completion reconciliation은 message `from_handle`이 원래 dispatch `assignee_handle`과 정확히 같을 때만 `worker_done`을 인정한다. Runtime rollover 뒤 `WorkerTerminalHandle`은 바뀔 수 있으므로 completion sender로 쓰면 정상 결과가 무시된다.

- `CoordinatorMailboxHandle`과 `WorkerMailboxHandle`은 dispatch 시 봉인된 immutable mailbox authority다. `WorkerTerminalHandle`은 terminal read/send/steering 같은 live control 관측용이며 rollover만 갱신한다.
- ownership completion은 immutable completion evidence와 deterministic projection intent(또는 no-call diagnostic)를 같은 cycle lock에서 한 번에 쓴다. lock 밖에서 sealed owner mailbox → sealed source mailbox로 외부 send를 최대 한 번만 호출한다.
- intent 이후 crash, timeout, malformed response, ambiguous outcome은 자동 재시도하지 않는다. Durable completion이 authority이고 notification success/failure는 cleanup authority가 아니다.
- 완료 worker의 Stop suppression은 session binding이나 active-cycle 조회에 의존하지 않는다. Native payload `cwd`에서 canonical source checkout과 현재 branch를 한 번 파생하고 deterministic `(repo, branch)` record ID 하나만 읽어 `done` 레코드까지 검증한다. Binding 목록이나 global IssueOps record set을 후보 선택에 쓰지 않는다.
- Hostless Stop hot path는 IssueOps data DB가 없을 때 `sqlstore.Open`이나 session-bucket scan을 시작하지 않아야 한다. 설치된 numbered-next-action flags 경로에서도 처음 비어 있던 state root와 기존 Stop 응답을 그대로 보존하는 회귀 테스트를 둔다.
- dispatch preamble은 공식 exact coordinator/task label line과 exact `--dispatch-id` token으로 검증한다. 단순 substring 포함은 spoofing 가능하므로 증거가 아니다.
- dispatch recovery는 sealed coordinator가 concrete `term_*` handle이고 256 bytes 이하인지 외부 `dispatch-show` 전에 검증한다. group, shell-like, overlong recipient는 Orca observation 없이 거부한다.

## Orca GitLab handoff에서 GitHub issue flag나 조기 warning을 합성하지 말 것

설치된 Orca의 worktree-create `--issue`는 GitHub issue 전용이고 public help에는 GitLab issue CLI option이 노출되지 않는다. GitLab supervised handoff는 이미 검증된 provider tracking ref를 쓰되 `--issue`와 사설·가상 GitLab flag를 모두 생략한다. `linkedGitLabIssue`는 nullable이므로 null/zero를 native metadata unavailable로 영속화하고 exact 값과 구분한다. nonzero `linkedIssue` 또는 mismatched nonzero GitLab 값은 identity mismatch다.

- `auto`의 Orca missing/unready/capability failure나 이후 definitive pre-external-mutation inline fallback은 inline JSON/text와 row bytes를 그대로 유지해야 하며 GitLab native-metadata warning이나 ownership field를 붙이지 않는다. Probe가 성공해 `resolved_mode=orca`가 된 preview/confirm만 warning을 가질 수 있다.
- warning 여부를 즉시 응답에만 저장하면 반복 prepare/runtime restart에서 사실이 바뀐다. bounded provider-link observation을 durable Orca identity에 저장하고 재투영한다.

## Orca handoff publication과 cleanup을 caller flag나 best-effort 순서에 맡기지 말 것

- Owner publish는 handoff start에서 봉인한 native host/session/agent와 exact worker cwd를 hook event와 core에서 모두 다시 대조한다. Publish는 모든 effective Git config file origin(관련 없는 system/global key와 include target 포함)을 deterministic lock으로 고정하고 local `refs/heads/<branch>`가 full `FinalHead`인지 확인한 exact push 뒤, 동일 remote ref가 같은 SHA임을 증명하는 durable provider-neutral receipt를 먼저 저장한다. PR/MR create 직전에도 provider/remote/branch/ref/local+remote head를 재검증한다. GitHub/GitLab 모두 같은 fence를 쓰고 direct create, missing/stale receipt, `--body-file`, branch/provider drift는 차단한다.
- Git은 enumeration 시점에 아직 없는 user XDG/include config도 이후 생성되면 적용한다. Parent가 없다는 이유로 authority path를 lock 집합에서 생략하지 말고, common/worktree/user/XDG/effective-include 중 하나라도 lock할 수 없으면 push 전에 fail closed하며 intended/unintended destination이 모두 unchanged인지 회귀로 증명한다.
- Supervised PR/MR는 `phase=pr`이고 기존 `RemoteArtifact`가 없을 때만 provider mutation 전에 통과하며 draft로만 생성한다. Shared wrapper는 title/body whitespace를 claim 전에 한 번만 canonicalize하고 GitHub/GitLab create와 즉시 readback은 timeout·stdout/stderr cap·secret redaction을 적용해 canonical artifact URL, source/target project, title/rendered body, head/base/draft, requested label/assignee inclusion을 검증한다. GitLab은 canonical full-HTTPS `--repo`를 `glab mr create`와 `glab mr view <IID> --repo <same-url> --output json`에 동일하게 사용하며 custom port/IPv6는 publication 전에 glab 1.82+를 증명한다. `glab api --hostname host:port`나 bespoke HTTP adapter로 우회하지 않는다. Reconcile list는 exact project+head만 필터하고 non-null bounded array를 요구하며 base drift는 core claim verifier에서 거부한다. Create가 시작된 뒤 timeout/nonzero/malformed/mismatch가 나면 unknown/needs-reconciliation이며 자동 재시도하지 않는다. Provider 성공 뒤 supervised wrapper는 durable `IssueURL` project authority로 `VerifyIssueOpsRemoteArtifact`를 수행해 `RemoteArtifact`를 원자적으로 기록한다.
- Human-approved cleanup은 `cleanup-approve` disposition을 mutation 전에 저장하고 `task_terminal` → `terminal_quiescent` receipt를 exact identity로 저장한다. `terminal_quiescent`는 stale snapshot이므로 raw `orca worktree rm --force` 권한이 아니다. 명시적 사용자 경계에서 외부/manual deletion이 끝난 뒤 complete worktree inventory가 absence를 증명할 때만 optional `worktree_removed`를 기록한다. Active ownership에는 cleanup disposition을 열지 않으며, ambiguous/truncated/incomplete inventory나 changed task/terminal/worktree identity는 receipt가 아니다.
- MCP tool name의 suffix는 authority가 아니다. `mcp__evil__issueops_remote_create_pr`처럼 privileged 이름을 뒤에 붙인 foreign namespace를 허용하면 copied payload가 lifecycle guard를 통과한다. Supervised handoff mutation은 exact bare name 또는 exact `mcp__agent_harness__<name>`만 허용하고 collision 회귀를 유지한다.
- Long-running verification tool이 outer cell에서 yield하고 `session_id`만 반환하면 같은 session을 `write_stdin`으로 terminal `exit_code`까지 poll한다. Outer cell wait만 반복하거나 완료를 추측하지 않으며, 살아 있는 yielded test와 겹치는 duplicate test를 새로 시작하지 않는다. Partial package output은 PASS가 아니다.
- 이 경로는 GitLab remote issue/branch/work item/MR을 생성·수정하지 않는다. verified provider ref와 sealed provider/IssueURL만 소비한다.

## worker/terminal create 또는 dispatch 직전 "sole writer" 요약을 그대로 신뢰하지 말 것

이전 dispatch가 "no other writer exists"라는 요약만 믿고 새 worktree/terminal을 만들어 동일 worktree에 두 번째 writer가 붙은 사고가 있었다. 요약(assistant prose)은 evidence가 아니다.

- 새 worker terminal/worktree를 create 또는 dispatch하기 직전에 반드시 exact, untruncated worktree terminal inventory(`totalCount`, `truncated=false` 확인 포함)를 다시 조회한다.
- 그 inventory에 connected 또는 writable한 다른 terminal이 하나라도 있으면 create/dispatch를 거부한다. designated active worker만 connected와 writable이 모두 true여야 하며, truncated 응답은 신뢰할 수 없으므로 동일하게 거부한다.
- 이 확인은 매 dispatch 직전에 반복한다. 과거 턴에서 확인했다는 사실은 현재 turn의 증거가 아니다.

## yielded 검증 command를 완료로 오인하거나 heartbeat 부재만으로 worker를 중단하지 말 것

- zsh에서 `path`는 `$PATH`와 연결된 특수 배열이므로 loop 변수로 쓰면 이후 `git`/`rg` 탐색이 깨진다. staging loop에는 `file_name` 같은 이름을 쓰고, shell command에 backtick이 든 검색 문자열은 안전한 단일 인용 또는 별도 argv로 전달한다. Orca terminal read/check cursor flag도 help로 확인한 exact public flag만 사용한다.

검증 command가 `session_id`만 반환하고 `exit_code`가 없는데 outer tool call이 끝났다는 이유로 통과 처리한 사고와, 실제 `go test -race` process가 실행 중인데 heartbeat 부재와 화면 spinner만 보고 worker를 interrupt한 사고가 있었다.

- `go test ... | tail` 또는 `go test ... | grep` 같은 pipeline은 test process의 실제 exit status를 별도로 증명하지 않는 한 검증 evidence가 아니다. 필요한 suite를 pipeline 없는 direct command로 다시 실행한다.
- command가 `session_id`와 함께 yield되면 아직 실행 중이다. 같은 shell session을 `write_stdin`으로 poll해 terminal `exit_code`와 남은 output을 회수할 때까지 완료로 기록하지 않는다.
- 완료된 outer `functions.exec` cell에 `functions.wait`를 호출하는 것은 yielded shell session 재개가 아니다. 반드시 반환된 exact shell `session_id`를 재개한다.
- `tui-idle`, heartbeat 부재, filesystem quiescence, spinner, partial package output만으로 worker completion/hang이나 검증 성공을 판정하지 않는다. interrupt/close 직전에 host session의 active tool/process와 latest `tool_result`를 확인하고, exact verification process가 active면 terminal exit까지 기다려 poll한다.

## 리뷰 finding의 production fix를 named RED보다 먼저 적용하지 말 것

리뷰가 구체적인 재현을 제공해도 그것은 코드 변경 전 named failing-test transcript를 대신하지 않는다. 먼저 regression test만 추가하고 exact test name이 `RUN` 뒤 의도한 assertion으로 FAIL하는 terminal exit를 기록한 다음 production code를 수정한다. 순서를 어겼다면 RED를 소급 합성하거나 RED→GREEN이라 부르지 말고 `RED skipped` process defect로 기록하며, reviewer repro는 별도 pre-fix evidence로만 남긴다.

## JSON 검증 wrapper에서 추측한 필드를 성공 조건으로 쓰지 말 것

`jq`는 존재하지 않는 필드를 `null`로 평가하므로 `.passed == true` 같은 잘못된 predicate가 schema error가 아니라 정상적인 `false`와 exit 1로 나타난다. 이 때문에 성공한 장기 self-verify를 제품 실패로 오인하고 전체 검증 파동을 반복한 사고가 있었다.

- 장기 명령의 JSON 성공 조건은 실행 전에 DTO의 실제 JSON tag 또는 response contract golden에서 확인한다.
- `self-verify`의 종료 조건은 top-level `.ok`, `.termination_eligible`, `.summary.termination_eligible`이며 top-level `.passed`가 아니다.
- wrapper 실패는 제품 명령의 raw exit와 JSON artifact를 보존해 product failure와 orchestration/predicate failure로 분리한다.

## 긴 QA 파동을 interactive `tmux send-keys` 한 줄로 주입하지 말 것

긴 ZLE 입력을 여러 `send-keys` chunk로 빠르게 붙이면 중간 suffix가 유실되어 테스트 selector와 다음 명령이 결합될 수 있다. 실행은 계속되고 일부 명령은 exit 0을 반환하므로 단계 marker가 없으면 false-green이 된다.

- 긴 검증 파동은 reviewable temp script로 고정하고 tmux에는 script path 한 줄만 전달한다.
- script는 `set -euo pipefail`을 사용하고 각 필수 단계 전후 marker를 남긴다.
- named test 출력의 `[no tests to run]`, 누락된 marker, 예상하지 않은 결합 token은 파동 실패다.
- temp script와 JSON artifact는 terminal exit를 회수한 뒤 삭제하고 process/session 부재를 확인한다.

## Stability audit 명령과 timeout을 현재 공개 계약·측정치에 맞출 것

- top-level install audit에 과거 `bootstrap --sync`를 남기지 않는다. 현재 install 표면은 `bootstrap`/`install-native`; docs sync는 `project bootstrap --sync`다.
- live 정합성 gate인 `operational_doctor`는 상위 live harness 환경을 그대로 사용해야 한다. 반대로 audit 내부 ordinary/race `go test`는 `HARNESS_ROOT`를 exact audited source checkout으로 고정하고 `HARNESS_STATE_DIR`, `HARNESS_DAEMON_DIR`, `HARNESS_WORKER_DIR`를 audit 전용 임시 루트로 격리한다. live 환경으로 회귀 테스트를 실행하면 성공한 테스트가 IssueOps session row를 다시 만들어 최종 정리가 영구히 종료되지 않으며, `HARNESS_ROOT`를 빈 임시 경로로 바꾸면 source identity를 잃어 정상 회귀 검사가 실패한다.
- full repository test timeout은 가장 느린 정상 package와 race의 관측 상한보다 커야 한다. 현재 regression timeout은 300초다.
- `self-verify --full --iterations=10`은 매 seed마다 test/race를 실제 실행한다. 현재 약 3712초가 관측됐으므로 audit timeout은 5400초다.
- timeout 실패는 마지막 성공 package, elapsed time, 살아 있는 child command를 확인해 hang과 짧은 wrapper 상한을 구분한다.
- 장기 self-verify가 nonzero 또는 JSON parse 실패하면 audit report에 exit code, timeout 여부, parse error, parsed 종료 필드, bounded stdout/stderr tail을 남긴다. `summary: null`만 남기면 제품 실패와 audit 해석 실패를 구분할 수 없다.
- JSON parse를 `returncode == 0` 분기 안에 두지 않는다. nonzero가 바로 구조화된 실패 summary를 보존해야 하는 경우이며, parse와 성공 판정은 별도 단계다.
- deterministic stability gate에서 `self-verify`를 호출할 때는 `--llm-eval=false`를 명시한다. 코드 250단계가 모두 성공해도 암묵적 LLM gate의 외부 실패가 command exit를 뒤집을 수 있다.

## Sealed reconciliation에서 target CAS만으로 전체 소유권을 보호했다고 간주하지 말 것

- 개별 ref OID나 IssueOps record digest가 그대로여도 seal 이후 새 task, record, session binding이 같은 branch/worktree를 새로 소유할 수 있다. target CAS만 확인하면 새 owner를 final gate에서야 발견해 이미 봉인된 ref/state를 삭제한 뒤가 된다.
- 매 operation 전후에 journal order로 exact phase projection을 계산하고 Orca terminal/worktree/task/dispatch/gate/inbox, Git worktree/local·remote ref, IssueOps record/session/other row, state artifact를 모두 비교한다. `started` recovery는 해당 operation의 before/after 중 하나만 허용하며, inventory drift를 target readback 성공으로 삼켜 `verified`로 전진하지 않는다.
- collection의 current terminal argument는 추측 가능한 입력이 아니라 official no-selector `orca terminal show`가 resolve한 current handle에 대한 assertion이다. 실제 runtime rollover에서 spawn-time `ORCA_TERMINAL_HANDLE`은 stale해졌지만 tab/leaf와 pane/worktree composite는 현재 pane을 계속 식별했다. 따라서 complete terminal/worktree inventories와 명시적 tab/pane/worktree 환경값으로 current row의 runtime/handle/PTY/tab/leaf/worktree ID/path/connected/writable tuple을 유일하게 증명하고, 환경 handle의 explicit probe는 같은 current row 또는 exact structured `terminal_handle_stale`만 허용한다. 이 검증을 collection 안정화, 모든 live CLI 진입, mutation fence 전후에 반복하며 sealed tuple과 비교한다. stale 환경 handle을 current value로 export/override하거나 manifest authority로 저장하지 않는다.
- sealed reconciliation의 final stability audit은 `manifest.current_terminal.handle`을 singular `--preserve-terminal` argv로 전달한다. `explicit or env`처럼 truthiness로 선택하면 blank assertion이 stale fallback을 되살리므로, explicit presence를 별도로 분기하고 blank/invalid/overlong/repeated input은 build·doctor·cleanup 전에 거부한다.
- mutation이 성공한 뒤 preservation fence의 `TimeoutExpired`가 raw exception으로 빠지면 planned-operation ambiguity handler가 target-only `_operation_satisfied`로 이를 성공 처리할 수 있다. Fence의 timeout/parse/incomplete/drift는 ordinary operation과 reset 모두 먼저 non-recoverable `InventoryDriftError`로 정규화하고, 그 호출에서는 journal을 `started`에 남긴다. Exact target readback은 완전한 post-fence가 성공한 invocation ambiguity만 복구할 수 있다.
- fresh recovery bundle의 `test_reconcile.py`는 bundle에 봉인되지 않은 `simulate_copy.py`를 import하지 않는다. 최종 bundle에 복사할 exact 파일 집합만 clean temporary directory에서 직접 실행해 PASS를 확인한다. Simulator는 별도 copied-DB gate로 실행하고 bundle executable dependency로 만들지 않는다.
- `REJECTED` 같은 unsealed marker 파일만 추가해도 현재 bundle validator는 이를 무시한다. 폐기 bundle은 증거 내용은 보존하되 sealed runner mode를 `0400`처럼 계약 밖으로 바꾸고 `validate` nonzero를 확인해 실행 불가능하게 만든다. Marker만 보고 재사용 방지가 됐다고 주장하지 않는다.
- Python canonical JSON과 맞추는 Go CAS decoder는 `UseNumber`를 사용한다. 일반 `json.Unmarshal(any)`의 `float64` 변환은 `2^53`보다 큰 integer를 반올림해 raw가 같은 record의 canonical SHA-256을 다른 값으로 계산한다.

## dropped child와 done parent를 Stop orchestration에 재진입시키지 말 것

IssueOps PR readiness는 `validation_verdict=dropped` child를 scope에서 제외하지만 Stop hook의 별도 reminder 경로가 같은 규칙을 적용하지 않아, 병합된 parent가 `child_incomplete`로 영구 재진입한 사고가 있었다.

- dropped child는 active child total, incomplete key, unvalidated key에서 모두 제외한다. `rejected`나 verdict 없는 child와 혼동하지 않는다.
- 이미 `phase=done`인 bound parent는 오래된 session binding이 남아 있어도 orchestration reminder/Stop relay 대상으로 다시 읽지 않는다.
- core readiness와 hook hot path가 같은 child verdict 의미를 갖는 named regression을 함께 유지한다.
- bounded scan은 raw child 목록을 먼저 자른 뒤 dropped를 건너뛰지 않는다. dropped를 제외한 처음 N개를 읽어야 앞의 removed scope가 뒤의 active child를 숨기지 않는다.

## Codex co-resident hook merge에서 matcher 배열 위치를 바꾸지 말 것

Codex hook trust는 command 내용만이 아니라 `source:event:matcher-index:hook-index` key의 `trusted_hash`에 결합된다. installer가 agent-harness 그룹을 제거한 뒤 끝에 append하면 co-resident Orca hook과 배열 위치가 바뀌고, 두 command가 모두 상대방의 stored hash와 비교되어 `modified`가 된다.

- 기존 agent-harness 그룹은 첫 발견 위치에서 in-place replacement하고, 유효한 제3자 그룹의 상대 순서를 보존한다.
- agent-harness 그룹이 없을 때만 append하고, 중복 agent-harness 그룹은 한 개로 축약한다.
- install JSON만으로 runtime hook trust를 주장하지 않는다. Fresh native session의 실제 SessionStart/PreToolUse smoke와 설치된 hook readback을 함께 확인한다.
- automation에서 `--dangerously-bypass-hook-trust`를 사용한 결과는 command 동작 증거일 뿐 정상 trust 상태의 증거가 아니다.

## cross-process 테스트에서 helper 시작·종료 오류를 버리지 말 것

두 helper의 `exec.Cmd.Run` error와 helper 내부 초기 read error를 버린 결과, provider 호출 횟수 파일이 없다는 2차 증상만 남고 실제 실패 원인이 사라진 적이 있다.

- 여러 helper를 동시에 검증할 때는 각 process를 순서대로 `Start`해 launch를 확인한 뒤 모두 `Wait`하여 동시 실행과 진단을 함께 보장한다.
- stdout/stderr와 exit error를 helper별로 보존하고, 승자·차단자처럼 기대하는 서로 다른 결과를 명시적 marker로 검증한다.
- count/result artifact 부재만 보고 production concurrency defect로 분류하지 않는다. helper launch/read/create 경계를 먼저 확인한다.

## SQLite state root 최초 초기화도 cross-process 경합으로 취급할 것

같은 state root를 두 process가 처음 열 때 process-local handle mutex는 data/span DB 파일 생성과 schema pragma를 직렬화하지 못한다. transaction lock만 재시도해도 open/schema 단계의 `SQLITE_BUSY`는 그대로 노출된다.

- `sqlstore.Open`의 초기화 재시도는 typed `SQLITE_BUSY`/`SQLITE_LOCKED`에만 적용하고 명명된 짧은 상한을 둔다. permission, symlink, schema, path 오류를 retry로 숨기지 않는다.
- cross-process 회귀는 양쪽 helper가 준비됐다는 barrier 뒤 동시에 진입시킨다. 단순히 process 두 개를 순서대로 `Start`한 것만 actual contention 증거로 삼지 않는다.
- expected loser error를 exact allowlist로 분류한다. 이미 artifact가 확정된 뒤의 phase exclusion과 live claim exclusion 외 오류는 `blocked`로 축약하지 말고 helper stderr와 nonzero exit로 남긴다.
- parent는 첫 `Wait` 실패에서 즉시 종료하지 말고 시작된 모든 helper를 회수해 orphan과 TempDir cleanup race를 방지한다.

## publication Git config authority를 diagnostic buffer나 platform 암묵성에 맡기지 말 것

- current-user writable 또는 owner-controlled config는 sibling `O_EXCL` lock 없이 publication을 진행하지 않는다. parent가 non-writable이라는 사실만으로 immutable이라고 분류하면 transient rewrite-and-restore 공격을 놓친다.
- immutable fallback은 canonical regular file이고 file/path chain 전체가 root(uid 0) 소유이며 current UID가 어느 것도 소유하거나 쓸 수 없고 sibling lock 실패가 permission/read-only filesystem인 경우로 제한한다. 임의의 ordinary non-current UID 소유자는 transient rewrite-and-restore가 가능하므로 defensible system authority가 아니며 fail-closed한다. protected callback 전후에 file identity, content fingerprint, origin/rewrite inventory를 다시 확인한다.
- origin/include/URL rewrite inventory는 diagnostic 4096-byte 출력 helper를 재사용하지 않는다. 별도 bounded-complete read를 공유하고 상한 초과는 partial parse 없이 fail-closed한다.
- Git은 conditional include key를 canonical lowercase `includeif`로 출력한다. active empty include는 origin inventory에 자체 entry가 없으므로 directive inventory가 이 canonical form을 놓치면 sibling lock authority도 사라진다.
- include path의 `~/`, `~user/`, `%(prefix)/` interpolation을 부분 재구현하지 않는다. `git config --type=path`로 Git 자신의 canonical 확장 결과를 inventory로 봉인하고, 확장되지 않은 `~`/`%(prefix)/` residue는 fail-closed한다. unresolvable `~user`는 git 자체가 nonzero로 실패하므로 그대로 fail-closed 전파한다.
- 최초 부재 default XDG git config는 authority set에서 생략하지 않는다. 부재 parent chain을 transient로 생성해 sibling lock을 잡고, release 시 생성분만 역순 제거하며, post-operation 검증에서 해당 config가 여전히 absent인지 확인한다. 그 외 authority path의 missing parent는 계속 fail-closed다.
- Unix의 `Stat_t`, access, effective UID, errno 판정은 build-tagged helper 안에 둔다. metadata/access 계약을 지원하지 않는 platform은 immutable fallback을 추측하지 않고 fail-closed한다.
- implementation evidence는 valid `branch_prepare.base_sha`를 immutable diff base로 사용한다. SHA가 없거나 검증 불가능하면 moving base ref로 추정하지 않고 fail closed한다.

## source main worktree를 supervised fence로 막지 말 것

`io-b9f8cd45e152`는 2026-07-20의 read-only incident evidence다. 이 기록에는 conversion, dispatch, cancellation, cleanup 명령을 제안하거나 실행하지 않는다. 새 구현 설치 뒤에도 별도의 live readback과 인간 결정 없이는 처분하지 않는다.

- Current ownership contract는 workspace provisioning before ownership transfer를 보장한다. source main worktree remains available before, during, and after handoff; generic session binding과 mirrored relative path는 authority가 아니다.
- Fence는 canonical worker root, exact cycle ID, native owner, or persisted Orca resource만 선택한다. Removed handoff record shapes are rejected rather than converted.
- Record 선택에 `SourceRoot`, source `cwd`, generic repo root를 다시 넣지 않는다. 명시적 canonical target이 있거나 command cwd 자체가 canonical root일 때만 cycle write lease를 적용한다. Codex hook의 top-level cwd는 effective `exec_command.workdir`가 아니므로 explicit canonical file target은 holder/lease/containment가 맞으면 source session cwd에서도 허용한다.
- Orca의 즉시 terminal-create 응답은 stable title이 아직 반영되지 않을 수 있다. 같은 worktree와 PTY의 bounded terminal inventory를 한 번 재조회해 sealed marker를 검증하고, 없거나 중복이면 fail closed한다.
- `cleanup_pending_human_decision`의 every non-`closed` ownership resource는 stale scan과 operational health가 보존한다. elapsed time, Stop hook, original source identity는 cleanup authority를 만들지 않는다.

## dirty main에서 원격 변경과 겹치는 파일을 바로 pull하지 말 것

로컬 `main`의 미커밋 변경과 `origin/main`의 fast-forward 변경이 같은 파일을 수정하면 `git pull --ff-only`는 overwrite 방지를 위해 중단한다. 이 실패를 branch divergence나 pull 설정 문제로 오인하지 않는다.

- 먼저 `git rev-list --left-right --count HEAD...origin/main`, 로컬 `git diff --name-only`, `git diff --name-only HEAD..origin/main`을 대조해 실제 겹침을 증명한다.
- tracked와 untracked 변경을 이름 있는 `git stash push --include-untracked`로 보존하고 stash SHA를 기록한 뒤 fast-forward한다.
- `git stash apply`를 사용해 복구 지점을 유지한 채 변경을 재적용하고, conflict marker·diff·focused/full tests와 두 번째 `git pull --ff-only`를 확인한 뒤에만 stash를 제거한다.
- dirty `main`을 맞추기 위해 `reset --hard`, `clean`, 사용자 변경 폐기, 임시 worktree branch 삭제를 사용하지 않는다.

## execution owner에게 lease 명령만 주고 lifecycle phase 전이를 추론시키지 말 것

Orca owner가 active lease를 정상 claim하고 구현·대상 검증까지 마쳤지만 lifecycle은 `problem`에 남아 있었다. 기존 sealed prompt가 `link-plan`, `phase --to implement`, `ai-slop-clean record`, `phase --to ai-slop-clean`, `phase --to pr`의 순서와 exact command를 제공하지 않았기 때문이다. cleanup evidence만 기록해도 phase와 fingerprint는 자동으로 전이되지 않으므로 implementation review가 `implement phase` 이전이라고 거부했다.

- sealed owner packet은 plan 연결부터 PR phase까지 필요한 lifecycle mutation을 exact command로 렌더하고 command catalog로 검증한다.
- staged plan과 기존 `plan_path`가 모두 없으면 임의 계획이나 phase jump로 우회하지 않고 blocker를 보고한다. `link-plan`은 같은 canonical path만 멱등 허용하며 다른 path로의 교체는 fail-closed한다.
- 구현 수정은 `phase=implement` readback 뒤에 시작하고, cleanup evidence 기록 뒤 `ai-slop-clean` 전이로 fingerprint를 봉인한다.
- implementation review는 실제 `pass|revise|stop` verdict를 기록한다. `revise`는 수정·재검증·fresh 리뷰를 반복하고 `stop`은 publication을 중단한다. 리뷰 뒤 diff를 바꾸면 cleanup/review fingerprint를 다시 기록한다. clean/synced push 뒤 `phase=pr`을 통과한 다음에만 governed PR/MR 생성 명령을 실행한다.
- 최신 사용자 지시가 전체 테스트를 제한하면 sealed issue의 오래된 full 명령을 강행하지 않고 targeted 검증과 생략 근거를 Turing report에 남긴다.

## active lease에서 atomic 스킬의 Python gate를 일반 관찰로 열지 말 것

`atomic-commit-push`의 필수 `git_preflight.py`가 읽기 전용이어도 Python 스크립트 실행은 정적 shell reader 목록에 없어서 `unsafe_mutation`으로 차단된 적이 있다. 반대로 파일 이름만 보고 observation으로 승격하면 저장소가 제공한 Python 코드가 non-holder에게도 열릴 수 있다.

- `git_preflight.py`와 `api_doc_gate.py`의 정확한 단일 `python3` 호출만 current holder workflow로 인정한다.
- 스크립트는 저장소 상대 경로와 사용자 홈의 설치·심볼릭 링크 경로를 모두 쓸 수 있으므로 사용자별 절대 경로를 하드코딩하지 않는다. 절대 경로는 명시적 expected worktree/source checkout, `HARNESS_ROOT`, `CODEX_HOME`, 사용자 홈의 Codex·Claude skill root처럼 설치기가 관리하는 base와 정확히 일치할 때만 허용한다. generic repo/cwd와 단순 `/skills/...` suffix 비교는 신뢰 근거로 쓰지 않는다.
- 상대 `skills/...` 스크립트는 active lifecycle의 canonical worktree root에서만 실행한다. 하위 디렉터리에서는 같은 상대 경로가 `<subdir>/skills/...`를 가리키므로 holder라도 허용하지 않는다.
- 선택적 repo 인자는 실제 shell 작업 디렉터리와 같은 canonical 경로만 허용한다. Codex `exec_command`는 `tool_input.workdir`, Claude Bash는 top-level `cwd`를 기준으로 삼고 해석이 모호한 상대 `workdir`는 거부한다.
- 비-shell tool은 같은 command 문자열을 실어도 이 경로로 분류하지 않는다. 공백이 포함된 argv, 추가 인자, 다른 인터프리터, 다른 스크립트, 외부 repo 대상은 계속 fail-closed한다.
- 이 경로는 일반 read-only observation이 아니다. 기존 native holder identity와 canonical worktree containment를 모두 통과한 뒤에만 실행한다.

## exact reader를 열기 전에 실제 구현의 무변이성을 확인할 것

Shannon 측정 중 `rg -c`와 `agent-harness state read --key ...`가 active lease에서 차단되었다. `rg --count`는 허용하면서 같은 read-only short flag `-c`를 빠뜨린 명세 누락이 있었고, 기존 `StateRead`는 이름과 달리 store가 없으면 SQLite 디렉터리를 생성했으므로 곧바로 observation으로 승격할 수 없었다.

- CLI 이름만 보고 read-only로 분류하지 않는다. 파일·DB·네트워크 구현이 누락 상태에서도 데이터를 만들거나 복구하지 않는지 먼저 테스트한다.
- state의 단일 row 조회는 `sqlstore.GetExisting`처럼 기존 store만 여는 경로를 사용하고, 없는 store에서 파일·디렉터리를 만들지 않는 회귀 테스트를 둔다.
- 외부 reader의 long/short 동의어는 실제 사용하는 형태를 모두 characterization corpus에 넣되, 실행기·전처리·출력 파일처럼 mutation으로 확장되는 flag는 계속 거부한다.
- 새 exact reader는 command parser, active lifecycle decision, 실제 hook CLI full payload를 함께 검증한다. parser 단위 테스트만 통과한 상태로 설치 바이너리를 갱신하지 않는다.

## Git porcelain의 선행 공백을 일반 문자열처럼 자르지 말 것

`git status --porcelain=v1`의 첫 행이 unstaged 변경이면 상태 코드는 선행 공백으로 시작한다. 전체 stdout에 `TrimSpace`를 적용하면 첫 경로의 첫 글자까지 상태 필드로 오인해, AI-slop-clean 지문이 같은 내용을 커밋한 뒤 달라지고 plan-only 변경도 구현 변경으로 오탐한다.

- porcelain처럼 공백이 스키마인 출력은 `GitCmdRaw`로 읽고 행 단위 파서에 원문을 전달한다.
- 사람이 읽는 단일 값에 쓰는 `GitCmd`/`GitOut`의 trim 계약을 structured Git 출력에 재사용하지 않는다.
- 회귀 테스트는 첫 행이 ` M <path>`인 tracked 변경을 포함하고, dirty 상태와 동일 내용 commit의 지문 일치 및 tracked plan-only 변경 제외를 함께 검증한다.
- 잘못 봉인된 기존 지문은 첫 파일 내용을 해시에 포함하지 않았으므로 호환 계산으로 통과시키지 않는다. 수정된 바이너리에서 committed diff를 다시 독립 검토하고 새 cleanup/review 지문을 기록한다.

## 2026-07-31 — released direct lease 복구는 유한한 next_command 체인을 제공해야 한다

- Kind: `caution`
- Source: IssueOps direct lease dogfood
- Summary: released direct execution의 status와 replacement 결과가 다음 명령을 누락하거나 fingerprint 없는 reseed를 안내하면 write lease 복구가 중간에서 막힌다.
- Context: 부모 #117 generation 9가 released인 상태에서 child accept가 active write lease 가드에 거부됐고, execution status는 next_command를 반환하지 않았다. 기존 prepare 안내도 inventory fingerprint 없이 reseed를 지시했다.
- Resolution: StatusExecution은 완료되지 않은 writerless execution에 상태별 next_command를 반환하고, released는 replacement preview부터 시작한다. preview는 generation과 inventory fingerprint가 포함된 reseed를, direct reseed/finalize는 current token path가 포함된 claim을 반환한다. Orca claimable은 generation 상태와 관계없이 멱등 resume로 안내한다.
- Evidence:
  - internal/core/issueops/execution_lease.go
  - internal/core/issueops/execution_prepare.go
  - TestReleasedDirectRecoveryRendersFiniteCommandChain RED 후 PASS
  - TestExecutionWriterAbsentCurrentOrcaGenerationStillPointsToResume RED 후 PASS
  - focused race PASS
  - CLI/MCP response golden 두 번 연속 PASS
- Alternatives / rejected options:
  - fingerprint 없이 reseed를 수동 실행하는 우회는 generation-CAS 계약을 깨므로 기각
  - status에 token placeholder만 남기는 방식은 durable resume 시 실제 claim 경로를 다시 추론해야 하므로 기각
  - Orca current-generation claimable에서 직접 claim을 안내하는 방식은 resume가 제공하는 sealed digest handoff를 건너뛰므로 기각

## 2026-08-03 — Orca resume은 현재 prompt template을 trust root로 쓰면 안 된다

- Kind: `caution`
- Source: IssueOps #248/#254 Orca dogfood
- Summary: terminal preparation intent를 삭제한 뒤 resume이 현재 바이너리의 owner prompt template으로 expected prompt를 다시 렌더링하면, 정상적인 template 변경만으로 이미 봉인된 실행이 영구 중단된다.
- Resolution: prepare와 Orca reseed는 identity version 1과 issue-body, context-packet, owner-prompt SHA-256을 generation-bound Orca binding에 함께 저장한다. Resume은 artifact bytes를 이 durable identity와만 비교하고 prompt를 다시 렌더링하지 않는다. Version marker와 세 digest가 모두 없는 기존 v1 binding만 status가 preview → generation-CAS reseed → resume 복구 체인으로 보낸다. Versioned all-empty는 새 persistence 결함이므로 invariant violation이며, unversioned-complete·일부 digest·future version·worktree 파일을 새 trust root로 채택하는 fallback은 fail-closed한다.
- Evidence:
  - `internal/core/issueops/execution_resume_identity_test.go`
  - `internal/adapter/outbound/issueopspreparation/repository_orca_test.go`
  - `internal/application/issueopslease/reseed_test.go`

## 2026-07-31 — Orca task mutation은 explicit Run과 coordinator consumer를 함께 봉인해야 한다

- Kind: `caution`
- Source: Orca 1.4.162 IssueOps #194 dogfood
- Summary: `task-list`를 전역 호출하거나 현재 terminal RPC에 `--from`을 강제로 붙이면 `legacy_read_only` 또는 `consumer_fenced`로 막힌다.
- Context: Orca 1.4.162는 task create/update/dispatch를 Run 단위로 격리하고, mutation을 실행하는 coordinator terminal이 그 Run의 current consumer인지 확인한다. CLI가 설치되어 있고 worktree/terminal 기능이 정상이어도 전역 task readiness probe 때문에 `mode=auto`가 direct로 내려갈 수 있었다.
- Resolution:
  - readiness는 `ORCA_TERMINAL_HANDLE` 형식을 먼저 검증한 뒤 `run-current --json`과 완전한 `run-list`로 확인하고 mutation을 실행하지 않는다.
  - prepare/resume는 `run_create`와 `run_bind`를 별도 journal stage로 기록한다.
  - 현재 coordinator가 실행하는 Run/task mutation은 `--from`을 생략해 Orca가 호출 프로세스의 terminal authority를 인증하게 한다. `--from`은 다른 terminal을 명시적으로 대리하는 복구·조회 경로에만 사용한다.
  - injected worker 제어 명령은 예전 `--to` 주소와 현재 `--dispatch-capability` 주소를 섞지 않는다. capability 경로는 exact `--from`이 필요하고 `worker_done`은 `--outcome succeeded|failed`를 반드시 포함한다. hook은 이 표면만 열거하며 capability 원문을 기록하지 않는다.
  - `branch_prepare.link_verified=false`인 GitHub owner packet에는 `gh issue develop --list <issue> --repo <owner/repo>`를 exact reader로 함께 봉인한다. owner가 임의 GraphQL을 만들게 하지 않고, 연결 readback 뒤에만 branch prepare recorder를 실행한다.
  - 환경의 exact concrete terminal handle은 coordinator 실행 여부를 fail-closed로 확인하는 gate이며, focus/cwd를 권한으로 추론하지 않는다.
  - task 인벤토리는 모든 explicit Run의 `task-list --run`을 합치며 Run ID와 runtime ID를 함께 검증한다.
  - Run 도입 전 binding은 같은 task ID가 정확히 한 explicit Run에 있을 때만 읽기·완료 처리를 복구한다.
- Evidence:
  - internal/adapter/orca/client.go
  - internal/adapter/orca/execution.go
  - internal/core/issueops/execution_orca_intent.go
  - TestExecutionOrcaRunBindCanConvergeAfterUnknownOutcome
  - TestClientTaskInventoryKeepsSameTaskIDDistinctAcrossRuns
  - TestExecutionAdmitsExactOrcaOwnerControlPlaneCommands
  - TestRunHookPreToolUseAllowsCurrentOrcaOwnerControlCommands

## 2026-08-04 — 생성된 IssueOps command의 PATH token만으로 실행 바이너리를 신뢰하지 말 것

- Kind: `caution`
- Source: IssueOps #303 stale installed-binary dogfood
- Summary: core가 만든 `next_command`를 CLI 출력 단계에서만 꾸미거나 executable 관측 실패를 빈 evidence로 대체하면 MCP 경로와 cleanup command가 오래된 PATH 바이너리를 그대로 실행할 수 있다.
- Resolution: contract는 canonical executable path·SHA-256·lease generation의 DTO와 pure bind/validate만 소유한다. Port는 contract를 import하지 않는 순수 observation receipt를 반환하고 application binder가 이를 contract evidence로 변환하며, 실제 observer는 `harnessapp` composition root만 생성한다. CLI와 MCP composition은 생성 command의 첫 token을 관측한 canonical executable literal로 바꾸고 같은 envelope를 결합한다. Hook은 self-declared envelope만 믿지 않고 absolute token을 durable worktree/source의 canonical `bin/agent-harness`로 제한하며 wrapper·substitution·outside-root target을 차단한다. IssueOps root는 subcommand dispatch 전에 현재 executable과 durable generation을 다시 검증한다. 관측 실패, incomplete envelope, binary mismatch, generation drift는 command fallback 없이 structured error로 끝낸다. 일반 수동 command의 PATH UX는 유지하고 pre-v1 바이너리는 reserved flag를 알 수 없는 flag로 거부한다.
- Evidence:
  - `internal/contract/issueops/generated_command_provenance.go`
  - `internal/adapter/outbound/issueopsprovenance/observer.go`
  - `cmd/harness/issueopscli/generated_command_provenance_test.go`
  - `cmd/harness/mcpcli/generated_command_provenance_test.go`
  - `cmd/harness/issueopscli/feedbackcleanup/generated_command_provenance_test.go`
