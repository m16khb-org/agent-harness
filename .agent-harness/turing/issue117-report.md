# Issue #117 Turing 최종 검증 보고서

- 이슈: https://github.com/m16khb/agent-harness/issues/117
- lifecycle: `io-e1796f7af7c5`, direct generation 5
- 대상 브랜치: `117-hexagonal-architecture-migration`
- main 기준: `6894d946aa69460e40ea7b3392bb1499e0e6eaab`
- 마지막 child 병합 HEAD: `eb4e498241c8ef2bf7959f8c313590268621ade1`
- local gate execution HEAD: `e707a53ee1e88cda35f0c5a1056914650368b820`
- 계획: `docs/superpowers/plans/2026-08-02-hexagonal-architecture-umbrella-finalization.md`

## 결과

#117의 native sub-issue 11개는 모두 `CLOSED`이고 parent branch는 마지막 child
PR #219의 merge commit을 포함한다. Umbrella 최종화 단계에서는 production Go,
public API, CLI/MCP JSON, persisted schema, hook deny semantics와 legacy baseline을
변경하지 않았다. `eb4e4982...e707a53e`는 계획 문서 한 파일, 134줄 추가뿐이다.

| child | merged PR | 상태 |
|---|---:|---|
| #190 | #201 | CLOSED |
| #191 | #202 | CLOSED |
| #192 | #205 | CLOSED |
| #193 | #206 | CLOSED |
| #194 | #214 | CLOSED |
| #195 | #216 | CLOSED |
| #196 | #203 | CLOSED |
| #197 | #204 | CLOSED |
| #198 | #217 | CLOSED |
| #199 | #218 | CLOSED |
| #200 | #219 | CLOSED |

`origin/main...e707a53e`는 `0/94`, 292 files,
`+33,699/-1,516`인 전체 11-child migration delta다. 이 큰 diff는 각 child PR의
Turing receipt와 최종 architecture, contract, full unit/race, self-verify gate에
매핑한다. Finalization-only delta와 전체 migration delta를 하나의 no-input
품질 주장으로 혼동하지 않았다.

## 훅 활성 환경 검증

현재 부모 Codex 프로세스는 `--disable hooks`로 실행됐으므로 합격 근거에서
제외했다. 검증은 `e707a53e`의 빌드 산출물과 repository hook 설정을 실제로 로드한
별도 native host process에서 수행했다.

### Fresh Codex

- 버전: `codex-cli 0.146.0`
- isolated `CODEX_HOME`의 `hooks.json`은 `configs/codex/hooks.json`과
  byte-identical했다.
- credential secret은 복사하지 않았다. 권한 `0600`인 기존
  `/Users/m16khb/.codex/auth.json`을 임시 home의 symlink로만 읽었다.
- `codex app-server --enable hooks --stdio`의 `hooks/list`는 source
  `/private/tmp/agent-harness-issue117-proof.Mle92Z/codex-home/hooks.json`,
  `enabled=true`, warnings/errors 없음과 exact PreToolUse command를 반환했다.
  PreToolUse current hash는
  `sha256:e423bc883dd52a5876ac014db300ecdfdb73defc40af9fd5fa7a0006385ff406`이다.
- fresh `codex exec --enable hooks --dangerously-bypass-hook-trust --ephemeral`
  session은 `019fc02c-def6-7483-92f4-a243aef71fa9`다.
- 정확히 한 번 요청한 canonical in-worktree sentinel `touch`는 PreToolUse에서
  `holder_identity_mismatch`, axis `session_id`, generation 5로 block됐다.
- sentinel은 process 종료 뒤에도 존재하지 않았다.

### Fresh Claude

- 버전: `2.1.220 (Claude Code)`
- `--settings configs/claude/hooks.settings.json --setting-sources local
  --include-hook-events --output-format stream-json --verbose
  --permission-mode bypassPermissions`로 fresh session
  `2e8cac41-a329-4ffe-8d9d-b216be67f022`을 실행했다.
- stream은 repository 설정의 `SessionStart`, `UserPromptSubmit`,
  `PreToolUse:Bash`, `Stop`에 대해 `hook_started`와 `hook_response`를 반환했다.
- canonical in-worktree sentinel `touch` 한 번은
  `hookSpecificOutput.permissionDecision=deny`, `holder_identity_mismatch`,
  axis `host`, generation 5로 거부됐다.
- `permission_denials`에 exact Bash input이 포함됐고 sentinel은 없었다.
- `--safe-mode`와 `--bare`는 사용하지 않았다.

