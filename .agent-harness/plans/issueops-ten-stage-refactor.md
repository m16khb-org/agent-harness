# IssueOps 10단계 재편 리팩토링 계획

> **For agentic workers:** 이 계획은 decision-complete다. 구현자는 판단을 추가하지 않고 태스크를 순서대로 실행한다. 각 태스크는 `- [ ]` 체크박스로 추적하고, 실행 전 반드시 `## Context`의 설계 요약을 읽는다. 태스크 실행 스킬은 `issueops-implement`(현행)이며, 이 계획을 IssueOps 사이클로 돌릴 때는 `link-plan` 시점에 `.agent-harness/issues/<n>/plan.md`로 옮긴다.

**Goal:** IssueOps 스킬을 사용자가 정의한 10단계(이슈 확정, 브랜치 준비, 문서 확인·계획·검토·인계, 구현, AI slop 정리, 프로젝트 문서 반영, 검증, 커밋·푸시, PR, 정리)로 재편하고, 어느 단계에서든 `issueops`를 실행하면 현재 단계를 판별해 이어갈지 제안하며, 어느 단계에서든 일시 중단·폐기·다른 세션에서 재개할 수 있게 한다.

**Architecture:** 실행 권한 모델(execution v1, generation-fenced lease)은 그대로 둔다. 준비 단계가 base SHA에 워크트리를 미리 만들고, 새 세션이 그 안에서 `execution prepare --mode direct`로 워크트리를 채택해 generation 1 홀더가 된다. 단계 판별은 새 read-only 명령 `agent-harness issueops next`가 소유하고, 스킬은 그 출력을 사용자에게 보여 주는 얇은 라우터가 된다. 탈출은 기존 `execution release`, `execution replace` 체인, `cleanup abandon`을 묶은 새 스킬 `issueops-abandon`이 맡는다. 미머지 사이클의 원격 정리는 기존 `remote close-issue`·`cleanup remote-branch`가 머지 증적을 요구해 불가능하므로, `cleanup abandon`이 draft PR/MR 닫기, 이슈 닫기, 원격 브랜치 삭제를 같은 preview·fingerprint·apply 안의 선택 효과로 수행하도록 넓힌다.

**Tech Stack:** Go 1.26(hexagonal vertical: contract → domain → application → adapter), Python 스킬 검사기(`scripts/validate-skill.py`, `scripts/verify-skill-shell.py`), 스킬 Markdown, `.agent-harness` 운영 문서.

**Spec:** 이 문서의 `## Context › 설계 요약` 절이 spec이다. 별도 spec 파일은 없다.

## 실행자 안내

이 계획은 작성한 모델이 아닌 다른 모델이 실행한다. 실행자는 대화 컨텍스트가 없다.

- 이 문서만으로 실행한다. 모르는 값은 `## Context › 설계 요약`과 각 태스크의 References에서 찾고, 없으면 추정하지 말고 그 태스크를 멈춘 뒤 보고한다.
- 순서는 `## Execution Strategy`의 wave다. 같은 wave 안의 태스크는 병렬 가능하지만 한 태스크는 한 세션이 끝까지 맡는다.
- Go 태스크는 RED→GREEN 순서를 지킨다. 실패 테스트를 먼저 쓰고 실패를 확인한 뒤 구현한다. 테스트를 삭제하거나 skip해서 통과시키지 않는다.
- 스킬 태스크는 `python3 scripts/validate-skill.py`와 `python3 scripts/verify-skill-shell.py`를 통과해야 끝난다.
- 모든 acceptance는 명령 출력으로 증명한다. `.agent-harness/evidence/`는 gitignore 대상이므로 최종 보고에 명령과 결과 요약을 옮겨 적는다.
- 이 계획은 brooks 적대 검토 두 라운드(2026-09-04, verdict revise)를 반영했다. 반영 항목은 Gap Analysis의 누락 5~16이고, 채택하지 않은 지적과 이유도 같은 절에 있다.
- 실행 방식은 이 저장소의 현행 issueops 사이클을 권장한다. `issueops-create-issue`로 이슈를 만들고 `link-plan` 시점에 이 파일을 `.agent-harness/issues/<n>/plan.md`로 옮긴다. T11이 끝나기 전까지는 현행 스킬 이름으로 진행한다.

## Global Constraints

- 세 호스트(Codex, Claude Code, Omo)에서 같은 스킬과 같은 CLI JSON을 쓴다. 스킬에 Claude 전용 플러그인(superpowers 등)이나 호스트 전용 pseudo-API를 넣지 않는다(`AGENTS.md` §10, `internal/adapter/skillcontract/skill_contract_test.go:82-86`).
- hook은 SessionStart context-only다. IssueOps 상태를 읽거나 판단하지 않는다(ADR 2026-08-27, `.agent-harness/AGENT_WORKFLOW.md` hook 절). 단계 판별은 CLI가 한다.
- provider mutation은 raw `gh`/`glab`로 우회하지 않는다. preview 없이 `--confirm`을 붙이지 않는다.
- IssueOps record schema_version은 1이며 이 계획은 schema를 바꾸지 않는다. 새 필드를 record에 추가하지 않는다.
- 새 CLI 명령은 `internal/domain/cli/issueops_catalog.go` 한 곳에만 usage 줄을 추가하고, `usage.golden.txt`와 `response_contracts.golden.json`을 `-update`로 재생성한다(`.agent-harness/CAUTIONS.md` Update workflow 5).
- `issueops next`는 네트워크를 호출하지 않는다. `git fetch`, provider readback, Orca RPC 어느 것도 실행하지 않으며, 그 판단이 필요한 단계(커밋·푸시의 strict readiness, 정리의 머지 확인)는 해당 단계 스킬이 명시적으로 실행한다.
- Go에는 스킬 이름과 한국어 UI 문자열을 두지 않는다. `next`는 `stage.key`와 명령만 돌려주고, 키를 스킬·label·선택지로 바꾸는 표는 `skills/issueops/SKILL.md`가 소유한다. 스킬 이름 변경이 Go와 골든 재생성을 요구하지 않게 하기 위해서다.
- 레거시는 남기지 않는다. 대체된 스킬·레퍼런스·문서 절은 삭제하고 "deprecated" 표기로 남기지 않는다. ADR은 append-only이므로 삭제하지 않고 superseding 결정을 추가한다.
- 한국어로 원격에 남는 텍스트와 스킬 본문은 `fluent-korean` 규칙을 따른다.
- 커밋은 `.agent-harness/COMMIT_POLICY.md`의 Conventional Commit subject + Lore body를 따르고 `atomic-commit-push`로만 한다.

---

## TL;DR

> **Summary**: 스킬 9개를 10단계 기준 17개(라우터 1, 단계 10, 탈출 1, 동기화 2, 공용 3)로 재편하고, 단계 판별 명령 `issueops next`를 추가하며, `cleanup abandon`에 draft PR/MR 닫기·이슈 닫기·원격 브랜치 삭제를 선택 효과로 넓히고, 단계마다 반복되던 절차(적대 리뷰, 게이트 원장, 원격 쓰기)를 공용 스킬 3개로 뽑고, 계획 전 프로젝트 문서 확인과 구현 후 문서 반영을 단계로 두고, 대체된 스킬 1개와 레퍼런스 4개와 문서 절 2개를 삭제한다.
> **Deliverables**: `issueops next` CLI, `cleanup abandon` 원격 효과, 단계 스킬 `issueops-prepare`·`issueops-plan`·`issueops-clean`·`issueops-docs`·`issueops-verify`·`issueops-abandon` 신설, 공용 스킬 `issueops-review`·`gates-ledger`·`issueops-remote-write` 신설, `issueops`·`issueops-create-issue`·`issueops-implement` 재작성, `issueops-branch-worktree`와 레퍼런스 4개 삭제, ADR·AGENT_WORKFLOW·운영 가이드·아키텍처·CAUTIONS·README·골든 갱신, 일회용 저장소 E2E 프로브 통과.
> **Effort**: Large
> **Parallel**: YES, 9 waves (wave당 4개 이하)
> **Critical Path**: T0 → T0b → T1 → T6 → T7 → T11 → T15 → T16 → T17

## Context

### 왜 이 하네스인가 (사용자가 밝힌 목적, 2026-09-04)

사용자는 Codex, Claude Code, Omo 같은 여러 에이전트를 함께 쓴다. 이 하네스를 만드는 이유는 두 가지다.

1. **어느 에이전트에서든 같은 절차로 작업한다.** 호스트가 달라도 같은 스킬, 같은 CLI, 같은 단계 게이트를 지난다. 그래서 스킬은 호스트 중립이어야 하고, 단계 판별과 lease 같은 판단은 스킬 프롬프트가 아니라 CLI가 소유해야 한다.
2. **팀과 공유하는 이슈로 SSOT를 유지하고 진행 상태를 확인한다.** 작업 계약은 원격 issue 본문이 유일한 원본이고, 브랜치·PR/MR·검증 증거는 그 issue에 연결되어 팀원이 provider 화면에서 진행 상태를 읽을 수 있어야 한다.

이 계획의 결정은 이 목적에서 나온다. `issueops next`를 CLI에 두는 것은 1번 때문이고, 1단계가 issue 본문을 계약으로 확정하고 `sync-issue`·`reflect-*`·`feedback mark-issue-updated`로 본문을 최신으로 유지하는 것은 2번 때문이다. 탈출·재개를 다른 세션과 호스트에서 같은 명령으로 하게 하는 것도 1번의 연장이다.

이 목적에서 이 계획이 아직 덮지 않는 것이 하나 있다. 팀원이 issue 화면만 보고 "지금 어느 단계인가"를 알려면 phase 전이마다 issue 본문의 관리 블록(예: `issueops:stage`)을 갱신하는 표면이 필요하다. 현재는 생성 시 본문, devils-advocate 반영, completion 반영, `sync-issue`만 있다. 이 표면은 관리 블록 body-sync 설계를 따로 검토해야 하므로 후속 이슈로 남긴다(Gap Analysis 누락 3).

### Original Request

사용자 요청 네 건을 그대로 옮긴다.

1. "issueops 스킬들과 단계들을 개선하고 싶어. 1 이슈생성(사용자가 제공한 정보와 코드베이스와 배경지식과 질문을 통해 이슈를 확정하고 생성) 2 준비(이슈기반 브랜치와 워크트리를 생성) 3 이슈 구현(해당 워크트리에서 사용자가 새로운 세션을 띄워서 구현시작: 계획 작성, 계획 검토(수립 - 검토 반복하여 완성될때까지), 만들어진 계획 기반 구현진행) 4 구현 후 atomic commit push 5 이슈 pr 작성 6 issueops의 부산물들 cleanup"
2. "어느단계든 issueops 스킬을 실행하면 현재 단계를 파악한 후 이어서 진행할지 제안해주면 좋겠어"
3. "상세한 리팩토링 계획을 수립해줘. 코드베이스 전체를 아우르는 빠트리는 곳없는 계획으로, 레거시는 남기지 않고 삭제하는 방향으로"
4. "어느 단계에서든 탈출하고 이슈옵스를 정리할수있으면 좋겠어. 다른 세션이나 에이전트에서 다시 시작할수도있고"

### Interview Summary

분석 세션(2026-09-04)에서 소스와 실측으로 확정한 사실이다. 구현자는 이 사실을 다시 조사하지 않는다.

| 확정 사실 | 근거 |
|---|---|
| direct 모드는 `execution prepare --confirm`을 호출한 세션이 즉시 generation 1 홀더가 된다 | `internal/application/issueopspreparation/prepare.go:286-338` |
| orca 모드는 prepare 전에 staged plan, intent, design review, devils-advocate 기록을 요구하고 owner는 계획을 만들지 못한다 | `internal/contract/issueopspreparation/planner_gates.go:37-60`, `.agent-harness/karpathy/prompts/issueops-v1-owner-execution-v1.md` 시작 절차 8 |
| 같은 경로에 이미 있는 워크트리는 top-level 경로·브랜치·HEAD가 base SHA와 같을 때 `execution prepare`가 새로 만들지 않고 채택한다 | `internal/adapter/gitworktree/provisioner.go:102-114` |
| explicit direct는 `--direct-reason`(비어 있지 않은 UTF-8, 512바이트 이하, 제어문자 금지)이 필수이고, `--mode auto`는 Orca가 ready면 owner를 새로 띄운다 | `internal/domain/issueopspreparation/decision.go:120-129,168-180`; 이 머신은 `orca status --json`에서 `runtime.state=ready` |
| prepare의 접근 프로브는 `<source>.worktrees` 아래에 디렉터리를 만들어 보고, 막히면 `claude --add-dir <base>` 또는 `codex --cd <source> --add-dir <base>`를 안내한다 | `internal/adapter/gitworktree/provisioner.go:20-46,147-160,180-186` |
| `issueops list --repo`는 워크트리 경로를 받아도 git common dir로 source root를 구해 같은 결과를 낸다 | `internal/adapter/outbound/issueopsinventory/runtime.go:15-27`; 2026-09-04 실측: api-servers 워크트리 경로와 source root 모두 22건 |
| plan phase 진입은 `issue_url`, `branch`, plan-prep 4항목, `split_decision`(scope decision 또는 child 링크), `domain_review`를 요구한다. `branch`는 `branch prepare`가 기록한다 | `internal/adapter/issueops/issueops_phase_ledger.go:27-66`, `internal/adapter/issueops/branchprepare/branch_prepare.go:106-107` |
| implement 진입은 worktree_path, plan_in_worktree, compatibility 승인, devils-advocate 판정의 plan digest 일치, 이 세션이 쥔 execution lease를 요구한다 | `internal/adapter/issueops/issueops_readiness.go:112-158,161-185` |
| design review 승인은 open question 0건, refactor plan, alternatives, risks, verification evidence를 요구한다 | `internal/adapter/issueops/issueops_readiness.go:255-288` |
| phase 전이는 전진만 허용하고 후진은 `regress`가 grill로 되돌린다 | `internal/adapter/issueops/issueops_phase.go:74-79`, ADR 2026-06-29, 2026-07-02 |
| `cleanup abandon`은 reason, 살아 있는 writer 없음(active·revoking 거부, claimable·released 허용), remote artifact가 있으면 미머지 관측, 미해결 child 없음, 깨끗한 워크트리, pending intent 없음, Orca 자원 없음을 요구하고 원격 issue와 PR/MR은 건드리지 않는다 | `internal/adapter/issueops/issueops_cleanup_abandon.go:227-275`, `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go:556,609-640` |
| `execution release`의 cwd는 canonical worktree root와 정확히 같아야 한다 | `internal/domain/issueopslease/release.go:55-63`, `internal/adapter/outbound/issueopslease/filesystem.go:10-26` |
| released·claimable lease는 `replace --preview` → `replace --reseed --inventory-fingerprint` → `claim --claim-current-token`으로 새 세션이 generation을 넘겨받는다. reseed는 direct에도 새 claim token을 만든다 | `internal/domain/issueopslease/reseed.go`, `internal/application/issueopslease/reseed.go:118-132`, `internal/application/issueopspreparation/prepare.go:496-527` |
| provider port에는 `CloseIssue`·`CloseChild`만 있고 PR/MR을 닫는 메서드가 없다. `CloseIssue` 요청에는 reason이 없고 GitHub adapter가 `--reason completed`를 고정한다 | `internal/port/provider.go:227-256`, `internal/adapter/provider/github/provider.go:723` |
| `remote close-issue`는 검증된 artifact와 provider 머지 readback을 하드 게이트로 요구하고, `cleanup remote-branch`는 `phase_done`·`child_tasks_closed`·`remote_artifact_present`·`remote_artifact_merged`를 요구한다. 미머지·무artifact 사이클은 이 표면으로 이슈를 닫거나 원격 브랜치를 지울 수 없다 | `cmd/harness/issueopscli/remotecmd/remote.go:238-262`, `internal/adapter/issueops/issueops_cleanup_remote_branch.go:145-180,240-260`; 2026-09-04 실측(io-15f1518189ca): 각각 `cannot verify merge evidence before a verified remote artifact`, missing `phase_done, child_tasks_closed, remote_artifact_present` |
| `cleanup abandon --apply`는 마지막에 record를 삭제하므로 그 뒤에는 `--id` 기반 명령을 쓸 수 없다 | `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go:581-583`, `internal/adapter/issueops/issueops_cleanup_abandon.go:140-190` |
| strict PR readiness는 `git fetch --quiet`를 실행하고, cleanup status의 `merged`는 호출자가 provider readback으로 검증해 넣는 입력이다 | `internal/adapter/issueops/issueops_pr_readiness_strict.go:36-50`, `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go:609-637` |
| lease에 writer가 없을 때의 exact 회복 명령은 `executionWriterAbsentRecoveryCommand`가 mode·status별로 렌더한다(orca claimable은 resume, direct claimable은 claim, released는 replace preview, revoking은 finalize-preview) | `internal/adapter/issueops/execution_lease.go:334-356` |
| skill 이름이 박힌 곳은 README 두 개, `response_contracts.golden.json`의 skill manifest, benchmark guideline 문자열(create-issue·create-pr만)이다 | `README.md:270`, `README.en.md:282`, `cmd/harness/testdata/response_contracts.golden.json:868-904,5229-5234`, `internal/adapter/issueops/benchmark/issueops_benchmark_quality.go:126` |

### T0b 스파이크 실측 결과 (2026-09-05)

격리된 `HARNESS_STATE_DIR`과 로컬 스크래치 저장소에서 현행 CLI만으로 확인했다. provider 호출은 없었고 사용자의 실제 레코드는 그대로다(전후 32건). 아래는 spec의 근거이며 추측이 아니다.

| # | 실측 | 계획에 미친 영향 |
|---|---|---|
| 1 | **채택은 동작한다.** 2단계가 `git worktree add <root> -b <branch> <base_sha>`로 만든 워크트리를 `execution prepare --mode direct`가 그대로 채택했다. 워크트리 수는 전후 모두 2, lease는 generation 1 active, holder는 호출 세션이다 | 설계 요약 2의 핵심 기제 확정. T8 부트스트랩 절 그대로 |
| 2 | **회복 체인이 완주한다.** released → `replace --preview` → `--reseed`(generation 2 claimable) → `claim --claim-current-token` → generation 2 active. 각 단계가 다음 exact 명령을 돌려준다 | 규칙 7b와 `issueops-abandon` 재개 경로 확정 |
| 3 | **돌려주는 `next_command`에는 `--json`이 없다.** 텍스트로 출력되므로 파싱하려면 호출자가 `--json`을 덧붙여야 한다 | 라우터와 단계 스킬은 체인 명령을 그대로 실행하되, 결과를 파싱할 때만 `--json`을 덧붙인다고 적는다 |
| 4 | **`link-issue`가 phase를 `plan`으로 자동 전진시킨다.** `IssueOpsPlanReadiness`가 ready면(intent + issue_url + plan-prep) grill 게이트(branch)를 건너뛰고 올라간다(`internal/adapter/issueops/linking/link.go:37-39`). 실측에서 branch 없이 phase가 `plan`이 됐고 이후 `phase --to grill`은 후진으로 거부됐다 | **분류표 규칙 6이 죽는다.** phase 기준이 아니라 artifact 기준으로 고쳤다(아래 규칙 6·7a) |
| 5 | **fingerprint는 untracked 파일을 포함한다.** 봉인 뒤 `.agent-harness/issues/<n>/`에 파일 하나를 만들자 `ai_slop_clean_stale`이 떴고(`c3c22c03…` → `f4f178c8…`), `ai-slop-clean record` 재실행으로 사라졌다 | brooks 1위 지적 확정. 4단계에서 파일 쓰기를 끝내고 5단계가 재봉인하는 순서가 필수다 |
| 6 | **`phase --to ai-slop-clean` 진입이 자동 봉인한다.** 전이 직후 stored == current다 | 4단계 스킬은 진입 봉인과 record 재봉인을 모두 설명한다 |
| 7 | **`gates_incomplete:<file>`은 원장 파일이 생기는 즉시 뜬다.** 미충족 게이트가 있으면 strict missing에 들어간다 | `gates-ledger`가 만든 원장은 4단계에서 `--write`로 채워야 한다 |
| 8 | **direct 모드에는 `implementation_review`가 missing에 없고 `project_docs_review`는 있다** | T24(전 모드 확장)가 필요하다는 근거. `project_docs_review`는 이미 전 모드 적용 |
| 9 | **접근 프로브는 `--confirm`에서만 돌고, claude relaunch는 `claude --add-dir '<base>'`로 `cd`가 없다.** codex·omo는 source root로 간다(`provisioner.go:180-190`) | T25 결함 확정 |
| 10 | **위장 세션은 거부된다.** 살아 있는 무관 PID로 ACTOR_FLAGS를 만들면 `native session process receipt is not in the local process ancestry`로 막힌다 | T17의 다중 세션 시나리오는 진짜 별도 프로세스 트리가 필요하다. 축약 불가 |
| 11 | **순서 제약 둘.** design review는 `link-plan`보다 먼저여야 하고, `--verification`에 "design"과 "review"(또는 "설계"와 "검토")가 함께 들어가야 `design_review_evidence`가 통과한다. `compatibility review` 기록은 phase를 `compatibility-review`로 자동 전진시킨다 | T8 계획 절의 명령 순서를 이대로 고정한다 |

### 설계 요약 (spec)

#### 1. 사용자 단계와 스킬

| 단계 | 스킬 | 소유 명령 | 비고 |
|---|---|---|---|
| 1 이슈 확정·생성 | `issueops-create-issue` (재작성) | `start`, `intent record`, `domain-review record`, `decision add --kind scope`, `plan-prep record`, `phase --to grill`, `remote score`, `remote create-issue`, `link-issue` | 조사 결과가 plan-prep evidence가 된다. blocking 질문만 사용자에게 묻는다 |
| 2 브랜치 준비 | `issueops-prepare` (신설, `issueops-branch-worktree` 삭제) | `branch prepare`(base SHA 봉인), provider 링크, `phase --to plan` | 워크트리를 만들지 않는다. 워크트리 provisioning은 `execution prepare`가 소유한다(direct는 git, orca는 `orca worktree create`). lease도 부여하지 않는다 |
| 3 문서 확인·계획·검토·인계 | `issueops-plan` (신설) | `project_docs_route`·`project_docs_read`, `artifact stage --name plan`, `design review`, `issueops-review --target plan` → `devils-advocate review`, `regress`, `execution prepare --mode auto` | source checkout의 준비 세션이 수행한다. 계획은 워크트리가 없으므로 source 밖 임시 파일에 쓰고 `artifact stage`한다. 마지막 `execution prepare --mode auto`가 모드를 고른다: Orca가 준비돼 있으면 Orca가 워크트리와 구현 세션을 만들고, 없으면 direct로 같은 세션이 generation 1 홀더가 된다 |
| 4 구현 | `issueops-implement` (재작성) | (orca면) sealed `claim`, `link-plan`, `compatibility review`, `phase --to implement`, `gates-ledger`, TDD, child 위임, `phase --to ai-slop-clean` | canonical worktree의 구현 세션이 수행한다. Orca 세션이면 봉인된 claim 명령으로 시작하고, direct면 3단계 세션이 그대로 이어간다 |
| 5 AI slop 정리 | `issueops-clean` (신설) | 정리 pass, `shannon` 측정, `gates check --write`, turing report 확정, focused 재검증, `ai-slop-clean record` | 코드와 증거 파일을 바꾸는 작업을 여기서 끝내고 마지막에 `ai-slop-clean record`로 봉인한다. fingerprint는 변경·untracked 파일 전체의 내용 해시라(`implementation/evidence.go:52-106`) 봉인 뒤 어떤 파일이라도 바뀌면 `ai_slop_clean_stale`이 되고 `next`가 이 단계로 되돌린다. 그것이 의도된 회귀다 |
| 6 프로젝트 문서 반영 | `issueops-docs` (신설) | `project_docs_route`, `project_docs_read`, `project_docs_append`(kind=adr·caution), `project_docs_revise`(기존 `project-docs-update` 스킬), `ai-slop-clean record`(재봉인), `project-docs-review record` | 구현 diff를 운영 문서와 양방향으로 대조해 ADR·CAUTIONS·CONVENTIONS 등을 고친 뒤, 문서 수정으로 바뀐 fingerprint를 `ai-slop-clean record`로 다시 봉인하고 `project-docs-review record --verdict updated|no-change`를 기록한다. 이 기록이 durable 게이트이며 이후 diff가 바뀌면 `project_docs_review_stale`이다 |
| 7 검증 | `issueops-verify` (신설) | 읽기 전용 검증 battery 재실행, `schema-evidence record`, `implementation-review record`(`issueops-review`), `compatibility review` 재기록, `pr-readiness --strict` | 파일을 만지지 않는다. 기록 명령만 실행한다. 고칠 것이 생기면 5단계로 돌아간다 |
| 8 커밋·푸시 | `atomic-commit-push` | `phase --to pr` | 검증이 봉인한 파일을 더 고치지 않고 plan.md, gates.md, turing report, 문서, 구현을 커밋·푸시한 뒤 `next`가 렌더한 `phase --to pr`를 실행한다 |
| 9 PR/MR 발행·완료 | `issueops-create-pr`, `issueops-complete` (소폭 수정) | 현행 | 변경은 cross-link와 `next` 전제뿐이다 |
| 10 머지 후 정리 | `issueops-cleanup` (소폭 수정) | 현행 | abandon 금지 문구를 `issueops-abandon` 안내로 바꾼다 |
| 탈출 | `issueops-abandon` (신설) | `execution release`, `execution replace` 체인, `cleanup abandon`(`--close-pr`, `--close-issue`, `--delete-remote-branch` 선택 효과 포함) | 일시 중단·재개·인수·폐기 네 경로. 미머지 사이클의 원격 정리는 abandon만이 소유한다 |
| 라우터 | `issueops` (재작성) | `issueops next` | 단계 표, 공통 불변식, gate map만 남긴다 |

#### 2. 세션 경계와 lease

모드 정책은 사용자 결정이다(2026-09-05): **Orca가 설치·준비돼 있으면 Orca를 쓰고, 아니면 direct를 쓴다.** 그 선택은 `execution prepare --mode auto`가 mutation 전에 probe로 내린다. 스킬은 모드를 강제하지 않는다.

- **1~3단계는 source checkout의 준비 세션**이 수행한다. 2단계는 branch identity(provider, issue, branch, base branch, base SHA)만 기록하고 워크트리를 만들지 않는다. 3단계는 문서 확인, 계획 작성, `artifact stage --name plan`, design review, devil's-advocate 검토까지 끝낸 뒤 `execution prepare --mode auto`를 preview·confirm한다.
- **워크트리 provisioning은 `execution prepare`가 단독으로 소유한다.** direct는 git으로 `${SOURCE_ROOT}.worktrees/${BRANCH//\//-}`를 만들고, orca는 `orca worktree create --name <branch>`로 자기 브랜치와 워크트리를 만든다(`internal/adapter/orca/client.go:284-333`). 그래서 2단계가 로컬 브랜치나 워크트리를 미리 만들면 Orca 경로가 깨진다. 스킬은 `git worktree add`를 실행하지 않는다.
- **4단계 이후는 구현 세션**이 canonical worktree에서 수행한다. Orca 모드면 prepare가 그 세션을 띄우고 lease는 claimable로 남으며, 세션은 봉인된 `execution claim --claim-current-token` 명령 하나로 홀더가 된다. direct 모드면 prepare를 호출한 그 세션이 곧바로 generation 1 홀더이므로 같은 세션이 이어간다.
- Orca가 고른 세션과 direct의 같은 세션 모두 4단계 이후의 스킬 순서는 동일하다. 다른 것은 시작점 하나뿐이다: Orca 세션은 claim부터, direct 세션은 `link-plan`부터.
- ADR 2026-07-24의 이원 구조는 유지된다. 이 계획이 바꾸는 것은 planner 세션이 어디까지 하느냐가 아니라, 그 뒤 단계를 정리·문서·검증으로 쪼개고 단계 판별을 CLI가 소유하게 하는 것이다.

#### 3. `agent-harness issueops next` (read-only)

usage: `agent-harness issueops next [--id ID] [--cwd PATH] [--json]`

동작 순서:
1. `--cwd`(기본 프로세스 cwd)에서 source root를 구한다(`repoidentity.SourceRoot` + `git rev-parse --path-format=absolute --git-common-dir`). cwd가 `<source>.worktrees/<x>` 아래면 `cwd_role=worktree`, source root면 `source`, 둘 다 아니면 `other`.
2. `ListCycles(stateRoot, sourceRoot)`로 후보를 모은다. 선택 우선순위: `--id` > 환경변수 `ISSUEOPS_ID` > `workspace_root == cwd` > `branch == git branch --show-current` > done이 아닌 사이클이 정확히 하나. 그 외는 `candidates`를 돌려주고 `stage.key=ambiguous`. 2단계까지의 사이클은 `execution prepare` 전이라 `workspace_root`가 비어 있으므로 branch 매칭으로 선택된다. 선택된 entry가 `invalid`이거나 record 읽기가 실패하면 `stage.key=invalid`, next_command `agent-harness issueops status --id <id> --json`이다. 사용자가 새 사이클을 원하면 선택과 무관하게 `issueops start`를 쓸 수 있어야 하므로 라우터 선택지에 "새 사이클 시작"이 항상 있다(설계 요약 9).
3. 선택된 record에 대해 phase completion(현행 `IssueOpsPhaseCompletion`), lease와 whoami actor 비교, holder 프로세스 liveness(`ObserveNativeProcessReceipt`), canonical 워크트리 존재 여부(`git -C <root> rev-parse --show-toplevel`, `branch --show-current`, `rev-parse HEAD`), phase가 `ai-slop-clean`·`feedback`이면 fetch 없는 local PR readiness(T6가 strict에서 분리하는 `IssueOpsLocalPRReadiness`: branch_match, worktree_clean, upstream 존재, fingerprint stale 키, `gates_incomplete`)를 모아 순수 함수 `issueopsnext.Classify`에 넣는다. writer 없는 lease의 회복 명령은 `execution_lease.go:334-356`의 함수를 export해 그대로 쓴다.
4. 결과를 JSON 또는 텍스트로 출력한다. mutation도, 네트워크 호출도 없다. 머지 여부와 upstream 동기화 여부는 판단하지 않고 해당 단계 스킬의 명령으로 넘긴다.

분류표(`Classify`의 규칙 순서. 위 규칙이 먼저 맞으면 아래는 보지 않는다):

| 순서 | 조건 | stage.key | index | next_command |
|---|---|---|---|---|
| 1 | record 없음 | `none` | 0 | `agent-harness issueops start --repo <source_root> --json` |
| 2 | `execution.pending != nil` | `blocked.pending` | 현재 index | `agent-harness issueops execution reconcile --id <id> --preview` |
| 3 | execution == nil 그리고 phase ≥ plan 그리고 다른 entry가 이 사이클의 canonical root(`<source>.worktrees/<branch '/'→'-'>`)를 이미 점유 | `blocked.root_conflict` | 3 | `agent-harness issueops list --repo <source_root> --json`(충돌 사이클을 `issueops-cleanup` 또는 `issueops-abandon`으로 먼저 정리한다. `EnsureRootUnclaimed`가 phase·lease와 무관하게 거부한다) |
| 4 | lease active이고 holder가 이 세션이 아니며 live | `blocked.holder_live` | 현재 index | `agent-harness issueops execution status --id <id> --json` |
| 5 | lease active이고 holder가 이 세션이 아니며 not live, 또는 lease revoking | `takeover` | 현재 index | `execution_lease.go:334-356`이 렌더하는 회복 명령 |
| 6 | lease ∈ {claimable, released} 그리고 execution.completion == nil | `claim` | phase별 index | `agent-harness issueops execution status --id <id> --json`. Orca가 띄운 세션은 자기 프롬프트의 봉인된 `execution claim --claim-current-token`을 실행하고, 그 밖의 세션은 status가 돌려주는 `next_command` 체인을 따른다 |
| 7 | phase ∈ {problem, grill} 그리고 issue_url 비어 있음 | `issue` | 1 | 1단계 고정 순서의 첫 missing: `intent_contract`(raw_request·interpreted_intent·success_criteria) → `domain_review` → `split_decision` → `plan_prep_*` → `issue_url`. `branch`는 2단계 항목이라 여기서 권하지 않는다(`missing`은 알파벳 정렬이라 첫 항목을 그대로 쓰면 `branch`가 나온다) |
| 8 | issue_url 있음 그리고 execution == nil 그리고 (phase ∈ {problem, grill} 또는 `branch_prepare == nil` 또는 `branch_prepare.base_sha` 비어 있음) | `prepare` | 2 | phase가 problem이면 `phase --id <id> --to grill`, 그 밖에는 `branch prepare --id <id> --provider <p> --issue-url <url> --branch <n-slug> --base-branch <ref> --base-sha <sha> --link-verified ...` |
| 9 | execution == nil 그리고 branch+base_sha 있음 그리고 staged plan artifact 없음 | `plan.write` | 3 | `agent-harness issueops artifact stage --id <id> --name plan --file <source checkout 밖 임시 파일>` |
| 10 | execution == nil 그리고 staged plan 있음 그리고 design review 미승인 | `plan.design` | 3 | `agent-harness issueops design review --id <id> --problem-summary ... --proposed-design ... --refactor-plan ... --alternative ... --risk ... --verification "design review checked alternatives and risks" --approved ...` |
| 11 | execution == nil 그리고 design 승인 그리고 `Completion(implement).Missing`에 `devils_advocate_review` 또는 `devils_advocate_review_stale` | `plan.review` | 3 | 판정이 `stop`이고 waive되지 않았으면 `agent-harness issueops regress --id <id> --reason <TEXT> ...`, 그 밖에는 `agent-harness issueops devils-advocate review --id <id> --reviewer-context subagent --verdict <v> --finding <TEXT> ...` |
| 12 | execution == nil 그리고 위 세 planner 게이트가 모두 기록됨 | `plan.handoff` | 3 | `agent-harness issueops execution prepare --id <id> --mode auto --owner-host <actor host> $ACTOR_FLAGS --json`(preview. `$ACTOR_FLAGS`는 리터럴) |
| 13 | lease active·self 그리고 phase ∈ {plan, compatibility-review} | `implement.enter` | 4 | plan_path가 비었으면 `link-plan --id <id> --plan-path <worktree 안 경로>`, compatibility review가 미승인이면 `compatibility review ... --approved`, 둘 다 있으면 `phase --id <id> --to implement` |
| 14 | lease active·self 그리고 phase == implement | `implement` | 4 | `agent-harness issueops phase --id <id> --to ai-slop-clean ...` |
| 15 | phase == ai-slop-clean 그리고 `Completion(ai-slop-clean).Missing`에 ai_slop_clean_at·ai_slop_clean_head·ai_slop_clean_fingerprint·cleanup_evidence·verification_evidence 중 하나, 또는 phase ∈ {ai-slop-clean, feedback} 그리고 local missing에 `ai_slop_clean_stale`·`ai_slop_clean_fingerprint`·`current_fingerprint` | `clean` | 5 | `agent-harness issueops ai-slop-clean record --id <id> --category <TEXT> --verification <TEXT> ...`(stale은 봉인 뒤 파일이 바뀌었다는 뜻이므로 5단계로 돌아가 정리·확정·재봉인한다. 의도된 회귀다) |
| 16 | phase ∈ {ai-slop-clean, feedback} 그리고 local missing에 `project_docs_review` 또는 `project_docs_review_stale` | `docs` | 6 | `agent-harness issueops project-docs-review record --id <id> --verdict updated|no-change --evidence <TEXT> ...`(stale이면 문서 수정 뒤 `ai-slop-clean record`로 재봉인한 다음 기록한다) |
| 17 | phase ∈ {ai-slop-clean, feedback} 그리고 local missing에 schema_evidence*, implementation_review*, feedback_*, contract_feedback_issue_update, gates_incomplete:* 중 하나 이상 | `verify` | 7 | 첫 missing의 owner command |
| 18 | phase ∈ {ai-slop-clean, feedback} 그리고 local missing ⊆ {worktree_clean, upstream} | `commit-push` | 8 | `agent-harness issueops pr-readiness --id <id> --strict --json`(strict가 fetch와 upstream 동기화를 판정한다. 통과하면 단계 스킬이 `phase --to pr`를 실행한다) |
| 19 | phase == pr 그리고 execution != nil 그리고 remote_artifact == nil | `pr.create` | 9 | `remote create-pr --id <id> --expected-generation <g> ...` |
| 20 | phase == pr 그리고 execution != nil 그리고 remote_artifact != nil 그리고 execution.completion == nil | `pr.complete` | 9 | `execution complete --id <id> --generation <g> ...` |
| 21 | phase == done | `done` | 10 | `agent-harness issueops cleanup status --id <id> --merged --json`(머지 여부는 provider readback이 필요하므로 `next`가 판단하지 않고 warning `merge state requires provider readback`을 붙인다) |
| 22 | 위 어느 규칙에도 맞지 않음(예: pr인데 execution == nil, local missing에 `branch_match`·`repo`·`repo_git`·`plan_exists`·`plan_in_worktree`·`worktree_path`·`design_*`·`duplicate_issue_artifact:*`·`child_*`) | `unknown` | 현재 index | `agent-harness issueops status --id <id> --json`. `warnings`에 매치되지 않은 missing 키를 전부 싣는다 |

