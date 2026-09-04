# CLI, MCP, Codex, Claude, and Omo host parity

[← TESTING.md](../TESTING.md) owns the test-strategy index. This module owns
CLI/MCP/Codex/Claude/Omo host parity: GitLab snapshot contracts, cross-host tool
conformance, reversible child-host smoke, and native integration parity. The
single-pass verification battery that exercises these commands lives in
[self-verification.md](self-verification.md); IssueOps CLI/MCP/Codex/Claude/Omo
adapter parity is owned by
[issueops-execution.md](issueops-execution.md).

## GitLab Issue Snapshot

GitLab MCP/CLI snapshot 계약을 바꿀 때는 다음 bounded set을 먼저 실행한다.

```bash
go test ./internal/adapter/issueops ./internal/adapter/provider/gitlab -run 'IssueSnapshot|ExecutionIssueSnapshot' -count=1
go test ./internal/adapter/mcp ./cmd/issueops/mcpcli ./internal/adapter/toolconformance -count=1
go test ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/issueopscli -run 'Snapshot|ExecutionCLI|Usage' -count=1
go test ./internal/adapter/skillcontract -run TestGitLabSnapshotSkillsPinPortableVCSContract -count=1
python3 scripts/validate-skill.py skills/gitlab-usecase
python3 scripts/validate-skill.py skills/issueops
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go build -o bin/issueops ./cmd/issueops
```

설치 갱신 뒤에는 installed Codex/Claude/Omo MCP schema에 `issue_snapshot`의 exact
다섯 필드만 있는지 확인하고, GitLab-linked lifecycle preview에서
`resolved_mode=orca`와 `issue_snapshot_source=glab_mcp|glab_cli`를 확인한다.
이 smoke는 worktree나 lease를 만들지 않는 preview로 제한한다.

## Cross-host tool contract conformance

기본 self-verify에는 network/auth가 없는 다음 deterministic baseline이 포함된다.

```bash
./bin/issueops contract conformance baseline --json
```

baseline은 representative schema 3개와 `valid`, `unknown_key`, `coercible_type_drift`, `noncoercible_type_drift` payload class의 preregistered 10 cases를 정확히 판정하고, 승격된 behavioral regression fixture가 있으면 handler 호출 0회·동일한 state digest·정규화된 final result를 재생한다.

Live 측정은 CI와 기본 self-verify에 포함하지 않는다. `ISSUEOPS_TOOL_CONFORMANCE_LIVE=1`과 host/model/auth 입력을 명시한 뒤 clean-context `3 hosts × 3 fixtures = 9 completed episodes`를 수집한다. environment/transport/no-call attempt는 model denominator에서 제외하며, case당 최대 3회 retry 후 9 episodes를 채우지 못하면 `inconclusive`다. invalid raw call은 동일 host/schema/diagnostic signature가 2회 이상 재현되어야 regression fixture와 canonical production enforcement 후보가 된다. 한 번뿐인 관측은 승격하지 않는다.

환경 실패율 5%는 조사 warning일 뿐 pass/fail threshold가 아니다. context-pressure profile과 10/20 reproduction batch는 clean initial matrix와 denominator를 합치지 않고 별도 승인·비용 경계로 실행한다. evidence는 `.issueops/evidence/tool-conformance/`에 mode 0600/0700으로 저장하고 git에 추가하지 않는다.

## Reversible child-host smoke

