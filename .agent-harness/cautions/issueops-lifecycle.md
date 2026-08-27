---
name: cautions/issueops-lifecycle.md
description: Cautions for IssueOps branches, worktree edits, numbered-choice hooks, domain vocabulary, readiness gates, and response-contract goldens.
---

# IssueOps lifecycle cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for IssueOps
branches, worktree isolation, numbered-choice Stop hooks, hooks-as-workers
boundary, CLI domain vocabulary, artifact URL parsing, response-contract
golden drift, and readiness-gate blast radius. Orca supervised-handoff
operations live in [issueops-orchestration.md](issueops-orchestration.md);
execution liveness and lease authority live in
[issueops-execution.md](issueops-execution.md).

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
- Tool-root drift: IssueOps may run implementation in a sibling worktree while a host session and some MCP servers remain rooted at the original source checkout. In a sampled `service-api` repository, verified 2026-06-05, Claude `.mcp.json` was gitignored and its `codegraph` server had `--path /Users/sample/workspace/service-api`, while Codex global `codegraph` was `codegraph serve --mcp` without `--path`. Do not solve this by asking the user to restart Claude Code in the worktree; preserve the current session and enforce per-call evidence instead.
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

## 24. IssueOps 도메인 어휘를 CLI 서브커맨드로 착각

에이전트가 `issueops grill`, `issueops domain`, `issueops split` 같은 존재하지 않는 서브커맨드를 반복적으로 호출한다. 원인은 IssueOps skill 산문이 grill/domain/split 같은 선명한 도메인 명사(phase 이름, 결정 동사, ledger artifact)를 1급 개념으로 쓰는 반면 CLI는 `phase`, `remote create-child`, `link-related` 같은 제네릭 동사를 쓰는 **명명 불일치**다.

주의:
- `issueops <domain-word>`를 추측으로 호출하지 말 것. 먼저 `issueops --help`로 실제 서브커맨드 레지스트리를 확인한다.
- `grill`/`problem`/`implement`/`ai-slop-clean`/`feedback`/`pr`은 lifecycle phase이며 `issueops phase --id ID --to <phase>`로 진입한다.
- `split`은 breakdown 결정(no-split 기본)이지 명령어가 아니다. child 생성은 `issueops remote create-child`, 기존 child 연결은 `issueops link-related --type splits-from`.
- `domain`(review), `compatibility`(review), `design`(review), `intent`(record)는 ledger artifact이며 각각 `issueops domain-review record`, `issueops compatibility review`, `issueops design review`, `issueops intent record`가 실제 명령이다.
- CLI는 잘못된 서브커맨드에 대해 did-you-mean 힌트를 출력한다(`issueops.go`의 `suggestIssueOpsSubcommand`). 매핑 전체는 `skills/issueops/SKILL.md`의 "Concept → Command Map" 섹션을 참조.

## 25. gh/glab create 출력은 항상 bare URL — JSON/Number 파싱 추가 금지

`gh issue/pr create`와 `glab issue/mr create`는 어떤 create 경로에서도 `--json` 플래그를 넘기지 않는다. 따라서 stdout은 항상 bare 아티팩트 URL 한 줄이다. 과거에 `parseGhOutput`/`parseGlabOutput`에 JSON-first 분기(`ghResult.Number`/`glabResult.IID` 채우기)를 뒀지만 도달 불가능한 죽은 코드였고 port 결과의 `Number`는 항상 ""였다(9d6be19에서 제거).

주의:
- create-output 파서는 URL을 trim해서 반환만 한다. JSON 디코딩/Number 추출 분기를 되살리지 말 것.
- 아티팩트 번호가 필요하면 URL에서 파싱한다(예: `parseGitHubIssueURL`), create 출력 JSON을 기대하지 않는다.
- provider가 JSON을 내는 경로는 `gh api`/`glab api graphql`(별도 `runGhAPIJSON`/`runGlabGraphQL`)뿐이며 create 파서와 무관하다.