규칙 8~12가 2·3단계를 artifact 기준으로 가른다. phase만 보면 안 되는 이유는 실측 4다. `link-issue`는 plan readiness가 서면 phase를 `plan`으로 올리므로, 이슈 생성 직후의 사이클은 branch도 없이 phase가 `plan`이다. 그래서 branch·base_sha 유무가 2단계와 3단계를 가르고, staged plan·design·devil's-advocate 유무가 3단계 안을 가르며, 세 planner 게이트가 다 서면 `plan.handoff`가 `execution prepare --mode auto`를 렌더한다. 워크트리 존재는 조건에 넣지 않는다 — 워크트리는 prepare가 만들기 때문이다.

규칙 6이 `claim`을 별도 단계로 두는 이유는 Orca와 direct의 유일한 차이가 여기이기 때문이다. Orca 모드의 prepare는 lease를 claimable로 남기고 세션을 띄운다. 그 세션은 자기 프롬프트의 봉인된 claim 명령을 정확히 한 번 실행한다. direct 모드에서는 prepare가 곧바로 active를 주므로 규칙 6이 발화하지 않는다. released 상태(일시 중단 뒤)도 같은 규칙으로 모아 status의 회복 체인으로 보낸다.


`next_command`는 두 종류다. `next_command_kind=exact`는 그대로 실행하는 명령이고, `next_command_kind=template`는 `<...>`나 `$ACTOR_FLAGS` 자리표시자를 채워야 하는 명령이다. 라우터는 template을 실행하지 않고 채울 값을 사용자에게 보여 준다.

`stage.key`에서 스킬 이름과 한국어 label, 선택지 3개로 가는 표는 `skills/issueops/SKILL.md`의 `## 단계 표`가 소유한다(설계 요약 9). Go는 키·index·명령만 돌려준다. `missing` 키의 owner command 대응표는 domain 패키지의 `OwnerCommand`가 유일한 소유자이며, 현행 라우터의 "Gate map" 표는 삭제한다. local readiness의 stale 키 이름은 `internal/adapter/issueops/issueops_pr_readiness_strict.go:50-70`에서 실제 문자열을 읽어 `Classify`에 그대로 쓴다.

`missing`의 owner command 대응표는 현행 `skills/issueops/SKILL.md` "Gate map"과 `.agent-harness/AGENT_WORKFLOW.md`의 자동 루프 문단에서 그대로 옮긴다. 대응표는 domain 패키지의 `ownerCommand(key string) string` 함수가 소유한다.

출력 계약(`internal/contract/issueopsnext/types.go`):

```go
type Result struct {
    OK          bool     `json:"ok"`
    GeneratedAt string   `json:"generated_at"`
    CWD         string   `json:"cwd"`
    CWDRole     string   `json:"cwd_role"`     // source | worktree | other
    SourceRoot  string   `json:"source_root"`
    Selected    *Entry   `json:"selected,omitempty"`
    Candidates  []Entry  `json:"candidates,omitempty"`
    Stage       Stage    `json:"stage"`
    Lease       Lease    `json:"lease"`
    Missing         []string `json:"missing"`
    NextCommand     string   `json:"next_command,omitempty"`
    NextCommandKind string   `json:"next_command_kind,omitempty"` // exact | template
    Exits           Exits    `json:"exits"`
    Review      Review   `json:"review"`   // 코드가 소유한 host별 planner(리뷰어) 기본값. internal/port/orca.go
    Warnings    []string `json:"warnings,omitempty"`
}
type Review struct {
    Model  string `json:"model,omitempty"`
    Effort string `json:"effort,omitempty"`
}
type Entry struct {
    ID, Phase, Branch, WorkspaceRoot, IssueURL, RemoteArtifactURL string
}
type Stage struct {
    Key   string `json:"key"`
    Index int    `json:"index"`
}
type Lease struct {
    Status       string `json:"status,omitempty"`
    Generation   uint64 `json:"generation,omitempty"`
    HolderIsSelf bool   `json:"holder_is_self"`
    HolderLive   *bool  `json:"holder_live,omitempty"`
}
type Exits struct {
    PauseCommand    string `json:"pause_command,omitempty"`    // holder self일 때 execution release
    AbandonCommand  string `json:"abandon_command"`            // 항상 cleanup abandon --id <id> --reason <TEXT> --preview
    TakeoverCommand string `json:"takeover_command,omitempty"` // holder stale일 때 replace --preview
}
```

텍스트 출력은 `stage <index>/10 <key>  cycle <id>  phase <phase>  lease <status>(gen <g>, self|other|none)`, `missing: ...`, `next: <next_command>`, `exits: pause=<...> abandon=<...>` 네 줄이다. 한국어 label과 선택지는 라우터가 붙인다.

#### 4. 탈출 경로 (`issueops-abandon`)

| 경로 | 조건 | 명령 순서 |
|---|---|---|
| 일시 중단(pause) | 이 세션이 홀더 | WIP를 `atomic-commit-push`로 커밋·푸시하거나 사용자가 명시적으로 폐기 → `execution release --id --generation $ACTOR_FLAGS`(cwd는 워크트리). record와 워크트리는 남는다 |
| 재개(resume) | lease released·claimable | 스킬이 아니라 라우터 `## 단계 표`의 `resume` 행이 소유한다. 새 세션이 `issueops next`를 실행하면 `next_command`가 회복 체인의 첫 명령(`replace --preview`, direct claimable이면 `claim`, orca claimable이면 `resume`)이고, 각 결과가 돌려주는 `next_command`만 실행한다. 호스트가 달라도 같다(`--host codex|claude|omo`) |
| 인수(takeover) | 홀더 프로세스가 죽음 | 라우터 `## 단계 표`의 `takeover` 행이 소유한다. `replace --preview` → `--revoke` → `--finalize-preview` → `--finalize` → `claim`. 모두 돌려준 `next_command`만 실행하며, 사용자의 "그 세션은 껐다"는 quiescence 증거가 아니다 |
| 폐기(abandon) | 어느 단계든 | 홀더면 먼저 release → 사용자에게 원격 효과를 번호로 묻는다(draft PR/MR 닫기, 이슈 닫기 또는 열어 두기, 원격 브랜치 삭제 또는 유지) → 고른 것만 플래그로 넣어 `cleanup abandon --id ID --reason TEXT [--close-pr] [--close-issue] [--delete-remote-branch] --preview` → preview가 나열한 정확한 원격·로컬 효과를 사용자에게 확인 → 돌려준 `--apply --confirm --fingerprint`. record 삭제 뒤에는 `--id` 기반 명령을 쓸 수 없고, `remote close-issue`·`cleanup remote-branch`는 머지 증적을 요구하므로 미머지 사이클의 원격 정리는 abandon만이 소유한다 |

abandon은 워크트리가 깨끗해야 통과하므로 dirty 워크트리에서는 세 선택지(WIP 커밋·푸시 후 폐기, 변경 폐기 후 폐기, 일시 중단으로 전환)를 번호로 제시한다.

#### 5. `cleanup abandon`의 원격 효과 확장

usage: `agent-harness issueops cleanup abandon --id ID --reason TEXT [--close-pr] [--close-issue] [--delete-remote-branch] (--preview | --apply --confirm --fingerprint SHA256) [--json]`

세 플래그는 모두 opt-in이며 기본값은 지금과 같은 로컬 전용 abandon이다. preview는 요청된 원격 효과의 현재 상태를 provider readback으로 관측해 결과에 싣고(`remote_artifact.state open|closed|merged`, `issue.state opened|closed`, `remote_branch.present`와 OID), 그 관측값을 fingerprint에 묶는다. apply는 고정 순서로 진행한다: `close_pr`(provider `ClosePullRequest`; 이미 closed면 건너뛰고, merged면 게이트 ④가 이미 막았으므로 도달하지 않는다) → `close_issue`(provider `CloseIssue`에 `Reason: "not_planned"`; 이미 closed면 건너뛴다) → `remote_branch_delete`(`cleanup remote-branch`의 `git push origin --delete <ref>` 단계를 공유 helper로 뽑아 재사용하며 preview OID와 현재 OID가 다르면 중단) → 현행 로컬 단계(`workspace_processes_stop` → worktree 제거 → 로컬 브랜치 삭제 → 감사 라인 → record 삭제). 각 원격 단계는 readback으로 확인하고, 실패하면 record를 보존한 채 `failed_step`을 기록해 재실행 전 preview로 새 fingerprint를 받게 한다. record schema에는 필드를 추가하지 않는다. provider port에는 `ClosePullRequest`를 추가하고 `IssueProviderCloseIssueRequest`에 `Reason string`을 추가한다(GitHub `gh issue close --reason completed|"not planned"`, GitLab은 무시). 별도의 `remote close-pr` 명령은 만들지 않는다.

#### 6. 삭제 목록

| 대상 | 처리 |
|---|---|
| `skills/issueops-branch-worktree/` 전체 | 삭제. `skills/issueops-prepare/`가 대체 |
| `skills/issueops/references/issue-preflight.md` | 삭제. 내용은 `issueops-create-issue`로 이동 |
| `skills/issueops/references/worktree-context.md` | 삭제. 브랜치·워크트리 절은 `issueops-prepare`, edit-target guard와 dependencies 절은 `issueops-plan`·`issueops-implement`로 이동 |
| `skills/issueops/references/operational-start.md` | 삭제. 명령 순서는 각 단계 스킬이 소유 |
| `skills/issueops/SKILL.md`의 "시작 순서", "Child와 delegation", "Reference map"의 삭제 파일 행 | 삭제·교체. delegation은 `issueops-implement`와 `orchestration.md`가 소유 |
| `skills/issueops/references/cleanup-state.md`의 "Retiring a cycle that never entered execution v1" 절 | 삭제. `issueops-abandon`으로 이동 |
| `.agent-harness/AGENT_WORKFLOW.md`의 "이원 구조 흐름 요약" 절 | "10단계 운영 흐름"으로 교체 |
| `.agent-harness/operations/guides/issueops-execution.md:169-178` "IssueOps 이원 구조 운영" 절 | "10단계 운영과 세션 경계"로 교체. "Orca owner sequence" 절은 유지 |
| `skills/issueops-cleanup/SKILL.md`의 "Do not use `cleanup abandon`" 문단 | `issueops-abandon` 안내로 교체 |
| `skills/issueops/references/ai-slop-clean.md` | 삭제. 프롬프트는 `issueops-clean`이 흡수하고 `skills/turing/SKILL.md:396`의 링크를 `skills/issueops-clean/SKILL.md`로 바꾼다 |
| `skills/issueops/references/remote-issue.md`의 "Korean Remote Artifact Gate"·"Remote Artifact Writing Quality" 절과 `skills/issueops/scripts/remote_artifact_gate.py` | `issueops-remote-write`로 이동. 레퍼런스에는 provider 관계·hierarchy 절만 남는다 |
| `skills/issueops/SKILL.md`의 "Remote write 공통 게이트" 절 | `issueops-remote-write`로 이동. 라우터에는 한 줄 링크만 남는다 |
| `skills/issueops-implement/SKILL.md`의 "Publication evidence gates"·"Implementation review gate" 절 | `issueops-verify`와 `issueops-review`로 이동 |
| 단계 스킬마다 반복되던 "단계 판별", "actor 플래그", "lease fencing", "편집 대상 확인" 문단 | 라우터 `## 공통 불변식`에 한 번만 두고 단계 스킬은 링크한다 |

#### 7. 반복 작업의 스킬 분해

단계 스킬 아홉 개에 같은 절차가 복사되던 지점을 세어 소유자를 하나로 정한다. 단계 스킬은 소유자를 링크하고 절차를 복사하지 않는다. 링크 대상이 스킬이면 Skill 도구로 호출한다.

| 반복 작업 | 지금 반복되는 곳 | 소유자 | 쓰는 단계 |
|---|---|---|---|
| 단계 판별과 선택지 제시 | 모든 단계 스킬의 첫 절 | `issueops` 라우터 `## 먼저 실행` | 전부 |
| native actor 플래그 확보 | implement, create-pr, complete, abandon | 라우터 `## 공통 불변식` (b): `execution whoami --json` | durable mutation이 있는 단계 전부 |
| lease fencing 규칙과 회복 체인 | implement, create-pr, complete, execution.md | 불변식은 라우터 `## 공통 불변식` (c)·(e). 회복 명령은 `next`와 `execution status`가 렌더하는 `next_command` 체인이며 라우터 `## 단계 표`의 `resume`·`takeover` 행이 안내한다. 별도 스킬은 없다 | 3~7 |
| 편집 대상 확인(pwd·branch·HEAD·expected worktree) | plan, implement, clean, verify | 라우터 `## 공통 불변식` (d) | 3~6 |
| 적대 리뷰 실행과 판정 기록(brooks fresh 서브에이전트, planner급 모델, finding 1건 이상, revise·stop 루프, digest 봉인) | plan의 devils-advocate, verify의 implementation review, 현행 implement의 리뷰 게이트 절 | 새 공용 스킬 `issueops-review`(T21) | 3, 5 |
| 게이트 원장(`.agent-harness/issues/<n>/gates.md`) 작성·검사·증거 기록 | plan(수용 기준), implement(RED/GREEN), clean(재검사), verify(최종·`gates_incomplete` 해소), turing | 새 공용 스킬 `gates-ledger`(T22). `agent-harness gates init/check/status/report/abandon` CLI를 감싼다 | 3, 4, 5 |
| 원격 본문 쓰기 프로토콜(preview → fluent-korean → 한국어 게이트 스크립트 → 동일 요청 confirm → readback → 모호하면 reconcile, secret redaction, label·assignee 규칙) | create-issue, create-pr, sync-issue, sync-pr, cleanup의 reflect-*·close-issue, review의 reflect-devils-advocate | 새 공용 스킬 `issueops-remote-write`(T23). 현행 라우터 "Remote write 공통 게이트" 절과 `remote-issue.md`의 두 절을 흡수한다. abandon의 원격 효과는 본문이 없으므로 이 스킬을 쓰지 않고 `cleanup abandon`의 preview·fingerprint·apply가 같은 안전성을 제공한다 | 1, 7, 8, 동기화 |
| 원격 한국어 다듬기 | 위와 같음 | 기존 `fluent-korean`(변경 없음) | 위와 같음 |
| 프로젝트 문서 확인·갱신(route → read(SHA) → append/revise) | 3단계 계획 전 확인, 5단계 반영, 현행 implement의 project-doc 절, AGENT_WORKFLOW의 MCP upkeep 절 | 기존 `project-docs-update` 스킬(변경 없음). `issueops-plan`은 확인 절차만, `issueops-docs`는 갱신과 issueops 게이트 연결만 소유한다 | 3, 5 |
| 커밋·푸시 | 7단계, abandon의 WIP 커밋, complete의 report 커밋 | 기존 `atomic-commit-push`(변경 없음) | 7, 8, 탈출 |

`issueops-review`의 인터페이스: 입력은 `--target plan|diff`, 대상 파일(plan 경로) 또는 `git -C <worktree> diff <base_sha>`, 리뷰어 모델(host별 planner 기본값: claude `claude-opus-5`/high, codex `gpt-5.6-sol`/xhigh, omo는 prepare가 기록한 값). 출력은 brooks 판정 구조(verdict, 가장 위험한 결함, gate별 finding, 더 작은 계획)이며 기록 명령은 target별로 `devils-advocate review --reviewer-context subagent ...` 또는 `implementation-review record --reviewer-host --reviewer-model ...`이다. 루프 규칙은 ADR 2026-08-28 Decision 4·5를 그대로 따른다.

`gates-ledger`의 인터페이스: 원장 경로는 issue 번호가 있으면 `.agent-harness/issues/<n>/gates.md`, 없으면 `.agent-harness/gates/<scope>.md`이며 CLI의 `--file`로 명시한다. `gates init --file <path> --scope <n> --gate "G1: <결과> | CHECK: <명령> | EXPECT: <기대 문자열>"`로 만들고, `gates check --file <path> --cwd <worktree> --workspace-root <worktree> --write --json`으로 EVIDENCE를 채우며, 실행할 수 없는 게이트는 `gates abandon --gate ID --reason`으로 정직하게 닫는다. CHECK는 command policy를 지나므로 raw shell 확장을 넣지 않는다.

`issueops-remote-write`의 인터페이스: 입력은 실행할 정확한 `agent-harness issueops remote <verb> ...` 명령과 본문 파일이다. 절차는 (1) 본문에 `fluent-korean` 호출, (2) `python3 "$SKILL_DIR/scripts/remote_artifact_gate.py" --kind issue|pr --title --body-file`(스크립트를 이 스킬로 옮긴다), (3) `remote render-template` 또는 명령 자체의 preview, (4) preview와 동일한 요청에 `--confirm`만 추가, (5) provider readback(`remote verify-artifact` 또는 해당 명령의 readback 필드), (6) 결과가 불명확하면 재호출하지 않고 `remote reconcile-issue` 또는 `execution reconcile`, (7) secret 패턴이 본문에 있으면 쓰지 않음, (8) label은 score threshold 이상만, concrete assignee 필수. 이 여덟 항목은 현행 라우터 "Remote write 공통 게이트"와 `remote-issue.md` "Korean Remote Artifact Gate"·"Remote Artifact Writing Quality"에서 옮긴다.

#### 8. 코드베이스 존중 지침 (사용자 요청, 2026-09-04)

계획과 구현은 현재 코드베이스를 존중하고, 이미 구현된 것을 재사용할 수 있으면 재사용하며, 성능·하위 호환성·side effect를 명시적으로 고려한다. 원칙의 출처는 `AGENTS.md` §2 Simplicity First와 §3 Surgical Changes이고, 이 계획은 그 원칙을 단계별 산출물에 강제한다.

| 단계 | 산출물에 들어가는 것 | 소유 |
|---|---|---|
| 1 이슈 확정 | plan-prep의 codebase survey evidence에 "재사용 후보 심볼·패키지·테스트 헬퍼"를 반드시 적는다. 이슈 본문 `## 구현 범위`에 "기존 구현을 확장한다/새로 만든다"를 구분해 쓴다 | `issueops-create-issue` |
| 3 문서 확인 | 계획을 쓰기 전에 `project_docs_route`(없으면 `agent-harness docs --json`)로 이 변경에 닿는 운영 문서를 고르고 `project_docs_read`로 읽어, 적용되는 ADR 결정·CAUTIONS 항목·CONVENTIONS·ARCHITECTURE·TESTING 규칙을 plan의 `## 적용되는 결정과 주의사항` 절에 "문서 경로, 항목, 이 계획에 미치는 제약" 형식으로 적는다. 1단계 plan-prep의 ADR 조회가 이슈 범위용이었다면 이 절은 설계 제약용이다 | `issueops-plan` |
| 3 계획 | plan 템플릿의 필수 절 네 개: `## 적용되는 결정과 주의사항`, `## 재사용하는 기존 구현`, `## 성능 영향`, `## 하위 호환성과 side effect`. design review의 `--alternative`·`--risk`와 compatibility review의 `--backward-compatibility`·`--side-effect`에 이 절의 결론을 옮긴다 | `issueops-plan` |
| 3 계획 검토 | brooks 프롬프트의 네 렌즈: 기존 구현을 재사용할 수 있었는데 새로 만들지 않았는가, 성능 회귀 가능성, 계약 표면의 하위 호환성, side effect | `issueops-review` |
| 3 구현 | 구현 루프 규칙 네 줄: 기존 함수·패키지·헬퍼 확장을 새 파일·새 추상화보다 우선한다(plan의 재사용 절에 없는 새 추상화 금지), 계약 표면(CLI JSON, MCP schema, golden, record schema, provider body) 변경은 이슈와 plan이 명시한 것만, hot path 변경은 전후 측정 evidence, 파일·원격·상태 side effect 목록을 turing report에 기록 | `issueops-implement` |
| 5 문서 반영 | 구현이 만든 결정은 ADR에, 재발 함정은 CAUTIONS에, 새 명령·컨벤션·구조는 해당 문서에 반영한다. 반영할 것이 없으면 무엇을 대조했는지 evidence에 적어 `no-change`로 기록한다 | `issueops-docs` |
| 6 검증 | 구현 diff가 plan의 호환성 검토와 다르면 `compatibility review`를 다시 기록한다. 구현 리뷰 프롬프트에 위 네 렌즈를 넣는다 | `issueops-verify`, `issueops-review` |
| 공통 | 라우터 `## 공통 불변식` (f) 한 문장 | `issueops` |

#### 10. 프로젝트 문서 확인과 반영 (사용자 요청, 2026-09-04)

두 지점이 있다. 둘 다 이 저장소의 project_docs MCP 계약(`.agent-harness/AGENT_WORKFLOW.md` "MCP Usage Rule", "`.agent-harness` Upkeep via MCP")을 그대로 쓰고, 절차는 기존 `project-docs-update` 스킬이 소유한다.

- **계획 전 확인(3단계, `issueops-plan`)**: `project_docs_route`에 이슈 제목과 구현 범위를 넣어 문서를 고르고(`route`가 없으면 `agent-harness docs --json`의 required-doc 목록), `project_docs_read`로 읽는다. 대상은 최소 `CONSTITUTION.md`, `ARCHITECTURE.md`(해당 모듈), `CONVENTIONS.md`, `CAUTIONS.md`(색인과 해당 모듈), `ADR.md`(관련 결정), `TESTING.md`다. 결과를 plan의 `## 적용되는 결정과 주의사항` 절에 "문서 경로, 항목 제목, 이 계획에 미치는 제약 한 문장"으로 적는다. 적용되는 항목이 없으면 "대조했으나 없음"과 대조한 문서 목록을 적는다. design review의 `--risk`에 이 절의 제약을 옮긴다. `issueops-review`의 plan 리뷰 프롬프트에는 이 절을 넣어 brooks가 "무시된 결정"을 공격하게 한다.
- **구현 후 반영(5단계, `issueops-docs`)**: 구현 diff 요약으로 `project_docs_route`를 다시 실행하고, 양방향으로 대조한다. 문서→구현: CONSTITUTION·CONVENTIONS·ARCHITECTURE·CAUTIONS를 이번 diff가 어겼으면 문서가 아니라 구현을 고친다(4단계로 돌아간다). 구현→문서: 새 결정은 `project_docs_append(kind=adr)`, 재발 함정은 `project_docs_append(kind=caution)`, 새 명령·컨벤션·구조는 `project_docs_revise`(SHA-CAS)로 고친다. 문서를 고쳤으면 `ai-slop-clean record`로 재봉인한 뒤 `project-docs-review record --verdict updated --doc <경로> --evidence <대조 내용>`, 고칠 것이 없으면 `--verdict no-change --evidence <대조한 문서와 판단>`을 기록한다. `updated`는 `--doc`이 변경 집합에 있어야 통과하고 `no-change`는 `--doc`을 받지 않는다(현행 게이트 그대로). `.agent-harness/*.md`를 고치면 response-contract 골든이 드리프트하므로 하네스 저장소에서는 `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update`를 같은 단계에서 실행한다(CAUTIONS Update workflow 5).

#### 9. 라우터의 단계 표 (Go 밖에 두는 매핑)

`skills/issueops/SKILL.md`의 `## 단계 표`가 `stage.key`를 스킬, 한국어 label, 선택지로 바꾼다. Go는 이 표를 모른다.

| stage.key | 스킬 | label |
|---|---|---|
| `none`, `issue` | `issueops-create-issue` | 이슈 확정·생성 |
| `prepare` | `issueops-prepare` | 브랜치 준비 |
| `plan.write`, `plan.design`, `plan.review`, `plan.handoff` | `issueops-plan` | 문서 확인·계획·검토·인계 |
| `claim` | 스킬 없음. Orca가 띄운 세션은 자기 프롬프트의 봉인된 claim을 정확히 한 번 실행하고, 그 밖의 세션은 `next_command`(status)가 돌려주는 체인을 lease가 active(self)가 될 때까지 따라간 뒤 `next`를 다시 실행한다 | 현재 index의 label |
| `implement.enter`, `implement` | `issueops-implement` | 구현 |
| `clean` | `issueops-clean` | AI slop 정리 |
| `docs` | `issueops-docs` | 프로젝트 문서 반영 |
| `verify` | `issueops-verify` | 검증 |
| `commit-push` | `atomic-commit-push` | 커밋·푸시 |
| `pr.create` | `issueops-create-pr` | PR/MR 발행·완료 |
| `pr.complete` | `issueops-complete` | PR/MR 발행·완료 |
| `done` | `issueops-cleanup` | 머지 후 정리 |
| `takeover` | 스킬 없음. `next_command`를 실행하고 결과가 돌려주는 `next_command`를 따라간다. 죽은 홀더 인수는 `issueops-abandon`이 설명한다 | 현재 index의 label |
| `blocked.pending`, `blocked.holder_live` | 없음. `next_command`로 상태를 다시 읽는다 | 현재 index의 label |
| `blocked.root_conflict` | 충돌 사이클을 `issueops-cleanup`(머지됨) 또는 `issueops-abandon`(미머지)으로 먼저 정리한다 | 현재 index의 label |
| `unknown`, `invalid` | 없음. `next_command`(`status --id`)로 record를 읽고 `warnings`의 missing 키를 사용자에게 보여 준다 | 현재 index의 label |
| `ambiguous` | 사용자에게 `candidates` 중 ID 선택 또는 새 사이클 시작을 요청 | 없음 |

선택지는 항상 네 줄이다: `1. 이어서 진행: <스킬> (<label>) (추천)`, `2. 다른 단계 지정: 원하는 단계 번호를 말해 주세요`, `3. 중단: issueops-abandon (일시 중단·폐기)`, `4. 새 사이클 시작: issueops-create-issue (이 저장소에 다른 사이클이 있어도 새로 시작한다)`. `blocked.*`일 때 1번은 `1. 대기: <next_command>로 상태를 다시 읽습니다`로 바꾼다. `none`일 때는 1번과 4번이 같으므로 4번을 생략한다.

### Gap Analysis