Reversible Codex/Claude child-host smoke는 일반 live matrix와 별도다. `scripts/verify-child-host-smoke.sh`는 literal `--confirm-user-activation`, clean local HEAD, exact singleton remote ref가 모두 일치할 때만 user-scope integration을 잠시 활성화한다. 활성화 직후 두 command-host의 managed `SessionStart` handler는 command·type·timeout·key set까지 exact contract로 검증하고 managed event set이 정확히 `SessionStart` 하나인지 확인하며, legacy event·enforcement flag·shell suffix를 거부한다. 실제 child episode는 `SessionStart`를 관찰하고 `PreToolUse`가 관찰되지 않음을 확인한다; 압축 후 `SessionStart(compact)` 재실행은 host가 compaction을 발생시킨 경우에만 일어나므로 config contract와 host 전사로 증명한다. Codex는 검증된 활성화 handler 자체를 user config·plugin·co-resident hook을 로드하지 않는 private episode `CODEX_HOME`에 투영한 뒤 invocation-scoped `--dangerously-bypass-hook-trust`를 사용하며 trust state는 수정·저장하지 않는다. Host runner는 marker와 native MCP result를 boolean/count/SHA-256/exit/duration projection으로 합친 뒤 원문 stream과 marker를 폐기한다. 영수증의 `validation_lane=native_host`는 Codex·Claude adapter 실동작만 증명하며 Orca Run/task/dispatch/claim 증거를 대신하지 않는다. 어떤 post-activation 실패도 source installer 1회, 원래 네 설정 파일의 private byte snapshot 원자 복원, before/restore raw+semantic digest equality를 모두 통과하지 못하면 `verdict=pass`가 될 수 없다.

managed regular command adoption 테스트는 기본 refusal과 승인 dry-run 무변경, 실제 staged candidate의 정적 build identity, file matrix/size boundary, atomic exchange 시점의 destination drift 보존, apply/finalize, injected rollback, transition-fenced Begin/Seal/Abort, direct-vs-explicit cleanup ownership을 각각 검증한다. child smoke에서 adoption은 literal confirmation 이후 child activation 한 번에만 전달되어야 하며 source restore에는 전달되지 않아야 한다.

## Native integration parity

Native integration smoke는 single-pass verification battery의 일부로
[self-verification.md](self-verification.md)가 실행한다. Codex/Claude/Omo
user-level skill 파일 존재, Codex/Claude MCP registration, Omo
`~/.omo/mcp.json`, 그리고 managed Omo lifecycle extension을 확인한다.
The deterministic battery does not require the external Omo runtime: it checks
installed Omo skill paths, exact MCP semantics, and exact generated extension
bytes. This keeps issueops independently verifiable.

Release/manual QA adds the runtime evidence that deterministic self-verification
cannot own. In an isolated `HOME`, run native install, then use the installed
Omo/Senpi `SettingsManager -> DefaultPackageManager -> loadSkills` pipeline and
require all pioneer skills with zero diagnostics. Use
`discoverAndLoadExtensions` and require exactly one managed
`issueops.js` with `session_start` and `session_compact`; bind
`runtime.sendMessage`, invoke both handlers, and require rejected compaction to
emit nothing while accepted events inject hidden `triggerTurn=false` context.
Record the Omo/Senpi package versions with that evidence.

Codex and Claude hook fixtures continue through the common hook input boundary;
the IssueOps hook matrices are owned by
[issueops-execution.md](issueops-execution.md).

## Omo IssueOps parallel worktree sessions

Durable Omo parallel-child QA must spawn one resident Omo agent per canonical
IssueOps child worktree. For native teams, set each
`team_create.members[].worktreePath`; the Omo runtime maps it to the child
process `cwd` and sandbox worktree boundary. A plain `task` spawn or a prompt
that merely says `cd` does not satisfy the contract.

Each child must independently prove:

- `pwd` and `git rev-parse --show-toplevel` equal the sealed canonical worktree
- `PI_SESSION_ID` is non-empty and process ancestry contains the live `omo`
  executable
- `issueops execution whoami --json` returns `host=omo` and that
  same session/process identity
- the assigned cross-process IssueOps regression passes while sibling Omo
  agents are active
- final `git status --short` contains no unowned change

When team category routing is unavailable, the bounded fallback is one branded
`omo -p` process started with the canonical worktree as its OS cwd. Record the
category failure separately; do not substitute lead-session test execution.