초기 탐색 중 isolated Codex home의 인증 없는 401, repo 밖 `/tmp` sentinel을
허용한 Claude, 추가 `.` 인자를 포함한 Codex command는 acceptance proof에서 모두
제외했다. Worktree lease 검증은 canonical parent 내부의 단일 exact target만
권위 있는 표적으로 삼았다.

## 최종 로컬 gate

다음 명령은 canonical parent의 `e707a53e`에서 순차 실행했고 모두 exit 0이다.

```text
go fmt ./...
git diff --exit-code
go test ./internal/architecture -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
git diff --check origin/main...HEAD
```

결정적 self-verify:

```text
./bin/agent-harness self-verify --full --iterations=10 --seed=100 \
  --target-score=95 --llm-eval=false --progress=jsonl --json

ok=true
total_runs=10
total_steps=250
passed_steps=250
failed_steps=0
minimum_goal_score=100
termination_eligible=true
elapsed_ms=1337323
contract_hash=f49b24d798532533a919d5f59a076eeac557d27494359805212eb1844c4df786
```

## Shannon 품질 gate와 AI slop clean

- Local gate execution delta `eb4e4982...e707a53e`: 계획 Markdown 1개,
  134 insertions, production input 0.
- Publication candidate content: 위 134-line 계획과 이 evidence report를 포함한
  Markdown 2개, production input 0. Production AST 입력이 없으므로 SNR을 0으로
  나누거나 임의 점수로 만들지 않았다.
- 전체 umbrella diff: 292 files, 33,699 insertions, 1,516 deletions. 이 delta의
  품질 책임은 11개 child review와 architecture/contract/full regression에 있다.
- 새 production abstraction, duplicate block, compatibility shim, placeholder,
  speculative error handling과 OpenWiki 변경은 없다.
- 최종화 문서의 각 문단은 active-hook proof, local gate, publication 또는 cleanup
  경계에 직접 연결된다.

## 독립 검토 checkpoint

Brooks devil's-advocate 첫 검토는 lifecycle 전이, isolated credential/trust,
command substitution, 두 diff 축 구분을 `revise`로 지적했다. 계획은 모든 항목을
수정했다. 두 번째 검토는 불필요한 unconditional feedback phase를 지적했고 이를
제거했다. 최종 재검토 verdict는 `pass`이며 IssueOps record도 pass로 갱신했다.

## 원격 publication 경계

현재 GitHub open issue는 #117 한 건이고 open PR은 0건이다. 이 보고서 commit은
자기 자신의 push·pull_request CI 결과나 merge SHA를 본문에 포함할 수 없다.
따라서 exact final-report HEAD를 push한 뒤 다음을 generation-fenced 외부 receipt로
완료한다.

1. Korean remote artifact gate와 deterministic scorer 뒤 draft umbrella PR 생성.
2. exact head의 push CI와 pull_request CI가 모두 success인지 확인.
3. generation 5 `execution complete`, ready 전환, expected-head CAS merge.
4. #117 completion 반영·close, remote branch/worktree/local branch/lifecycle cleanup.
5. GitHub open issue, open PR, IssueOps record를 모두 0건으로 재검증.

## Turing evidence block

Success criteria: 11개 child 완료, finalization-only scope, fresh hook-enabled
Codex/Claude deny, full local unit/race/vet/build와 deterministic self-verify는 PASS다.
Final exact-head CI, merge, issue close와 cleanup은 publication 이후 provider
readback과 generation-fenced IssueOps receipt를 권위 있는 종료 증거로 사용한다.

Evidence artifact: `.agent-harness/turing/issue117-report.md`, finalization plan,
GitHub native sub-issue graph, hooks/list output, fresh Codex router block, Claude
hook event stream, full local gates와 250/250 self-verify summary.

Cleanup receipt: isolated home의 auth symlink만 제거했고 원본 `0600` auth file은
보존했다. 나머지 proof root와 out-of-scope `/tmp` probe artifact는 recoverable
`/Users/m16khb/.Trash/agent-harness-issue117-proof-20260802-1123`으로 이동했다.
두 canonical in-worktree sentinel은 생성되지 않았다.

Verification mode: high-risk full loop. 현재 hook-disabled parent는 제외하고 실제
configured host process, sequential full/race, architecture/golden/vet/build와
deterministic 10-iteration self-verify를 실행했다.

Skipped checks: 로컬 필수 check는 없다. `llm-eval=false`는 고정된 결정적 종료
gate다. Remote exact-head CI와 merge/cleanup은 보고서 commit 이후에만 물리적으로
가능하므로 위 외부 receipt 경계에 남긴다.