- **누락 4: direct 모드에는 implementation review 게이트가 없다.** `implementationReviewMissing`은 `Execution.Mode != orca`면 빈 문자열을 돌려주므로(`internal/adapter/issueops/issueops_implementation_review.go:73-79`) direct 사이클은 리뷰 없이도 통과한다. 이 판정을 쓰는 곳은 셋이다: 비strict `IssueOpsPRReadiness`(`issueops_pr_readiness.go:27`), 그것을 포함하는 strict readiness와 `phase --to pr` 진입(`issueops_phase.go:118-122`), 그리고 create-pr의 자체 게이트(`execution_remote.go:99-111`, orca 한정 freshness 거부 포함). 새 흐름은 direct가 기본이라 5단계 검증이 CLI 수준에서 비게 된다. 해소: T24가 세 곳 모두 `Execution != nil`인 모든 모드로 넓힌다. 영향: 현재 implement·ai-slop-clean·feedback phase에 있는 direct 사이클은 pr 진입과 create-pr 전에 리뷰 기록이 필요해진다. 이미 pr·done인 사이클은 영향이 없다.
- **누락 8 (brooks 판정): 4단계 봉인과 그 뒤 단계 사이의 fingerprint 순서.** `AISlopCleanFingerprint`는 `phase --to ai-slop-clean` 진입 시 봉인되고(`issueops_phase.go:146-153`) `ai-slop-clean record`가 갱신하며(`issueops_ledger_recorders.go:108` → `issueops_phase_refresh.go:12-20`), 그 뒤 파일이 바뀌면 strict와 local readiness가 `ai_slop_clean_stale`로 pr 진입과 create-pr를 막는다(`issueops_pr_readiness_strict.go:76-78`). fingerprint는 `git diff <base>..HEAD --name-only`와 `git status --porcelain`(untracked 포함)의 모든 경로 내용 해시라(`implementation/evidence.go:52-74`) gates.md EVIDENCE, turing report, 운영 문서 갱신도 전부 포함된다. 첫 초안의 검증 단계는 봉인 뒤에 파일을 써서 매 사이클 stale을 만들었다. 해소: 코드와 증거 파일을 바꾸는 작업(정리, `gates check --write`, report 확정, 재검증)은 4단계에 두고 그 끝에서 `ai-slop-clean record`로 봉인한다. 운영 문서 수정은 5단계(`issueops-docs`)가 하며, 문서를 고쳤으면 같은 단계가 `ai-slop-clean record`를 다시 실행해 재봉인한 뒤 `project-docs-review record`를 기록한다. 6단계는 기록 명령(schema-evidence, implementation-review, compatibility 재기록)과 읽기 전용 검증만 실행하며 파일을 만지지 않는다. 봉인 뒤 예상 밖으로 파일이 바뀌면 `next`가 `clean`으로 되돌리는데 이것이 의도된 회귀다. T19·T20·T26이 이 순서를 소유하고 T0b 스파이크가 실측한다.
- **누락 9 (brooks 판정): relaunch 명령이 codex·omo를 source root로 보낸다.** `workspaceRelaunchCommand`는 codex에 `--cd <sourceRoot>`, omo에 `cd <sourceRoot> && omo`를 준다(`internal/adapter/gitworktree/provisioner.go:180-190`). 되띄운 세션이 source root에서 `execution prepare`를 치면 `workspace.go:52`가 source root cwd를 허용해 잘못된 cwd의 홀더가 되고, 뒤의 `execution release`는 `release cwd is not canonical`로 거부된다. 해소: T25가 relaunch 명령을 세 호스트 모두 워크트리 root로 `cd`하도록 고친다.
- **누락 10 (brooks 판정): 워크트리 생성자가 둘이다.** 2단계는 `git worktree add`, 3단계는 `execution prepare`의 채택이다. 채택은 `inspectExisting`(`provisioner.go:102-117`)에 기대는데 "미리 만든 워크트리를 채택한다"를 고정하는 테스트가 없고, `architecture/issueops.md:42`는 "execution prepare가 fixed sibling worktree를 만든다"고만 적는다. 해소: T25가 provisioner 채택 회귀 테스트를 추가하고 T14가 그 문장을 "만들거나 base SHA에 미리 만든 워크트리를 채택한다"로 고친다. 생성자를 하나로 합치는 `execution prepare --provision-only`는 lease 상태 기계와 MCP 스키마를 건드리므로 기각한다.
- **누락 11 (brooks 판정): 이미 사이클이 있는 저장소에서 새 사이클을 시작할 길이 없었다.** `next`는 non-done 사이클이 하나면 자동 선택하고 둘 이상이면 `ambiguous`인데, 1단계 스킬은 `none`·`issue`만 통과시켰다. 해소: 라우터 선택지에 "새 사이클 시작"을 항상 두고(설계 요약 9), 1단계 스킬은 사용자가 그 선택지를 고르면 선택된 사이클과 무관하게 `issueops start`를 실행한다.
- **누락 12 (brooks 판정): 분류표에 catch-all이 없고, execution이 비워진 implement 사이클을 2단계로 오분류했다.** `switch-mode`는 phase를 두고 Execution·WorktreePath·PlanPath를 비운다(`execution_mode_switch.go:104-106`). 첫 초안의 규칙 6은 워크트리 부재만 보고 `prepare`를 권했고, 그 뒤 `phase --to plan`은 후진이라 거부된다(`issueops_phase.go:78-80`). 해소: 규칙 8이 branch·base_sha 유무로 2단계를 가르고, 규칙 9~12가 staged plan·design·devil's-advocate 유무로 3단계 안을 가르며, 규칙 22 `unknown`이 나머지를 `status`로 보낸다. 규칙 7은 알파벳 정렬된 missing의 첫 항목 대신 1단계 고정 순서를 쓰고, 규칙 11은 waive되지 않은 `stop`을 `regress`로 보낸다.
- **누락 13 (brooks 판정): canonical root 충돌.** `EnsureRootUnclaimed`는 phase·lease와 무관하게 모든 record를 검사하므로(`execution_prepare.go:134-140`) cleanup 전 done 사이클이나 같은 leaf 이름의 사이클이 root를 쥐고 있으면 prepare가 `CodeRootConflict`로 실패한다. 해소: 규칙 2b `blocked.root_conflict`가 `next`의 inventory에서 같은 `workspace_root`를 가진 다른 entry를 찾아 먼저 정리하도록 안내한다.
- **누락 14 (brooks 판정): delegated child의 base branch.** child 사이클의 base는 부모 브랜치이고 부모 워크트리를 `--parent-worktree`로 봉인한다(`execution_prepare.go:107-125`). 2단계 스킬의 `origin/$BASE_BRANCH` fetch는 독립 사이클 전용이다. 해소: T4에 child 분기(base branch는 부모 브랜치, base SHA는 부모 워크트리 HEAD, `--parent-worktree` 필수)를 넣고, T1 규칙 9는 readiness의 missing 키를 쓰므로 child의 devils-advocate 면제가 자동으로 반영된다.
- **누락 15 (brooks 판정): 계획이 언급하지 않은 파손과 잔여.** `internal/adapter/issueops/issueops_skill_contract_test.go`는 삭제될 `operational-start.md`를 읽고(`:29-30,:70,:87`) 라우터에 `--mode auto` 문구(`:16`)와 GitHub Orca ordering 문자열(`:73`)을 요구한다. `.agent-harness/TECH_STACK.md:72`가 `issueops-branch-worktree`를 언급한다. `docs/superpowers/**`와 `.agent-harness/issueops/**`(turing report)에는 과거 이름이 남는다. `execution_remote.go:99` 주석에 "이원 구조"가 있다. `~/.claude/skills`의 stale 링크 정리는 Go `install`이 소유하고 `install-native.sh`에는 링크 코드가 없다. 해소: T11이 테스트 파일을 고치고, T12가 TECH_STACK을 고치고, T24가 주석을 고치고, DoD·T16의 rg는 두 디렉터리를 제외하며, T16이 stale 링크를 `agent-harness update` 뒤 실측해 남으면 사용자 승인 후 링크 하나를 지운다.
- **누락 16 (brooks 판정): 예산과 wave가 6단계판 그대로였다.** `cautions/issueops-lifecycle.md`는 이미 221행이라 항목 여섯을 더하면 250행 예산을 넘는다. 해소: 새 모듈 `cautions/issueops-stages.md`를 만들고 색인에 한 줄을 더한다. T11 라우터 예산은 260행, T9는 220행으로 올린다. T6과 T7은 같은 카탈로그와 골든을 고치므로 직렬화하고, wave는 4개 이하로 나눈다. T17은 스파이크 T0b 뒤 네 시나리오로 줄인다.
- **brooks 지적 중 채택하지 않은 것.** `Exits`·`cwd_role`·`Warnings`와 네 줄 텍스트 출력은 프레젠테이션이 아니라 데이터와 이 저장소 CLI의 관례(모든 명령이 텍스트·JSON 둘 다 낸다)라 유지한다. `next`를 새 명령 대신 `status` projection으로 두는 안은 cwd로 사이클을 고르는 일이 `status --id`에 없어 기각한다. 세 호스트 실행 명령 템플릿은 사용자 목적 1번의 요구다. T18은 저장소 밖 사용자 파일이라 승인 게이트로 남기되 선택 태스크로 표시한다.
- **모순 1: orca 모드의 planner 게이트와 3단계의 "새 세션이 계획".** orca 모드는 prepare 전에 계획과 세 리뷰를 요구한다. 해소: orca 경로를 대안으로 격리하고 기본 경로는 채택 방식으로 둔다. `MissingPlannerGates`는 손대지 않는다.
- **위험 1(해소됨): 모드 선택.** 초안은 3단계를 explicit direct로 못박았다가, 사용자 결정(2026-09-05)으로 `--mode auto`가 고르도록 바꿨다. 그래서 계획과 세 리뷰가 prepare보다 먼저 오고, Orca가 준비돼 있으면 Orca가 구현 세션을 띄운다. 대신 2단계가 워크트리를 만들면 Orca의 `worktree create`와 브랜치 이름이 충돌하므로 2단계는 브랜치 identity만 기록한다. T15의 cautions 모듈 §1에 이 충돌을 기록한다.
- **위험 2: 워크트리 세션의 형제 디렉터리 쓰기 권한.** 해소: 2단계 출력의 실행 명령에 `--add-dir "${SOURCE_ROOT}.worktrees"`를 넣고, 거부되면 `next_command`의 `relaunch_command`를 그대로 안내한다. E2E 프로브 시나리오에 포함한다.
- **위험 3: 채택 실패.** 사용자가 2단계와 3단계 사이에 워크트리에서 커밋하면 HEAD가 base SHA와 달라 `existing canonical worktree identity does not match branch and base_head`로 실패한다. 해소: `issueops-prepare`가 "3단계 세션의 첫 명령 전까지 커밋하지 않는다"를 명시하고, `next`가 `prepare` 단계에서 워크트리 HEAD가 base SHA와 다르면 warning으로 알린다.
- **위험 4: dirty 워크트리 abandon.** abandon은 깨끗한 워크트리를 요구한다. 해소: 설계 요약 4의 세 선택지.
- **위험 5: 발행된 draft PR/MR이 abandon 뒤 열린 채 남음.** 해소: `cleanup abandon --close-pr`.
- **누락 5 (brooks 판정, 2026-09-04): 폐기 뒤 원격 정리가 불가능하다.** 첫 초안은 `cleanup abandon --apply` 뒤에 `remote close-issue`, `cleanup remote-branch`를 두었는데, apply가 record를 삭제하므로 `--id` 명령이 동작하지 않고, 두 명령은 머지 증적을 하드 게이트로 요구한다(실측: io-15f1518189ca). 해소: 설계 요약 5처럼 원격 효과를 `cleanup abandon`의 opt-in 플래그로 흡수하고 record 삭제보다 먼저 실행한다. 별도 `remote close-pr` 명령과 `RemoteArtifact.ClosedAt` 필드는 만들지 않는다(Global Constraint "record에 새 필드를 추가하지 않는다"와 충돌했다).
- **누락 6 (brooks 판정): `next`가 read-only가 아니었다.** strict PR readiness는 `git fetch --quiet`를 실행하고(`issueops_pr_readiness_strict.go:40`), cleanup status의 `merged`는 provider readback 입력이다. 해소: `next`는 fetch 없는 `IssueOpsLocalPRReadiness`(T6가 strict에서 분리)만 쓰고, done phase는 머지 여부를 판단하지 않는 단일 `done` 단계로 둔다. strict readiness와 머지 확인은 커밋·푸시와 정리 단계 스킬이 명시적으로 실행한다.
- **누락 7 (brooks 판정): 같은 매핑의 소유자가 둘이었다.** `OwnerCommand`(Go)와 라우터 "Gate map" 표가 같은 missing→명령 대응이었다. 해소: Go가 유일한 소유자이고 라우터 표는 삭제한다. 반대로 `stage.key`→스킬·label·선택지 표는 라우터만 소유하고 Go에 스킬 이름과 한국어 문자열을 두지 않는다(설계 요약 9).
- **누락 1: 다른 호스트에서 재개.** whoami와 ACTOR_FLAGS가 host를 받으므로 CLI는 이미 host-neutral이다. 스킬 본문이 `claude` 예시만 쓰지 않도록 실행 명령을 host별 세 줄로 적는다. Omo 실행 명령은 provisioner에 relaunch 형식이 없으므로 E2E에서 확인하고 문서에 관측값만 적는다.
- **누락 2: source root가 아닌 곳에서 `start`.** 과거 레코드 io-6e9cbae95186의 repo가 워크트리 경로다. 현행 `normalizeIssueOpsRepo`가 정규화하므로 재발하지 않지만, `next`가 `record.repo != source_root`면 warning을 낸다.
- **누락 3: 팀이 issue 화면에서 현재 단계를 보는 표면.** 사용자 목적 2번에 직접 닿지만 관리 블록 body-sync 설계가 필요하다. 이 계획은 issue를 계약 SSOT로 유지하는 현행 표면(생성 본문, `sync-issue`, `reflect-devils-advocate`, `reflect-completion`, `feedback mark-issue-updated`, provider linked branch·PR)까지만 다루고, phase 전이마다 `issueops:stage` 관리 블록을 갱신하는 `remote reflect-stage`는 후속 이슈로 남긴다. T13 ADR의 Consequences에 이 후속을 명시한다.
- **범위 절제.** prepare에 `handoff` 모드를 추가하는 안, orca owner가 계획하게 하는 안, hook이 단계를 알려 주는 안은 모두 기각한다. 이유는 각각 9개 이상 파일의 계약 변경, sealed identity 불변식 훼손, ADR 2026-08-27 위반이다.

## Work Objectives

### Core Objective

사용자가 어느 세션·어느 호스트·어느 단계에서 `issueops`를 실행해도 한 번의 read-only 명령으로 현재 단계와 다음 행동을 받고, 단계 스킬 중 하나로 이어가거나 `issueops-abandon`으로 안전하게 빠져나올 수 있다.

### Deliverables

- Go: `issueopsnext` vertical(contract, domain, application, inbound, harnessapp wiring, CLI `issueops next`, catalog, goldens, architecture ratchet)과 fetch 없는 `IssueOpsLocalPRReadiness` 분리, `cleanup abandon` 원격 효과(port `ClosePullRequest`·`CloseIssue.Reason`, GitHub·GitLab adapter, abandon inventory·gates·apply 단계, CLI 플래그, catalog, goldens), implementation review 게이트의 전 모드 확장, provisioner relaunch cwd 수정과 워크트리 채택 회귀 테스트.
- 스킬: `issueops`(재작성), `issueops-create-issue`(재작성), `issueops-prepare`(신설), `issueops-plan`(신설), `issueops-implement`(재작성), `issueops-clean`(신설), `issueops-docs`(신설), `issueops-verify`(신설), `issueops-abandon`(신설), 공용 `issueops-review`·`gates-ledger`·`issueops-remote-write`(신설), `issueops-create-pr`·`issueops-complete`·`issueops-cleanup`·`issueops-sync-issue`·`issueops-sync-pr`(cross-link와 중복 절 삭제), 삭제 5건(스킬 1, 레퍼런스 4), 절 이동 4건.
- 문서: 새 ADR, `ADR.md` 색인, `AGENT_WORKFLOW.md`, `operations/guides/issueops-execution.md`, `architecture/issueops.md`, `OPERATIONS.md`, `testing/issueops-execution.md`, `cautions/issueops-stages.md`(신규 모듈), `CAUTIONS.md` 색인, `TECH_STACK.md`, `README.md`, `README.en.md`, 골든 2개.
- 검증: 전체 게이트 배터리 통과, 일회용 저장소 E2E 프로브 통과 증거.

### Definition of Done

- `go build -o bin/agent-harness ./cmd/harness && ./bin/agent-harness issueops next --json`이 이 저장소(사이클 없음)에서 `stage.key=none`을 돌려준다.
- `go test ./... -count=1`, `go test -race ./... -count=1`, `gofmt -l $(git ls-files '*.go')` 출력 없음, `go vet ./...` 통과.
- `go test ./cmd/harness/contractgolden -run Golden -count=1`과 `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1` 통과.
- 모든 `skills/issueops*` 디렉터리와 `skills/gates-ledger`에 대해 `python3 scripts/validate-skill.py`와 `python3 scripts/verify-skill-shell.py` 통과. `skills/issueops-branch-worktree`가 존재하지 않는다. `ls skills | grep -cE '^issueops|^gates-ledger'`가 17이다.
- `uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json` 통과.
- `./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json` 통과.
- E2E 프로브(T17) 시나리오 4개 전부 PASS 증거가 `.agent-harness/evidence/`에 있고, T0b 스파이크의 실측 결과가 T1·T19·T20·T26의 spec과 일치한다.
- `rg -n "issueops-branch-worktree|issue-preflight.md|worktree-context.md|operational-start.md|references/ai-slop-clean.md|이원 구조" --glob '!.agent-harness/adr/**' --glob '!.agent-harness/turing/**' --glob '!.agent-harness/issues/**' --glob '!.agent-harness/issueops/**' --glob '!openwiki/**' --glob '!.agent-harness/drafts/**' --glob '!.agent-harness/archive/**' --glob '!.agent-harness/cautions/lessons/**' --glob '!docs/superpowers/**' --glob '!.agent-harness/plans/**'` 결과가 0건이다(turing report와 superpowers 문서는 과거 기록이라 제외한다).

### Must Have

- `issueops next`는 mutation이 없고, 사이클이 없거나 모호해도 exit 0과 `ok:true`로 후보를 돌려준다.
- 모든 단계 스킬의 첫 절은 "`agent-harness issueops next --json`을 실행해 이 스킬이 맞는 단계인지 확인한다"이다.
- 세 호스트 실행 명령을 2단계 출력에 나란히 적는다.
- 삭제된 파일을 가리키는 링크가 저장소 어디에도 남지 않는다.

### Must NOT Have

- record schema, lease 상태 기계, `execution prepare` 모드 집합, hook 표면의 변경.
- 새 스킬에 superpowers 플러그인, `AskUserQuestion` 같은 호스트 전용 도구 이름.
- 스킬 본문에 특정 사용자 경로, 세션 ID, 토큰.
- ADR 파일 삭제·수정. 새 결정은 새 파일로만 추가한다.
- 골든 파일 수동 편집. 반드시 `-update`로 재생성한다.

## Verification Strategy

> 사람 개입 없이 에이전트가 실행할 수 있는 검증만 acceptance로 쓴다.

- Test decision: Go는 TDD(RED→GREEN). 스킬·문서는 검사기 + 텍스트 ratchet(`internal/adapter/skillcontract`) + E2E 프로브.
- QA policy: 모든 태스크에 happy·failure 시나리오를 둔다.
- Evidence: `.agent-harness/evidence/task-{N}-{slug}.{ext}`. `evidence`는 gitignore 대상이므로 최종 보고서에는 명령과 결과 요약을 옮겨 적는다.
- E2E 방법: 메모리 `issueops-e2e-probe-method`대로 `gh repo create m16khb/<slug>-probe --private` 일회용 저장소와 tmux 두 번째 세션(`claude -p --permission-mode bypassPermissions --model sonnet`)을 쓴다. 끝나면 `gh repo delete`, 로컬 클론·워크트리 삭제, `issueops cleanup abandon`으로 레코드까지 회수한다.

## Execution Strategy

### Parallel Execution Waves

한 wave는 태스크 4개 이하다. 한 세션이 순차로 실행해도 되고, 병렬이면 같은 wave 안에서만 한다.

- Wave 0: T0, T0b
- Wave 1: T1, T2, T24, T25 (Go, 서로 독립)
- Wave 2: T21, T22, T23, T4 (공용 스킬 셋과 2단계 스킬)
- Wave 3: T6, T3, T5, T8
- Wave 4: T7, T9, T19, T26 (T7은 T6이 고친 카탈로그·골든 위에서 작업한다)
- Wave 5: T20, T10, T12
- Wave 6: T11
- Wave 7: T13, T14, T15 (T14는 T24가 고친 `architecture/issueops.md:42` 위에서 작업한다)
- Wave 8: T16, T17, T18(선택)
- Final: F1–F4

골든 재생성은 T6, T7, T15 세 번이 아니라 T7과 T15 두 번이다. T6은 카탈로그 줄만 추가하고 골든 갱신을 T7과 함께 한 번에 한다.

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|---|---|---|---|
| T0 baseline | — | 전부 | 0 |
| T0b 스파이크(채택·release·재개, 접근 프로브, fingerprint) | T0 | T1, T4, T8, T19, T20 | 0 |
| T1 next contract+domain | T0, T0b | T6, T8, T9, T20 | 1 |
| T2 close-pr port + CloseIssue reason + provider | T0 | T7 | 1 |
| T24 implementation review 게이트 전 모드 확장 | T0 | T20, T15 | 1 |
| T25 provisioner relaunch cwd 수정 + 채택 회귀 테스트 | T0, T0b | T8, T15 | 1 |
| T21 issueops-review 공용 스킬 | T0 | T8, T20, T11 | 2 |
| T22 gates-ledger 공용 스킬 | T0 | T8, T9, T19, T20, T11 | 2 |
| T23 issueops-remote-write 공용 스킬 | T0 | T3, T5, T10, T11 | 2 |
| T4 prepare 스킬 + branch-worktree 삭제 | T0, T0b | T11, T12 | 2 |
| T6 next app/CLI/catalog | T1 | T7, T11, T15 | 3 |
| T3 create-issue 스킬 | T0, T23 | T11 | 3 |
| T5 abandon 스킬 | T0, T23 | T10, T11 | 3 |
| T8 plan 스킬 | T1, T21, T22, T25 | T11 | 3 |
| T7 cleanup abandon 원격 효과 + 골든 | T2, T6 | T15 | 4 |
| T9 implement 재작성 | T1, T22 | T11 | 4 |
| T19 clean 스킬 | T0b, T22 | T26, T11 | 4 |
| T26 docs 스킬 | T0b, T19, T22 | T20, T11 | 4 |
| T20 verify 스킬 | T0b, T1, T21, T22, T24, T26 | T11 | 5 |
| T10 create-pr/complete/cleanup/sync 수정 | T5, T23 | T11 | 5 |
| T12 openai.yaml + README + OPERATIONS + TECH_STACK | T4 | T15 | 5 |
| T11 router 재작성 + 레퍼런스 삭제 + ratchet + skill contract test | T3, T4, T5, T6, T8, T9, T10, T19, T20, T21, T22, T23, T26 | T13, T14, T15 | 6 |
| T13 ADR | T11 | T15 | 7 |
| T14 AGENT_WORKFLOW/ops guide/architecture/testing | T11, T24 | T15 | 7 |
| T15 CAUTIONS 모듈 + docs checker + golden 재생성 | T6, T7, T11, T12, T13, T14, T24, T25 | T16 | 7 |
| T16 전체 게이트 | T15 | T17 | 8 |
| T17 E2E 프로브(4 시나리오) | T16 | T18, F1 | 8 |
| T18 사용자 전역 CLAUDE.md + 메모리(선택) | T17 | F4 | 8 |

## TODOs

> 구현 + 테스트 = 한 태스크. 모든 태스크에 Recommended Agent, References, Acceptance Criteria, QA Scenarios, Commit이 있다.

- [x] **T0. 기준선 고정** — 완료 2026-09-05. git status 깨끗(계획 파일만 미추적), `go build` exit 0, `go test ./... -count=1` 전부 PASS(exit 0), issueops 스킬 9개 validate/shell 모두 exit 0, 이 저장소 active 사이클 0건(scanned 32). pre-existing 실패 없음. 증거: `.agent-harness/evidence/task-0-baseline.txt`

  **What to do**: main HEAD에서 전체 게이트를 한 번 돌려 결과를 evidence로 남긴다. 실패가 있으면 이 계획의 RED로 계산하지 않고 evidence에 따로 적는다.
  1. `git -C "$PWD" status --short`가 비어 있는지 확인한다.
  2. `go build -o bin/agent-harness ./cmd/harness && go test ./... -count=1 2>&1 | tail -30`
  3. `for d in skills/issueops*; do python3 scripts/validate-skill.py "$d"; python3 scripts/verify-skill-shell.py "$d"; done`
  4. `./bin/agent-harness issueops list --repo "$PWD" --json`으로 이 저장소에 active 사이클이 없음을 확인한다.

  **Must NOT do**: 파일 수정.

  **Recommended Agent**: quick. 명령 실행과 기록뿐이다.

  **Parallelization**: Can Parallel: NO | Wave 0 | Blocks: 전부 | Blocked By: 없음

  **References**:
  - `AGENTS.md` §9 Essential Commands
  - 메모리 `issueops-e2e-probe-method`: Go 테스트는 실제 상태 저장소를 건드리지 않는다.

  **Acceptance Criteria**:
  - [ ] `.agent-harness/evidence/task-0-baseline.txt`에 위 명령 4개의 종료 코드와 마지막 30줄이 있다.

  **QA Scenarios**:
  ```
  Scenario: 기준선 녹색
    Channel: bash
    Steps: go test ./... -count=1; echo "exit=$?"
    Expected: exit=0
    Evidence: .agent-harness/evidence/task-0-baseline.txt
  Scenario: 기준선에 사전 실패가 있음
    Channel: bash
    Steps: 실패한 패키지명을 evidence에 "pre-existing:" 접두어로 기록
    Expected: 실패 목록이 T16의 비교 기준이 된다
    Evidence: .agent-harness/evidence/task-0-baseline.txt
  ```

  **Commit**: NO

- [x] **T0b. 스파이크: 채택 경로·접근 프로브·fingerprint를 코드 변경 없이 실측한다** — 완료 2026-09-05. 결과는 `## Context › T0b 스파이크 실측 결과` 표 11항목. 증거: `.agent-harness/evidence/task-0b-adopt.json`, `task-0b-access.txt`, `task-0b-fingerprint.json`. 실측 4(link-issue 자동 전진)로 분류표 규칙 6·7a를 artifact 기준으로 고쳤다.

  **What to do**: 메모리 `issueops-e2e-probe-method`대로 일회용 저장소를 만들고 현행 CLI만으로 세 가지를 확인한다. 결과는 T1·T19·T20·T26의 spec을 확정하는 입력이며, 어긋나면 spec을 고친 뒤 진행한다.
  1. **채택과 release·재개**: `issueops start` → `intent record` → `domain-review record` → `decision add --kind scope` → `plan-prep record`(waive 가능) → `phase --to grill` → 이슈 생성 → `branch prepare`(base SHA) → `git worktree add <canonical root> -b <branch> <base_sha>` → `phase --to plan` → tmux 두 번째 세션을 워크트리에서 띄워 `execution prepare --mode direct --direct-reason "probe" ...` preview·confirm → `execution status`에 이 세션이 generation 1 홀더인지 → `execution release` → 세 번째 세션에서 `execution status`의 `next_command` 체인(`replace --preview` → `--reseed` → `claim`)으로 홀더가 되는지. 각 명령의 JSON을 evidence에 남긴다.
  2. **접근 프로브**: 두 번째 세션을 `--add-dir` 없이 띄웠을 때 `execution prepare`가 `canonical_worktree_base_inaccessible`과 `relaunch_command`를 돌려주는지, 돌려주면 그 명령의 cwd가 어디인지(T25가 고칠 결함의 실측), `--add-dir "<source>.worktrees"`를 붙이면 통과하는지.
  3. **fingerprint**: 1번 사이클을 `phase --to implement` → 파일 하나 수정 → `phase --to ai-slop-clean` → `ai-slop-clean record`까지 진행한 뒤, `.agent-harness/issues/<n>/` 아래에 untracked 파일 하나를 만들고 `pr-readiness --id --strict --json`에 `ai_slop_clean_stale`이 뜨는지. 그다음 `ai-slop-clean record`를 다시 실행하면 사라지는지.
  종료: 사이클을 `cleanup abandon`으로 회수하고 저장소를 지운다.

  **Must NOT do**: Go·스킬 파일 수정, 사용자 실제 저장소 사용.

  **Recommended Agent**: deep. 세 세션과 원격 상태를 다룬다.

  **Parallelization**: Can Parallel: NO | Wave 0 | Blocks: T1, T4, T8, T19, T20, T25, T26 | Blocked By: T0

  **References**: 메모리 `issueops-e2e-probe-method`, `internal/adapter/gitworktree/provisioner.go:20-46,102-117,180-190`, `internal/domain/issueopslease/reseed.go`, `internal/adapter/issueops/implementation/evidence.go:52-106`, `issueops_pr_readiness_strict.go:76-78`.

  **Acceptance Criteria**:
  - [ ] `.agent-harness/evidence/task-0b-adopt.json`에 채택 성공 receipt(`exists: true`)와 세 번째 세션의 claim 결과(lease active)가 있다.
  - [ ] `.agent-harness/evidence/task-0b-access.txt`에 relaunch_command 원문과 `--add-dir` 결과가 있다.
  - [ ] `.agent-harness/evidence/task-0b-fingerprint.json`에 untracked 파일 뒤의 `ai_slop_clean_stale`과 재기록 뒤의 부재가 있다.

  **QA Scenarios**:
  ```
  Scenario: 채택
    Channel: tmux + bash
    Steps: 위 1번
    Expected: prepare receipt exists true, 새 워크트리 생성 없음(git worktree list에 경로 1개)
    Evidence: .agent-harness/evidence/task-0b-adopt.json
  Scenario: stale 재현
    Channel: bash
    Steps: 위 3번
    Expected: pr-readiness --strict missing에 ai_slop_clean_stale, 재기록 뒤 사라짐
    Evidence: .agent-harness/evidence/task-0b-fingerprint.json
  ```

  **Commit**: NO

