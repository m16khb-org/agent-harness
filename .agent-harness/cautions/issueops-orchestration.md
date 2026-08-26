---
name: cautions/issueops-orchestration.md
description: Cautions for Orca supervised handoff create/dispatch/terminal/mailbox/rollover, sealed reconciliation, publication, and ownership schema.
---

# Orca supervised-handoff orchestration cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for Orca
worktree/terminal/task create ambiguity, correction/resume attestation,
supervised dispatch writer-lease confusion, runtime rollover identity,
worker_done mailbox authority, GitLab handoff, publication/cleanup authority,
sole-writer attestation, yielded-verification, sealed reconciliation, and
source-main-worktree fencing. Execution liveness and lease authority live in
[issueops-execution.md](issueops-execution.md).

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
- Parent issue create도 provider 호출 전에 marker/provider/project
  authority/request digest를 durable하게 봉인한다. `gh`/`glab`가 시작된 뒤
  timeout, nonzero, malformed URL, live verification 실패가 발생하면 같은 create를
  다시 호출하지 않는다. `issueops remote reconcile-issue`에서 marker 후보가
  정확히 하나이고 sealed title/body digest와 live label/assignee가 모두 맞을
  때만 채택한다. Zero/many는 absence/identity 증거가 아니다.
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
- `orca orchestration check`는 기본이 unread이고 `--all --inject`는 더 많은 history를 주입할 수 있다. repeat-prevention PreToolUse guard는 direct check의 any explicit `--inject`(equals/reordered 포함)를 record lookup 전에 차단한다. read-only JSON envelope의 message array는 `.result.messages`에 있다. top-level `.messages`를 조회하면 오류 없이 빈 결과가 나올 수 있으므로 absence 증거가 아니다. opaque `msg_*` ID를 lexical order/filter에 쓰지 말고 numeric `sequence`와 exact `taskId`/`dispatchId`, sender/recipient direction을 선택한다. Sequence는 selection evidence일 뿐 lease fence가 아니다. exact executable projection은 `skills/issueops/references/execution.md`만 원본으로 둔다. live terminal handle을 historical mailbox identity로 간주하지 않으며 urgent worker correction은 uniquely persisted handle의 literal-safe source-coordinator guidance만 사용한다. 자동 handle/mailbox 동기화는 issue #17이다.
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

## Sealed reconciliation에서 target CAS만으로 전체 소유권을 보호했다고 간주하지 말 것

- 개별 ref OID나 IssueOps record digest가 그대로여도 seal 이후 새 task, record, session binding이 같은 branch/worktree를 새로 소유할 수 있다. target CAS만 확인하면 새 owner를 final gate에서야 발견해 이미 봉인된 ref/state를 삭제한 뒤가 된다.
- 매 operation 전후에 journal order로 exact phase projection을 계산하고 Orca terminal/worktree/task/dispatch/gate/inbox, Git worktree/local·remote ref, IssueOps record/session/other row, state artifact를 모두 비교한다. `started` recovery는 해당 operation의 before/after 중 하나만 허용하며, inventory drift를 target readback 성공으로 삼켜 `verified`로 전진하지 않는다.
- collection의 current terminal argument는 추측 가능한 입력이 아니라 official no-selector `orca terminal show`가 resolve한 current handle에 대한 assertion이다. 실제 runtime rollover에서 spawn-time `ORCA_TERMINAL_HANDLE`은 stale해졌지만 tab/leaf와 pane/worktree composite는 현재 pane을 계속 식별했다. 따라서 complete terminal/worktree inventories와 명시적 tab/pane/worktree 환경값으로 current row의 runtime/handle/PTY/tab/leaf/worktree ID/path/connected/writable tuple을 유일하게 증명하고, 환경 handle의 explicit probe는 같은 current row 또는 exact structured `terminal_handle_stale`만 허용한다. 이 검증을 collection 안정화, 모든 live CLI 진입, mutation fence 전후에 반복하며 sealed tuple과 비교한다. stale 환경 handle을 current value로 export/override하거나 manifest authority로 저장하지 않는다.
- sealed reconciliation의 final stability audit은 `manifest.current_terminal.handle`을 singular `--preserve-terminal` argv로 전달한다. `explicit or env`처럼 truthiness로 선택하면 blank assertion이 stale fallback을 되살리므로, explicit presence를 별도로 분기하고 blank/invalid/overlong/repeated input은 build·doctor·cleanup 전에 거부한다.
- mutation이 성공한 뒤 preservation fence의 `TimeoutExpired`가 raw exception으로 빠지면 planned-operation ambiguity handler가 target-only `_operation_satisfied`로 이를 성공 처리할 수 있다. Fence의 timeout/parse/incomplete/drift는 ordinary operation과 reset 모두 먼저 non-recoverable `InventoryDriftError`로 정규화하고, 그 호출에서는 journal을 `started`에 남긴다. Exact target readback은 완전한 post-fence가 성공한 invocation ambiguity만 복구할 수 있다.
- fresh recovery bundle의 `test_reconcile.py`는 bundle에 봉인되지 않은 `simulate_copy.py`를 import하지 않는다. 최종 bundle에 복사할 exact 파일 집합만 clean temporary directory에서 직접 실행해 PASS를 확인한다. Simulator는 별도 copied-DB gate로 실행하고 bundle executable dependency로 만들지 않는다.
- `REJECTED` 같은 unsealed marker 파일만 추가해도 현재 bundle validator는 이를 무시한다. 폐기 bundle은 증거 내용은 보존하되 sealed runner mode를 `0400`처럼 계약 밖으로 바꾸고 `validate` nonzero를 확인해 실행 불가능하게 만든다. Marker만 보고 재사용 방지가 됐다고 주장하지 않는다.
- Python canonical JSON과 맞추는 Go CAS decoder는 `UseNumber`를 사용한다. 일반 `json.Unmarshal(any)`의 `float64` 변환은 `2^53`보다 큰 integer를 반올림해 raw가 같은 record의 canonical SHA-256을 다른 값으로 계산한다.

## source main worktree를 supervised fence로 막지 말 것

`io-b9f8cd45e152`는 2026-07-20의 read-only incident evidence다. 이 기록에는 conversion, dispatch, cancellation, cleanup 명령을 제안하거나 실행하지 않는다. 새 구현 설치 뒤에도 별도의 live readback과 인간 결정 없이는 처분하지 않는다.

- Current ownership contract는 workspace provisioning before ownership transfer를 보장한다. source main worktree remains available before, during, and after handoff; generic session binding과 mirrored relative path는 authority가 아니다.
- Fence는 canonical worker root, exact cycle ID, native owner, or persisted Orca resource만 선택한다. Removed handoff record shapes are rejected rather than converted.
- Record 선택에 `SourceRoot`, source `cwd`, generic repo root를 다시 넣지 않는다. 명시적 canonical target이 있거나 command cwd 자체가 canonical root일 때만 cycle write lease를 적용한다. Codex hook의 top-level cwd는 effective `exec_command.workdir`가 아니므로 explicit canonical file target은 holder/lease/containment가 맞으면 source session cwd에서도 허용한다.
- Orca의 즉시 terminal-create 응답은 stable title이 아직 반영되지 않을 수 있다. 같은 worktree와 PTY의 bounded terminal inventory를 한 번 재조회해 sealed marker를 검증하고, 없거나 중복이면 fail closed한다.
- `cleanup_pending_human_decision`의 every non-`closed` ownership resource는 stale scan과 operational health가 보존한다. elapsed time, Stop hook, original source identity는 cleanup authority를 만들지 않는다.
