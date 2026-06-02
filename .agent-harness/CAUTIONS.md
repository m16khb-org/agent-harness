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

`agent-harness mcp`가 daemon을 자동 시작하므로 오래된 binary가 이미 떠 있으면 새 코드 검증과 실제 MCP 동작이 갈라질 수 있다.

주의:
- 설치/빌드 후 MCP smoke 전에는 필요하면 `agent-harness daemon stop --json`으로 기존 daemon을 내린다.
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

## MCP tool-use risks

- Broad tool descriptions make agents over-call tools or pass wrong arguments.
- Always injecting all project documents at session start wastes context and can hide task-specific evidence.
- Writable tools need explicit write semantics; prefer dry-run or append-only behavior.
- Tool output is evidence, not proof: verify file existence, warnings, and command/test results before claiming completion.

## 2026-05-31 — Codex PreCompact hook stdout schema

- Kind: `caution`
- Source: cli
- Summary: Codex 0.135 PreCompact and PostCompact hook output rejects hookSpecificOutput/additionalContext; compact hook stdout must stay in compact-control shape such as {}, suppressOutput-only, or systemMessage. Model-facing additionalContext injection belongs to SessionStart/UserPromptSubmit/PostToolUse-style hooks whose installed Codex schema explicitly allows it.

## 2026-05-31 — PreToolUse false-positive risk

- Kind: `caution`
- Source: codex/claude runtime evidence
- Summary: Codex 0.135.0 and Claude Code 2.1.158 both expose PreToolUse, but the hook runs before every matched tool call and can block or rewrite execution. Keep agent-harness PreToolUse host stdout as `{}` by default and expose only raw `--json` diagnostics until a deterministic policy has host-schema tests and false-positive coverage.
- Do not record lifecycle upkeep from PreToolUse; the tool may not succeed. Use PostToolUse only for observed successful mutating changes, not read-only searches whose output happens to mention lifecycle-relevant paths.
- Follow-up: when an agent-harness hook process exits non-zero, record a redacted JSONL failure event in user state with hook subcommand, host, cwd/repo, tool name, argv, relevant command/query snippet, and error. Codex UI may only show `PreToolUse hook (failed) error: hook exited with code 1`, which is insufficient to distinguish user hooks from plugin hooks or payload-specific failures.

## 2026-05-31 — Do not patch upstream companion plugin caches

- Kind: `caution`
- Source: manual
- Summary: Do not edit installed upstream plugin cache files such as `~/.codex/plugins/cache/claude-mem/...`; fix duplicate or host-specific integration issues in user-owned Codex/Claude settings, wrappers, or upstream itself.
- If an upstream memory provider is installed as a Codex plugin, do not also install the same hooks in `~/.codex/hooks.json`; that double-runs capture hooks and creates duplicated observations/summaries.

## 2026-06-03 — Headroom install does not imply runtime savings

- Kind: `caution`
- Source: codex-cli
- Summary: A missing or installed Headroom CLI does not by itself mean Headroom is active; runtime savings require explicit proxy or wrapper execution.
- Context: User asked to confirm why Headroom was not operating after finding no CLI, no process, no pipx, and no Python package.
- Resolution: Verified local environment and repo contract: pipx absence causes installer skip, and repository docs/tests intentionally avoid automatic Headroom proxy/wrap/learn/MCP activation.
- Evidence:
  - `command -v headroom` exited 1; `command -v pipx` exited 1; Python import failed with `ModuleNotFoundError: No module named 'headroom'`.
  - `ps aux | rg -i '[h]eadroom|HEADROOM'` returned only the current verification commands.
  - scripts/install-native.sh:176-186 returns success after logging `pipx not found; skipping Headroom setup` when pipx is absent.
  - .agent-harness/OPERATIONS.md:70 says Headroom runtime use is explicit and bootstrap/hooks do not auto-route through proxy or wrapper.
  - internal/adapter/install_contract_matrix_test.go:174-182 forbids auto-enable strings for `headroom wrap`, `headroom proxy --port`, and `headroom learn`.
  - `go test ./internal/adapter -run 'TestInstallNativeUpstreamToolsUseHeadroom' -count=1` passed.

## 2026-06-03 — Headroom Codex init can overwrite existing hooks