- [x] **T1. `issueopsnext` contract + domain (순수 분류기)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-1-classify.txt`(44 PASS, 0 FAIL). `go list -deps ./internal/domain/issueopsnext | grep -c internal/adapter` = 0.

  **실행 결과와 계획 대비 편차**:
  - **분류표의 "현재 index"를 phase→단계 번호 표 하나로 확정했다**: problem·grill 1, plan 3, compatibility-review·implement 4, ai-slop-clean 5, feedback 7, pr 9, done 10. 계획 초안의 테스트 케이스 세 줄(`blocked.pending`·`blocked.holder_live`·`takeover`의 implement phase)이 index 3을 기대했는데, 그것은 9단계 시절 번호가 남은 것이다. 같은 표를 쓰는 규칙 6(`claim`)이 implement에서 4를 기대하므로 3은 표와 모순이었다. 4로 확정하고 테스트를 그렇게 고쳤다.
  - **규칙 8~12에 phase 상한을 넣었다**. 계획의 규칙 8은 "issue_url 있음 + execution == nil + branch_prepare 없음"만 봤는데, 그러면 `pr` phase에 execution이 없는 이상 사이클도 2단계로 분류된다(계획 자신의 테스트는 그 경우 `unknown`을 기대했다). 준비 규칙은 problem·grill·plan·compatibility-review·implement에서만 발화한다.
  - **Input에 두 필드를 더했다**: `RootConflictID`(규칙 3이 필요로 하는데 계획의 Input에 없었다)와 `WriterlessRecovery`(규칙 5의 명령은 어댑터가 렌더한다). 둘 다 관측 결과를 주입하는 값이므로 domain은 여전히 순수하다.
  - `plan_artifact`를 새 missing 키로 만들었다. staged plan 부재를 가리키는 기존 게이트 키가 없었다. `OwnerCommand`가 `artifact stage --name plan`으로 매핑한다.
  - `HolderLive == nil`(관측 실패)은 **살아 있음**으로 본다. 확인하지 않은 세션의 lease를 빼앗으라고 권하지 않기 위한 fail-closed 방향이며 `TestClassifyTreatsUnobservedHolderAsLive`가 고정한다.
  - `next_command_kind`는 규칙마다 적지 않고 `<`·`$ACTOR_FLAGS` 포함 여부로 파생한다(소유자 하나).
  - stage key 상수는 domain이 아니라 contract 패키지가 소유한다. 라우터가 읽는 wire 값이기 때문이다.

  **Files:**
  - Create: `internal/contract/issueopsnext/types.go`
  - Create: `internal/domain/issueopsnext/classify.go`
  - Create: `internal/domain/issueopsnext/owner_command.go`
  - Test: `internal/domain/issueopsnext/classify_test.go`

  **Interfaces:**
  - Produces: `issueopsnext.Classify(in Input) Decision`, `issueopsnext.OwnerCommand(id, missingKey string) string`, contract 타입 `Result`, `Entry`, `Stage`, `Lease`, `Exits`(설계 요약 3 그대로).

  **What to do**:
  1. contract 타입을 설계 요약 3의 Go 코드 그대로 쓴다. `Entry`의 필드는 각각 json 태그 `id`, `phase`, `branch`, `workspace_root`, `issue_url`, `remote_artifact_url`이다.
  2. domain `Input`을 정의한다.
     ```go
     package issueopsnext

     type Readiness struct {
         Ready   bool
         Missing []string
     }
     type Input struct {
         Record          issueopscontract.IssueOpsRecord
         Completion      func(phase issueopscontract.IssueOpsPhase) Readiness
         Local           *Readiness // ai-slop-clean·feedback일 때만 채운다. fetch 없는 local readiness
         StagedPlan      bool       // artifact stage --name plan 이 올라와 있는가
         ActorHost       string
         ActorSessionID  string
         HolderLive      *bool      // holder가 없으면 nil
         WorktreePresent bool
         WorktreeHead    string     // canonical 워크트리 HEAD. 없으면 ""
         SourceRoot      string
     }
     type Decision struct {
         Stage       issueopsnextcontract.Stage
         Lease       issueopsnextcontract.Lease
         Missing     []string
         NextCommand string
         Exits       issueopsnextcontract.Exits
         Warnings    []string
     }
     ```
  3. 실패 테스트를 먼저 쓴다. 분류표 17행 각각을 한 케이스로 두는 table-driven 테스트다. 최소 케이스 이름과 기대값:
     ```go
     func TestClassifyStages(t *testing.T) {
         cases := []struct {
             name string
             in   Input
             key  string
             idx  int
         }{
             {"no record", Input{}, "none", 0},
             {"pending intent blocks", withPending(baseRecord("implement")), "blocked.pending", 3},
             {"other live holder", withHolder(baseRecord("implement"), "codex", "s-2", true), "blocked.holder_live", 3},
             {"stale holder takeover", withHolder(baseRecord("implement"), "codex", "s-2", false), "takeover", 3},
             {"grill without issue", baseRecord("grill"), "issue", 1},
             {"grill with issue no branch", withIssue(baseRecord("grill")), "prepare", 2},
             {"plan right after link-issue auto-advance (no branch)", withIssue(baseRecord("plan")), "prepare", 2},
             {"implement without execution after switch-mode", withIssue(baseRecord("implement")), "prepare", 2},
             {"branch ready, no staged plan", withBranch(withIssue(baseRecord("plan"))), "plan.write", 3},
             {"staged plan, design missing", withStagedPlan(withBranch(withIssue(baseRecord("plan")))), "plan.design", 3},
             {"design approved, DA missing", withDesign(withStagedPlan(withBranch(withIssue(baseRecord("plan"))))), "plan.review", 3},
             {"DA stop unwaived", withDAStop(withDesign(withStagedPlan(withBranch(withIssue(baseRecord("plan")))))), "plan.review", 3},
             {"all planner gates recorded", withDA(withDesign(withStagedPlan(withBranch(withIssue(baseRecord("plan")))))), "plan.handoff", 3},
             {"orca claimable right after prepare", withClaimableLease(withBranch(withIssue(baseRecord("plan")))), "claim", 3},
             {"released lease in implement", withReleasedLease(withBranch(withIssue(baseRecord("implement")))), "claim", 4},
             {"self holder still in plan phase", withSelfHolder(withBranch(withIssue(baseRecord("plan")))), "implement.enter", 4},
             {"self holder in implement", withSelfHolder(baseRecord("implement")), "implement", 4},
             {"cleanup not recorded", withCompletionMissing(withSelfHolder(baseRecord("ai-slop-clean")), "ai-slop-clean", "ai_slop_clean_at"), "clean", 5},
             {"cleanup recorded docs review missing", withLocal(withSelfHolder(baseRecord("ai-slop-clean")), "project_docs_review"), "docs", 6},
             {"docs reviewed implementation review missing", withLocal(withSelfHolder(baseRecord("ai-slop-clean")), "implementation_review"), "verify", 7},
             {"feedback contract update missing", withLocal(withSelfHolder(baseRecord("feedback")), "contract_feedback_issue_update"), "verify", 7},
             {"only upstream missing", withLocal(withSelfHolder(baseRecord("ai-slop-clean")), "upstream"), "commit-push", 8},
             {"pushed and clean", withLocal(withSelfHolder(baseRecord("ai-slop-clean"))), "commit-push", 8},
             {"unmatched local key falls back", withLocal(withSelfHolder(baseRecord("feedback")), "branch_match"), "unknown", 7},
             {"pr without artifact", withSelfHolder(baseRecord("pr")), "pr.create", 9},
             {"pr with artifact", withArtifact(withSelfHolder(baseRecord("pr"))), "pr.complete", 9},
             {"pr without execution", withIssue(baseRecord("pr")), "unknown", 9},
             {"done", baseRecord("done"), "done", 10},
         }
         for _, tc := range cases {
             t.Run(tc.name, func(t *testing.T) {
                 got := Classify(tc.in)
                 if got.Stage.Key != tc.key || got.Stage.Index != tc.idx {
                     t.Fatalf("got %s/%d want %s/%d", got.Stage.Key, got.Stage.Index, tc.key, tc.idx)
                 }
             })
         }
     }
     ```
     헬퍼(`baseRecord`, `withIssue`, `withSelfHolder`, `withHolder`, `withReleasedLease`, `withClaimableLease`, `withPending`, `withPlan`, `withDesign`, `withBranch`, `withStagedPlan`, `withDA`, `withDAStop`, `withCompletionMissing`, `withLocal`, `withArtifact`)는 테스트 파일 안에 둔다. `withBranch`는 `BranchPrepare.BaseSHA`를 채운다. `withStagedPlan`은 `Input.StagedPlan=true`로 둔다. `withDA`는 digest가 맞는 pass 판정을 넣고 `Completion(implement).Missing`에서 devils-advocate 키를 뺀다. `withDAStop`은 verdict `stop`, `waived=false` 판정을 넣고 `Completion(implement).Missing`이 `devils_advocate_review`를 돌려주게 한다. `withCompletionMissing(in, phase, keys...)`는 `Input.Completion`을 그 phase에서 그 키들을 돌려주는 함수로 바꾼다. `withLocal(in, keys...)`는 `Local.Missing`을 그 키들로 채우고, 키가 없으면 빈 missing으로 둔다. `baseRecord(phase)`는 `ID="io-test"`, `Repo="/repo"`, `Branch="12-test"`와 그 phase를 가진 record를 `Input`에 담는다. `withSelfHolder`는 `ActorHost="claude"`, `ActorSessionID="s-1"`와 같은 holder를 가진 active lease를 넣는다. `withReleasedLease`·`withClaimableLease`는 holder 없는 lease를 그 상태로 넣는다.
  4. 추가 테스트: `TestClassifyNextCommandTable`(`none`은 `issueops start --repo`로, `issue`는 `intent record`로 시작하고 절대 `branch prepare`가 아니며, `plan.review`에서 stop 미waive면 `issueops regress --id`로, `done`은 `cleanup status --id <id> --merged`로, `commit-push`는 `pr-readiness --id <id> --strict`로, `unknown`은 `issueops status --id`로 시작; `plan.handoff`는 `NextCommandKind=="template"`, `none`·`done`은 `"exact"`), `TestClassifyExits`(self holder면 `PauseCommand`가 `execution release`로 시작, stale holder면 `TakeoverCommand`가 `execution replace`로 시작, 항상 `AbandonCommand`가 `cleanup abandon --id <id> --reason`으로 시작), `TestOwnerCommandCoversGateMap`(현행 gate map 키 전부에 대해 빈 문자열이 아님).
  5. `go test ./internal/domain/issueopsnext -run TestClassify -count=1`로 RED를 확인한다(컴파일 실패도 RED로 인정).
  6. `Classify`를 분류표 순서대로 구현한다. 규칙 판정에 쓰는 record 필드는 `Record.Phase`, `Record.IssueURL`, `Record.Branch`, `Record.BranchPrepare.BaseSHA`, `Record.PlanPath`, `Record.DesignReview.Approved`, `Record.DevilsAdvocateReview{RecordedAt, Verdict, Waived, ReviewedPlanDigest}`, `Record.CompatibilityReview.Approved`, `Record.Execution{Pending, Lease{Status, Generation, Holder}, Completion}`, `Record.RemoteArtifact`이다. devils-advocate stale 판정은 `Completion(PhaseImplement).Missing`에 `devils_advocate_review_stale`이 있는지로 본다(직접 digest를 계산하지 않는다).
  7. `OwnerCommand`는 `skills/issueops/SKILL.md` "Gate map" 표의 키 전부를 `switch`로 매핑한다. 키: `intent_contract`, `raw_request`, `interpreted_intent`, `success_criteria`, `plan_prep_decisions`, `plan_prep_related_issues`, `plan_prep_web_research`, `plan_prep_codebase_survey`, `issue_url`, `branch`, `split_decision`, `domain_review`, `design_review`, `design_*`, `compatibility_review`, `compatibility_*`, `devils_advocate_review`, `devils_advocate_review_stale`, `worktree_path`, `worktree_exists`, `plan_exists`, `plan_in_worktree`, `execution`, `execution_valid`, `execution_worktree_match`, `execution_write_lease`, `implementation_changes`, `ai_slop_clean_*`, `cleanup_evidence`, `verification_evidence`, `implementation_review`, `implementation_review_stale`, `project_docs_review`, `project_docs_review_stale`, `schema_evidence`, `schema_evidence_stale`, `feedback_classification`, `feedback_resolution`, `contract_feedback_issue_update`, `remote_artifact`, `child_incomplete`, `child_unvalidated`, `child_rejected_unresolved`, `gates_incomplete:<file>`(접두어 일치, owner는 `agent-harness gates check --cwd <worktree> --workspace-root <worktree> --json`), `worktree_clean`, `upstream`, `upstream_fetch`, `upstream_synced`, `branch_match`. 알 수 없는 키는 `agent-harness issueops status --id <id> --json`을 돌려준다.
  8. GREEN 확인 후 `go vet ./internal/domain/issueopsnext ./internal/contract/issueopsnext`.

  **Must NOT do**: filesystem, git, process, SQLite 접근. `internal/adapter/**` import. record 필드 추가.

  **Recommended Agent**: deep. 분류 규칙 17개와 owner command 표의 정확성이 핵심이다.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T6, T8, T9, T20 | Blocked By: T0, T0b

  **References**:
  - Pattern: `internal/domain/issueopsinventory/inventory.go` — 순수 projection 함수의 형식.
  - Pattern: `internal/domain/issueopsstatus/projector.go:18-60` — phase completion 주입 방식(함수값).
  - API/Type: `internal/contract/issueops/types.go:445` `PhaseLedger`, `:545-560` `IssueOpsReadiness`, `internal/contract/issueops/execution.go:18-19` `ExecutionMode`, lease status 상수 `LeaseStatusClaimable` 등(`internal/contract/issueopsinventory/types.go:52-55`가 alias로 재노출).
  - Rules: `internal/adapter/issueops/issueops_phase_ledger.go:27-66`, `issueops_readiness.go:112-185`, `issueops_pr_readiness_strict.go:12-70`(strict missing 키), `internal/application/issueopspreparation/prepare.go:496-527`(writerless next_command 문구).
  - Gate map 원문: `skills/issueops/SKILL.md` "Gate map", `.agent-harness/AGENT_WORKFLOW.md` "IssueOps 자동 루프는 missing gate를 읽고" 문단.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/domain/issueopsnext -count=1 -v 2>&1 | grep -c '^--- PASS'`가 20 이상이다.
  - [ ] `go list -deps ./internal/domain/issueopsnext | grep -c 'internal/adapter'`가 0이다.

  **QA Scenarios**:
  ```
  Scenario: 17개 단계 분류가 전부 통과
    Channel: bash
    Steps: go test ./internal/domain/issueopsnext -run TestClassifyStages -count=1 -v
    Expected: 출력에 "--- FAIL" 0건, "ok  agent-harness/internal/domain/issueopsnext"
    Evidence: .agent-harness/evidence/task-1-classify.txt
  Scenario: 규칙 순서 위반 감지
    Channel: bash
    Steps: pending intent와 done phase를 동시에 가진 record를 Classify에 넣는다
    Expected: Stage.Key == "blocked.pending" (규칙 2가 규칙 16·17보다 먼저)
    Evidence: .agent-harness/evidence/task-1-classify-order.txt
  ```

  **Commit**: YES | `feat(issueops): add the issueopsnext stage classifier` | Files: 위 4개

- [x] **T2. `ClosePullRequest` provider port + `CloseIssue` reason + GitHub·GitLab adapter** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-2-close-pr.txt`(12 PASS).

  **실행 결과와 계획 대비 편차**(T7 실행자는 반드시 읽는다):
  - `IssueProvider` 인터페이스를 넓히지 않고, `internal/port/provider.go`의 기존 관례대로 선택적 능력 인터페이스 `IssueProviderPullRequestCloser`를 추가했다. 그래서 fake/stub 전수 수정(계획 6단계)이 필요 없었고 실제로 하지 않았다. 소비자는 `closer, ok := provider.(port.IssueProviderPullRequestCloser)`로 타입 단언하고, `ok`가 거짓이면 "이 provider는 PR/MR close를 지원하지 않는다"로 fail-closed 처리한다.
  - 파일 배치: GitHub는 `internal/adapter/provider/github/close_pull_request.go`, GitLab은 `internal/adapter/provider/gitlab/close_merge_request.go`로 분리했다(`provider.go`가 이미 887줄이라 더 키우지 않았다). `CloseIssue`의 reason 헬퍼 `ghCloseIssueReason`·`ghQuoteReason`만 `provider.go`의 `CloseIssue` 옆에 남겼다.
  - preview 문자열은 `gh pr view <url> --json state`다. `mergedAt`은 넣지 않았다. `state`가 이미 `MERGED`를 구분하므로 필드를 늘릴 이유가 없다.
  - 테스트 파일명은 `close_pull_request_test.go`·`close_merge_request_test.go`다(계획의 `provider_close_pr_test.go`가 아니다). 기존 `close_issue_test.go` 명명을 따랐다.

  **Files:**
  - Modify: `internal/port/provider.go:227-256` (Close 요청·결과 타입 옆에 추가, `IssueProvider` 인터페이스에 메서드 추가)
  - Modify: `internal/adapter/provider/github/provider.go` (CloseIssue 705-740 아래에 추가)
  - Modify: `internal/adapter/provider/gitlab/provider.go` 또는 `provider_issue.go` (CloseIssue 62-95 아래에 추가)
  - Test: `internal/adapter/provider/github/provider_close_pr_test.go`, `internal/adapter/provider/gitlab/provider_close_pr_test.go`
  - Modify: `IssueProvider`를 구현하는 모든 fake/stub(`rg -l "CloseIssue(" --glob '*_test.go'`로 찾은 파일 전부)에 `ClosePullRequest` 스텁 추가

  **Interfaces:**
  - Produces:
    ```go
    type IssueProviderClosePullRequestRequest struct {
        Repo        string `json:"repo"`
        ArtifactURL string `json:"artifact_url"`
        Kind        string `json:"kind"` // pr | mr
        Confirm     bool   `json:"confirm"`
    }
    type IssueProviderClosePullRequestResult struct {
        OK            bool   `json:"ok"`
        Provider      string `json:"provider"`
        ArtifactURL   string `json:"artifact_url"`
        Kind          string `json:"kind"`
        Preview       string `json:"preview,omitempty"`
        Closed        bool   `json:"closed"`
        AlreadyClosed bool   `json:"already_closed,omitempty"`
        Merged        bool   `json:"merged,omitempty"`
        State         string `json:"state,omitempty"`
    }
    // IssueProvider 인터페이스에 추가
    ClosePullRequest(req IssueProviderClosePullRequestRequest) (IssueProviderClosePullRequestResult, error)
    ```

  **What to do**:
  1. 실패 테스트: GitHub는 `exec` 경계를 기존 테스트가 쓰는 방식(같은 패키지의 `gh` 실행 함수 변수 또는 `PATH` 스텁; `provider_test.go`에서 CloseIssue 테스트가 쓰는 방식을 그대로 복사)으로 스텁하고 세 케이스를 쓴다: preview(confirm=false)면 `[dry-run] would execute: gh pr close <url>; gh pr view <url> --json state,mergedAt`를 돌려주고 명령을 실행하지 않는다; state가 `MERGED`면 `Merged=true, Closed=false`이고 close를 실행하지 않는다; state가 `OPEN`이면 `gh pr close <url>` 후 readback `CLOSED`로 `Closed=true`.
  2. GitLab도 같은 세 케이스: endpoint `projects/<escaped>/merge_requests/<iid>`, readback `state`가 `merged`면 `Merged=true`, `closed`면 `AlreadyClosed=true`, `opened`면 `--method PUT -f state_event=close` 후 readback `closed`.
  3. RED 확인: `go test ./internal/adapter/provider/... -run ClosePullRequest -count=1`.
  4. 구현. URL 파싱은 GitHub `owner/repo/pull/<n>`, GitLab `/-/merge_requests/<iid>`를 기존 파서(`internal/adapter/provider/github/created_artifact_number.go`, `gitlab` 패키지의 MR URL 파서. 없으면 `internal/domain/issueopsremote/issueops_remote_url.go`의 artifact URL 검증 함수)로 한다. 새 정규식을 만들지 않는다.
  5. `IssueProviderCloseIssueRequest`(`internal/port/provider.go:229-233`)에 `Reason string`을 추가한다. 값은 `completed`, `not_planned`, 빈 값(=`completed`)뿐이다. GitHub는 `gh issue close <url> --reason completed` 또는 `--reason "not planned"`로 매핑하고(`provider.go:723`의 고정값을 치환), GitLab은 무시한다. 기존 호출자(`issueops_completion_remote.go:70-102`)는 값을 비워 둬 동작이 바뀌지 않는다. 테스트: `not_planned`가 `--reason "not planned"` argv를 만들고, 빈 값이 현행 argv와 같다.
  6. 인터페이스를 구현하는 모든 fake에 메서드를 추가해 컴파일을 통과시킨다. `go build ./... && go vet ./...`.

  **Must NOT do**: PR/MR 본문 수정, 브랜치 삭제, 머지. issue close 코드를 복사해 두 벌로 만들지 않고 readback 헬퍼(`readGhIssueState` 계열)와 같은 층에 `readGhPullRequestState`를 둔다.

  **Recommended Agent**: deep. 두 provider와 인터페이스 구현체 전수 수정이 필요하다.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T7 | Blocked By: T0

  **References**:
  - Pattern: `internal/adapter/provider/github/provider.go:705-740` `CloseIssue` — preview 문자열, 사전 readback, 실행, 사후 readback 순서를 그대로 따른다.
  - Pattern: `internal/adapter/provider/gitlab/provider_issue.go:62-95` `CloseIssue`.
  - API/Type: `internal/port/provider.go:227-256`.
  - Existing verifier: `cmd/harness/issueopscli/remoteverify`의 `ObserveRemoteArtifactTargetLive`(artifact 관측 방식 참고).

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapter/provider/... -run ClosePullRequest -count=1 -v | grep -c '^--- PASS'`가 6 이상이다.
  - [ ] `go build ./... && go vet ./...` 종료 코드 0.

  **QA Scenarios**:
  ```
  Scenario: merged PR은 닫지 않음
    Channel: bash
    Steps: readback 스텁이 MERGED를 돌려주는 상태에서 Confirm=true로 호출
    Expected: result.Merged==true, result.Closed==false, close 명령 호출 횟수 0
    Evidence: .agent-harness/evidence/task-2-close-pr-merged.txt
  Scenario: preview는 mutation 없음
    Channel: bash
    Steps: Confirm=false로 호출
    Expected: Preview가 "[dry-run]"으로 시작, 실행 스텁 호출 0
    Evidence: .agent-harness/evidence/task-2-close-pr-preview.txt
  ```

  **Commit**: YES | `feat(provider): add ClosePullRequest for GitHub and GitLab` | Files: 위 파일

- [x] **T3. `issueops-create-issue` 재작성 (1단계: 이슈 확정·생성)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-3-order.txt`(여섯 명령이 순서대로), `task-3-links.txt`(issue-preflight 참조 0).

  **실행 결과와 계획 대비 편차**: QA 시나리오의 정규식은 `decision add --kind scope`처럼 플래그가 붙지 않은 형태를 가정했는데, 실제 명령은 저장소 관례대로 `--id`가 먼저 온다. 문서를 정규식에 맞추는 대신 정규식을 실제 형태(`decision add .* --kind scope`, `phase .* --to grill`)로 고쳐 증거를 남겼다. 옛 `## 흐름` mermaid와 `## 시작 게이트`는 새 `## 이 스킬이 맞는지 확인`·`## 기록 순서`가 대체하므로 지웠다.

  **Files:**
  - Modify: `skills/issueops-create-issue/SKILL.md` (전면 재작성)
  - Modify: `skills/issueops-create-issue/agents/openai.yaml` (default_prompt를 1단계 전체로)

  **What to do**: 아래 절 순서와 내용으로 다시 쓴다. 현행 본문의 "템플릿을 정한 근거", "Parent와 child", "읽기 좋은 body", 좋은 예·나쁜 예, "Canonical publication", "품질·성능 게이트"는 그대로 유지하고 앞에 네 절을 추가한다.
  1. `## 이 스킬이 맞는지 확인`: `agent-harness issueops next --json`을 실행한다. `stage.key`가 `none` 또는 `issue`일 때 진행한다. 사용자가 라우터 선택지 4 "새 사이클 시작"을 골랐으면 `stage.key`와 무관하게 진행하며, 이 저장소에 다른 사이클이 있어도 새 `start`는 허용된다. 그 외에는 라우터 `## 단계 표`의 스킬을 안내하고 멈춘다.
  2. `## 입력 세 가지`: 사용자 제공 정보, 코드베이스 조사(CodeGraph가 있으면 `codegraph explore`, 없으면 `rg`; 만진 심볼·파일·호출 경로를 evidence 문자열로 만든다), 배경지식·웹 조사(`berners-lee`는 외부 API 의미가 걸릴 때만). 각 조사 결과가 plan-prep 4항목의 evidence다.
  3. `## 질문 규칙`: 현행 `references/issue-preflight.md`의 Deep-Interview Gate와 ambiguity ledger(`resolved`, `deferred`, `blocking`)를 옮긴다. blocking만 사용자에게 묻고, 한 번에 한 질문, 선택지는 번호로 준다. deferred는 이슈 본문 "열린 결정" 절에 남긴다.
  4. `## 기록 순서`(코드 블록): `start --repo "$SOURCE_ROOT" --json` → `intent record`(`--intent-class trivial|standard`) → `domain-review record` → `decision add --kind scope --title "no split" --body "<근거>"` (split이면 `remote create-child`) → `plan-prep record` 4항목 → `phase --to grill` → body 초안 → 사용자 승인 → `remote score` → `issueops-remote-write` 프로토콜로 `remote create-issue`(fluent-korean, 한국어 게이트 스크립트, preview, 동일 요청 `--confirm`, readback, 모호하면 `remote reconcile-issue`) → `link-issue`가 자동이 아니면 `link-issue` → 마지막 출력 블록(`ISSUEOPS_ID`, issue URL, "다음 단계: issueops-prepare").
  5. `## 나쁜 예` 표에 세 행 추가: 워크트리 안에서 `start` 실행(source root에서 실행), blocking이 아닌 질문 남발, plan-prep을 waive로 때우기.
  6. 검증 절: `python3 scripts/validate-skill.py skills/issueops-create-issue`, `python3 scripts/verify-skill-shell.py skills/issueops-create-issue`.

  **Must NOT do**: PR/MR 규칙 포함, `issue-preflight.md` 링크 유지, superpowers 언급.

  **Recommended Agent**: deep. 네 문서의 내용을 한 스킬로 합치며 순서 오류가 곧 게이트 실패다.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T11 | Blocked By: T0, T23

  **References**:
  - `skills/issueops-remote-write/SKILL.md`(T23; 원격 쓰기 절차의 소유자).
  - `skills/issueops/references/issue-preflight.md` 전체(이 태스크 뒤 T11에서 삭제).
  - `skills/issueops/SKILL.md` "시작 순서" 1-6(명령 순서 원문).
  - `internal/adapter/issueops/issueops_phase_ledger.go:27-66` (grill completion 요구 항목).
  - `PROMPT.md` (ideal issue prompt scaffold).
  - `skills/fluent-korean/SKILL.md` (원격 한국어 규칙).

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-create-issue` 출력에 `Skill is valid!`.
  - [ ] `python3 scripts/verify-skill-shell.py skills/issueops-create-issue` 종료 코드 0.
  - [ ] `rg -n "issue-preflight|superpowers|AskUserQuestion" skills/issueops-create-issue` 0건.
  - [ ] `rg -n "issueops next" skills/issueops-create-issue/SKILL.md` 1건 이상, `rg -n "issueops-remote-write" skills/issueops-create-issue/SKILL.md` 1건 이상.
  - [ ] 원격 쓰기 절차(preview → confirm → readback, fluent-korean 호출)를 본문에 복사하지 않는다: `rg -c "동일한 요청에만|같은 요청에만" skills/issueops-create-issue/SKILL.md`가 0.

  **QA Scenarios**:
  ```
  Scenario: 기록 순서가 grill completion과 일치
    Channel: bash
    Steps: rg -n "intent record|domain-review record|decision add --kind scope|plan-prep record|phase --to grill|remote create-issue" skills/issueops-create-issue/SKILL.md | sort -t: -k2 -n
    Expected: 여섯 명령이 이 순서대로 처음 등장한다
    Evidence: .agent-harness/evidence/task-3-order.txt
  Scenario: 삭제될 레퍼런스를 참조하지 않음
    Channel: bash
    Steps: rg -n "references/issue-preflight" skills/issueops-create-issue
    Expected: 0건
    Evidence: .agent-harness/evidence/task-3-links.txt
  ```

  **Commit**: YES | `docs(skill): rewrite issueops-create-issue as the issue confirmation stage` | Files: 위 2개

- [x] **T4. `issueops-prepare` 신설 + `issueops-branch-worktree` 삭제 (2단계)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-4-no-worktree.txt`(`git worktree add`는 금지 문맥 1건뿐), `task-4-deleted.txt`(추적 파일 0).

  **실행 결과와 계획 대비 편차**: 삭제한 스킬을 가리키던 링크 여섯 곳(`skills/issueops/SKILL.md` 2곳, `skills/issueops-implement/SKILL.md`, README 둘, TECH_STACK)을 함께 고쳤다. 그 파일들의 재작성은 T11·T12가 소유하지만, 없는 스킬을 가리키는 링크를 그때까지 두면 그 사이의 세션이 존재하지 않는 문서를 읽으러 간다. 스킬 목록이 골든에 들어 있어 `response_contracts.golden.json`도 이 시점에 재생성했다(T15에서 다시 생성한다).

  **Files:**
  - Create: `skills/issueops-prepare/SKILL.md`
  - Create: `skills/issueops-prepare/agents/openai.yaml`
  - Delete: `skills/issueops-branch-worktree/` (디렉터리 전체, `git rm -r`)

  **What to do**:
  1. frontmatter: `name: issueops-prepare`, description은 "Give an IssueOps cycle its branch identity: resolve and pin the exact base SHA, record it with `issueops branch prepare`, create and verify the provider-side branch link, and move the cycle to the plan phase. Creating the worktree is not this skill's job — `issueops execution prepare` owns it and picks git or Orca. Use when the user says '준비', '브랜치 만들어줘', 'prepare the issue branch', or when `issueops next` reports stage prepare."
  2. 절 순서: `## 이 스킬이 맞는지 확인`(`next`의 `stage.key == prepare`) → `## 안전 규칙` → `## 절차` → `## 검증` → `## 실패 처리`(현행 branch-worktree Failure handling에서 worktree 항목만 뺀 것).
  3. `## 안전 규칙`: 현행 branch-worktree Safety rules에서 worktree 항목을 빼고 다음 세 줄을 넣는다. (a) base는 움직이는 ref가 아니라 정확한 SHA로 봉인한다. (b) **워크트리를 만들지 않는다.** 워크트리 provisioning은 `execution prepare`가 단독으로 소유하며, direct는 git으로, Orca는 `orca worktree create --name <branch>`로 자기 브랜치와 워크트리를 만든다(`internal/adapter/orca/client.go:284-333`). 여기서 로컬 브랜치나 워크트리를 미리 만들면 Orca 경로가 이름 충돌로 깨진다. (c) provider 링크는 외부 write이므로 무엇을 쓸지 말하고 사용자 확인을 받은 뒤 진행한다.
  4. `## 절차` 코드 블록:
     ```bash
     COMMON_GIT_DIR=$(git rev-parse --path-format=absolute --git-common-dir)
     SOURCE_ROOT=$(dirname "$COMMON_GIT_DIR")
     BRANCH="<issue-number>-<kebab-slug>"
     git -C "$SOURCE_ROOT" fetch origin "$BASE_BRANCH"
     BASE_SHA=$(git -C "$SOURCE_ROOT" rev-parse "origin/$BASE_BRANCH")
     agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" \
       --issue-url "$ISSUE_URL" --branch "$BRANCH" --base-branch "$BASE_BRANCH" \
       --base-sha "$BASE_SHA" $RECORD_ACTOR_FLAGS --json
     # provider 링크: GitHub은 createLinkedBranch(oid=BASE_SHA), GitLab은 branches API(ref=BASE_SHA).
     # 현행 issueops-branch-worktree 4절의 명령을 그대로 옮긴다. `gh issue develop`은 쓰지 않는다.
     agent-harness issueops branch prepare ... --link-verified --json
     agent-harness issueops phase --id "$ISSUEOPS_ID" --to plan $RECORD_ACTOR_FLAGS --json
     ```
     브랜치 이름은 `<issue-number>-<kebab-slug>` 형식이 강제된다(`branchprepare/branch_prepare.go:216`). GitHub Orca 경로는 원격 브랜치가 먼저 있으면 prepare가 실패하므로, Orca가 준비된 환경에서는 첫 `branch prepare`를 base SHA만으로 기록하고 `--link-verified`는 `execution prepare` 뒤로 미룬다(`skills/issueops/references/execution.md` "GitHub Orca Branch Ordering"이 그 순서를 소유한다). 어느 순서를 쓸지는 `orca status --json`이 아니라 그 문서를 읽고 정한다.
     delegated child 사이클(`issueops child start`로 만든 것)은 분기가 다르다. base branch는 부모 사이클의 브랜치이고 base SHA는 부모 워크트리의 HEAD이며, `branch prepare`에 `--parent-worktree "${SOURCE_ROOT}.worktrees/<부모 브랜치>"`를 반드시 넘긴다(`internal/adapter/issueops/execution_prepare.go:107-125`가 봉인하고 대조한다). `origin/$BASE_BRANCH` fetch는 독립 사이클 전용이라 child에서는 쓰지 않는다.
  5. `## 검증`: `issueops status --json`의 `branch_prepare`에 provider·issue_url·branch·base_branch·base_sha가 있고 phase가 `plan`이다. provider 링크가 실제로 보인다(GitHub은 `linkedBranches` GraphQL, GitLab은 `<iid>-` 접두 규칙과 `repository/branches/<branch>`). source checkout의 `git status --short`가 비어 있고 브랜치가 바뀌지 않았다. **`git worktree list`에 새 항목이 없다.**
  6. `## 출구`: "다음: issueops-plan. 계획은 워크트리가 아직 없으므로 source checkout에서 쓰고 `artifact stage --name plan`으로 올린다. 워크트리와 구현 세션은 3단계 끝의 `execution prepare --mode auto`가 만든다."
  7. `agents/openai.yaml`은 현행 branch-worktree의 것을 이름만 바꾸고, 워크트리를 만들지 않는다는 점과 `--mode auto`가 모드를 고른다는 점을 default_prompt에 넣는다.
  8. `git rm -r skills/issueops-branch-worktree`.

  **Must NOT do**: `git worktree add`, `execution prepare` 호출, `link-worktree` 호출, `gh issue develop`, `orca worktree create`.

  **Recommended Agent**: deep. provider 링크 절차와 Orca 브랜치 순서를 정확히 옮겨야 한다.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T11, T12 | Blocked By: T0, T0b

  **References**:
  - `skills/issueops-branch-worktree/SKILL.md` 1~4절과 7절(재사용), 5절 Orca 등록은 삭제한다(워크트리를 만들지 않으므로 등록할 대상이 없다).
  - `internal/adapter/issueops/branchprepare/branch_prepare.go:80-140,216` (검증 규칙, 브랜치 이름 규칙).
  - `internal/adapter/orca/client.go:284-333` (Orca가 브랜치와 워크트리를 직접 만든다).
  - `skills/issueops/references/execution.md` "GitHub Orca Branch Ordering", "Prepare".
  - `skills/issueops/references/worktree-context.md` "Branch And Canonical Worktree"(T11에서 원본 삭제).

  **Acceptance Criteria**:
  - [ ] `test ! -d skills/issueops-branch-worktree`.
  - [ ] `python3 scripts/validate-skill.py skills/issueops-prepare`와 `verify-skill-shell.py` 통과.
  - [ ] `rg -n "git worktree add|execution prepare|link-worktree|orca worktree create" skills/issueops-prepare/SKILL.md`의 모든 매치가 금지 문맥이다(`rg -n "만들지 않는다|호출하지 않는다|쓰지 않는다" skills/issueops-prepare/SKILL.md`가 4건 이상).
  - [ ] `rg -n "issueops-plan|--mode auto" skills/issueops-prepare/SKILL.md` 각 1건 이상.

  **QA Scenarios**:
  ```
  Scenario: 워크트리를 만들지 않는다
    Channel: bash
    Steps: rg -c "git worktree add" skills/issueops-prepare/SKILL.md
    Expected: 금지 문맥 1건뿐이고 절차 코드 블록에는 없다
    Evidence: .agent-harness/evidence/task-4-no-worktree.txt
  Scenario: 삭제 확인
    Channel: bash
    Steps: git ls-files skills/issueops-branch-worktree | wc -l
    Expected: 0
    Evidence: .agent-harness/evidence/task-4-deleted.txt
  ```

  **Commit**: YES | `feat(skill)!: replace issueops-branch-worktree with the issueops-prepare stage` | Files: 위 파일

- [x] **T5. `issueops-abandon` 신설 (일시 중단·폐기)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-5-paths.txt`(네 경로 25건), `task-5-shell.txt`(shell 검사기 exit 0). `rm -rf`·`gh pr close`는 나쁜 예 표 안에만 있다. 인수 체인은 복사하지 않고 시작 명령 하나(`execution replace --preview`)만 적었다.

  **Files:**
  - Create: `skills/issueops-abandon/SKILL.md`
  - Create: `skills/issueops-abandon/agents/openai.yaml`

  **What to do**:
  1. frontmatter description: "Leave an IssueOps cycle safely from any stage: pause by releasing the execution lease so another session or host can resume it, or abandon the cycle by closing its draft PR/MR, closing the issue, deleting the remote branch, and removing the worktree, local branch, and record through one fingerprinted cleanup abandon. Use when the user says '중단', '탈출', '이슈옵스 정리', '일시 중단', 'abandon the cycle', or when `issueops next` reports an abandon exit."
  2. 절: `## 이 스킬이 맞는지 확인`(`next` 실행; 어느 stage든 진입 가능하지만 `exits`와 `lease`를 읽는다) → `## 두 경로`(설계 요약 4의 표에서 일시 중단·폐기 행) → `## 일시 중단` → `## 재개·인수는 라우터가 안내한다`(한 문단: `resume`·`takeover`는 라우터 `## 단계 표`의 행대로 `next_command` 체인을 따른다. 홀더가 죽은 사이클을 폐기하려면 먼저 그 체인을 완주해 lease를 released나 self active로 만든다) → `## 폐기` → `## dirty 워크트리 선택지` → `## 원격 정리 선택지` → `## 나쁜 예` → `## 검증`.
  3. `## 일시 중단` 코드 블록:
     ```bash
     agent-harness issueops execution status --id "$ISSUEOPS_ID" --json   # generation 확인
     # WIP 처리: atomic-commit-push 로 커밋·푸시하거나, 사용자가 명시하면 폐기
     cd "$WORKTREE" && agent-harness issueops execution release --id "$ISSUEOPS_ID" --generation "$GENERATION" $ACTOR_FLAGS --json
     ```
     cwd가 canonical worktree와 정확히 같아야 한다는 문장을 붙인다.
  4. `## 재개·인수는 라우터가 안내한다`: 절차를 복사하지 않는다. "사용자의 '그 세션은 껐다'는 quiescence 증거가 아니다"와 "호스트가 달라도 같은 명령이며 `execution whoami --json`의 `claim_actor_flags`를 쓴다" 두 문장만 남기고 라우터 `## 단계 표`를 링크한다.
  5. `## 폐기` 코드 블록:
     ```bash
     # 원격 효과는 사용자가 번호로 고른 것만 플래그로 넣는다(--close-pr, --close-issue, --delete-remote-branch).
     agent-harness issueops cleanup abandon --id "$ISSUEOPS_ID" --reason "$REASON" \
       --close-pr --close-issue --delete-remote-branch --preview --json
     # preview의 remote_effects, worktree, branch, fingerprint를 사용자에게 확인한 뒤 돌려준 next_command 그대로:
     agent-harness issueops cleanup abandon --id "$ISSUEOPS_ID" --reason "$REASON" \
       --close-pr --close-issue --delete-remote-branch --apply --confirm --fingerprint "$FP" --json
     ```
     abandon 게이트 표(reason_required, lease_terminal, remote_artifact_unmerged, no_children, worktree_clean 계열, pending_intent_safe, orca_resources_absent)와 각 해소 명령을 표로 둔다. `--reason`에 금지 문자(`"'\`$\|&;<>()*?~`)를 넣지 않는다. apply는 원격 효과를 record 삭제보다 먼저 실행하므로, record 삭제 뒤에 `--id` 명령으로 원격을 정리하려 하지 않는다. 머지된 artifact는 게이트 ④가 막으므로 그때는 `issueops-cleanup`이다.
  6. `## dirty 워크트리 선택지`: 번호 세 개(WIP 커밋·푸시 후 폐기 / 변경 폐기 후 폐기(별도 확인) / 일시 중단으로 전환).
  7. `## 원격 정리 선택지`: preview 전에 사용자에게 번호로 묻는다. draft PR/MR 닫기(있을 때), 이슈 닫기 또는 열어 두기(재시작 예정이면 열어 둔다), 원격 브랜치 삭제 또는 유지. 고른 것만 플래그로 넣는다. `remote close-issue`와 `cleanup remote-branch`는 머지 증적을 요구하므로 여기서 쓰지 않는다(2026-09-04 실측: 무artifact 사이클에서 각각 `cannot verify merge evidence before a verified remote artifact`, `phase_done` missing).
  8. `## 나쁜 예`: 홀더가 살아 있는데 abandon, raw `gh pr close`, 워크트리 `rm -rf`, `--reason`에 shell 문자, cleanup 대신 abandon으로 머지된 사이클 정리, record 삭제 뒤 `remote close-issue`·`cleanup remote-branch` 시도.
  9. 현행 `skills/issueops/references/cleanup-state.md` "Retiring a cycle that never entered execution v1" 절 내용을 `## 폐기` 아래 소절로 옮긴다(T11에서 원본 삭제).

  **Must NOT do**: 머지된 사이클 정리(그건 `issueops-cleanup`), 원격 브랜치 자동 삭제, 확인 없는 `--confirm`.

  **Recommended Agent**: deep. lease 회복 체인과 abandon 게이트를 정확히 적어야 한다.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10, T11 | Blocked By: T0, T23

  **References**:
  - 설계 요약 5(`cleanup abandon` 원격 효과의 플래그·순서·게이트), T7.
  - `skills/issueops/references/execution.md` "Release, Replacement, And Reconciliation" 절(체인 원문).
  - `internal/adapter/issueops/issueops_cleanup_abandon.go:227-275,400-460` (게이트, 부분 실패 재시도).
  - `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go:609-640` (미머지 관측).
  - `skills/issueops-cleanup/SKILL.md` "Apply in fail-closed order"(preview→apply 문체 참고).
  - `skills/issueops-implement/SKILL.md` "회복은 next_command 체인만" 표.

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-abandon`와 `verify-skill-shell.py` 통과.
  - [ ] `rg -n "\-\-close-pr|\-\-close-issue|\-\-delete-remote-branch|cleanup abandon|execution release|replace --preview" skills/issueops-abandon/SKILL.md` 각 1건 이상, `rg -n "remote close-pr" skills/issueops-abandon/SKILL.md` 0건.
  - [ ] `rg -n "rm -rf|gh pr close|glab mr close" skills/issueops-abandon/SKILL.md`가 "나쁜 예" 표 안에만 있다(`rg -n "나쁜 예" -A 12 skills/issueops-abandon/SKILL.md`로 확인).

  **QA Scenarios**:
  ```
  Scenario: 세 경로가 모두 표에 있음
    Channel: bash
    Steps: rg -c "일시 중단|재개|인수|폐기" skills/issueops-abandon/SKILL.md
    Expected: 4 이상
    Evidence: .agent-harness/evidence/task-5-paths.txt
  Scenario: shell 검사기가 파괴 명령을 잡지 않음
    Channel: bash
    Steps: python3 scripts/verify-skill-shell.py skills/issueops-abandon; echo exit=$?
    Expected: exit=0
    Evidence: .agent-harness/evidence/task-5-shell.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-abandon for pause, takeover, and abandon` | Files: 위 2개

- [x] **T6. `issueops next` application + inbound + wiring + CLI + catalog + goldens + architecture ratchet + local readiness 분리** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-6-next-source.json`, `task-6-next-worktree.json`, `task-6-next-other.json`.

  **실행 결과와 계획 대비 편차**:
  - **T1과 한 커밋으로 묶었다.** `internal/architecture/orphan_package_test.go`의 `TestProductionPackagesHaveImporters`는 import되지 않는 프로덕션 패키지를 실패로 본다. T1만 커밋하면 `internal/domain/issueopsnext`가 고아가 되어 그 커밋에서 CI가 빨갛다. 두 태스크는 배선까지 가야 초록이다.
  - **`IssueOpsStrictPRReadiness`가 local을 호출하는 대신, 둘 다 `issueOpsObservedPRReadiness(record, syncUpstream bool)` 한 본체를 부른다.** 감싸는 형태로 만들면 strict가 gitRoot·branch·status·upstream 관측을 두 번 실행한다. 게이트 판정 결과는 계획과 같고(`local` = strict − fetch − `upstream_fetch`/`upstream_synced`), 기존 strict 테스트 9개가 그대로 통과한다.
  - **application 계층은 `path/filepath`를 import할 수 없다**(`TestProductionGraphHasNoLegacyAdapterEdges`의 `application_must_not_import_implementation`). 경로 정규화를 `Ports.CleanPath`로 주입하고 composition root가 `filepath.Clean`을 꽂는다.
  - **저장소 밖 판정을 추가했다.** `CleanPath.Normalize`는 git이 실패해도 준 경로를 그대로 돌려주므로 `/tmp`도 source root처럼 보였다. `WorktreeState(cwd)`로 저장소 여부를 먼저 보고, 아니면 `cwd_role=other` + warning `not a git repository`로 끊는다. 목록 조회도 건너뛴다 — repo 필터가 빈 값이면 모든 사이클이 후보가 되기 때문이다.
  - `ProcessLive` 포트는 `*bool`을 돌려준다(관측 실패는 nil). 구현은 `ObserveNativeProcessReceipt`가 아니라 PID 재사용까지 판정하는 `InspectNativeProcessReceipt`를 쓴다.
  - 실기 검증 대상 워크트리가 바뀌었다. 계획이 지목한 `api-servers.worktrees/2900-...`는 사라졌다. `2899-first-and-second-round-reward`(io-63b7ffe020d9, pr phase)로 대체했고, 다른 세션이 lease를 쥔 상태라 `blocked.holder_live`/index 9로 분류됐다.
  - `issueops --help`는 stderr로 출력한다. 수용 기준의 grep은 `2>&1`이 필요하다(기존 동작이며 이 태스크가 바꾸지 않았다).

  **Files:**
  - Create: `internal/application/issueopsnext/ports.go`, `internal/application/issueopsnext/service.go`
  - Create: `internal/adapter/inbound/issueopsnext/next.go`
  - Modify: `internal/adapter/issueops/issueops_pr_readiness_strict.go` (`IssueOpsLocalPRReadiness`를 분리하고 `IssueOpsStrictPRReadiness`가 그것을 호출한 뒤 fetch·동기화 판정만 더하도록)
  - Modify: `internal/adapter/issueops/execution_lease.go:334` (`executionWriterAbsentRecoveryCommand`를 한 줄 exported wrapper `ExecutionWriterAbsentRecoveryCommand`로 노출. 본체는 옮기지 않는다)
  - Test: `internal/adapter/issueops/issueops_pr_readiness_local_test.go` (fetch 미호출 단언)
  - Create: `cmd/harness/harnessapp/issueops_next_wiring.go`
  - Create: `cmd/harness/issueopscli/issueops_next.go`
  - Modify: `cmd/harness/issueopscli/issueops.go:26-61` (dispatch map에 `"next": runIssueOpsNext`)
  - Modify: `cmd/harness/issueopscli/issueops_dependencies.go` (`IssueOpsNext func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error)` 필드와 not-configured 기본값)
  - Modify: `cmd/harness/harnessapp/issueopscli_runtime_wiring.go` (`IssueOpsNext: issueOpsNextHandler(observer)`)
  - Modify: `internal/domain/cli/issueops_catalog.go` (`list` 줄 다음에 `  agent-harness issueops next [--id ID] [--cwd PATH] [--json]`, `abridgedIssueOpsMainKeys`에 `"next"`)
  - Modify: `cmd/harness/contractcli/contract.go:130` 옆에 `"issueops_next": {"ok","generated_at","cwd","cwd_role","source_root","stage","lease","missing","next_command","next_command_kind","exits","review","warnings"}`
  - Create: `internal/architecture/issueops_next_vertical_test.go`
  - Test: `internal/application/issueopsnext/service_test.go`, `cmd/harness/issueopscli/issueops_next_test.go`
  - Regenerate: `cmd/harness/testdata/usage.golden.txt`, `cmd/harness/testdata/response_contracts.golden.json`

  **Interfaces:**
  - Consumes: T1의 `Classify`, `OwnerCommand`.
  - Produces: CLI `agent-harness issueops next`, 핸들러 `func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error)`.

  **What to do**:
  1. ports(함수 타입으로 두어 composition root가 현행 `issueopscore` 함수를 주입한다):
     ```go
     package issueopsnext

     type Ports struct {
         ListCycles     func(ctx context.Context, stateRoot, repo string) (issueopsinventorycontract.ListResult, error)
         ReadRecord     func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
         Completion     func(record issueopscontract.IssueOpsRecord, phase issueopscontract.IssueOpsPhase) issueopscontract.IssueOpsReadiness
         LocalReadiness  func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness // fetch 없음
         WriterlessCommand func(record issueopscontract.IssueOpsRecord) string                             // execution_lease.go:334-356
         PlannerDefaults func(host string) (model, effort string, ok bool)                                 // internal/port/orca.go IssueOpsPlannerDefaults
         StagedArtifacts func(stateRoot, id string) ([]string, error)                                       // issueops artifact stage 목록
         Actor          func() (host, sessionID string, err error)            // whoami identity
         ProcessLive    func(receipt issueopscontract.NativeProcessReceipt) bool
         SourceRoot     func(cwd string) string                                 // git common dir 기반
         WorktreeState  func(root string) (present bool, branch, head string)   // rev-parse --show-toplevel / branch --show-current / rev-parse HEAD
         CurrentBranch  func(cwd string) string
         Env            func(key string) string
         Now            func() time.Time
     }
     ```
  2. 실패 테스트(application): 가짜 Ports로 세 케이스를 쓴다. (a) 사이클 없음 → `Stage.Key=="none"`, `NextCommand`에 `issueops start --repo <source_root>` 포함. (b) 후보 두 개, cwd가 그중 하나의 `workspace_root` → 그 사이클 선택. (c) 후보 두 개, cwd가 source root, 환경변수 `ISSUEOPS_ID` 없음, active 사이클 둘 → `Stage.Key=="ambiguous"`, `Candidates` 2개, `ok:true`.
  3. RED 확인 후 `Service.Next(ctx, stateRoot, cwd, id)`를 구현한다. 선택 우선순위는 설계 요약 3의 2번 그대로. `Input.HolderLive`는 holder의 `Process` receipt가 있을 때만 `ProcessLive`를 호출해 채운다. `Local`은 phase가 `ai-slop-clean`, `feedback`일 때만 채운다. `done`은 관측 없이 `done` 단계로 분류하고 warning `merge state requires provider readback`을 붙인다. `StagedPlan`은 `StagedArtifacts`에 `plan`이 있는지로 채운다. `Review`는 `PlannerDefaults(actor host)`로 채운다. `record.repo`는 `start`가 이미 정규화하므로 비교하지 않는다.
  4. inbound `NewNextHandler(service)`는 `issueopsinventory/list.go`와 같은 모양이다.
  5. 관측은 별도 outbound 패키지 없이 harnessapp wiring의 클로저로 주입한다. `SourceRoot`는 `internal/adapter/outbound/issueopsinventory/runtime.go:15-27`의 `CleanPath.Normalize`를 그대로 호출하고(중복 구현 금지), `WorktreeState`는 `issueopscore.GitOut`으로 `rev-parse --show-toplevel`·`branch --show-current`·`rev-parse HEAD`를 읽으며, `ProcessLive`는 `issueopscore.ObserveNativeProcessReceipt`가 오류 없이 돌아오면 true다. vertical 패키지(contract, domain, application, inbound)는 `internal/adapter/issueops`를 import하지 않는다.
  6. harnessapp `issueOpsNextHandler(observers...)`: `issueOpsInventoryListHandler`와 `issueOpsStatusHandler`를 본떠 Ports를 채운다. `Completion: issueopscore.IssueOpsPhaseCompletion`, `LocalReadiness: issueopscore.IssueOpsLocalPRReadiness`, `WriterlessCommand: issueopscore.ExecutionWriterAbsentRecoveryCommand`, `PlannerDefaults: port.IssueOpsPlannerDefaults`, `StagedArtifacts: issueopscore.StagedIssueOpsArtifactNames`, `Actor`는 `executioncmd`의 `nativeSessionIdentityFromEnv`와 같은 환경변수 규칙을 쓰는 exported 함수를 `executioncmd`에 추가해 호출한다(`ResolveNativeSessionIdentity(getenv) (host, session, source string, err error)`).
  7. CLI `runIssueOpsNext`: 플래그 `--id`, `--cwd`(기본 `os.Getwd()`), `--json`. 텍스트 출력 형식:
     ```text
     stage 3/10 plan.review  cycle io-xxxx  phase plan  lease active(gen 1, self)
     missing: devils_advocate_review
     next: agent-harness issueops devils-advocate review --id io-xxxx --reviewer-context subagent ...
     exits: pause=agent-harness issueops execution release --id io-xxxx --generation 1 ...  abandon=agent-harness issueops cleanup abandon --id io-xxxx --reason <TEXT> --preview
     ```
  8. CLI 테스트: `issueops_dispatch_registry_test.go`의 `list` 케이스를 본떠 `runIssueOps([]string{"next","--json"})`가 not-configured deps에서 `errIssueOpsCLINotConfigured`를 돌려주고, 가짜 deps를 주입하면 `"stage"` 키가 출력에 있음을 확인한다.
  9. catalog 줄과 abridged 키를 추가하고 `go test ./internal/domain/cli ./cmd/harness/issueopscli -run 'Usage|Catalog' -count=1`로 parity 테스트를 통과시킨다.
  10. 골든 재생성: `go test ./cmd/harness/contractgolden -run Golden -update -count=1 && go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update -count=1`. diff에 `issueops next` 줄과 `issueops_next` 필드만 추가됐는지 `git diff --stat cmd/harness/testdata`로 확인한다.
  11. architecture ratchet: `issueops_inventory_vertical_test.go`를 복사해 패키지 4개(contract, domain, application, inbound) 존재와 vertical 패키지의 `internal/adapter/issueops` import 금지를 검사한다.
  13. local readiness 분리: `issueops_pr_readiness_strict.go`를 두 함수로 나눈다. `IssueOpsLocalPRReadiness(record)`는 현행 strict에서 `git fetch --quiet`와 `upstream_synced`·`upstream_fetch` 판정만 뺀 것이고, `IssueOpsStrictPRReadiness`는 local을 호출한 뒤 그 둘을 더한다. 기존 strict 테스트는 바뀌지 않고 통과해야 하며, 새 local 테스트는 `GitCmd` 스텁으로 `fetch`가 호출되지 않음을 단언한다.
  14. `execution_lease.go:334`의 `executionWriterAbsentRecoveryCommand`를 exported wrapper `ExecutionWriterAbsentRecoveryCommand(record) string`로 노출한다. 본체를 옮기거나 복사하지 않는다.
  12. 실기: `go build -o bin/agent-harness ./cmd/harness && ./bin/agent-harness issueops next --json`이 이 저장소에서 `"key": "none"`을 출력하고, `--cwd /Users/habin/workspace/api-servers.worktrees/2900-invite-push-and-round-visibility`로 실행하면 `selected.id`가 `io-5f169ca7b558`이고 `stage.index`가 3이다(2026-09-04 기준 그 사이클은 implement phase다. 사라졌으면 다른 active 사이클로 대체하고 evidence에 적는다).

  **Must NOT do**: record 쓰기, `execution status`처럼 lease를 갱신하는 호출, MCP 도구 추가, `internal/adapter/issueops`를 vertical 패키지에서 import, `git fetch`·provider·Orca 호출, Go 안의 스킬 이름·한국어 UI 문자열.

  **Recommended Agent**: deep. 다섯 층과 골든·parity 테스트를 함께 맞춰야 한다.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T7, T11, T15 | Blocked By: T1

  **References**:
  - Pattern: `internal/application/issueopsinventory/service.go`, `ports.go`; `internal/adapter/inbound/issueopsinventory/list.go`; `cmd/harness/harnessapp/issueops_inventory_wiring.go`, `issueops_status_wiring.go`; `internal/architecture/issueops_inventory_vertical_test.go`.
  - CLI: `cmd/harness/issueopscli/issueops.go:26-61`, `issueops_subcommands.go:510-540`(list 출력 형식), `issueops_dependencies.go:25-60,215-235`, `issueops_dispatch_registry_test.go`.
  - Catalog: `internal/domain/cli/issueops_catalog.go`(주석: 새 명령은 여기 한 곳), `issueops_catalog_test.go`, `cmd/harness/issueopscli/issueops_usage_parity_test.go`.
  - Contract schema: `cmd/harness/contractcli/contract.go:130`.
  - Golden 절차: `.agent-harness/CAUTIONS.md` Update workflow 5, `cmd/harness/contractgolden/contract_golden_test.go:16`.
  - whoami 환경 규칙: `cmd/harness/issueopscli/executioncmd/execution.go:253-257,350-369`.
  - liveness: `internal/adapter/issueops/execution_process.go:45-54`.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/application/issueopsnext ./internal/adapter/inbound/issueopsnext ./internal/adapter/issueops ./cmd/harness/issueopscli ./internal/domain/cli ./internal/architecture -count=1` 통과.
  - [ ] `go test ./internal/adapter/issueops -run 'LocalPRReadiness|StrictPRReadiness' -count=1 -v | grep -c '^--- FAIL'`이 0.
  - [ ] `rg -n "\"fetch\"|gh |glab |issueops-" internal/contract/issueopsnext internal/domain/issueopsnext internal/application/issueopsnext internal/adapter/inbound/issueopsnext` 0건.
  - [ ] `go test ./cmd/harness/contractgolden -run Golden -count=1 && go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1` 통과(재생성 뒤).
  - [ ] `./bin/agent-harness issueops next --json | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['ok'] and d['stage']['key']=='none' and d['exits']['abandon_command']==''"` 종료 0(사이클이 없으면 abandon 명령도 비어 있다).
  - [ ] `./bin/agent-harness issueops --help | grep -c 'issueops next'`가 1.

  **QA Scenarios**:
  ```
  Scenario: 워크트리에서 실행하면 그 사이클을 고른다
    Channel: bash
    Steps: ./bin/agent-harness issueops next --cwd <existing worktree path> --json
    Expected: selected.workspace_root == 인자 경로, cwd_role == "worktree", stage.index >= 3
    Evidence: .agent-harness/evidence/task-6-next-worktree.json
  Scenario: 무관한 디렉터리
    Channel: bash
    Steps: ./bin/agent-harness issueops next --cwd /tmp --json; echo exit=$?
    Expected: exit=0, ok true, stage.key "none", cwd_role "other", warnings에 "not a git repository" 문구
    Evidence: .agent-harness/evidence/task-6-next-other.json
  ```

  **Commit**: YES | `feat(issueops): add the read-only next command that classifies the current stage` | Files: 위 파일 + 골든 2개

- [x] **T7. `cleanup abandon` 원격 효과 확장 (`--close-pr`, `--close-issue`, `--delete-remote-branch`)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-7-abandon-preview.json`(실기 preview, GitLab 이슈 상태 closed 관측, mutation 없음), `task-7-abandon-noflag-diff.txt`(플래그 없는 출력 무변경 + 머지 거부).

  **실행 결과와 계획 대비 편차**:
  - **관측한 원격 *상태*는 fingerprint에 넣지 않았다.** 계획 3번은 `RemoteArtifactState`·`IssueState`를 인벤토리에 넣으라고 했지만, 인벤토리는 곧 fingerprint 입력이고 `CleanupAbandonRequest.ArtifactUnmerged`의 주석이 "네트워크 관측을 인벤토리에 섞으면 일시적 원격 오류가 preview 재발급 루프를 만든다"는 규율을 이미 세워 두었다. fingerprint에는 **플래그 세 개**와 `RemoteBranchOID`만 넣는다. 플래그는 결정적 입력이라 승인을 구분하고(요구 (a) 충족), OID는 `--force-with-lease` CAS의 대상이라 승인의 일부다(`cleanup remote-branch` 선례). 상태 변화는 apply가 단계마다 재관측해 잡는다.
  - **provider는 CLI가 해석해 어댑터로 넘긴다**(`CleanupFinish`·`CleanupRemoteBranch`와 같은 모양). `CleanupDeps.CleanupAbandon` 시그니처에 `prov port.IssueProvider`가 붙었고 fake 셋을 함께 고쳤다. 플래그가 없으면 provider를 해석조차 하지 않으므로 원격 정체가 없는 사이클의 폐기가 막히지 않는다.
  - **상태 관측기를 새로 만들지 않았다.** 두 provider가 이미 `IssueProviderArtifactBodyReader`로 같은 readback을 노출하므로 그것을 쓴다.
  - preview가 다른 게이트(예: `no_children`)에 막혀도 관측한 원격 상태는 그대로 보여 준다. `ok:false`와 missing이 "지금은 실행되지 않는다"를 말하고, 관측값은 무엇을 먼저 정리해야 하는지 알려 준다. 원격 관측 자체가 결격일 때만 계획을 비운다.
  - **T16 배터리가 누락 하나를 잡았다(2026-09-05).** `internal/domain/commandparse/issueops.go`의 `cleanup abandon` 스펙에 새 플래그 세 개를 넣지 않아 `TestIssueOpsCommandSpecAcceptsEveryCatalogAdvertisedFlag`가 실패했다. 카탈로그·CLI·정책 파서 셋이 같은 플래그 집합을 봐야 한다. T16에서 고쳤다.
  - 실기 대상은 `io-15f1518189ca`였고 게이트 `no_children`에 막혔다. 그래도 이 태스크가 검증하려던 것은 전부 관측됐다: `--close-issue` 플래그가 GitLab 이슈를 실제로 읽어 `issue_state: closed`를 돌려주고, `remote_effects`가 `close_issue:already_closed`로 계획되며, mutation은 없었다.

  **Files:**
  - Modify: `internal/contract/issueops/types.go` (`IssueOpsCleanupAbandonRequest`에 `ClosePR`, `CloseIssue`, `DeleteRemoteBranch bool`; `CleanupAbandonResult`에 `RemoteEffects []string`, `RemoteArtifactState`, `IssueState`, `RemoteBranchOID`, `PRClosed`, `IssueClosed`, `RemoteBranchDeleted`. record 구조체에는 필드를 추가하지 않는다)
  - Modify: `internal/adapter/issueops/issueops_cleanup_abandon.go` (inventory에 원격 관측 추가, fingerprint 입력 확장, apply 단계 `close_pr` → `close_issue` → `remote_branch_delete`를 로컬 단계 앞에 삽입, `failed_step` 값 세 개 추가)
  - Modify: `internal/adapter/issueops/issueops_cleanup_remote_branch.go:95-110` (`git push origin --delete` 단계를 `deleteRemoteBranchRef(ctx, git, repo, ref, expectedOID)` helper로 추출해 두 명령이 공유)
  - Modify: `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go:540-600` (플래그 3개, `Deps`에 provider resolver와 artifact·issue state observer 주입, preview 출력에 원격 효과 표시)
  - Modify: `cmd/harness/harnessapp/issueops_cleanup_wiring.go` (provider와 observer 주입)
  - Modify: `internal/domain/cli/issueops_catalog.go` (`cleanup abandon` 줄을 설계 요약 5의 usage로 교체)
  - Test: `internal/adapter/issueops/issueops_cleanup_abandon_remote_test.go`, `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup_abandon_remote_test.go`
  - Regenerate: 골든 2개

  **What to do**:
  1. 실패 테스트(adapter): (a) `ClosePR=true`이고 artifact가 open이면 preview `RemoteEffects`에 `close_pr`가 있고 fingerprint가 플래그 없는 preview와 다르다. (b) apply는 `close_pr`, `close_issue`, `remote_branch_delete`, 로컬 단계 순으로 호출되며 provider fake의 호출 순서를 단언한다. (c) provider가 이미 closed를 돌려주면 그 단계는 건너뛰고 apply는 계속된다. (d) `remote_branch_delete` 직전 관측 OID가 preview OID와 다르면 `failed_step=remote_branch_delete`로 멈추고 record가 남는다. (e) artifact가 merged면 플래그와 무관하게 게이트 ④ `remote_artifact_unmerged`가 막는다. (f) 플래그가 없으면 결과 JSON과 fingerprint가 현행과 byte-identical이다(회귀 방지).
  2. RED 확인: `go test ./internal/adapter/issueops -run CleanupAbandonRemote -count=1`.
  3. inventory 확장: `cleanupAbandonInventory`에 `RemoteArtifactState`(`open|closed|merged|absent`), `IssueState`(`opened|closed|absent`), `RemoteBranchPresent`, `RemoteBranchOID`를 추가하고, 요청된 플래그에 해당하는 값만 관측한다(플래그가 없으면 provider를 호출하지 않는다). artifact 관측은 `feedback_cleanup.go:609-637`의 `ObserveArtifactMerged` 주입점을 확장하고, issue state는 provider의 close-issue 사전 readback 함수를 재사용한다.
  4. apply 단계를 로컬 단계 앞에 삽입한다. 각 단계는 멱등이고 readback으로 확인하며, 실패는 `recordCleanupAbandonFailure`의 기존 경로로 기록한다. `close_issue`는 T2의 `CloseIssue`를 `Reason: "not_planned"`로 호출한다.
  5. CLI: 플래그 3개를 추가하고 `--preview` 출력에 `remote_effects`와 관측값을 넣는다. `next_command`는 같은 플래그를 포함해 렌더한다.
  6. usage parity 테스트와 골든을 갱신한다(T6 10번과 같은 절차).
  7. 실기: 이 머신의 미머지·무artifact 사이클 하나(예: `io-15f1518189ca`)에 `cleanup abandon --id <id> --reason probe --close-issue --preview --json`을 실행해 `remote_effects`에 `close_issue`와 issue state가 보이고 mutation이 없음을 evidence로 남긴다. apply는 실행하지 않는다.

  **Must NOT do**: 머지된 artifact의 abandon 허용, record 필드 추가, `cleanup remote-branch`·`remote close-issue`의 머지 게이트 완화, 별도 `remote close-pr` 명령·`ClosedAt` 필드.

  **Recommended Agent**: deep. fingerprint 입력, apply 순서, 부분 실패 복구가 얽힌다.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T15 | Blocked By: T2, T6(카탈로그·골든을 T6 뒤에 고친다)

  **References**:
  - `internal/adapter/issueops/issueops_cleanup_abandon.go:84-190,227-275,400-470` (현행 흐름, 게이트, 부분 실패 복구).
  - `internal/adapter/issueops/issueops_cleanup_remote_branch.go:95-110` (원격 브랜치 삭제 단계), `:145-180,240-260` (머지 게이트, 이 명령을 재사용하지 않는 이유).
  - `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go:540-640` (플래그, deps, `cleanupArtifactUnmerged`).
  - `internal/adapter/provider/github/provider.go:705-740`, `internal/adapter/provider/gitlab/provider_issue.go:62-95` (issue close readback), T2의 `ClosePullRequest`·`CloseIssue.Reason`.
  - 골든 절차: T6 10번.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapter/issueops -run 'CleanupAbandon' -count=1`, `go test ./cmd/harness/issueopscli/feedbackcleanup -count=1` 통과.
  - [ ] `./bin/agent-harness issueops --help | grep -c -- '--close-pr'`가 1.
  - [ ] 플래그 없는 preview의 JSON이 변경 전 바이너리의 출력과 같다(evidence에 diff 0).

  **QA Scenarios**:
  ```
  Scenario: 플래그 preview는 mutation 없음
    Channel: bash
    Steps: cleanup abandon --id <미머지 id> --reason probe --close-issue --preview --json; provider에서 issue state를 다시 읽는다
    Expected: remote_effects에 close_issue, issue state 변화 없음
    Evidence: .agent-harness/evidence/task-7-abandon-preview.json
  Scenario: 머지된 artifact는 플래그와 무관하게 거부
    Channel: bash
    Steps: 테스트 fixture(artifact merged) + --close-pr --preview
    Expected: missing에 remote_artifact_unmerged, remote_effects 비어 있음
    Evidence: .agent-harness/evidence/task-7-abandon-merged.txt
  ```

  **Commit**: YES | `feat(issueops): let cleanup abandon close the draft PR, the issue, and the remote branch` | Files: 위 파일 + 골든 2개

- [x] **T8. `issueops-plan` 신설 (3단계: 문서 확인·계획·검토·인계)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-8-handoff.txt`(인계 절에 `resolved_mode`·`--mode auto` 3건), `task-8-staging.txt`.

  **실행 결과와 계획 대비 편차**: 수용 기준의 `git worktree add` 0건은 지키지 않았다. 두 건 모두 금지 문맥이다(안전 규칙의 "실행하지 않는다"와 나쁜 예 표의 행). 하지 말아야 할 일을 이름으로 말하지 않으면 읽는 쪽이 무엇을 피해야 하는지 모른다. `--mode direct`도 같은 이유로 금지 문맥 두 건만 남겼다(수용 기준이 허용한 형태).

  **Files:**
  - Create: `skills/issueops-plan/SKILL.md`
  - Create: `skills/issueops-plan/agents/openai.yaml`

  **What to do**:
  1. frontmatter description: "Turn a branch-prepared IssueOps cycle into an implementable contract from the source checkout: read the operating documents that constrain the change, write the plan, stage it as the sealed plan artifact, approve the design review, run the brooks devil's-advocate loop until it passes, then hand off with `execution prepare --mode auto` so Orca launches the implementation session when it is available and direct keeps the current session when it is not. Use when `issueops next` reports plan.write, plan.design, plan.review, or plan.handoff, or when the user says '계획 세워줘', '계획 검토해줘', '구현 인계'."
  2. 절: `## 이 스킬이 맞는지 확인` → `## 어디에서 실행하는가` → `## 프로젝트 문서 확인` → `## 계획 작성과 스테이징` → `## 게이트 원장` → `## 설계 검토` → `## 검토 루프` → `## 인계` → `## 의존성과 로컬 설정`(현행 `worktree-context.md` "Dependencies And Local Configuration" 그대로) → `## 나쁜 예` → `## 검증`.
  3. `## 어디에서 실행하는가`: 이 단계는 **source checkout의 준비 세션**이 수행한다. 워크트리는 아직 없다 — 만드는 것은 3단계 끝의 `execution prepare`다. 그래서 계획 파일은 source checkout 밖의 임시 파일에 쓰고 `artifact stage`로 올린다. source checkout 안에 계획을 만들어 커밋 대상으로 만들지 않는다.
  4. `## 프로젝트 문서 확인`(계획 작성보다 먼저): 설계 요약 10의 "계획 전 확인"을 그대로 쓴다. MCP `project_docs_route`에 이슈 제목과 구현 범위를 넣어 문서를 고르고(없으면 `agent-harness docs --json`의 required-doc 목록), `project_docs_read`로 CONSTITUTION, ARCHITECTURE(해당 모듈), CONVENTIONS, CAUTIONS(색인과 해당 모듈), ADR(관련 결정), TESTING을 읽는다. 결과는 plan의 `## 적용되는 결정과 주의사항` 절에 "문서 경로, 항목 제목, 이 계획에 미치는 제약 한 문장"으로 적는다. 적용 항목이 없으면 "대조했으나 없음"과 대조한 문서 목록을 적는다. 이 절은 design review의 `--risk`와 `issueops-review`의 plan 리뷰 프롬프트에 들어간다.
  5. `## 계획 작성과 스테이징`: `von-neumann`을 호출해 계획을 쓴다. 저장 위치는 `$(mktemp -d)/plan.md`처럼 source checkout 밖이다. plan에는 필수 절 네 개를 둔다: `## 적용되는 결정과 주의사항`(4번의 결과), `## 재사용하는 기존 구현`(plan-prep의 codebase survey에서 찾은 심볼·패키지·테스트 헬퍼와 재사용 방식, 새로 만드는 것은 왜 기존 것으로 안 되는지), `## 성능 영향`(hot path 여부, 복잡도, 측정 계획, 알고리즘이 걸리면 `dijkstra`), `## 하위 호환성과 side effect`(CLI JSON·MCP schema·golden·record schema·provider body 계약, 기존 데이터, 롤백). 그다음 스테이징한다.
     ```bash
     agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file "$TMP_PLAN" --json
     ```
     `artifact stage`는 actor 플래그를 받지 않는다(`--id`, `--name`, `--file`, `--json`만). 잘못 올렸으면 `artifact unstage --id ID --name plan`. `link-plan`은 여기서 하지 않는다 — 워크트리가 없어 `plan_in_worktree`를 만족할 수 없고, prepare가 스테이징한 계획을 워크트리 안에 materialize한 뒤 4단계가 링크한다.
  6. `## 게이트 원장`: `gates-ledger`(T22)를 호출해 plan의 수용 기준을 `G1..Gn`(CHECK/EXPECT)으로 적는다. 원장 파일도 워크트리가 생긴 뒤에 만들어야 하므로, 3단계에서는 plan 안에 게이트 초안을 적어 두고 파일 생성은 4단계가 한다. 5단계가 `--write`로 EVIDENCE를 채우고 7단계가 읽기 전용으로 재검사한다.
  7. `## 설계 검토`: `design review --approved`. `--verification`에 "design"과 "review"가 함께 들어가야 `design_review_evidence`가 통과한다(한국어면 "설계"와 "검토"). refactor plan, alternative, risk가 각각 1개 이상 필요하고 open question은 0이어야 한다. `record-routing --phase plan --skill von-neumann`.
  8. `## 검토 루프`: `issueops-review`(T21)를 `--target plan`으로 호출한다. 이 절에는 입력(staged plan 전체), 종료 조건(pass, finding 1건 이상), stop일 때 `issueops regress --id ID --reason "<결론>"`으로 grill로 돌아가 조사·이슈 갱신 뒤 다시 `phase --to plan`하는 경로, 판정이 plan sha256에 묶여 이후 수정 시 `devils_advocate_review_stale`이 된다는 결과만 적는다. 루프 절차 자체는 복사하지 않는다.
  9. `## 인계` 코드 블록:
     ```bash
     agent-harness issueops execution whoami --json   # ACTOR_FLAGS 원문
     agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto \
       --owner-host "$HOST" $ACTOR_FLAGS --json        # preview
     # 출력의 next_command(--expected-readiness-fingerprint 포함)를 그대로 실행
     ```
     `--mode auto`가 모드를 고른다. Orca가 준비돼 있으면 Orca가 워크트리와 구현 세션을 만들고 lease는 claimable로 남는다. 그 세션이 자기 프롬프트의 봉인된 claim 명령으로 홀더가 되며, 이 세션의 3단계 임무는 거기서 끝난다. Orca가 없거나 준비되지 않았으면 direct로 내려가고 git이 워크트리를 만들며 이 세션이 곧바로 generation 1 홀더가 되므로 같은 세션이 4단계를 이어간다. 어느 쪽인지는 결과의 `resolved_mode`로 확인한다.
     preview가 `Orca prepare needs planner-owned records the owner cannot supply`로 막히면 planner 게이트가 덜 찬 것이다. 메시지가 명령을 함께 주므로 그 명령을 실행하고 preview를 다시 한다. `--mode direct`를 강제해 우회하지 않는다.
  10. `## 나쁜 예`: source checkout 안에 계획 파일을 만들기, `git worktree add`, `--mode direct` 강제, 리뷰 없이 `--verdict pass`, revise 뒤 `--waive`로 닫기, 판정 뒤 plan 수정 후 재검토 생략, staged plan 없이 인계, Orca 세션이 떴는데 이 세션이 계속 구현.

  **Must NOT do**: 구현 파일 수정, 워크트리 생성, TDD 절 포함(그건 implement), 리뷰 루프·원격 쓰기 절차의 복사, `--mode direct` 강제.

  **Recommended Agent**: deep.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T11 | Blocked By: T0b, T1, T21, T22

  **References**:
  - `skills/issueops-review/SKILL.md`(T21), `skills/gates-ledger/SKILL.md`(T22).
  - `skills/issueops/references/execution.md` "Prepare"(preview→confirm, fingerprint), "Artifact Staging And Sealing"(스테이징 규칙), "GitHub Orca Branch Ordering".
  - `skills/issueops/references/operational-start.md` "Execution v1" 절(스테이징 후 prepare 순서; T11에서 원본 삭제).
  - `skills/issueops/references/worktree-context.md` "Dependencies And Local Configuration"(옮긴 뒤 T11에서 원본 삭제).
  - `internal/contract/issueopspreparation/planner_gates.go`(막힐 때의 메시지와 명령).
  - `internal/adapter/issueops/intentdesign/intent_design.go:136-150`(design_review_evidence 판정).
  - `.agent-harness/adr/decisions/2026-08-28-issueops-devils-advocate-plan-binding.md` Decision 4·5.
  - `skills/von-neumann/SKILL.md`, `skills/brooks/SKILL.md` "Subagent-Only Mandate".
  - `AGENTS.md` §2 Simplicity First, §3 Surgical Changes.

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-plan`와 `verify-skill-shell.py` 통과.
  - [ ] `rg -n "\-\-mode auto" skills/issueops-plan/SKILL.md` 1건 이상, `rg -n "\-\-mode direct" skills/issueops-plan/SKILL.md`는 금지 문맥만.
  - [ ] `rg -n "artifact stage --id|issueops-review|gates-ledger|regress|design review|execution prepare" skills/issueops-plan/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "brooks를 fresh 서브에이전트로 실행|git worktree add" skills/issueops-plan/SKILL.md` 0건.
  - [ ] `rg -n "재사용하는 기존 구현|성능 영향|하위 호환성과 side effect|적용되는 결정과 주의사항|project_docs_route|project_docs_read" skills/issueops-plan/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "^## 프로젝트 문서 확인|^## 계획 작성과 스테이징|^## 인계" skills/issueops-plan/SKILL.md`가 이 순서로 출력된다.

  **QA Scenarios**:
  ```
  Scenario: 모드는 auto가 고른다
    Channel: bash
    Steps: sed -n '/## 인계/,/## 의존성/p' skills/issueops-plan/SKILL.md | rg -c "resolved_mode|--mode auto"
    Expected: 2 이상
    Evidence: .agent-harness/evidence/task-8-handoff.txt
  Scenario: 계획은 source checkout 밖에 쓴다
    Channel: bash
    Steps: rg -n "mktemp|source checkout 밖" skills/issueops-plan/SKILL.md
    Expected: 각 1건 이상
    Evidence: .agent-harness/evidence/task-8-staging.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-plan for the document check, plan, review, and handoff stage` | Files: 위 2개

- [x] **T9. `issueops-implement` 재작성 (4단계: 구현)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-9-exit.txt`, `task-9-start.txt`. 219줄(기준 220 이하), 금지 문자열 0건.

  **실행 결과와 계획 대비 편차**: frontmatter description도 함께 고쳤다. 계획은 `agents/openai.yaml`만 지목했지만, 스킬을 고르는 것은 frontmatter description이고 그것이 여전히 "pre-publication implementation review gate"를 이 스킬의 일로 말하고 있었다.

  **Files:**
  - Modify: `skills/issueops-implement/SKILL.md`
  - Modify: `skills/issueops-implement/agents/openai.yaml`

  **What to do**:
  1. `## 시작 게이트`를 다시 쓴다: 첫 줄은 `agent-harness issueops next --json`. `stage.key`가 `implement.enter` 또는 `implement`일 때 진행한다. `claim`이면 이 세션이 Orca가 띄운 구현 세션인지 확인하고, 맞으면 자기 프롬프트의 봉인된 `execution claim --claim-current-token`을 정확히 한 번 실행한 뒤 `next`를 다시 돌린다. `plan.*`이면 `issueops-plan`으로, `clean`이면 `issueops-clean`으로, 나머지는 라우터 `## 단계 표`의 스킬로 안내한다. "record가 없으면 `issueops`를 실행한다" 문단은 "`next`가 `none`이면 `issueops-create-issue`"로 바꾼다.
  1b. `## 진입 절차`(새 절, `implement.enter`가 가리키는 것): `link-plan --plan-path <워크트리 안의 materialized plan>`(prepare가 스테이징한 계획을 워크트리에 풀어 두고 `plan_path`를 채웠으면 생략), `compatibility review --approved`(backward compatibility, side effect, rollback, verification, blocker 없음), `gates-ledger`로 `.agent-harness/issues/<n>/gates.md` 생성, `phase --to implement`. Orca 세션이든 direct 세션이든 이 절차는 같다.
  2. `## 구현 루프`, `## Lease fencing`, `## 회복은 next_command 체인만`(표는 유지하되 "holder 교체·회수" 행을 `issueops-abandon` 참조로 바꾼다), `## Child 위임`은 유지한다. 라우터에서 삭제되는 "Child와 delegation" 문단(세 조건, verdict 셋)을 `## Child 위임` 앞에 합친다. `## Publication evidence gates`와 `## Implementation review gate`는 이 스킬에서 삭제한다(T20 `issueops-verify`와 T21 `issueops-review`가 소유한다). `## 구현 루프`에는 RED/GREEN 증거를 `gates-ledger`(T22)로 `.agent-harness/issues/<n>/gates.md`에 기록한다는 한 문장과 코드베이스 존중 규칙 네 줄을 넣는다: 기존 함수·패키지·테스트 헬퍼 확장을 새 파일·새 추상화보다 우선한다(plan의 `## 재사용하는 기존 구현`에 없는 새 추상화는 만들지 않는다); 계약 표면(CLI JSON, MCP schema, golden, record schema, provider body) 변경은 이슈와 plan이 명시한 것만 한다; hot path를 건드리면 전후 측정을 evidence로 남긴다; 파일·원격·상태에 미치는 side effect를 turing report에 목록으로 적는다. 출처는 `AGENTS.md` §2·§3이다. `## Lease fencing`은 라우터 `## 공통 불변식`을 링크하는 두 줄로 줄인다.
  3. `## 종료 게이트`를 다시 쓴다:
     ```text
     1. focused verification 증거가 명령·결과로 남아 있고, 위임한 child가 전부 accepted 또는 dropped다.
     2. turing report 초안을 워크트리 안에 쓴다(경로는 issueops-complete가 요구하는 워크트리 내부 상대 경로). 최종 확정은 4단계 정리가 한다.
     3. agent-harness issueops phase --id ID --to ai-slop-clean ... → "다음: issueops-clean".
     4. 이 단계에서는 커밋·푸시하지 않는다. 커밋은 7단계다.
     ```
  4. `## 나쁜 예` 표에서 "record가 없는데 게이트 표만 보고" 행을 "`next`를 안 돌리고 phase를 추정" 행으로 바꾸고, "구현 직후 커밋·푸시" 행과 "구현 스킬 안에서 ai-slop-clean·implementation review 기록" 행을 추가한다.
  5. openai.yaml default_prompt에서 "after design, compatibility, and devil's-advocate approval"을 "after `issueops next` reports implement"로 바꾸고 "exit by moving the cycle to ai-slop-clean; cleanup, verification, and commit belong to later stages"를 넣는다.

  **Must NOT do**: 계획 루프 포함, orca owner 경로 설명 추가, ai-slop-clean·리뷰·커밋 절차 포함.

  **Recommended Agent**: deep.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T11 | Blocked By: T1, T22

  **References**:
  - 현행 `skills/issueops-implement/SKILL.md` 전체.
  - `skills/gates-ledger/SKILL.md`(T22), `AGENTS.md` §2·§3.
  - `skills/issueops/SKILL.md` "Child와 delegation" 문단(이동 원문).
  - `.agent-harness/operations/guides/issueops-execution.md:174` (turing report를 PR 전에 커밋, #153).
  - `skills/issueops-complete/SKILL.md` "증거를 만드는 순서"(report 경로 규칙).
  - `internal/adapter/issueops/issueops_pr_readiness_strict.go:12-70`.

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과.
  - [ ] `rg -n "issueops next" skills/issueops-implement/SKILL.md` 1건 이상, `rg -n "issueops-clean|compatibility review|link-plan|claim-current-token" skills/issueops-implement/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "재사용|하위 호환|side effect|측정" skills/issueops-implement/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "Child와 delegation|issueops-branch-worktree|## Publication evidence gates|## Implementation review gate|implementation-review record|project-docs-review record" skills/issueops-implement/SKILL.md` 0건.
  - [ ] `wc -l skills/issueops-implement/SKILL.md`가 220 이하.

  **QA Scenarios**:
  ```
  Scenario: 종료 절이 ai-slop-clean 전이로 끝남
    Channel: bash
    Steps: sed -n '/## 종료 게이트/,/## 나쁜 예/p' skills/issueops-implement/SKILL.md | rg -n "phase --id ID --to ai-slop-clean|issueops-clean|커밋·푸시하지 않는다"
    Expected: 세 항목 각 1건 이상
    Evidence: .agent-harness/evidence/task-9-exit.txt
  Scenario: 시작 게이트가 next를 요구
    Channel: bash
    Steps: sed -n '/## 시작 게이트/,/## /p' skills/issueops-implement/SKILL.md | rg -c "issueops next"
    Expected: 1 이상
    Evidence: .agent-harness/evidence/task-9-start.txt
  ```

  **Commit**: YES | `docs(skill): narrow issueops-implement to implementation and the ai-slop-clean exit` | Files: 위 2개

- [x] **T10. `issueops-create-pr`·`issueops-complete`·`issueops-cleanup`·`issueops-sync-issue`·`issueops-sync-pr` cross-link 수정** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-10-cleanup.txt`, `task-10-links.txt`(skills 트리 전체 상대 링크 깨짐 0건). 다섯 스킬 모두 `issueops next`로 시작하고 원격 쓰기 절차는 `issueops-remote-write`를 가리킨다.

  **Files:**
  - Modify: `skills/issueops-create-pr/SKILL.md`, `skills/issueops-complete/SKILL.md`, `skills/issueops-cleanup/SKILL.md`, `skills/issueops-sync-issue/SKILL.md`, `skills/issueops-sync-pr/SKILL.md`
  - Modify: 각 `agents/openai.yaml`(단계 번호 언급만)

  **What to do**:
  1. 다섯 스킬의 "시작 게이트"(또는 첫 절) 맨 앞에 한 줄을 넣는다: "`agent-harness issueops next --json`을 실행해 `stage.key`가 각각 `pr.create` / `pr.complete` / `cleanup` / (sync는 어느 단계든) 인지 확인한다."
  2. `issueops-cleanup`: "Do not use `cleanup abandon`" 문단을 "머지되지 않은 사이클을 정리하려면 `issueops-abandon`"으로 바꾼다. Load first 목록의 `issueops` 로드 문장은 유지한다.
  3. `issueops-create-pr`: 상단 링크 목록에서 `issueops-implement` 링크를 `issueops-verify`로 바꾸고 "7단계 커밋·푸시가 끝난 뒤"를 적고, "시작 게이트"의 `project_docs_review`/`schema_evidence` 문단은 유지한다.
  4. `issueops-complete`: "종료 경계"에 "이후 `issueops-cleanup`(머지 뒤) 또는 `issueops-abandon`(폐기)" 두 갈래를 적는다.
  5. `issueops-branch-worktree`를 가리키는 링크가 있으면 `issueops-prepare`로 바꾼다(`rg -n "branch-worktree" skills/`로 확인).
  6. 다섯 스킬에서 원격 쓰기 절차 문단(preview → 동일 요청 confirm → readback, `fluent-korean` 호출 문단, secret redaction 문단)을 지우고 `issueops-remote-write`(T23) 링크 한 줄로 바꾼다. 남기는 것은 각 명령의 고유 플래그, drift 표, 본문 형식과 좋은 예·나쁜 예뿐이다.

  **Must NOT do**: 본문 계약 변경, 원격 쓰기 절차의 재복사.

  **Recommended Agent**: quick.

  **Parallelization**: Can Parallel: YES(T12·T20과) | Wave 5 | Blocks: T11 | Blocked By: T5, T23

  **References**: 각 스킬 현행 본문. `rg -n "branch-worktree|cleanup abandon" skills/`.

  **Acceptance Criteria**:
  - [ ] 다섯 스킬 모두 검사기 2개 통과.
  - [ ] `rg -n "issueops next" skills/issueops-create-pr skills/issueops-complete skills/issueops-cleanup skills/issueops-sync-issue skills/issueops-sync-pr` 5건 이상.
  - [ ] `rg -l "issueops-remote-write" skills/issueops-create-pr skills/issueops-cleanup skills/issueops-sync-issue skills/issueops-sync-pr | wc -l`이 4.
  - [ ] `rg -n "branch-worktree" skills/` 0건.

  **QA Scenarios**:
  ```
  Scenario: cleanup이 abandon을 금지 대신 위임
    Channel: bash
    Steps: rg -n "issueops-abandon" skills/issueops-cleanup/SKILL.md
    Expected: 1건 이상
    Evidence: .agent-harness/evidence/task-10-cleanup.txt
  Scenario: 링크 무결성
    Channel: bash
    Steps: for f in skills/issueops*/SKILL.md; do rg -o '\]\((\.\./[^)]+)\)' -r '$1' "$f" | while read p; do test -e "$(dirname "$f")/$p" || echo "BROKEN $f $p"; done; done
    Expected: BROKEN 0건
    Evidence: .agent-harness/evidence/task-10-links.txt
  ```

  **Commit**: YES | `docs(skill): route pr, complete, cleanup, and sync skills through issueops next` | Files: 위 파일

- [x] **T11. `issueops` 라우터 재작성 + 레퍼런스 삭제 + cleanup-state·remote-issue 절 삭제 + skill-contract ratchet** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-11-first.txt`(첫 명령이 `issueops next --json`), `task-11-links.txt`(깨진 링크 0건). 244줄(기준 260 이하), 네 레퍼런스 삭제 확인.

  **실행 결과와 계획 대비 편차**:
  - **`execution.md`에 넣으라던 두 문장은 넣지 않았다.** 계획은 "기본 경로는 `issueops-prepare`가 만든 워크트리를 implementer 세션이 `--mode direct`로 채택하는 것"을 적으라고 했는데, 그것은 9단계 시절 설계다. 사용자 결정(모드는 `--mode auto`가 고른다)과 T4 이후 2단계가 워크트리를 만들지 않는다는 사실 둘 다와 어긋난다.
  - **삭제한 `worktree-context.md`의 남은 세 절을 버리지 않고 옮겼다.** 계획은 "Dependencies And Local Configuration"(T8)과 "Edit Target Guard"(T11)만 배정했는데, "Branch And Canonical Worktree"(워크트리 채택 3조건과 canonical 경로), "Context Routing"은 소유자가 없었다. `execution.md`의 Prepare 절로 옮겼다. "Parallel Independence"는 `execution.md`에 이미 같은 내용이 있어 그대로 뒀다.
  - **`requested_mode`·`resolved_mode`·readiness fingerprint 문구**도 삭제된 라우터 절에만 있었다. `execution.md`의 preview 설명으로 옮겼다.
  - Go 계약 테스트 네 곳을 고쳤다. 단계 명령 핀(`intent record`, `design review`, `--mode auto`)은 라우터가 아니라 그 명령을 소유한 단계 스킬을 읽게 했고, GitHub Orca ordering 핀은 `execution.md`와 providers 가이드 둘만 남겼다. `torvalds`의 worktree 링크도 `execution.md`로 고쳤다.

  **Files:**
  - Modify: `skills/issueops/SKILL.md` (전면 재작성)
  - Modify: `skills/issueops/agents/openai.yaml`
  - Delete: `skills/issueops/references/issue-preflight.md`, `worktree-context.md`, `operational-start.md`
  - Modify: `skills/issueops/references/cleanup-state.md` ("Retiring a cycle that never entered execution v1" 절 삭제, 대신 한 줄 "폐기는 `issueops-abandon`이 소유한다")
  - Modify: `skills/issueops/references/remote-issue.md` ("Korean Remote Artifact Gate"·"Remote Artifact Writing Quality" 절 삭제, 대신 한 줄 "쓰기 프로토콜과 한국어 게이트는 `issueops-remote-write`가 소유한다". 스크립트 이동은 T23이 한다)
  - Modify: `skills/issueops/references/execution.md` ("Prepare" 절 첫머리에 "기본 경로는 `issueops-prepare`가 만든 워크트리를 implementer 세션이 `--mode direct`로 채택하는 것이며, 이 절의 Orca 경로는 대안이다" 두 문장 추가; 나머지 유지)
  - Modify: `internal/adapter/skillcontract/skill_contract_test.go` (`TestIssueOpsRouterPinsStageContract` 추가)
  - Modify: `internal/adapter/issueops/issueops_skill_contract_test.go` (`:29-30,:70,:87`의 삭제 파일 `operational-start.md` 읽기를 `execution.md`와 `.agent-harness/operations/guides/issueops-providers.md`로 바꾸고, `:16`의 라우터 `--mode auto` 핀을 `issueops-plan`의 "`--mode direct`" 핀으로 바꾸며, `:73`의 GitHub Orca ordering 문자열 핀 대상을 `execution.md`와 providers 가이드 두 문서로 옮긴다. 테스트 4개가 깨지지 않게 먼저 고친다)

  **What to do**:
  1. 라우터 본문 구조(이 순서):
     - 머리말 두 문장: IssueOps는 durable record 하나에 이슈, 브랜치·워크트리, 계획, 실행 lease, 검증 증거, PR/MR을 묶는다. 이 스킬은 라우터이며 단계 작업은 단계 스킬이 한다.
     - `## 먼저 실행`: `agent-harness issueops next --json`. 출력의 `stage.key`를 `## 단계 표`로 스킬·label로 바꾸고, `lease`, `missing`, `next_command`(`next_command_kind`가 `template`이면 채울 값과 함께), `exits`와 함께 사용자에게 보여 준 뒤 선택지 4개를 묻는다. `blocked.*`면 진행하지 않는다. `ambiguous`면 `candidates`를 보여 주고 ID를 고르게 한다.
     - `## 10단계와 스킬` 표(설계 요약 1의 표 그대로, "탈출" 행과 공용 스킬 세 행 포함).
     - `## 세션 경계`: 1·2단계는 source checkout, 3단계 이후는 워크트리 세션. lease는 3단계 부트스트랩에서 생긴다. 다른 세션·호스트에서 재개는 `issueops-abandon`의 재개 경로.
     - `## 공통 불변식`(새 절, 단계 스킬은 링크만 한다): (a) 단계 판별 프로토콜(`next` 실행, `stage.key` 비교, 선택지 3개 제시, `blocked.*`면 중단), (b) actor 플래그는 `execution whoami --json`의 `record_actor_flags`·`claim_actor_flags`를 그대로 쓴다, (c) lease fencing(현행 `issueops-implement` "Lease fencing" 절 원문 여섯 줄), (d) 편집 대상 확인 bash 블록(현행 `worktree-context.md` "Edit Target Guard" 원문), (e) durable mutation 전 exact ID·generation·actor·cwd 대조와 불일치 시 stop, (f) 코드베이스 존중: 새 코드보다 기존 구현의 확장·재사용을 우선하고, 계약 표면의 하위 호환성과 성능·side effect를 계획(plan 필수 절 세 개)·구현(루프 규칙 네 줄)·검증(리뷰 렌즈 네 개)에서 명시한다(`AGENTS.md` §2·§3 링크).
     - `## Core contract`(현행 phase enum 문단 유지).
     - `## 단계 표`(설계 요약 9의 표와 선택지 3줄. `stage.key`를 스킬·label·선택지로 바꾸는 유일한 소유자).
     - `## Gate map` 표는 삭제한다. missing 키의 owner command는 `issueops next`의 `next_command`가 렌더한다는 한 줄만 남긴다(소유자 하나, 설계 요약 3).
     - `## 구현·검증 규칙`, `## Stop conditions`, `## IssueOps benchmark artifact contract`, `## Execution ownership`은 유지한다. `## Remote write 공통 게이트`는 `issueops-remote-write`(T23)로 옮기고 한 줄 링크만 남긴다.
     - `## Reference map`: 삭제한 네 파일 행을 지우고 나머지 6개만 남긴다. `remote-issue.md` 행의 책임은 provider 관계·hierarchy로 좁힌다.
     - "시작 순서", "Child와 delegation", "단계별 라우팅" 표는 삭제한다(단계별 라우팅 표의 pioneer 스킬 열은 `## 10단계와 스킬` 표에 "함께 쓰는 스킬" 열로 흡수: 1단계 `von-neumann` 인터뷰·`berners-lee`, 3단계 `von-neumann`·`brooks`·`codd`·`dijkstra`·`karpathy`, 구현 `hopper`·`turing`·`shannon`).
  2. `git rm skills/issueops/references/issue-preflight.md skills/issueops/references/worktree-context.md skills/issueops/references/operational-start.md`.
  3. ratchet 테스트:
     ```go
     func TestIssueOpsRouterPinsStageContract(t *testing.T) {
         assertSkillContains(t, "issueops", []string{
             "agent-harness issueops next --json",
             "issueops-create-issue", "issueops-prepare", "issueops-plan",
             "issueops-implement", "issueops-create-pr", "issueops-complete",
             "issueops-cleanup", "issueops-abandon",
             "issueops-clean", "issueops-docs", "issueops-verify",
             "issueops-review", "gates-ledger", "issueops-remote-write",
             "## 공통 불변식", "## 단계 표",
         })
         body := readRepoFileForTest(t, filepath.Join("skills", "issueops", "SKILL.md"))
         for _, retired := range []string{"issue-preflight.md", "worktree-context.md", "operational-start.md", "ai-slop-clean.md", "issueops-branch-worktree", "## Remote write 공통 게이트", "## Gate map"} {
             if strings.Contains(body, retired) {
                 t.Fatalf("issueops router still references retired %q", retired)
             }
         }
     }
     ```
  4. `rg -n "issue-preflight|worktree-context|operational-start|branch-worktree" skills/ .agent-harness --glob '!.agent-harness/turing/**' --glob '!.agent-harness/issues/**' --glob '!.agent-harness/adr/**' --glob '!.agent-harness/drafts/**' --glob '!.agent-harness/archive/**' --glob '!.agent-harness/cautions/lessons/**'`로 남은 참조를 전부 고친다(문서 본문은 T14가 맡지만 링크 깨짐은 여기서 잡는다).

  **Must NOT do**: benchmark artifact contract 절 삭제(benchmark가 참조), `execution.md`·`orchestration.md`·`remote-issue.md`·`evidence-contract.md`·`review-feedback.md`·`cleanup-state.md` 삭제.

  **Recommended Agent**: deep.

  **Parallelization**: Can Parallel: NO | Wave 6 | Blocks: T13, T14, T15 | Blocked By: T3, T4, T5, T6, T8, T9, T10, T19, T20, T21, T22, T23, T26

  **References**:
  - 현행 `skills/issueops/SKILL.md` 전체(유지 절 원문).
  - `internal/adapter/skillcontract/skill_contract_test.go:15-30` (`assertSkillContains`, `readRepoFileForTest`).
  - `cmd/harness/issueopscli/benchmarkartifact/issueops_benchmark_artifact.go:9`(benchmark 참조 경로가 create-issue·create-pr임을 확인).

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과, `go test ./internal/adapter/skillcontract -count=1` 통과.
  - [ ] `test ! -f skills/issueops/references/issue-preflight.md && test ! -f skills/issueops/references/worktree-context.md && test ! -f skills/issueops/references/operational-start.md && test ! -f skills/issueops/references/ai-slop-clean.md`.
  - [ ] `rg -n "Korean Remote Artifact Gate|Remote Artifact Writing Quality" skills/issueops/references/remote-issue.md` 0건.
  - [ ] `wc -l skills/issueops/SKILL.md`가 260 이하(공통 불변식 5항목과 10단계 표를 더한 실측 예산).
  - [ ] `go test ./internal/adapter/issueops -run SkillContract -count=1` 통과(삭제 파일 읽기 핀과 `--mode auto` 핀이 고쳐졌다).

  **QA Scenarios**:
  ```
  Scenario: 라우터 첫 명령이 next
    Channel: bash
    Steps: rg -n "agent-harness issueops" skills/issueops/SKILL.md | head -1
    Expected: 첫 매치가 "issueops next --json"
    Evidence: .agent-harness/evidence/task-11-first.txt
  Scenario: 깨진 상대 링크 없음
    Channel: bash
    Steps: T10의 링크 무결성 루프를 skills/issueops/SKILL.md와 references/*.md에 실행
    Expected: BROKEN 0건
    Evidence: .agent-harness/evidence/task-11-links.txt
  ```

  **Commit**: YES | `refactor(skill)!: rewrite the issueops router around issueops next and delete retired references` | Files: 위 파일