## 26. `ValidateArtifactURL`은 verify-artifact(pr/mr) 전용 — issue 케이스 추가 금지

`remote.ValidateArtifactURL`의 유일한 prod 호출자는 `artifactverify.verificationFromRequest`이고, 이 함수는 호출 전에 `kind != pr/mr`을 하드 거부한다(이슈는 사이클의 remote-artifact가 아니다 — 사이클의 RemoteArtifact는 PR/MR이다). `create-issue --confirm`의 라이브 검증 게이트는 이 계층을 **거치지 않고** `VerifyRemoteArtifactLive` → `fetchGitHubIssueArtifact`/`fetchGitLabIssueArtifact`로 직행하며, fetcher가 자체 URL 파싱(GitLab은 `SplitGitLabIssuePath`로 project/IID만 요구하고 `issues`·`work_items` 별칭을 모두 받는다 — §31, GitHub은 `gh issue view`)을 한다.

주의:
- `ValidateArtifactURL`/`verificationFromRequest`에 `github:issue`/`gitlab:issue` 분기를 추가하면 죽은 코드가 된다(116ebef 리뷰에서 지적·제거).
- 새 아티팩트 종류의 라이브 검증을 배선할 때는 실제 도달 경로(`VerifyRemoteArtifactLive` switch + fetcher)를 확장하고, "이미 라우팅된다"는 주석은 도달 경로를 실증한 뒤에만 쓴다.
- 게이트 배선은 prod에서 CLI `issueOpsRemoteDeps`(`VerifyLive`)와 MCP `harnessapp/mcp_facade`(`VerifyIssueOpsRemoteArtifactLive`)가 주입한다. 미배선 기본값은 "dependency is not configured"를 반환하므로 게이트가 실제로 살아있는지 이 배선을 확인한다.

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

## 30. IssueOps worktree 세션의 source-checkout mirror edit 오인

IssueOps worktree 세션에서도 host cwd, MCP root, file-edit tool root가 원본 source checkout에 남아 있으면 worktree에 존재하는 같은 상대경로 파일을 source checkout에서 수정하는 실수가 발생한다. 이 경우 단순 branch gate만으로는 부족하다. 같은 파일이 worktree에도 있으면 source checkout edit는 대개 target drift다.

주의:
- `issueops execution prepare`가 반환한 canonical path를 `HARNESS_EXPECTED_WORKTREE`에 반영하고, 편집 전 cwd와 절대경로가 그 worktree를 가리키는지 확인한다.
- Worktree 세션에서는 file tool에 worktree 절대경로를 넘기고, shell은 `git -C "$HARNESS_EXPECTED_WORKTREE"` 또는 `rg "$pattern" "$HARNESS_EXPECTED_WORKTREE"`처럼 명시 root로 실행한다.
- Guard는 source checkout의 모든 edit를 막지 않는다. §21의 multi-path deadlock 방지 때문에 non-cycle branch에서 source checkout에 새 파일을 만드는 정상 작업은 허용되어야 한다.
- 방어층은 세 겹이다: PostToolUse source-checkout warning, PreToolUse mirror-file `ask`, SessionStart/UserPrompt worktree reminder. Host가 `ask`를 지원하지 않으면 Codex처럼 `block`으로 degrade될 수 있다.
- 선택된 cycle의 구현은 canonical worktree로 이동한다. Holder 교체가 필요하면 source에서 구현을 계속하지 말고 `issueops execution status`가 안내하는 generation-CAS replacement 절차를 따른다. Source checkout은 unrelated work에 계속 사용할 수 있다.

## 31. GitLab 이슈 URL의 `/-/issues/`·`/-/work_items/`는 같은 identity — 경로로 타입을 판별하지 말 것