- Kind: `caution`
- Source: codex-cli
- Summary: `headroom init -g codex` may rewrite `~/.codex/hooks.json`; preserve or merge existing `agent-harness` lifecycle hooks before restarting Codex.
- Evidence:
  - Before Headroom init, `~/.codex/hooks.json` contained `agent-harness hook post-compact`, `post-tool-use`, `pre-compact`, `pre-tool-use`, `session-start`, `stop`, and `user-prompt`.
  - After Headroom init, the file contained only Headroom `SessionStart` and `PreToolUse` hook entries until manually merged.
  - `python3 -m json.tool ~/.codex/hooks.json` verified the merged hook file is valid JSON.
- Resolution: Back up `~/.codex/config.toml` and `~/.codex/hooks.json` before running Headroom init, then verify both Headroom and agent-harness hook entries remain present with `rg -n "headroom|agent-harness" ~/.codex/hooks.json`.

## 2026-06-03 — Headroom first start can outlive readiness timeout

- Kind: `caution`
- Source: codex-cli
- Summary: The first `headroom install start --profile init-user` can report `Deployment 'init-user' did not become ready after start` while the proxy continues warming up and later becomes healthy.
- Evidence:
  - `~/.headroom/deploy/init-user/runner.log` showed startup downloading `chopratejas/kompress-base` and `answerdotai/ModernBERT-base` assets from Hugging Face.
  - `~/.headroom/logs/proxy.log` later showed `Kompress ONNX INT8 loaded`, `Headroom Proxy started`, and `Anonymous telemetry: DISABLED`.
  - `HEADROOM_TELEMETRY=off headroom install status --profile init-user` later reported `Status: running` and `Healthy: yes`.
  - `/usr/bin/curl -i http://127.0.0.1:8787/readyz` returned HTTP 200 with `"ready":true`.
- Resolution: After a first-start readiness timeout, inspect `runner.log`/`proxy.log` and re-run `headroom install status --profile init-user` before restarting or changing configuration.

## 2026-06-03 — Prefer Headroom health endpoint over status wording

## 2026-06-03 — Do not leave self-verify invariant failures as reports only

- Kind: `caution`
- Source: self-verify
- Summary: When `agent-harness self-verify` fails on harness invariants, treat the failure as actionable unless the evidence proves it is physically impossible to fix in the current environment.
- Evidence: `self-verify --progress=jsonl --json` stops at the first failed invariant step, so later coverage gaps are a consequence of early termination, not separate proof that the README or code change is safe.
- Resolution: For forbidden legacy name hits, inspect the exact file:line evidence, generalize personal GitHub owners and absolute local paths to neutral examples, run `rg` for all forbidden needles, then rerun self-verify.

- Kind: `caution`
- Source: codex-cli
- Summary: Headroom `install status --profile init-user` can report `Status: stopped` while a proxy is listening and `/health` is healthy, especially after repeated starts around a stale runner process.
- Evidence:
  - `headroom install status --profile init-user` reported `Status: stopped` and `Healthy: yes` at the same time.
  - `lsof -nP -iTCP:8787 -sTCP:LISTEN` showed a Python Headroom proxy listening on `127.0.0.1:8787`.
  - `/usr/bin/curl -i http://127.0.0.1:8787/health` returned HTTP 200 with `"status":"healthy"` and `"ready":true`.
- Resolution: Reproducible setup must treat `/health` or `/readyz` as the final runtime evidence. `scripts/setup-headroom-runtime.sh` now verifies the health endpoint after running Headroom init/start for both Codex and Claude Code.

## 2026-06-03 — Separate PR/MR merge from IssueOps worktree cleanup

- Kind: `caution`
- Source: PR #15 merge attempt
- Summary: `gh pr merge --merge --delete-branch` can merge the GitHub PR remotely and then fail during local branch cleanup when the base branch, such as `main`, is already checked out in another linked worktree. In IssueOps worktree flows, run provider merge first without local cleanup flags, then verify merged state, remote source branch state, and worktree cleanliness before deleting the remote source branch, removing the feature worktree, and deleting the local branch.
- Do not rely on provider CLI merge flags that also perform local branch/worktree cleanup from a feature worktree. For GitHub, avoid `gh pr merge "$PR_NUMBER" --merge --delete-branch`; use `gh pr merge "$PR_NUMBER" --merge`, then explicit post-merge cleanup. For GitLab, verify whether the installed `glab`/API flag is remote-only before using it; otherwise merge first and clean up remote/local state in separate commands.