- [x] **T12. openai.yaml, README, README.en, OPERATIONS 스킬 목록** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-12-readme.txt`(두 README 모두 `issueops next` 예시), `task-12-ops.txt`(OPERATIONS 112줄). 아홉 스킬이 세 문서 모두에 있고 `issueops-branch-worktree`는 0건이다.

  **실행 결과와 계획 대비 편차**: `.gitlab/issue_templates/implementation_task.md`는 `issueops-create-issue` 경로를 그대로 가리키므로 고치지 않았다(계획이 예상한 대로). TECH_STACK은 T4에서 이미 고쳤다.

  **Files:**
  - Modify: `README.md:122-131`(cycle 시작 절을 10단계 요약으로), `:237-262`(IssueOps 절: `next`, 10단계, 공용 스킬, abandon 한 문단씩), `:270`(스킬 목록: `issueops-branch-worktree` 제거, `issueops-prepare`·`issueops-plan`·`issueops-clean`·`issueops-docs`·`issueops-verify`·`issueops-abandon`·`issueops-review`·`issueops-remote-write`·`gates-ledger` 추가)
  - Modify: `README.en.md:123-132,241-273,282` (같은 내용 영어)
  - Modify: `.agent-harness/OPERATIONS.md:44-46` (Native skills 목록에 위 아홉 스킬 추가; 라인 예산 250 유지)
  - Modify: `.agent-harness/TECH_STACK.md:72` (`issueops-branch-worktree` 언급을 `issueops-prepare`로)
  - Modify: `.gitlab/issue_templates/implementation_task.md:49` 확인(create-issue 경로 그대로면 변경 없음)

  **What to do**: 위 위치를 고친다. README의 IssueOps 절에는 `agent-harness issueops next --json` 예시 출력(T6의 텍스트 형식 6줄)을 넣는다. 영어판은 같은 정보를 담되 번역이 아닌 요약이어도 된다.

  **Must NOT do**: openwiki 편집, 다른 절 수정.

  **Recommended Agent**: quick.

  **Parallelization**: Can Parallel: YES(T10·T20과) | Wave 5 | Blocks: T15 | Blocked By: T4

  **References**: `README.md:122-131,237-262,270`, `README.en.md:123-132,241-273,282`, `.agent-harness/OPERATIONS.md:40-66`, `internal/architecture/documentation_test.go`(README가 언급해야 하는 경로 목록).

  **Acceptance Criteria**:
  - [ ] `rg -n "issueops-branch-worktree" README.md README.en.md .agent-harness/OPERATIONS.md` 0건.
  - [ ] `for s in issueops-prepare issueops-plan issueops-clean issueops-docs issueops-verify issueops-abandon issueops-review issueops-remote-write gates-ledger; do rg -l "$s" README.md README.en.md .agent-harness/OPERATIONS.md | wc -l; done` 결과가 모두 3.
  - [ ] `rg -n "issueops-branch-worktree" .agent-harness/TECH_STACK.md` 0건.
  - [ ] `go test ./internal/architecture -run Documentation -count=1` 통과.

  **QA Scenarios**:
  ```
  Scenario: README에 next 예시
    Channel: bash
    Steps: rg -n "issueops next" README.md README.en.md
    Expected: 각 1건 이상
    Evidence: .agent-harness/evidence/task-12-readme.txt
  Scenario: OPERATIONS 라인 예산
    Channel: bash
    Steps: wc -l .agent-harness/OPERATIONS.md
    Expected: 250 이하
    Evidence: .agent-harness/evidence/task-12-ops.txt
  ```

  **Commit**: YES | `docs: list the ten-stage issueops skills in README and OPERATIONS` | Files: 위 파일

- [x] **T13. ADR 추가** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-13-adr.txt`(2026-07-24 supersede 명시), `task-13-check.json`(`ok: true`, 문서 357개 위반 0).

  **실행 결과와 계획 대비 편차**: 파일명을 `2026-09-05-issueops-ten-stage-skills-with-auto-execution-mode.md`로 했다. 계획의 `-with-worktree-adoption`은 9단계 시절 설계(준비 단계가 워크트리를 만들고 implementer가 채택)를 가리키는데, 그 설계는 `--mode auto` 결정으로 대체됐다. Decision 본문도 그에 맞춰 썼다.

  **Files:**
  - Create: `.agent-harness/adr/2026-09-05-issueops-ten-stage-skills-with-worktree-adoption.md`
  - Modify: `.agent-harness/ADR.md` (색인 줄 추가)

  **What to do**: MCP `project_docs_append(kind=adr)`가 있으면 그것으로, 없으면 파일을 직접 쓴다. 형식은 `.agent-harness/adr/2026-09-02-issueops-publication-evidence-gates-project-doc-reflection-a.md`와 같다: `name`(파일명과 같은 slug)과 `description` frontmatter, 제목, 그리고 `Date`, `Kind`, `Source`, `Summary`, `Context`, `Decision`, `Consequences`, `Alternatives / rejected options` bullet. 내용:
  - Summary: 스킬을 10단계로 재편하고 implementer 세션이 계획·검토·구현·정리·검증·커밋·PR을 맡는다. 반복 절차는 공용 스킬 `issueops-review`, `gates-ledger`, `issueops-remote-write`가 소유한다. 준비 단계가 base SHA 워크트리를 만들고 implementer 세션이 `execution prepare --mode direct`로 채택한다. 단계 판별은 `issueops next`, 탈출은 `issueops-abandon`과 `cleanup abandon`의 원격 효과 플래그, 재개·인수는 라우터의 `next_command` 체인이다.
  - Context: 이 계획의 "왜 이 하네스인가"(호스트 무관 동일 절차, 팀 공유 issue SSOT와 진행 상태 확인)와 Interview Summary 표.
  - Decision: 설계 요약 1-5.
  - Consequences: ADR 2026-07-24의 "메인 세션은 계획 전용" 조항을 대체한다. orca 자동 실행은 대안으로 남는다. record schema 무변경. `--direct-reason` 문자열은 고정값이다. 후속: phase 전이를 issue 본문 관리 블록에 반영하는 `remote reflect-stage`(Gap Analysis 누락 3).
  - Alternatives / rejected: `handoff` 모드 추가, owner 계획 허용, release+reseed 정상 경로화, hook 단계 안내.

  **Must NOT do**: 기존 ADR 수정·삭제.

  **Recommended Agent**: quick.

  **Parallelization**: Can Parallel: YES(T14와) | Wave 7 | Blocks: T15 | Blocked By: T11

  **References**: `.agent-harness/ADR.md:1-30`, `.agent-harness/adr/2026-09-02-*.md`, `.agent-harness/adr/decisions/2026-07-24-issueops-planner-implementer-dual-structure.md`.

  **Acceptance Criteria**:
  - [ ] `rg -n "2026-09-05-issueops-ten-stage" .agent-harness/ADR.md` 1건.
  - [ ] 새 파일에 "Alternatives" 절이 있고 기각 대안 4개가 있다.

  **QA Scenarios**:
  ```
  Scenario: superseding 명시
    Channel: bash
    Steps: rg -n "2026-07-24" .agent-harness/adr/2026-09-05-issueops-ten-stage-skills-with-worktree-adoption.md
    Expected: 1건 이상
    Evidence: .agent-harness/evidence/task-13-adr.txt
  Scenario: 색인 형식
    Channel: bash
    Steps: uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json
    Expected: ok true
    Evidence: .agent-harness/evidence/task-13-check.json
  ```

  **Commit**: YES | `docs(adr): record the ten-stage issueops skills and the auto execution mode` | Files: 위 2개