GitLab 18.10+(work items list GA, `work_item_planning_view` 플래그 제거;
docs.gitlab.com/user/work_items/: "URLs that contain /epics/:iid or /issues/:iid
automatically redirect to /work_items/:iid")는 일반 이슈(`type=ISSUE`)의 `web_url`도
`/-/work_items/:iid`로 돌려주고(19.2.4-ee에서 관측) `glab issue create/view`는 그 값을
그대로 출력하므로, 레코드의 `issue_url`은 `work_items` 별칭으로 봉인될 수 있다. provider
파서 `parseGitLabIssueURL`이 `kind == "issues"`만 부모 이슈로 인정해 `close-issue`,
`reflect-completion`, `create-child`, `close-child`가 레코드 자신의 URL을
`parent_issue_url must be a GitLab issue URL`로 거부했고, `create-issue --confirm`의
라이브 게이트(`fetchGitLabIssueArtifact`)도 같은 검사로 원격 이슈가 이미 생성된 뒤에
실패할 수 있었다(2026-08-26 lesson).

주의:
- identity는 authority + project path + IID다. 두 별칭 모두 REST
  `projects/:path/issues/:iid`로 해석하고, Issue/Task 구분은 payload의
  `type`/`issue_type`으로만 판정한다(`verifyGitLabIssuePayloadIsTask` 선례).
- 파서에 `kind == "issues"`류의 경로 표식 검사를 되살리지 말 것. 자식 Task URL을 받는
  `parseGitLabWorkItemURL`은 GraphQL `workItemCreate`의 `webUrl`이 항상 `work_items`라
  그대로 둔다.
- "work_items URL은 부모 이슈가 아니다"를 단언하던 테스트는 잘못된 가정을 고정하는
  테스트였다. URL 형태로 원격 객체 타입을 고정하는 테스트는 provider가 실제로 돌려주는
  `web_url`을 재현해야 한다. 회귀는 `TestParseGitLabIssueURLAcceptsWorkItemsAlias`,
  `TestGitLabCloseIssueAcceptsWorkItemsIssueURL`,
  `TestGitLabUpdateIssueBodySectionAcceptsWorkItemsIssueURL`,
  `TestGitLabCreateChildPreviewAcceptsWorkItemsParentURL`,
  `TestFetchGitLabIssueArtifactAcceptsWorkItemsAlias`가 고정한다.

## 32. cleanup은 워크트리 점유 프로세스를 종료 대상으로 본다 — 요청자는 제외가 아니라 거부

`cleanup finish/abandon`은 워크트리를 점유한 프로세스와 그 워크트리에 매인 Orca
터미널을 차단 사유가 아니라 apply ①′의 종료 대상으로 다룬다(#477). preview가
receipt(pid·시작 시각·실행 파일)·자손/부수 피해 수·터미널 handle을 싣고 fingerprint에
결속하며, apply는 handle별 `orca terminal close --terminal`(handle 일치·`ptyKilled=true`
receipt 필수) → HUP+TERM → 최대 5초 → KILL → 재관측(점유·터미널 0 증명) 순서로
닫는다. 대화형 zsh/bash는 SIGTERM을 무시하고 SIGHUP에 종료된다(2026-08-27 실측).
터미널 close가 실패하면 시그널 경로로 넘어가지 않고
`workspace_processes_stop`에서 멈춘다 — 터미널만 죽고 orca 회수는 실패하는 부분 apply를
막기 위해서다(fagan #478 finding 2).

주의:
- 요청자 보호는 거부다. lease 경로(`executionQuiescenceFingerprint`)는 요청자 자손을
  quiescence 후보에서 *제외*하지만(직접 승계가 성립해야 하므로), cleanup은 워크트리를
  지우므로 요청자가 그 안에 서 있으면 진행 자체가 잘못이다 — `requester_occupies_worktree`,
  `requester_terminal_outside_worktree`, `requester_terminal_unresolved`,
  `worktree_is_source_checkout`으로 막는다. 한쪽을 다른 쪽에 맞춰 "고치지" 말 것.
- 요청자 터미널은 `ORCA_PANE_KEY`(tabId:leafId)·`ORCA_TERMINAL_HANDLE` env를 `orca terminal
  list --json` 전체 행과 join해서만 확정한다. 무선택자 `orca terminal show`는 호출자가 아니라
  UI-active 터미널을 돌려준다(brooks 실측). bulk `orca terminal stop --worktree`는
  fingerprint에 없던 동시 생성 터미널까지 닫으므로 cleanup에서 쓰지 않는다. preview가
  봉인한 exact handle만 닫고, 교체·신규 터미널은 최종 inventory에서 거부한다.
- 신호는 apply 시점에 실제로 점유 중인 프로세스에게만 보낸다. 자손 관계는 preview 점유자가
  같은 receipt로 아직 점유 중일 때의 stale 허용 근거일 뿐, cwd만 워크트리인 공유 서버(tmux)의
  다른 세션까지 죽이는 종료 범위 확장이 아니다.
- 생존 판정은 `kill(pid,0)`이 아니라 점유 재관측이다(좀비는 점유하지 않는다). lsof에 있는데
  ps 스냅샷에 없는 pid는 ESRCH면 소멸, 살아 있으면 `workspace_processes_observable`이다.
- Orca 바인딩 사이클은 런타임이 ready가 아니면 `orca_runtime_ready`로 막는다(터미널을 죽인
  뒤 orca 회수가 실패하는 부분 apply 방지). Orca 표면이 배선되지 않은 호출은 pid-조상
  게이트만 남는다. 미등록 워크트리의 `terminal list`는 `selector_not_found`이며 빈 목록이다.
- 런타임이 ready면 점유·바인딩·요청자 호스팅과 무관하게 워크트리 터미널을 나열한다. cwd를
  밖으로 옮긴 셸은 점유자가 아니어도 Orca 레지스트리에는 워크트리 터미널로 남아 있고 apply가
  닫아야 한다(fagan #478 finding 3). Orca 앱 pid(시그널 제외 대상)는 fingerprint 입력이라
  preview 뒤 런타임이 사라지거나 재시작되면 apply가 stale fingerprint로 멈춘다; apply ①′는
  Orca 상태를 다시 묻지 않는다.
- 새 실패 단계 `workspace_processes_stop`은 `knownCleanupFinishFailureStep`/
  `knownCleanupAbandonFailureStep`과 abandon failure inventory matcher에 등록돼 있다. 되돌릴
  때 이 값을 가진 receipt가 남아 있으면 `ReadIssueOps`가 거부한다. abandon receipt는 arm 시점
  자원 모양을 paired·worktree-only·branch-only·absent 네 가지로 인정하므로 #433 비대칭 잔여에서
  ①′가 실패해도 재-preview가 `cleanup_failure_inventory`로 영구히 막히지 않는다(fagan #478
  finding 1).

## dropped child와 done parent를 Stop orchestration에 재진입시키지 말 것

IssueOps PR readiness는 `validation_verdict=dropped` child를 scope에서 제외하지만 Stop hook의 별도 reminder 경로가 같은 규칙을 적용하지 않아, 병합된 parent가 `child_incomplete`로 영구 재진입한 사고가 있었다.

- dropped child는 active child total, incomplete key, unvalidated key에서 모두 제외한다. `rejected`나 verdict 없는 child와 혼동하지 않는다.
- 이미 `phase=done`인 bound parent는 오래된 session binding이 남아 있어도 orchestration reminder/Stop relay 대상으로 다시 읽지 않는다.
- core readiness와 hook hot path가 같은 child verdict 의미를 갖는 named regression을 함께 유지한다.
- bounded scan은 raw child 목록을 먼저 자른 뒤 dropped를 건너뛰지 않는다. dropped를 제외한 처음 N개를 읽어야 앞의 removed scope가 뒤의 active child를 숨기지 않는다.