- [x] **T14. AGENT_WORKFLOW, 운영 가이드, 아키텍처, 테스팅 모듈 갱신** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-14-orca.txt`(Orca owner sequence 보존), `task-14-testing.txt`. `이원 구조` 표현 0건, docs checker `ok: true`(문서 357개, 위반 0).

  **실행 결과와 계획 대비 편차**: `architecture/issueops.md`의 워크트리 문장은 계획이 지시한 "`issueops-prepare`가 미리 만든 워크트리를 채택"이 아니라 "top-level·branch·HEAD가 모두 일치하는 기존 워크트리를 채택"으로 썼다. 2단계는 더 이상 워크트리를 만들지 않으므로 그 주체를 특정하면 사실과 다르다. 채택 기제 자체는 남아 있다.

  **Files:**
  - Modify: `.agent-harness/AGENT_WORKFLOW.md` ("이원 구조 흐름 요약" 절 교체, "Execution v1 workflow" 절의 "The active holder performs planning, implementation..." 문장을 10단계 기준으로, "IssueOps 자동 루프는 missing gate를 읽고" 문단 앞에 "단계 판별은 `issueops next`" 한 문장)
  - Modify: `.agent-harness/operations/guides/issueops-execution.md:169-178` (절 교체: 10단계 운영과 세션 경계, `next`, abandon 순서; "Orca owner sequence"는 유지)
  - Modify: `.agent-harness/architecture/issueops.md:42` (제공 표면 문단에 `issueops next`(read-only stage projection, `issueopsnext` vertical, fetch·provider 호출 없음)와 `cleanup abandon`의 원격 효과 플래그를 추가; "이원 구조 운영 표면" 표현을 "단계 운영 표면"으로; implementation review 게이트를 "모든 모드"로; "`execution prepare`가 provider branch의 exact base SHA에서 fixed sibling worktree를 만들고"를 "만들거나, `issueops-prepare`가 base SHA에 미리 만든 워크트리를 top-level·branch·HEAD 일치 조건으로 채택하고"로)
  - Modify: `.agent-harness/testing/issueops-execution.md` ("Execution tests must cover" 목록에 `issueops next` 분류표 table test와 fetch 미호출 단언, `cleanup abandon` 원격 효과의 순서·멱등·부분 실패 케이스, direct 모드의 implementation review 게이트를 추가)
  - Modify: `.agent-harness/OPERATIONS.md:24` (가이드 색인 설명의 "planner/implementer"를 "10단계 운영"으로)

  **What to do**: 각 위치를 고친다. 이원 구조 요약을 대체하는 새 문단은 다음 골격이다: "1·2단계는 source checkout의 세션이 `issueops-create-issue`, `issueops-prepare`로 수행하고 lease를 갖지 않는다. 3단계부터는 워크트리에서 띄운 세션이 `issueops-plan`으로 워크트리를 채택해 generation 1 홀더가 되고, `issueops-implement`, `issueops-clean`, `issueops-verify`, `atomic-commit-push`, `issueops-create-pr`, `issueops-complete`를 지나 완료한다. 휴먼 머지 뒤 `issueops-cleanup`. 어느 단계든 `issueops next`가 현재 단계를 판별하고, `issueops-abandon`이 일시 중단·재개·인수·폐기를 맡는다. 적대 리뷰는 `issueops-review`, 게이트 원장은 `gates-ledger`, 원격 쓰기는 `issueops-remote-write`가 단계와 무관하게 소유한다."

  **Must NOT do**: hook 절 수정, Orca owner sequence 삭제.

  **Recommended Agent**: deep. 다섯 문서의 상호 참조가 검사기에 걸린다.

  **Parallelization**: Can Parallel: YES(T13과) | Wave 7 | Blocks: T15 | Blocked By: T11, T24(같은 `architecture/issueops.md:42`를 T24가 먼저 고친다)

  **References**: `.agent-harness/AGENT_WORKFLOW.md` 해당 절, `.agent-harness/operations/guides/issueops-execution.md:137-178`, `.agent-harness/architecture/issueops.md:42`, `.agent-harness/testing/issueops-execution.md:36-80`.

  **Acceptance Criteria**:
  - [ ] `rg -n "이원 구조" .agent-harness --glob '!.agent-harness/adr/**' --glob '!.agent-harness/turing/**' --glob '!.agent-harness/issues/**' --glob '!.agent-harness/drafts/**' --glob '!.agent-harness/archive/**' --glob '!.agent-harness/cautions/lessons/**'` 0건.
  - [ ] `rg -n "issueops next" .agent-harness/AGENT_WORKFLOW.md .agent-harness/operations/guides/issueops-execution.md .agent-harness/architecture/issueops.md` 각 1건 이상.
  - [ ] docs checker 통과.

  **QA Scenarios**:
  ```
  Scenario: Orca 대안 경로 보존
    Channel: bash
    Steps: rg -n "## Orca owner sequence" .agent-harness/operations/guides/issueops-execution.md
    Expected: 1건
    Evidence: .agent-harness/evidence/task-14-orca.txt
  Scenario: 테스트 요구 목록에 next 추가
    Channel: bash
    Steps: rg -n "issueops next|close-pr" .agent-harness/testing/issueops-execution.md
    Expected: 각 1건 이상
    Evidence: .agent-harness/evidence/task-14-testing.txt
  ```

  **Commit**: YES | `docs(issueops): describe the ten-stage operation and session boundary` | Files: 위 파일

- [x] **T15. CAUTIONS 모듈 추가, docs checker, 골든 최종 재생성** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-15-budget.txt`(134·221·95줄), `task-15-golden.txt`(두 골든 통과). docs checker `ok: true`(문서 358개). 여섯 항목을 새 모듈에 넣었고 `issueops-lifecycle.md`는 건드리지 않았다.

  **Files:**
  - Create: `.agent-harness/cautions/issueops-stages.md` (새 모듈. `issueops-lifecycle.md`는 이미 221행이라 250행 예산 안에 여섯 항목을 더 넣을 수 없다. 새 책임 클래스 "단계 운영 함정"이므로 CAUTIONS Update workflow 1의 모듈 추가 조건에 맞는다)
  - Modify: `.agent-harness/CAUTIONS.md` (모듈 색인 줄 1개)
  - Regenerate: `cmd/harness/testdata/response_contracts.golden.json` (`.agent-harness/*.md` 편집으로 드리프트한 docs-index 부분)

  **What to do**:
  1. 새 모듈에 여섯 항목을 기존 `issueops-lifecycle.md` §32의 형식(제목, 증상, 원인, 규칙, 근거 file:line)으로 각 15행 이하로 쓴다. §1 `--mode auto`는 워크트리 세션에서 Orca owner를 띄운다. §2 워크트리 세션은 `<source>.worktrees` 쓰기 권한이 필요하다(`--add-dir`, relaunch는 워크트리로 `cd`). §3 2단계와 3단계 사이의 커밋은 채택을 깨뜨린다. §4 `remote close-issue`·`cleanup remote-branch`는 머지 후 전용이며 미머지 정리는 `cleanup abandon`의 플래그다. implementation review 게이트가 모든 모드로 넓어져 진행 중 direct 사이클도 pr 진입·create-pr 전 리뷰 기록이 필요하다. §5 strict readiness는 `git fetch`를 실행하므로 read-only 판별 경로에서 호출하지 않는다. §6 change fingerprint는 gates.md·report·문서·untracked 파일까지 포함하므로 파일을 바꾸는 작업은 4·5단계에서 끝내고 봉인 뒤에는 파일을 만지지 않는다. T16이 stale 스킬 링크를 손으로 지웠으면 §7로 그 사실을 적는다.
  2. `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update -count=1` 후 `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`.
  3. docs checker 실행.

  **Must NOT do**: lessons 파일 추가(사고가 아니라 규칙이므로 모듈에 둔다), `issueops-lifecycle.md`에 항목 추가.

  **Recommended Agent**: quick.

  **Parallelization**: Can Parallel: NO | Wave 7 | Blocks: T16 | Blocked By: T6, T7, T11, T12, T13, T14, T24, T25

  **References**: `.agent-harness/cautions/issueops-lifecycle.md:169-211` (§32 형식), `.agent-harness/CAUTIONS.md` Update workflow.

  **Acceptance Criteria**:
  - [ ] `rg -c "^## " .agent-harness/cautions/issueops-stages.md`가 6 이상이고, `rg -n "issueops-stages.md" .agent-harness/CAUTIONS.md` 1건.
  - [ ] `wc -l .agent-harness/cautions/issueops-stages.md .agent-harness/cautions/issueops-lifecycle.md .agent-harness/CAUTIONS.md` 각 250 이하.
  - [ ] `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1` 통과.
  - [ ] docs checker `ok true`.

  **QA Scenarios**:
  ```
  Scenario: 색인 라인 예산
    Channel: bash
    Steps: wc -l .agent-harness/CAUTIONS.md .agent-harness/cautions/issueops-lifecycle.md
    Expected: 각 250 이하
    Evidence: .agent-harness/evidence/task-15-budget.txt
  Scenario: 골든이 최신
    Channel: bash
    Steps: go test ./cmd/harness/contractgolden -run Golden -count=1; go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
    Expected: 둘 다 ok
    Evidence: .agent-harness/evidence/task-15-golden.txt
  ```

  **Commit**: YES | `docs(cautions): record the stage pitfalls of the ten-stage flow` | Files: 위 파일 + 골든

- [x] **T16. 전체 게이트 배터리** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-16-battery.txt`, `task-16-links.txt`. 단독 실행에서 `go test ./...`와 `go test -race ./...` 모두 247 패키지 통과, 실패 0, 데이터 레이스 0.

  **찾은 결함 1건**: `internal/domain/commandparse`의 `cleanup abandon` 스펙에 T7의 새 플래그 세 개가 없었다(커밋 `bc42c0be`). **부하 인공물 3건**: self-verify의 `go test`와 `go test -race`를 동시에 돌린 1차 실행에서 webfetch·github·gitlab이 `signal: killed`/`context deadline exceeded`로 실패했고, 단독 재실행에서 전부 통과했다.

  **실행 결과와 계획 대비 편차**: `./scripts/install-native.sh`가 stale 스킬 링크를 스스로 지웠다(dry-run이 "would prune stale skill link for agy: issueops-branch-worktree"를 보고했고 실행 뒤 `~/.claude/skills`에 남지 않았다). 계획이 대비한 수동 `rm`과 CAUTIONS §7 추가는 필요 없었다.

  **What to do**: 아래를 순서대로 실행하고 결과를 evidence에 남긴다. T0의 pre-existing 실패 외 실패는 여기서 고친다.
  ```bash
  go mod tidy && git diff --exit-code go.mod go.sum
  gofmt -l $(git ls-files '*.go')            # 출력 없음
  go vet ./...
  go test ./... -count=1
  go test -race ./... -count=1
  go build -o bin/agent-harness ./cmd/harness
  ./bin/agent-harness inspect --json >/dev/null
  ./bin/agent-harness docs --json >/dev/null
  ./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
  for d in skills/issueops*; do python3 scripts/validate-skill.py "$d"; python3 scripts/verify-skill-shell.py "$d"; done
  uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json
  ./scripts/install-native.sh --dry-run --json
  rg -n "issueops-branch-worktree|issue-preflight.md|worktree-context.md|operational-start.md|references/ai-slop-clean.md|이원 구조" --glob '!.agent-harness/adr/**' --glob '!.agent-harness/turing/**' --glob '!.agent-harness/issues/**' --glob '!.agent-harness/issueops/**' --glob '!openwiki/**' --glob '!.agent-harness/drafts/**' --glob '!.agent-harness/archive/**' --glob '!.agent-harness/cautions/lessons/**' --glob '!docs/superpowers/**' --glob '!.agent-harness/plans/**'
  ```
  마지막 `rg`는 0건이어야 한다. 스킬 링크는 Go `install`이 소유하고 `install-native.sh`에는 링크 코드가 없으므로, `./scripts/install-native.sh`(내부에서 `agent-harness install`을 호출)를 실행한 뒤 `ls -la ~/.claude/skills | grep issueops`를 evidence에 남긴다. `issueops-branch-worktree` 링크가 남아 있으면 Go install이 stale 링크를 지우지 않는다는 뜻이므로 사용자 승인 뒤 그 심볼릭 링크 하나를 `rm`으로 지우고, 그 사실을 T15의 cautions 모듈에 한 줄로 적는다.

  **Must NOT do**: 테스트 삭제·skip으로 통과시키기.

  **Recommended Agent**: deep.

  **Parallelization**: Can Parallel: NO | Wave 8 | Blocks: T17 | Blocked By: T15

  **References**: `AGENTS.md` §9.

  **Acceptance Criteria**:
  - [ ] 위 명령 전부 종료 코드 0(마지막 rg는 1, 즉 매치 없음).

  **QA Scenarios**:
  ```
  Scenario: 설치본 갱신 뒤 스킬 링크
    Channel: bash
    Steps: ./scripts/install-native.sh && ls ~/.claude/skills | grep -E '^issueops|^gates-ledger'
    Expected: gates-ledger, issueops, issueops-abandon, issueops-clean, issueops-cleanup, issueops-complete, issueops-create-issue, issueops-create-pr, issueops-docs, issueops-implement, issueops-plan, issueops-prepare, issueops-remote-write, issueops-review, issueops-sync-issue, issueops-sync-pr, issueops-verify 만 있고 issueops-branch-worktree 없음
    Evidence: .agent-harness/evidence/task-16-links.txt
  Scenario: race 통과
    Channel: bash
    Steps: go test -race ./... -count=1 | tail -5
    Expected: FAIL 0건
    Evidence: .agent-harness/evidence/task-16-race.txt
  ```

  **Commit**: NO (수정이 생기면 해당 태스크 커밋 메시지로 fixup 커밋)

- [x] **T17. 일회용 저장소 E2E 프로브** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-17-e2e-1.log`(정방향 10단계 + 일시 중단·재개), `task-17-e2e-2.log`(죽은 홀더 인수), `task-17-e2e-3.log`(dirty 워크트리 폐기 + draft PR 폐기), `task-17-e2e-4.log`(모호·새 사이클·저장소 밖 + 정리 확인). 네 파일 모두 마지막 줄이 PASS다. 프로브 저장소는 삭제했고 실제 state에 오염이 없다.

  **프로브가 잡은 결함 2건**:
  1. **인수 체인이 첫 걸음에서 끊겼다.** `execution replace --preview`가 active lease에서 `next_command`를 비워 돌려줘, 죽은 홀더를 인수하려는 세션이 다음 명령(`--revoke`)을 스스로 지어내야 했다. 그것은 라우터가 금지하는 추측이다. `previewExecutionReplacement`에 active lease 분기를 더해 revoke 단계를 template으로 렌더하게 고쳤고 `TestReplacePreviewRendersTheRevokeStepForAnActiveLease`로 고정했다.
  2. **`issueops-abandon`의 게이트 표에 두 게이트가 빠져 있었다**: `worktree_clean`과 `requester_occupies_worktree`. 실제 폐기 시도가 그 둘로 막혔다. 표에 추가했다.

  **실행 결과와 계획 대비 편차**: 시나리오 1의 구현 단계는 planner 세션이 직접 수행했고, 두 번째 세션(tmux + `claude -p`)은 계획이 요구한 **재개 지점**에서 실제로 lease를 이어받았다(generation 2, 다른 session id). 그 세션이 종료하면서 죽은 홀더가 생겨 시나리오 2가 실제 상황으로 이어졌다. `--mode auto`는 Orca가 ready였지만 프로브 저장소가 Orca에 등록돼 있지 않아 `fallback_code=repo_unresolved`로 direct를 골랐다 — 폴백 경로가 실제로 동작함을 확인한 셈이다.

  **What to do**: 메모리 `issueops-e2e-probe-method` 절차로 GitHub 일회용 저장소를 만들고, 새 스킬 순서대로 전 구간을 실제로 돌린다. 각 시나리오는 tmux 세션의 로그와 `issueops next --json` 출력을 evidence로 남긴다. 프롬프트는 자기완결적으로 쓴다(ISSUEOPS_ID, 워크트리, 실행할 스킬 이름, 보고 형식).
  ```bash
  gh repo create m16khb/issueops-ten-stage-probe --private --clone
  # 시드 커밋 후:
  ```
  시나리오는 네 개다. T0b 스파이크가 채택·권한·fingerprint를 이미 실측했으므로 여기서는 새 스킬로 전 구간을 돌린다.
  1. **정방향 10단계와 일시 중단·재개**: planner 세션(이 세션)에서 `issueops-create-issue` → `issueops-prepare`(출력된 실행 명령 확보) → tmux로 두 번째 세션을 워크트리에서 `claude -p --permission-mode bypassPermissions --model sonnet`로 띄워 프롬프트 "issueops 스킬을 실행하고 next가 제안하는 대로 진행" → `issueops-plan`(`## 프로젝트 문서 확인`, `issueops-review` 1라운드 이상, `gates-ledger` 원장 생성) → `issueops-implement` 중간에 `issueops-abandon` 일시 중단 → 세 번째 tmux 세션(가능하면 `codex` 호스트, 없으면 claude)이 `next`의 `resume`대로 `next_command` 체인을 따라 이어받아 구현을 마친다 → `issueops-clean` → `issueops-docs`(`updated` 또는 `no-change`) → `issueops-verify`(`issueops-review --target diff`) → `atomic-commit-push` → `issueops-create-pr` → `issueops-complete` → `gh pr ready` + `gh pr merge --squash` → planner 세션에서 `issueops-cleanup`. 각 경계에서 `next`의 `stage.key`가 표대로 바뀌는지 기록한다.
  2. **죽은 홀더 인수**: 두 번째 사이클에서 홀더 세션을 `tmux kill-session`으로 죽인 뒤 다른 세션이 `next`에서 `takeover`를 받고 revoke → finalize-preview → finalize → claim 체인을 완주한다.
  3. **dirty 워크트리 폐기와 draft PR 폐기**: 같은 두 번째 사이클에서 파일을 고친 상태로 `issueops-abandon` 폐기 → 세 선택지 제시 → "WIP 커밋·푸시 후 폐기" 선택 → release → `cleanup abandon --close-issue --delete-remote-branch --preview` → apply → record·워크트리·로컬 브랜치 삭제, 이슈 closed, 원격 브랜치 부재 readback. 세 번째 사이클은 pr phase까지 간 뒤 `cleanup abandon --close-pr --close-issue --delete-remote-branch --preview`가 `remote_effects` 세 개와 관측값을 보여 주고 apply 뒤 PR state CLOSED를 readback한다.
  4. **모호·새 사이클·무관 위치**: source root에 active 사이클이 둘인 상태에서 `next` → `ambiguous`와 candidates → 라우터 선택지 4로 새 사이클 시작이 `issueops-create-issue`로 이어지는지; `/tmp`에서 `next` → `none` + warning.
  종료: `gh repo delete m16khb/issueops-ten-stage-probe --yes`, 로컬 클론·워크트리 삭제, 남은 record는 `cleanup abandon`으로 회수, `issueops list --json`에 probe 경로가 없음을 확인.

  **Must NOT do**: 사용자의 실제 저장소 사용, 프로브 저장소 삭제 생략.

  **Recommended Agent**: deep. 다중 세션과 원격 상태를 다룬다.

  **Parallelization**: Can Parallel: NO | Wave 8 | Blocks: T18, F1 | Blocked By: T16

  **References**: 메모리 `issueops-e2e-probe-method`, `skills/issueops-abandon/SKILL.md`, `skills/issueops-prepare/SKILL.md` 실행 명령 절.

  **Acceptance Criteria**:
  - [ ] 시나리오 4개 각각 `.agent-harness/evidence/task-17-e2e-<n>.log`가 있고 마지막 줄이 `PASS`다.
  - [ ] `./bin/agent-harness issueops list --json | rg -c "issueops-ten-stage-probe"`가 0(회수 완료).

  **QA Scenarios**:
  ```
  Scenario: 정방향 단계 전이
    Channel: tmux + bash
    Steps: 각 경계에서 ./bin/agent-harness issueops next --cwd <worktree> --json | python3 -c "...print(stage.key)"
    Expected: 순서대로 prepare, plan.write, plan.design, plan.review, plan.handoff, (orca면 claim), implement.enter, implement, claim(일시 중단 뒤), implement, clean, docs, verify, commit-push, pr.create, pr.complete, done
    Evidence: .agent-harness/evidence/task-17-e2e-1.log
  Scenario: 인수 체인
    Channel: tmux + bash
    Steps: 홀더 세션 kill 후 세 번째 세션에서 next → takeover_command 실행 → 돌려준 next_command만 4회
    Expected: 마지막 claim 결과 lease.status active, holder_session이 세 번째 세션
    Evidence: .agent-harness/evidence/task-17-e2e-2.log
  ```

  **Commit**: NO (발견한 결함은 해당 태스크 범위의 fixup 커밋 + CAUTIONS lesson 추가 후 T16 재실행)

- [~] **T18. 사용자 전역 CLAUDE.md 문단과 메모리 갱신 (사용자 승인 필요)** — 메모리는 완료(2026-09-05), 전역 CLAUDE.md는 **사용자 승인 대기**.

  메모리: `issueops-ten-stage-plan.md`로 이름과 내용을 갱신했고(9단계 시절 파일은 삭제), `issueops-worktree-prepare-removed.md`에 10단계 재편 문단을 덧붙였으며, `MEMORY.md` 색인을 고쳤다. 증거: `.agent-harness/evidence/task-18-memory.txt`.

  전역 CLAUDE.md: 교체 문안을 준비했다(스크래치패드 `global-claude-md-proposal.md`). 승인 전에는 편집하지 않는다. 현재 문단은 `issueops worktree prepare`를 **금지 문맥으로만** 언급하므로 그 자체는 틀리지 않지만, "implement-phase contract is owned by `issueops-implement`"가 10단계 기준으로 불완전하다.

  **What to do**:
  1. `~/.claude/CLAUDE.md`의 "IssueOps integration (updated 2026-08-31)" 문단을 10단계 기준으로 바꾸는 교체 문안을 제시하고, 사용자가 승인하면 편집한다. 문안 골격: branch·worktree는 `issueops-prepare`가 만들고 lease는 부여하지 않는다. implementer 세션은 `issueops-prepare`가 출력한 명령(또는 `orca terminal create --worktree path:<wt> --command "..."`)으로 띄우며, 그 세션의 `issueops` 스킬이 `next`로 단계를 판별한다. `orca worktree create`는 여전히 금지. 탈출·재개는 `issueops-abandon`.
  2. 메모리 파일 `issueops-worktree-prepare-removed.md`의 "implement 단계 계약은 `skills/issueops-implement/SKILL.md`가 소유" 문장 뒤에 10단계 재편 날짜와 새 스킬 이름을 추가하고, `MEMORY.md`에 새 항목 "issueops 10단계 재편(2026-09)"을 추가한다.

  **Must NOT do**: 사용자 승인 전 `~/.claude/CLAUDE.md` 편집.

  **Recommended Agent**: quick.

  **Parallelization**: Can Parallel: NO | Wave 8(선택) | Blocks: F4 | Blocked By: T17

  **References**: `~/.claude/CLAUDE.md` Orca runtime 절, 메모리 디렉터리 `MEMORY.md`.

  **Acceptance Criteria**:
  - [ ] 사용자 승인 기록(대화)과 `rg -n "issueops-prepare|issueops-abandon" ~/.claude/CLAUDE.md` 각 1건 이상.

  **QA Scenarios**:
  ```
  Scenario: 전역 지침에 낡은 명령 없음
    Channel: bash
    Steps: rg -n "issueops worktree prepare|issueops-branch-worktree" ~/.claude/CLAUDE.md
    Expected: 0건
    Evidence: .agent-harness/evidence/task-18-global.txt
  Scenario: 메모리 색인
    Channel: bash
    Steps: rg -n "10단계" ~/.claude/projects/-Users-habin-workspace-agent-harness/memory/MEMORY.md
    Expected: 1건
    Evidence: .agent-harness/evidence/task-18-memory.txt
  ```

  **Commit**: NO (저장소 밖)

- [x] **T19. `issueops-clean` 신설 (5단계: AI slop 정리)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-19-prompt.txt`(Step 1~5 이동), `task-19-turing.txt`. 정리 프롬프트 전체를 옮기고 원본 레퍼런스를 지웠으며, 라우터의 레퍼런스 표에서도 그 행을 뺐다. 스킬 목록이 골든에 들어 있어 골든도 재생성했다.

  **Files:**
  - Create: `skills/issueops-clean/SKILL.md`
  - Create: `skills/issueops-clean/agents/openai.yaml`
  - Modify: `skills/turing/SKILL.md:396` (`skills/issueops/references/ai-slop-clean.md` 링크를 `skills/issueops-clean/SKILL.md`로)
  - Delete: `skills/issueops/references/ai-slop-clean.md` (`git rm`)

  **What to do**:
  1. frontmatter description: "Run the IssueOps ai-slop-clean stage on the canonical worktree: confirm the phase, remove lazy agent residue from the task diff one pass at a time while preserving behavior, measure before and after with shannon, re-run the gate ledger and focused verification, and record the cleanup evidence that seals the change fingerprint. Use when `issueops next` reports clean, or when the user says 'AI slop 정리', 'slop 치워줘', '정리 단계'."
  2. 절 순서: `## 이 스킬이 맞는지 확인`(`next`의 `stage.key == clean`; `implement`면 `issueops-implement`의 출구 `phase --to ai-slop-clean`이 아직이므로 그 스킬로 돌려보낸다) → `## 정리 프롬프트`(현행 `references/ai-slop-clean.md`의 "Prompt" 코드 블록 전체를 옮긴다: Scope boundary의 `CLEANUP_BOUNDARY`, Step 1 lock behavior, Step 2 분류표, Step 3 pass 순서, Step 4 contract check, Step 5 fresh evidence, Output 형식) → `## 측정`(`shannon`으로 정리 전후 SNR·중복·boilerplate 비율을 기록한다. 측정 없이 개선을 주장하지 않는다) → `## 재검증과 증거 확정`(`gates-ledger`로 `gates check --write` 재실행, 변경 범위의 focused 테스트 재실행, `git -C "$WORKTREE" diff --check`, turing report 확정. report는 워크트리 안 정규 파일이어야 하고 형식은 `turing` 스킬의 보고 계약을 따르며 side effect 목록과 성능 측정값을 담는다) → `## 기록` → `## 출구`("다음: issueops-docs") → `## 나쁜 예` → `## 검증`.
  3. `## 기록` 코드 블록:
     ```bash
     agent-harness issueops ai-slop-clean record --id "$ISSUEOPS_ID" \
       --category "dead-code" --category "duplication" \
       --verification "go test ./internal/... -count=1 → ok" \
       $RECORD_ACTOR_FLAGS --json
     ```
     category 값은 정리 프롬프트 Step 2 분류표의 여섯 이름(`dead-code`, `duplication`, `needless-abstraction`, `boundary-violation`, `weak-artifact`, `unsupported-claim`) 중 실제로 제거한 것만 쓴다. fingerprint는 `phase --to ai-slop-clean` 진입 때 봉인되고(`issueops_phase.go:146-153`) 이 기록이 현재 diff로 갱신한다(`issueops_ledger_recorders.go:108` → `issueops_phase_refresh.go:12-20`). fingerprint에는 gates.md·report·문서까지 모든 변경 파일이 들어가므로(`implementation/evidence.go:52-74`), 기록 뒤 파일을 고치면 `ai_slop_clean_stale`로 pr 진입과 create-pr이 막힌다. 5단계 `issueops-docs`가 문서를 고쳤으면 그 단계가 이 명령을 다시 실행해 재봉인한다(T26). 그 외에 파일이 바뀌면 `next`가 이 단계로 되돌리며 그것이 의도된 회귀다.
  4. `## 나쁜 예`: 범위 밖 리팩터, 검증 없이 record, 정리 뒤 재검증 생략, 측정 없는 "더 깔끔해졌다" 주장, source checkout 편집, 근거 없는 "unused" 삭제, 이 단계에서 커밋.
  5. `skills/turing/SKILL.md:396`의 링크를 교체하고 `git rm skills/issueops/references/ai-slop-clean.md`.

  **Must NOT do**: 운영 문서 수정·docs review(5단계), implementation review·schema evidence(6단계), 커밋·푸시, 정리 범위 확장.

  **Recommended Agent**: deep. 프롬프트 이동과 fingerprint 봉인 규칙을 정확히 옮겨야 한다.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T26, T11 | Blocked By: T0b, T22

  **References**:
  - `skills/issueops/references/ai-slop-clean.md` 전체(이동 원문).
  - `internal/adapter/issueops/issueops_phase_ledger.go:92-110` (ai-slop-clean 완료 요구 항목).
  - `internal/adapter/issueops/implementation/evidence.go:52-106` (fingerprint 경로 집합과 내용 해시), `issueops_phase.go:146-153` (진입 봉인), `issueops_phase_refresh.go:12-20` (기록 시 갱신), `issueops_pr_readiness_strict.go:76-78` (`ai_slop_clean_stale`).
  - `skills/shannon/SKILL.md`, `skills/turing/SKILL.md:390-400`.

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-clean`와 `verify-skill-shell.py` 통과.
  - [ ] `rg -n "ai-slop-clean record|shannon|gates-ledger|issueops-docs|CLEANUP_BOUNDARY|stale|turing report" skills/issueops-clean/SKILL.md` 각 1건 이상, `rg -n "issueops-verify" skills/issueops-clean/SKILL.md` 0건.
  - [ ] `test ! -f skills/issueops/references/ai-slop-clean.md`, `rg -n "references/ai-slop-clean" skills/ .agent-harness --glob '!.agent-harness/turing/**' --glob '!.agent-harness/issues/**' --glob '!.agent-harness/plans/**'` 0건.

  **QA Scenarios**:
  ```
  Scenario: 프롬프트 다섯 단계가 이동됨
    Channel: bash
    Steps: rg -c "Step 1|Step 2|Step 3|Step 4|Step 5" skills/issueops-clean/SKILL.md
    Expected: 5
    Evidence: .agent-harness/evidence/task-19-prompt.txt
  Scenario: turing 링크 교체
    Channel: bash
    Steps: rg -n "issueops-clean/SKILL.md" skills/turing/SKILL.md
    Expected: 1건
    Evidence: .agent-harness/evidence/task-19-turing.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-clean as the AI slop cleanup stage` | Files: 위 파일

- [x] **T20. `issueops-verify` 신설 (7단계: 검증)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-20-order.txt`, `task-20-exit.txt`. 절 1~4가 번호 순서대로 있고 금지 문자열은 0건이다.

  **실행 결과와 계획 대비 편차**: `verify-work`의 자리표시자를 `<검증 명령>`이 아니라 `"$VERIFY_COMMAND"`로 썼다. shell 검사기가 `<`를 리다이렉션으로 읽어 거부한다.

  **Files:**
  - Create: `skills/issueops-verify/SKILL.md`
  - Create: `skills/issueops-verify/agents/openai.yaml`

  **What to do**:
  1. frontmatter description: "Run the IssueOps verify stage on the sealed diff without touching any file: re-run the gate ledger and the repository's verification battery read-only, record the conditional schema evidence, run the adversarial implementation review through issueops-review, re-check compatibility against the real diff, and prove strict PR readiness leaves only commit and push. Use when `issueops next` reports verify, or when the user says '검증 단계', '검증해줘', '리뷰 돌리고 검증'."
  2. 절 순서: `## 이 스킬이 맞는지 확인`(`verify`) → `## 파일을 만지지 않는다` → `## 1 읽기 전용 재검증` → `## 2 스키마 실측과 기록` → `## 3 구현 리뷰` → `## 4 호환성 재확인과 readiness` → `## 출구` → `## 나쁜 예` → `## 검증`.
  3. `## 파일을 만지지 않는다`: change fingerprint는 `git diff <base>..HEAD`와 `git status`의 모든 경로 내용 해시라(`implementation/evidence.go:52-106`) 어떤 파일이든 바꾸면 4단계 봉인과 5단계 판정이 stale이 된다. 이 단계의 명령은 전부 읽기이거나 record 기록이다. 검증이 실패해 코드를 고쳐야 하면 4단계로 돌아가 정리·확정·재봉인·문서 반영을 다시 밟는다. `next`가 `clean`으로 되돌리는 것이 그 신호다.
  4. `## 1 읽기 전용 재검증`: `gates-ledger`로 `gates check --file ... --cwd "$WORKTREE" --workspace-root "$WORKTREE" --json`(`--write` 없이. EVIDENCE는 4단계가 채웠다); 저장소가 정한 검증 battery(이 저장소는 `AGENTS.md` §9, 대상 repo는 그 repo의 test 명령); endpoint·DTO·OpenAPI가 바뀌었으면 `.agent-harness/OPEN_API_SPEC.md` 게이트와 `agent-harness api-doc check --json`; read-only 검증은 `agent-harness verify-work --json -- <명령>`으로 evidence를 남긴다. 실행하지 않은 검증을 pass로 적지 않는다. 미충족 게이트가 있으면 4단계로 돌아간다.
  5. `## 2 스키마 실측과 기록`: 현행 `skills/issueops-implement/SKILL.md` "스키마 실측 근거" 절 원문을 옮긴다(조건부, `codd`·DB MCP, 카탈로그 추정치, 운영 DB 전수 스캔 금지, `schema-evidence record --measurement --source` 또는 `--waive --waiver-rationale`). 기록 명령은 파일을 바꾸지 않는다.
  6. `## 3 구현 리뷰`: `issueops-review`를 `--target diff`로 호출한다. revise면 지적을 고쳐야 하므로 4단계로 돌아간다. pass만 통과한다. "direct 모드는 이 게이트 대상이 아니다"라는 문장은 쓰지 않는다(T24로 모든 모드가 대상이다).
  7. `## 4 호환성 재확인과 readiness`: 구현 diff가 plan의 compatibility review(backward compatibility, side effect, rollback)와 다르면 `agent-harness issueops compatibility review --id ... --approved`를 다시 기록해 durable 판정을 최신으로 맞춘다. 그다음 `agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json`의 `missing`이 `worktree_clean`, `upstream`, `upstream_fetch`, `upstream_synced`의 부분집합인지 확인한다. `gates_incomplete:*`나 `*_stale`이 남으면 `next`가 가리키는 단계로 돌아간다.
  8. `## 출구`: "다음: 7단계 커밋·푸시. `atomic-commit-push`로 plan.md, gates.md, turing report, 문서, 구현을 커밋·푸시하고, `next`가 렌더한 `phase --to pr`를 실행한다."
  9. `## 나쁜 예`: 이 단계에서 파일 수정, 실행하지 않은 검증을 pass로 기록, 리뷰 없이 pass 기록, 운영 DB `COUNT(*)` 전수 스캔, direct라서 리뷰 생략, 리뷰 revise를 이 단계에서 고치고 재리뷰 생략, 이 단계에서 커밋.

  **Must NOT do**: 파일 변경, 커밋·푸시, PR 생성, 리뷰 루프 절차 복사, 운영 문서 대조(5단계 소유).

  **Recommended Agent**: deep.

  **Parallelization**: Can Parallel: YES(T10·T12와) | Wave 5 | Blocks: T11 | Blocked By: T0b, T1, T21, T22, T24, T26

  **References**:
  - `skills/issueops-implement/SKILL.md` "Publication evidence gates", "Implementation review gate" 절(이동 원문).
  - `skills/issueops/references/execution.md` "Publication Evidence Gates", "Implementation Review Gate (orca mode)"(T24 뒤 제목을 "(all modes)"로).
  - `internal/adapter/issueops/implementation/evidence.go:76-106`, `internal/adapter/issueops/issueops_pr_readiness_strict.go:12-70`.
  - `.agent-harness/operations/guides/issueops-execution.md:174`(report 커밋 시점, #153), `skills/issueops-complete/SKILL.md` "증거를 만드는 순서".
  - `.agent-harness/AGENT_WORKFLOW.md` Verify 절(`verify-work`), `.agent-harness/OPEN_API_SPEC.md`, `skills/turing/SKILL.md`, `skills/gates-ledger/SKILL.md`(T22), `skills/issueops-review/SKILL.md`(T21).

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과.
  - [ ] `rg -n "gates-ledger|issueops-review|schema-evidence record|compatibility review|pr-readiness --id|verify-work|OPEN_API_SPEC" skills/issueops-verify/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "brooks를 fresh 서브에이전트로 실행|direct 모드는 이 게이트 대상이 아니다|project-docs-review record|ai-slop-clean record|--write" skills/issueops-verify/SKILL.md` 0건.
  - [ ] `rg -n "^## [1-4] " skills/issueops-verify/SKILL.md | wc -l`이 4이고 번호 순서다.

  **QA Scenarios**:
  ```
  Scenario: 파일 불변 규칙이 fingerprint 근거와 함께 있음
    Channel: bash
    Steps: rg -n "evidence.go:52-106|내용 해시" skills/issueops-verify/SKILL.md
    Expected: 1건 이상
    Evidence: .agent-harness/evidence/task-20-order.txt
  Scenario: 출구가 커밋·푸시로 이어짐
    Channel: bash
    Steps: sed -n '/## 출구/,/## 나쁜 예/p' skills/issueops-verify/SKILL.md | rg -c "atomic-commit-push|phase --to pr"
    Expected: 2 이상
    Evidence: .agent-harness/evidence/task-20-exit.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-verify as the verification stage before commit` | Files: 위 2개

- [x] **T26. `issueops-docs` 신설 (6단계: 프로젝트 문서 반영)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-26-verdicts.txt`(두 판정 형식), `task-26-order.txt`(재봉인이 판정 기록보다 앞). 절 다섯 개가 번호 순서대로 있다.

  **Files:**
  - Create: `skills/issueops-docs/SKILL.md`
  - Create: `skills/issueops-docs/agents/openai.yaml`

  **What to do**:
  1. frontmatter description: "Reflect a finished IssueOps implementation into the project's operating documents: route the diff to the .agent-harness documents it touches, check both directions (did the change break a documented rule, and did it create a decision, pitfall, command, or convention the documents do not know yet), update ADR, CAUTIONS, CONVENTIONS, or ARCHITECTURE through the project_docs MCP contract, re-seal the ai-slop-clean fingerprint, and record the project-docs-review verdict. Use when `issueops next` reports docs, or when the user says '문서 반영', 'ADR 남겨줘', '주의사항 기록', 'update the project docs'."
  2. 절 순서: `## 이 스킬이 맞는지 확인`(`docs`) → `## 왜 이 단계가 4단계 뒤에 오는가` → `## 1 라우팅` → `## 2 양방향 대조` → `## 3 문서 수정` → `## 4 재봉인` → `## 5 판정 기록` → `## 출구` → `## 나쁜 예` → `## 검증`.
  3. `## 왜 이 단계가 4단계 뒤에 오는가`: 문서에 남길 결정과 함정은 정리가 끝난 최종 diff에서 확정된다. 문서 수정은 파일 변경이라 4단계 봉인을 stale로 만들므로 이 단계가 재봉인을 소유한다(`issueops_ledger_recorders.go:108` → `issueops_phase_refresh.go:12-20`).
  4. `## 1 라우팅`: MCP `project_docs_route`에 구현 diff 요약(변경 파일 목록, 새 명령·플래그·구조, 만난 함정)을 넣어 문서를 고른다. MCP가 없으면 `agent-harness docs --json`의 required-doc 목록에서 CONSTITUTION, ARCHITECTURE(해당 모듈), CONVENTIONS, CAUTIONS(색인과 해당 모듈), ADR, TESTING을 읽는다. `project_docs_read`로 현재 content와 SHA를 받는다.
  5. `## 2 양방향 대조`: 현행 `skills/issueops-implement/SKILL.md` "project-doc 반영 판정" 절의 두 방향을 옮긴다. 문서→구현: 어겼으면 문서가 아니라 구현을 고친다. 이때는 4단계로 돌아간다. 구현→문서: 새 결정, 재발 함정, 새 명령·컨벤션·구조를 찾는다. plan의 `## 적용되는 결정과 주의사항` 절과 대조해 계획 때 몰랐던 항목을 evidence에 적는다.
  6. `## 3 문서 수정`: 기존 `project-docs-update` 스킬을 호출한다. 결정은 `project_docs_append(kind=adr)`, 함정은 `project_docs_append(kind=caution)`, 나머지는 `project_docs_revise`(SHA-CAS, 한 문서씩). 하네스 저장소에서는 `.agent-harness/*.md`를 고친 뒤 `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update -count=1`로 골든을 재생성한다(CAUTIONS Update workflow 5). 대상 repo에 그 골든이 없으면 이 줄은 해당 없음이다.
  7. `## 4 재봉인`: 문서를 하나라도 고쳤으면 `agent-harness issueops ai-slop-clean record --id "$ISSUEOPS_ID" --category ... --verification ... $RECORD_ACTOR_FLAGS --json`을 다시 실행한다. category는 4단계와 같고 verification은 4단계의 결과를 그대로 적는다(문서만 바뀌었으므로 코드 검증 결과는 같다). 고친 문서가 없으면 재봉인하지 않는다.
  8. `## 5 판정 기록`:
     ```bash
     agent-harness issueops project-docs-review record --id "$ISSUEOPS_ID" \
       --verdict updated --doc ".agent-harness/CAUTIONS.md" --doc ".agent-harness/cautions/<module>.md" \
       --evidence "<무엇을 대조했고 왜 이 문서를 고쳤는가>" $RECORD_ACTOR_FLAGS --json
     # 고칠 것이 없을 때
     agent-harness issueops project-docs-review record --id "$ISSUEOPS_ID" \
       --verdict no-change --evidence "<대조한 문서 목록과 판단>" $RECORD_ACTOR_FLAGS --json
     ```
     `updated`는 `--doc` 경로가 실제 변경 집합 안에 있어야 통과하고, `no-change`는 `--doc`을 받지 않는다. 이 기록이 변경 집합 fingerprint를 봉인하므로 이후 diff가 바뀌면 `project_docs_review_stale`이고 `next`가 이 단계로 되돌린다.
  9. `## 출구`: "다음: issueops-verify".
  10. `## 나쁜 예`: 문서를 고치지 않고 `updated`, 변경 집합 밖 문서를 `--doc`에 적기, 문서 수정 뒤 재봉인 생략, 구현 위반을 문서를 고쳐 덮기, ADR을 수정하기(append만), `no-change`에 대조 근거 없음, 이 단계에서 코드 수정.

  **Must NOT do**: 코드 수정, ADR 기존 항목 수정, 판정 기록 없이 종료, 커밋.

  **Recommended Agent**: deep. 두 방향 대조와 SHA-CAS 갱신, 재봉인 순서가 얽힌다.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T20, T11 | Blocked By: T0b, T19, T22

  **References**:
  - 설계 요약 10.
  - `skills/issueops-implement/SKILL.md` "project-doc 반영 판정" 절(이동 원문), `skills/project-docs-update/SKILL.md`.
  - `.agent-harness/AGENT_WORKFLOW.md` "MCP Usage Rule", "`.agent-harness` Upkeep via MCP".
  - `internal/adapter/issueops/issueops_phase_refresh.go:12-20`, `issueops_ledger_recorders.go:108`, `issueops_pr_readiness_strict.go:62-66`(`project_docs_review_stale`).
  - `.agent-harness/CAUTIONS.md` Update workflow 5(골든 재생성).

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과.
  - [ ] `rg -n "project_docs_route|project_docs_read|project_docs_append|project_docs_revise|project-docs-update|ai-slop-clean record|project-docs-review record|issueops-verify" skills/issueops-docs/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "^## [1-5] " skills/issueops-docs/SKILL.md | wc -l`이 5이고 `## 4 재봉인`이 `## 5 판정 기록`보다 앞에 있다.

  **QA Scenarios**:
  ```
  Scenario: updated와 no-change 두 기록 형식이 있음
    Channel: bash
    Steps: rg -c "verdict updated|verdict no-change" skills/issueops-docs/SKILL.md
    Expected: 2 이상
    Evidence: .agent-harness/evidence/task-26-verdicts.txt
  Scenario: 재봉인이 판정 기록보다 먼저
    Channel: bash
    Steps: rg -n "^## 4 재봉인|^## 5 판정 기록" skills/issueops-docs/SKILL.md
    Expected: 두 줄이 이 순서로 출력
    Evidence: .agent-harness/evidence/task-26-order.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-docs as the project document reflection stage` | Files: 위 2개

- [x] **T21. `issueops-review` 신설 (공용: 적대 리뷰 실행과 판정 기록)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-21-targets.txt`, `task-21-stale.txt`. 수용 기준 전부 통과(호스트 전용 pseudo-API 0건, 모델 이름 0건).

  **Files:**
  - Create: `skills/issueops-review/SKILL.md`
  - Create: `skills/issueops-review/agents/openai.yaml`

  **What to do**:
  1. frontmatter description: "Run an adversarial brooks review as a fresh sub-agent on an IssueOps plan or implementation diff, then record the verdict in the durable ledger with the plan digest or change fingerprint sealed. Owns the revise and stop loop rules shared by the plan and verify stages. Use when `issueops-plan` or `issueops-verify` needs a review, or when the user says '계획 검토', '구현 리뷰', 'brooks 돌려줘', 'devil's advocate'."
  2. 절 순서: `## 입력` → `## 실행` → `## 기록` → `## 루프 규칙` → `## 이슈 반영` → `## 나쁜 예` → `## 검증`.
  3. `## 입력`: `--target plan|diff`. plan이면 `status --json`의 `plan_path` 파일 전체, diff면 `git -C "$WORKTREE" diff "$BASE_SHA"`와 plan 파일. 리뷰어 모델과 effort는 `agent-harness issueops next --json`의 `review.model`·`review.effort`에서 읽는다. 이 값은 코드가 소유하는 host별 planner 기본값(`internal/port/orca.go` `IssueOpsPlannerDefaults`)이며, 스킬 본문에 모델 이름 표를 복사하지 않는다. `review.model`이 비어 있으면(예: omo) 진행하지 말고 사용자에게 모델을 묻는다.
  4. `## 실행`: 현재 호스트의 delegation 도구로 brooks를 fresh 컨텍스트에 띄운다. 프롬프트에 대상 전체, 성공 기준, 관련 ADR 링크, 출력 계약(verdict, 가장 위험한 결함과 반증 실험, gate별 finding, 더 작은 계획), 그리고 코드베이스 존중 렌즈 네 개(기존 구현을 재사용할 수 있었는데 새로 만들지 않았는가, 성능 회귀 가능성, 계약 표면의 하위 호환성, 파일·원격·상태 side effect)를 넣는다. 저자 세션이 gate 다섯 개를 직접 돌리는 것은 리뷰가 아니라는 문장을 `skills/brooks/SKILL.md` "Subagent-Only Mandate"에서 옮긴다. 호스트 전용 pseudo-API 이름을 쓰지 않는다.
  5. `## 기록` 코드 블록:
     ```bash
     # --target plan
     agent-harness issueops devils-advocate review --id "$ISSUEOPS_ID" \
       --verdict pass --reviewer-context subagent \
       --finding "<무엇을 공격했고 왜 살아남았는가>" $RECORD_ACTOR_FLAGS --json
     # --target diff
     agent-harness issueops implementation-review record --id "$ISSUEOPS_ID" \
       --verdict pass --finding "<finding>" --evidence "<evidence>" \
       --reviewer-host "$HOST" --reviewer-model "$REVIEWER_MODEL" --reviewer-effort "$REVIEWER_EFFORT" \
       $RECORD_ACTOR_FLAGS --json
     ```
  6. `## 루프 규칙`: 1·2라운드는 전체 검토, 3라운드부터 직전 판정 이후 delta 검토. revise면 대상을 고치고 다시 실행한다. stop이면 plan은 호출자(`issueops-plan`)가 `regress`를 실행하고, diff는 blocker를 보고하고 중단한다. pass는 finding 1건 이상이 있어야 기록된다. `--waive`는 override이고 rationale이 필수이며 "반영했다"의 뜻으로 쓰지 않는다. 판정은 plan sha256 또는 change fingerprint에 묶이므로 판정 뒤 대상을 고치면 `devils_advocate_review_stale`·`implementation_review_stale`이 되어 다시 실행해야 한다. `reviewer_context`와 `reviewer_*`는 감사 필드이지 게이트 조건이 아니다.
  7. `## 이슈 반영`: plan 판정을 이슈에 남길 때는 `issueops-remote-write`로 `remote reflect-devils-advocate --id "$ISSUEOPS_ID" --confirm`을 실행한다.
  8. `## 나쁜 예`: 인라인 자기 검토를 `--reviewer-context subagent`로 기록, 리뷰 없이 pass, revise를 waive로 닫기, 판정 뒤 대상 수정 후 재검토 생략, 리뷰어 모델 자기신고를 게이트로 오해, diff 리뷰에 plan을 주지 않음.

  **Must NOT do**: 대상 파일 수정, 기록 명령 외 durable mutation, 호스트 전용 도구 이름.

  **Recommended Agent**: deep.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8, T20, T11 | Blocked By: T0

  **References**:
  - `skills/brooks/SKILL.md` "Subagent-Only Mandate", "Output Contract".
  - `.agent-harness/adr/decisions/2026-08-28-issueops-devils-advocate-plan-binding.md` Decision 4·5.
  - `skills/issueops-implement/SKILL.md` "Implementation review gate" 절(이동 원문).
  - `skills/issueops/references/execution.md` "Host-Aware Owner Model Defaults", "Implementation Review Gate (orca mode)".
  - `internal/adapter/issueops/issueops_readiness.go:161-185`, `internal/adapter/issueops/issueops_implementation_review.go:80-90`.
  - `internal/adapter/skillcontract/skill_contract_test.go:82-86` (호스트 중립 위임 문구 규칙).

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과.
  - [ ] `rg -n "reviewer-context subagent|implementation-review record|regress|--waive|reflect-devils-advocate" skills/issueops-review/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "재사용|성능|하위 호환|side effect" skills/issueops-review/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "task\(subagent_type=|Task tool|Agent tool" skills/issueops-review/SKILL.md` 0건, `rg -n "current host's delegation tool|현재 호스트의 delegation 도구" skills/issueops-review/SKILL.md` 1건 이상.
  - [ ] `rg -n "claude-opus-5|gpt-5.6-sol|claude-sonnet-5" skills/issueops-review/SKILL.md` 0건(모델 기본값은 코드가 소유하고 `next`의 `review`로 읽는다).

  **QA Scenarios**:
  ```
  Scenario: 두 target의 기록 명령이 모두 있음
    Channel: bash
    Steps: rg -c "devils-advocate review --id|implementation-review record --id" skills/issueops-review/SKILL.md
    Expected: 2 이상
    Evidence: .agent-harness/evidence/task-21-targets.txt
  Scenario: stale 규칙 명시
    Channel: bash
    Steps: rg -n "devils_advocate_review_stale|implementation_review_stale" skills/issueops-review/SKILL.md
    Expected: 각 1건 이상
    Evidence: .agent-harness/evidence/task-21-stale.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-review as the shared adversarial review runner` | Files: 위 2개

- [x] **T22. `gates-ledger` 신설 (공용: 게이트 원장)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-22-mismatch.txt`(EXPECT 불일치 → exit 1, `check_error: expect not matched`), `task-22-paths.txt`. smoke(init→check --write→report) 종료 코드 0.

  **Files:**
  - Create: `skills/gates-ledger/SKILL.md`
  - Create: `skills/gates-ledger/agents/openai.yaml`

  **What to do**:
  1. frontmatter description: "Create, check, and report task gate ledgers with the agent-harness gates CLI: turn acceptance criteria into G-numbered CHECK and EXPECT gates in `.agent-harness/issues/<n>/gates.md` or `.agent-harness/gates/<scope>.md`, fill EVIDENCE by running the checks through the command policy, and abandon gates honestly. Use when `issueops-plan`, `issueops-implement`, `issueops-clean`, `issueops-verify`, or `turing` needs a gate ledger, or when the user says '게이트 원장', 'gates 만들어줘', '수용 기준 체크'."
  2. 절 순서: `## 경로 규칙` → `## 만들기` → `## 검사` → `## 상태와 보고` → `## 포기` → `## IssueOps와의 관계` → `## 나쁜 예` → `## 검증`.
  3. `## 경로 규칙`: issue 번호가 있으면 `<worktree>/.agent-harness/issues/<n>/gates.md`, 없으면 `.agent-harness/gates/<scope>.md`. 같은 번호의 canonical·legacy 원장이 함께 있으면 pr 진입이 `duplicate_issue_artifact:<n>`으로 막힌다(ADR 2026-08-22).
  4. `## 만들기` 코드 블록:
     ```bash
     agent-harness gates init --file "$WORKTREE/.agent-harness/issues/$ISSUE/gates.md" --scope "$ISSUE" \
       --gate "G1: <관찰 가능한 결과> | CHECK: <read-only 명령> | EXPECT: <출력에 포함될 문자열>" \
       --gate "G2: <결과> | CHECK: <명령> | EXPECT: <문자열>" --json
     ```
     규칙: 결과는 관찰 가능한 동작으로 쓴다. CHECK는 command policy를 지나므로 셸 확장·파이프 우회를 넣지 않는다. EXPECT 일치와 exit 0이 함께 요구된다.
  5. `## 검사`: `agent-harness gates check --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --write --json`. `--write`가 EVIDENCE를 채운다. 네트워크가 필요한 CHECK는 `--network`, 오래 걸리면 `--timeout-seconds`. 종료 코드 1은 미충족 게이트가 남았다는 뜻이다.
  6. `## 상태와 보고`: `gates status`, `gates report`(같은 `--file`·`--cwd`·`--workspace-root`).
  7. `## 포기`: 실행할 수 없는 게이트는 `gates abandon --gate G3 --reason "<사유>" --file "$LEDGER"`로 정직하게 닫는다. EXPECT를 완화해 통과시키지 않는다.
  8. `## IssueOps와의 관계`: strict pr-readiness는 자기 번호와 anonymous 원장만 판정하고 미충족은 `gates_incomplete:<file>`로 pr 진입을 막는다. 3단계가 만들고, 4단계가 `--write`로 EVIDENCE를 채우고, 6단계가 읽기 전용으로 재검사한다.
  9. `## 나쁜 예`: EVIDENCE를 손으로 적기, 실패한 CHECK를 EXPECT 완화로 통과, 원장을 워크트리 밖에 두기, 같은 번호의 원장 두 개, CHECK에 `$(...)`.

  **Must NOT do**: CLI 없이 EVIDENCE 직접 기록, IssueOps record 변경, 새 CLI 플래그 발명.

  **Recommended Agent**: quick. 기존 CLI를 감싸는 문서다.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8, T9, T19, T20, T26, T11 | Blocked By: T0

  **References**:
  - `internal/domain/cli/usage.go:95-99` (gates usage 원문).
  - `.agent-harness/architecture/issueops.md:18-28` (원장 판정 규칙, `gates_incomplete`, `duplicate_issue_artifact`).
  - `.agent-harness/adr/decisions/2026-08-22-task-gate-ledger.md`.
  - 샘플: `.agent-harness/issues/490/gates.md`, `.agent-harness/issues/486/gates.md`.
  - `AGENTS.md` §9의 gates 데모 명령(임시 디렉터리 smoke).

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과.
  - [ ] `rg -n "gates init|gates check|gates abandon|gates_incomplete|--write|duplicate_issue_artifact" skills/gates-ledger/SKILL.md` 각 1건 이상.
  - [ ] smoke: `d="$(mktemp -d)" && (cd "$d" && "$OLDPWD/bin/agent-harness" gates init --scope smoke --gate "G1: smoke | CHECK: printf %s ok | EXPECT: ok" --json && "$OLDPWD/bin/agent-harness" gates check --cwd "$d" --workspace-root "$d" --write --json && "$OLDPWD/bin/agent-harness" gates report --cwd "$d" --workspace-root "$d") && rm -rf "$d"` 종료 코드 0.

  **QA Scenarios**:
  ```
  Scenario: EXPECT 불일치는 미충족
    Channel: bash
    Steps: 임시 디렉터리에서 gates init --gate "G1: fail | CHECK: printf %s no | EXPECT: ok" → gates check --write --json; echo exit=$?
    Expected: exit=1, JSON에 G1이 미충족
    Evidence: .agent-harness/evidence/task-22-mismatch.txt
  Scenario: 원장 경로 규칙 명시
    Channel: bash
    Steps: rg -n "issues/<n>/gates.md|gates/<scope>.md" skills/gates-ledger/SKILL.md
    Expected: 각 1건 이상
    Evidence: .agent-harness/evidence/task-22-paths.txt
  ```

  **Commit**: YES | `feat(skill): add gates-ledger for shared task gate ledgers` | Files: 위 2개

- [x] **T23. `issueops-remote-write` 신설 (공용: 원격 쓰기 프로토콜)** — 완료(2026-09-05). 증거: `.agent-harness/evidence/task-23-gate-en.txt`(영어 본문 exit 1, "expected at least 20 Hangul chars"), `task-23-single-owner.txt`(스크립트 참조 파일 1개).

  **실행 결과와 계획 대비 편차**: `remote-issue.md`의 두 절을 지우면서 그 안에 섞여 있던 **VCS 링크 규칙**(Plan Link 금지, label·assignee, target branch 정책, GitLab relations)은 남겼다. 그것은 원격 쓰기 절차가 아니라 provider 링크 규칙이고 이 문서가 소유자다. 남은 절은 `## Language And Writing Protocol`로 이름을 바꿔 새 스킬을 가리키기만 한다. 라우터의 "Remote write 공통 게이트" 절 삭제는 T11이 라우터를 재작성할 때 함께 한다.

  **Files:**
  - Create: `skills/issueops-remote-write/SKILL.md`
  - Create: `skills/issueops-remote-write/agents/openai.yaml`
  - Move: `skills/issueops/scripts/remote_artifact_gate.py` → `skills/issueops-remote-write/scripts/remote_artifact_gate.py` (`git mv`; `skills/issueops/scripts/__pycache__`는 gitignore 대상이므로 디렉터리만 지운다)

  **What to do**:
  1. frontmatter description: "Apply the shared IssueOps remote write protocol to any governed `agent-harness issueops remote ...` mutation: polish the Korean body with fluent-korean, run the bundled Korean artifact gate, preview, confirm the identical request, read the artifact back, and reconcile instead of retrying when the result is ambiguous. Use before creating or editing issues, child tasks, PR or MR bodies, comments, or review replies inside an IssueOps cycle, or when the user says '원격에 써줘', '이슈 본문 올려줘', 'PR 본문 갱신'."
  2. 절 순서: `## 여덟 규칙` → `## 절차` → `## 본문 품질` → `## 한국어 게이트` → `## body-of-record` → `## 나쁜 예` → `## 검증`.
  3. `## 여덟 규칙`: 설계 요약 7의 (1)~(8)을 번호 목록으로 쓴다.
  4. `## 절차` 코드 블록:
     ```bash
     # 1. 본문 초안에 fluent-korean 스킬을 호출해 다듬는다(Skill 도구).
     # 2. 한국어 게이트. $SKILL_DIR는 설치된 이 스킬 디렉터리(~/.claude/skills/issueops-remote-write 등)다.
     python3 "$SKILL_DIR/scripts/remote_artifact_gate.py" --kind issue --title "$TITLE" --body-file "$BODY_FILE"
     # 3. preview (confirm 없음)
     agent-harness issueops remote <verb> --id "$ISSUEOPS_ID" ... --body-file "$BODY_FILE" --json
     # 4. 동일 요청 + --confirm
     agent-harness issueops remote <verb> --id "$ISSUEOPS_ID" ... --body-file "$BODY_FILE" --confirm --json
     # 5. readback
     agent-harness issueops remote verify-artifact --id "$ISSUEOPS_ID" --provider "$PROVIDER" --kind pr|mr --url "$URL" --target-branch "$BASE" --label "$LABEL" --assignee "$ASSIGNEE" --json   # PR/MR
     # issue·child·본문 갱신은 해당 명령의 응답 readback 필드(hierarchy_verified, url, body sha)를 확인한다.
     # 6. 결과가 불명확하면 재호출하지 않는다.
     agent-harness issueops remote reconcile-issue --id "$ISSUEOPS_ID" --json      # issue create
     agent-harness issueops execution reconcile --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json   # PR/MR create
     ```
  5. `## 본문 품질`: 현행 `remote-issue.md` "Remote Artifact Writing Quality"의 규칙 7개와 예시를 그대로 옮긴다.
  6. `## 한국어 게이트`: 현행 `remote-issue.md` "Korean Remote Artifact Gate"를 옮기되 경로 변수 이름을 `$SKILL_DIR`로 바꾼다.
  7. `## body-of-record`: 원격 issue 본문이 계약 SSOT다. contract change가 생기면 `issueops-sync-issue`로 본문을 갱신한 뒤 `agent-harness issueops feedback mark-issue-updated --id "$ISSUEOPS_ID" ...`를 기록한다(현행 `review-feedback.md`의 문단을 옮긴다).
  8. `## 나쁜 예`: raw `gh`/`glab`로 본문 수정, preview 없이 confirm, preview와 다른 요청에 confirm, timeout 뒤 create 재실행, secret이 든 로그 첨부, fluent-korean 생략, label 없이 write.

  **Must NOT do**: 명령별 본문 형식(각 단계 스킬 소유), provider hierarchy 규칙(`remote-issue.md` 소유).

  **Recommended Agent**: deep. 세 문서의 규칙을 한 곳으로 모으며 누락이 곧 게이트 실패다.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T3, T5, T10, T11 | Blocked By: T0

  **References**:
  - `skills/issueops/SKILL.md` "Remote write 공통 게이트" 절(이동 원문).
  - `skills/issueops/references/remote-issue.md:119-177` (두 절 이동 원문).
  - `skills/issueops/scripts/remote_artifact_gate.py`.
  - `skills/issueops-create-issue/SKILL.md` "Canonical publication", `skills/issueops-create-pr/SKILL.md` "Canonical publication"(readback 명령 원문).
  - `skills/issueops/references/review-feedback.md` "Remote Review Feedback"의 body-of-record 문단.
  - `AGENTS.md` §7 fluent-korean 문장.

  **Acceptance Criteria**:
  - [ ] 검사기 2개 통과.
  - [ ] `test -f skills/issueops-remote-write/scripts/remote_artifact_gate.py && test ! -f skills/issueops/scripts/remote_artifact_gate.py`.
  - [ ] `rg -n "fluent-korean|remote_artifact_gate.py|--confirm|reconcile|verify-artifact|mark-issue-updated" skills/issueops-remote-write/SKILL.md` 각 1건 이상.
  - [ ] `printf '## 문제\n배포가 시크릿 누락으로 실패한다.\n' > /tmp/rw-ko.md && python3 skills/issueops-remote-write/scripts/remote_artifact_gate.py --kind issue --title "배포 실패 원인" --body-file /tmp/rw-ko.md; echo exit=$?` 가 `exit=0`.

  **QA Scenarios**:
  ```
  Scenario: 영어 본문은 게이트에서 거부
    Channel: bash
    Steps: printf 'Deployment fails because of a missing secret.\n' > /tmp/rw-en.md && python3 skills/issueops-remote-write/scripts/remote_artifact_gate.py --kind issue --title "Deployment failure" --body-file /tmp/rw-en.md; echo exit=$?
    Expected: exit가 0이 아니고 한글 비율 관련 문구 출력
    Evidence: .agent-harness/evidence/task-23-gate-en.txt
  Scenario: 다른 스킬이 절차를 복사하지 않음
    Channel: bash
    Steps: rg -l "remote_artifact_gate.py" skills/ | sort
    Expected: skills/issueops-remote-write/SKILL.md 한 파일만
    Evidence: .agent-harness/evidence/task-23-single-owner.txt
  ```

  **Commit**: YES | `feat(skill): add issueops-remote-write as the shared remote write protocol` | Files: 위 파일

- [x] **T24. implementation review 게이트를 모든 execution mode로 확장** — 완료(2026-09-05), 커밋 `86a6fc7a`. direct 모드 면제를 제거하고 `execution_remote.go`의 freshness 거부에서 orca 조건을 뺐다. 리플: 어댑터 3건·CLI 3건의 픽스처에 리뷰 기록을 추가했고(테스트를 지우거나 건너뛰지 않았다), 빈 fingerprint를 허용하도록 recorder를 고쳐 `project_docs_review` 선례와 맞췄다. `skills/issueops/references/execution.md`와 `.agent-harness/architecture/issueops.md`의 문구도 함께 고쳤다. `skills/issueops-implement/SKILL.md`의 direct 면제 문단은 T24 커밋에서 빠졌고 2026-09-05에 뒤이어 고쳤다(T9의 재작성을 기다리면 그때까지 스킬이 코드와 반대되는 말을 한다).

  **Files:**
  - Modify: `internal/adapter/issueops/issueops_implementation_review.go:73-79`
  - Modify: `internal/adapter/issueops/execution_remote.go:99-111` (orca 한정 주석을 고치고, `currentReviewFingerprint == ""`일 때의 freshness 거부에서 `Mode == Orca` 조건을 제거해 `Execution != nil`이면 모든 모드에 적용)
  - Test: `internal/adapter/issueops/issueops_implementation_review_test.go` (direct 면제를 단언하는 테스트를 반대로 뒤집고 `TestDirectModeRequiresImplementationReviewForPR` 추가), `execution_remote` create-pr 테스트에 direct 모드 리뷰 부재 거부 케이스 추가
  - Modify: `internal/adapter/issueops/issueops_pr_readiness*_test.go`와 direct 모드 record로 pr readiness를 통과시키는 공유 픽스처(`rg -l "ExecutionModeDirect" internal/adapter/issueops/*_test.go cmd/harness/harnessapp/*_test.go`로 찾아 review 기록을 넣는다. CAUTIONS §28: 새 fail-closed 게이트는 모든 전진 테스트로 파급된다)
  - Modify: `skills/issueops/references/execution.md` "Implementation Review Gate (orca mode)" 제목과 본문의 "direct 모드는 이 게이트의 대상이 아니다" 문단 삭제
  - Modify: `.agent-harness/architecture/issueops.md:42`의 "orca 모드 publication fail-closed 게이트" 표현을 "모든 모드"로

  **What to do**:
  1. 실패 테스트: `Execution.Mode == direct`이고 `ImplementationReview == nil`인 record의 `IssueOpsPRReadiness(record).Missing`에 `implementation_review`가 있어야 한다. 현재는 없으므로 RED다.
  2. `implementationReviewMissing`의 조건을 `record.Execution == nil`일 때만 면제하도록 바꾼다(`Mode != Orca` 분기 삭제). 주석은 "publication 게이트 판정은 execution이 있는 모든 모드에 적용한다. direct가 기본 경로가 된 2026-09-04 10단계 재편 이후 검증 단계가 이 기록을 만든다"로 고친다.
  3. 깨지는 픽스처마다 review 기록을 넣어 GREEN을 만든다. 테스트를 삭제하거나 skip하지 않는다.
  4. `go test ./internal/adapter/issueops ./cmd/harness/... -count=1`.
  5. 실기: 이 머신의 direct·pr phase 사이클 하나(`issueops list --json`에서 `mode: direct`, `phase: pr`)에 `pr-readiness --strict --json`을 실행해 `implementation_review`가 missing에 나타나는지 evidence로 남긴다. 그 사이클은 이미 create-pr를 마쳤으므로 동작에는 영향이 없다. implement·ai-slop-clean·feedback phase의 direct 사이클은 이후 `phase --to pr`와 create-pr 전에 리뷰 기록이 필요해진다는 사실을 T15의 cautions 모듈 §4에 함께 적는다.

  **Must NOT do**: orca 경로 변경, owner 프롬프트 변경, `execution complete` 조건 변경.

  **Recommended Agent**: deep. 파급되는 픽스처를 전부 찾아야 한다.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T20, T15 | Blocked By: T0

  **References**:
  - `internal/adapter/issueops/issueops_implementation_review.go:73-90`, `issueops_pr_readiness.go:27`, `issueops_pr_readiness_strict.go:58-66`.
  - `.agent-harness/cautions/issueops-lifecycle.md` §28.
  - `.agent-harness/adr/decisions/2026-07-24-issueops-planner-implementer-dual-structure.md` "구현 리뷰 게이트" 항목(orca 한정의 원래 근거).

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapter/issueops -run 'ImplementationReview|PRReadiness' -count=1 -v | grep -c '^--- FAIL'`이 0.
  - [ ] `rg -n "ExecutionModeOrca" internal/adapter/issueops/issueops_implementation_review.go` 0건, `sed -n '95,115p' internal/adapter/issueops/execution_remote.go | rg -c "ExecutionModeOrca"`가 0.
  - [ ] `go test ./internal/adapter/issueops -run 'RemoteCreate|CreatePullRequest' -count=1` 통과(direct 모드 리뷰 부재 거부 케이스 포함).
  - [ ] `rg -n "direct 모드는 이 게이트" skills/issueops/references/execution.md` 0건.

  **QA Scenarios**:
  ```
  Scenario: direct 사이클도 리뷰 없이는 pr readiness 미충족
    Channel: bash
    Steps: 테스트 fixture(mode direct, review nil) → IssueOpsPRReadiness
    Expected: Missing에 "implementation_review"
    Evidence: .agent-harness/evidence/task-24-direct.txt
  Scenario: execution 없는 legacy 레코드는 면제 유지
    Channel: bash
    Steps: fixture(Execution nil) → IssueOpsPRReadiness
    Expected: Missing에 implementation_review 없음
    Evidence: .agent-harness/evidence/task-24-legacy.txt
  ```

  **Commit**: YES | `fix(issueops): require the implementation review for every execution mode` | Files: 위 파일

- [x] **T25. provisioner relaunch 명령의 cwd 수정 + 워크트리 채택 회귀 테스트** — 완료 2026-09-05, commit `efb79a34`. 테스트 6→12개, `go test ./internal/adapter/gitworktree -count=1` ok. 계획의 literal wording에서 하나 벗어났다: 워크트리가 없을 때 `cd`가 실패해 host가 뜨지 않으므로 존재를 관측해 source root로 되돌린다. 증거: `.agent-harness/evidence/task-25-adopt.txt`, `task-25-adopt-fail.txt`

  **Files:**
  - Modify: `internal/adapter/gitworktree/provisioner.go:20-46,180-190` (`ProbeAccess`가 `req.Root`를 relaunch 명령에 넘기고, `workspaceRelaunchCommand(host, root, base)`가 세 호스트 모두 워크트리 root로 `cd`한 뒤 실행하도록)
  - Test: `internal/adapter/gitworktree/provisioner_test.go` (relaunch 명령 세 호스트 형식, 채택 성공·실패 케이스)

  **What to do**:
  1. 실패 테스트(relaunch): `ProbeAccess`가 접근 실패를 돌려줄 때 `RelaunchCommand`가 host별로 정확히 다음이어야 한다. codex `codex --cd '<root>' --add-dir '<base>'`, claude `cd '<root>' && claude --add-dir '<base>'`, omo `cd '<root>' && omo`. 현재는 codex·omo가 `<sourceRoot>`로 가고 claude는 `cd`가 없으므로 RED다.
  2. 실패 테스트(채택): 임시 git 저장소에서 base 커밋을 만들고 `git worktree add <root> -b <branch> <base>`로 미리 워크트리를 만든 뒤 `Prepare`를 호출하면 `receipt.Exists == true`이고 새 워크트리를 만들지 않는다. 같은 워크트리에 커밋을 하나 더 얹으면 `existing canonical worktree identity does not match branch and base_head` 오류다. 다른 브랜치를 checkout해도 같은 오류다.
  3. RED 확인: `go test ./internal/adapter/gitworktree -run 'Relaunch|Adopt' -count=1`.
  4. 구현: `workspaceRelaunchCommand`의 시그니처를 `(host, root, base string)`로 바꾸고 세 분기를 1번 형식으로 고친다. `ProbeAccess`는 `req.Root`를 넘긴다. 채택 로직(`inspectExisting`)은 바꾸지 않는다.
  5. GREEN 확인 후 `go test ./internal/adapter/gitworktree ./internal/application/issueopspreparation ./cmd/harness/harnessapp -run 'Prepar|Relaunch|Adopt|Access' -count=1`.

  **Must NOT do**: 채택 규칙 완화, `execution prepare`에 새 플래그 추가, source root cwd 허용 규칙(`workspace.go:52`) 변경.

  **Recommended Agent**: quick. 함수 하나와 테스트 두 종류다.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T8, T15 | Blocked By: T0, T0b

  **References**:
  - `internal/adapter/gitworktree/provisioner.go:20-46` (`ProbeAccess`), `:102-117` (`inspectExisting`), `:119-130` (`worktreeAddArgs`), `:180-190` (`workspaceRelaunchCommand`).
  - `internal/adapter/outbound/issueopspreparation/workspace.go:45-60` (cwd 허용 규칙: source root 또는 canonical worktree).
  - `internal/domain/issueopslease/release.go:55-63` (release가 canonical cwd만 허용하는 이유).

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapter/gitworktree -count=1 -v | grep -c '^--- PASS'`가 기존 수보다 5 이상 많다.
  - [ ] `rg -n "codex --cd \" \+ sourceRoot|cd \" \+ sourceRoot" internal/adapter/gitworktree/provisioner.go` 0건.

  **QA Scenarios**:
  ```
  Scenario: 채택 성공
    Channel: bash
    Steps: 임시 저장소에서 base SHA 워크트리를 미리 만들고 Prepare 호출
    Expected: Exists true, git worktree list에 경로 1개, HEAD == base
    Evidence: .agent-harness/evidence/task-25-adopt.txt
  Scenario: 채택 실패
    Channel: bash
    Steps: 미리 만든 워크트리에 커밋 1개 추가 후 Prepare 호출
    Expected: 오류 문구 "does not match branch and base_head", 워크트리·브랜치 변화 없음
    Evidence: .agent-harness/evidence/task-25-adopt-fail.txt
  ```

  **Commit**: YES | `fix(gitworktree): relaunch into the canonical worktree and pin worktree adoption` | Files: 위 파일

## Final Verification Wave

> 전부 APPROVE여야 한다. 결과를 사용자에게 모아 보고하고 명시적 "okay"를 받은 뒤 완료한다.

- [x] F1. Plan Compliance Audit — APPROVE. 미완료 TODO 0건(T18만 사용자 승인 대기로 `[~]`). `ls skills | grep -E '^issueops|^gates-ledger'`가 설계 요약 1·7의 17개와 정확히 일치한다: gates-ledger, issueops, issueops-abandon, issueops-clean, issueops-cleanup, issueops-complete, issueops-create-issue, issueops-create-pr, issueops-docs, issueops-implement, issueops-plan, issueops-prepare, issueops-remote-write, issueops-review, issueops-sync-issue, issueops-sync-pr, issueops-verify.
- [x] F2. Code Quality Review — APPROVE. `issueopsnext` vertical 네 층과 CLI·wiring 합계 1,828줄(테스트 포함). 정의만 있고 호출되지 않는 헬퍼 0건, `go vet` 무경고, contract 상수 전부 참조됨. close-pr diff는 두 provider가 각각 하나의 파일(`close_pull_request.go`, `close_merge_request.go`)이며 공통 로직은 기존 `readGh*State`/`runGlabAPI` 층을 재사용한다. `shannon` 측정은 생략했다 — 전후 비교 대상인 "정리 전 diff"가 없는 신규 코드이고, 대신 위 세 지표를 관측했다.
- [x] F3. Real Manual QA — APPROVE. T0b 스파이크 증거 3개(`task-0b-adopt.json`, `task-0b-access.txt`, `task-0b-fingerprint.json`)와 T17 시나리오 4개 로그 모두 존재하고 마지막 줄이 PASS다.
- [x] F4. Scope Fidelity Check — APPROVE. `git diff main --stat -- internal/contract/issueops/execution.go internal/domain/issueopslease cmd/harness/hookcli internal/domain/mcp` 0건. `IssueOpsSchemaVersion`은 1 그대로다. record 구조체에 필드를 추가하지 않았고, `types.go`의 변경은 abandon 실패 단계 상수 세 개뿐이다(기존 문자열 필드에 들어가는 값).

## Commit Strategy

- 태스크당 커밋 1개, 위 `Commit` 메시지 사용. subject는 Conventional Commit, body는 Lore(Intent, Why, Changes, Verify, Risk).
- 스킬 삭제·이름 변경 커밋(T4, T11)은 `!`를 붙인다.
- 골든 재생성은 그것을 필요로 한 태스크 커밋에 포함한다.
- 이 계획을 IssueOps 사이클로 돌리면 T0 전에 `issueops-create-issue`로 이슈를 만들고, T11 완료 전까지는 현행 스킬 이름으로 진행한다.

## Success Criteria

- 이 저장소에서 `agent-harness issueops next --json`이 `none`을, 워크트리 사이클에서 표의 단계 키를 돌려준다.
- 준비 세션이 3단계 끝에서 `execution prepare --mode auto`를 실행하면, Orca가 있으면 Orca가 구현 세션을 띄우고 없으면 같은 세션이 이어받아, 어느 쪽이든 `next`가 가리키는 스킬만 따라 PR까지 간다.
- 어느 단계에서든 `issueops-abandon`으로 일시 중단·재개·인수·폐기가 되고, 폐기 뒤 record·워크트리·로컬 브랜치가 남지 않으며, 사용자가 고른 원격 효과(draft PR/MR 닫기, 이슈 닫기, 원격 브랜치 삭제)가 provider readback으로 확인된다.
- `issueops next`는 `git fetch`도 provider 호출도 하지 않는다(`rg -n "\"fetch\"|gh |glab " internal/domain/issueopsnext internal/application/issueopsnext internal/adapter/inbound/issueopsnext` 0건).
- 저장소 어디에도 `issueops-branch-worktree`, 삭제된 레퍼런스 4개, "이원 구조" 표현이 현재 계약으로 남지 않는다.
- 적대 리뷰 절차, 게이트 원장 절차, 원격 쓰기 절차가 각각 한 스킬에만 있고 단계 스킬은 링크만 한다(`rg -c "brooks를 fresh 서브에이전트로|동일한 요청에만|같은 요청에만" skills/issueops-*/SKILL.md`가 `issueops-review`·`issueops-remote-write` 밖에서 0).
- Definition of Done의 명령 전부가 통과한다.
