# Go, package, port, and adapter conventions

> Family index: [`../CONVENTIONS.md`](../CONVENTIONS.md). This module owns Go
> naming and file structure, the core/port/adapter layer boundaries, the
> dependency principle and fitness ratchet, the concrete-adapter removal
> ordering, SOLID/YAGNI/KISS guidance, and shared-skill packaging. CLI/MCP and
> response contracts live in [`cli-mcp-and-output.md`](cli-mcp-and-output.md);
> runtime state, policy, guard, hook, and lifecycle rules live in
> [`state-policy-and-hooks.md`](state-policy-and-hooks.md).

## 1. 네이밍 / 구조

- 프로젝트 이름: `agent-harness`
- CLI 바이너리 이름: `agent-harness`
- Go module 이름은 repo 원격이 확정되면 `agent-harness`를 현재 로컬 module 이름으로 사용한다.
- 파일명은 snake_case를 사용한다.
- Go 패키지명은 짧은 소문자 단어를 사용한다.
- 테스트 파일은 대상 파일 가까이에 `*_test.go`로 둔다.

현재 구조(대표 경로):

```text
cmd/harness/main.go
cmd/harness/<cli>/                 # harnessapp, issueopscli, mcpcli, workercli, daemoncli, hookcli, installcli, ...
cmd/harness/testdata/*.golden.*
internal/contract/<capability>/    # transport/state가 공유하는 versioned DTO
internal/domain/<capability>/      # I/O를 모르는 순수 규칙과 reducer
internal/application/<capability>/ # domain과 port를 조합하는 use case
internal/port/<capability>/        # 외부 capability interface와 error contract
internal/adapter/inbound/          # capability inbound adapter
internal/adapter/outbound/         # state/SQL/webfetch 등 capability outbound adapter
internal/adapter/codex/
internal/adapter/claude/
internal/domain/hook/
cmd/harness/hookcli/
internal/adapter/installutil/
internal/adapter/provider/         # github/gitlab issue provider
configs/codex/
configs/claude/
skills/
.agent-harness/
```

---

## 2. 레이어 경계

| 레이어 | 책임 | 의존 가능 | 금지 |
|--------|------|-----------|------|
| `contract` | transport/state가 공유하는 DTO, schema version, error vocabulary | 다른 contract, 표준 라이브러리 | 판정 로직, filesystem/process/DB I/O |
| `domain` | 순수 규칙, reducer, classifier | contract, 순수 domain helper, 표준 라이브러리 | adapter/cmd, filesystem/process/DB I/O. clock은 기본 주입하되 `auditid` timestamp ID 생성은 현재 명시적 예외 |
| `application` | domain과 좁은 port를 조합하는 use case | contract, domain, port | concrete adapter, cmd transport |
| `port` | 외부 capability interface와 error contract | contract, 표준 라이브러리 | domain/application/adapter/cmd concrete 구현 |
| `adapter/inbound` | capability request를 application 호출로 변환 | contract, application | outbound adapter 직접 호출 |
| `adapter/outbound` | filesystem, process, Git, DB, network 구현 | contract, port, domain의 순수 helper | transport 정책 복제 |
| `cmd/harness/<cli>` | flag/stdout/stderr/JSON-RPC와 command dispatch | contract, domain catalog, application, 주입된 dependency | host별 정책 복제, domain 판정 재구현 |
| `adapter/codex` | Codex user skill/MCP 설치 구현 | contract, port, 표준 라이브러리 | 적용 대상 repo 파일 쓰기 |
| `adapter/claude` | Claude user skill/hook/MCP 설치 구현 | contract, port, 표준 라이브러리 | 기본 설치에서 `.claude/skills` 같은 repo-local 파일 쓰기 |
| `domain/hook` + `cmd/harness/hookcli` | host별 hook 출력 변환과 context command 전달 | contract, domain catalog | host schema와 다른 응답, lifecycle 변경 |
| `adapter/provider` | github/gitlab issue·PR/MR·child 생성/검증(gh·glab CLI) | contract, port, os/exec | 정책 복제, root 밖 접근 |

> `cmd/harness/harnessapp`가 concrete adapter를 조립하는 유일한 composition root다. command별 CLI와 daemon/MCP transport 구현은 현재 `cmd/harness/*cli`에 있고, 공통 catalog/판정은 `internal/domain`, DTO는 `internal/contract`가 소유한다.
> filesystem/git/process 구현은 하나의 범용 fs adapter에 모으지 않고 capability별 outbound adapter로 둔다. `internal/adapter/install`처럼 아직 application orchestration을 함께 가진 기존 package는 새 의존을 확대하지 않고 capability vertical로 점진 이동한다.

---

## 8. Dependency 원칙

- 새 dependency는 표준 라이브러리로 명확히 부족할 때만 추가한다.
- CLI/MCP library는 기능보다 안정성과 유지보수성을 우선한다.
- dependency 추가 시 `go.mod`, license, 보안 위험을 확인한다.
- 생성물은 직접 수정하지 않는다. 생성 스크립트와 source를 고친 뒤 재생성한다.

### Dependency fitness ratchet

- `internal/architecture/dependency_test.go`는 direct production import만 검사한다. edge 표기는 항상 `importer -> imported`이며 정렬 순서를 바꾸지 않는다.
- legacy baseline은 없다. 전환이 끝났으므로 `internal/adapter/*`를 composition root 밖에서 import하는 edge는 새로 추가할 수 없다. `TestProductionGraphHasNoLegacyAdapterEdges`가 즉시 실패한다. 어댑터 기능이 필요하면 세 갈래 처방(순수 규칙은 domain, 타입은 contract, I/O는 주입)을 따른다.
- composition root 예외는 `cmd/harness/harnessapp` 하나로 제한한다. 새 concrete-adapter import가 그 밖에 필요하다면 먼저 boundary를 재검토한다.
- IssueOps 수직 마이그레이션은 capability별 contract/domain/application/inbound/outbound 패키지를 사용한다. domain은 JSON·filesystem·process·SQLite·clock을 import하지 않고, application port는 해당 capability가 실제로 쓰는 좁은 연산만 선언한다. persisted bytes가 공개 계약이면 legacy facade와 새 vertical의 differential 및 race evidence를 함께 유지한다.

### concrete-adapter 의존을 걷어내는 순서

legacy edge를 없앨 때는 **소비되는 심볼의 성격**이 처방을 결정한다. 심볼을 먼저 분류하고
그에 맞는 방법을 쓴다. 순서를 뒤집으면 옮길 곳이 없거나 edge가 줄지 않는다.

| 소비 형태 | 처방 | 근거 |
|---|---|---|
| 순수 함수·규칙 | `internal/domain/<cap>`으로 이동 | 규칙은 I/O보다 아래 계층에 속한다 |
| **타입** | `internal/contract/<cap>`으로 이동 | 구조체 필드 타입은 주입으로 대체할 수 없다 |
| I/O 함수 | composition root가 주입 | 구현 선택은 root의 결정이다 |

- **타입 이동과 함수 주입은 대개 둘 다 필요하다.** 한 capability의 소비자가 타입과 함수를
  함께 쓰면, 타입만 옮겨도 여전히 adapter를 import하고 함수만 주입하면 시그니처에 쓸 타입이
  없다.
- domain은 contract와 순수 domain helper를 import할 수 있으며 Go import graph는 acyclic이어야
  한다. 특정 IssueOps vertical의 stricter ratchet이 contract-only dependency를 요구할 때만 그
  범위를 좁힌다. 두 capability가 공유하는 wire/persisted 타입은 contract에 두고, redaction 같은
  보안 규칙을 중복 선언해 dependency 규칙을 우회하지 않는다.
- **주입에 default 구현을 두지 않는다.** default가 concrete를 가리키면 그 package가 다시
  adapter를 알게 된다. 미주입은 조용히 통과시키지 말고 구조화된 오류로 드러낸다.
- 하위 package만 정리해서 상위가 대신 그 adapter를 import하게 되면 edge는 **이동만 하고 줄지
  않는다.** 상위가 이미 그 adapter를 아는 경우에만 하위 정리가 순감이 된다.
- fitness graph는 test import를 수집하지 않는다. 테스트는 production wiring과 **같은 concrete
  구현**을 주입해 동작을 그대로 검증한다. 스텁으로 바꾸면 검증 대상이 달라진다.

---

## SOLID / Design Pattern 적용 지침

SOLID, YAGNI, KISS는 함께 적용한다. SOLID는 인터페이스와 계층을 많이 만들라는 뜻이 아니라, 실제 변경 축이 확인된 곳에서 책임과 의존 방향을 선명하게 하라는 지침이다. Design Pattern은 문제를 설명하고 유지보수를 줄이는 이름일 때만 사용한다.

### 좋은 케이스

- 기존 코드에 이미 있는 Adapter, Strategy, Factory, Repository 같은 패턴을 같은 문제에 일관되게 적용한다.
- 외부 host, SDK, filesystem, process, network처럼 교체 가능성이 실제로 있는 경계에 interface/port를 둔다.
- 두 개 이상의 구현이 있거나 테스트 double이 필요한 경계에서 dependency inversion을 사용한다.
- 책임이 섞인 코드에서 변경 이유가 서로 다른 부분을 작게 분리한다.
- 새 패턴을 도입할 때 ADR에 문제, 선택한 패턴, 기각한 단순 대안, 비용을 기록한다.

### 나쁜 케이스

- 단일 사용처를 위해 interface, factory, registry, plugin layer를 먼저 만든다.
- “미래 확장성”만 근거로 추상화하거나 설정 가능하게 만든다.
- 간단한 함수 호출을 패턴 이름에 맞추려고 class/object graph로 늘린다.
- 기존 repo 스타일과 다른 패턴을 작은 변경에 끼워 넣는다.
- SOLID를 이유로 core 정책을 host adapter에 복제하거나, host별 구현을 중복한다.

### 적용 규칙

- 먼저 가장 단순한 구현을 선택하고, 실제 variation point가 확인될 때 패턴을 도입한다.
- 새 abstraction은 최소 두 사용처, 명확한 테스트 경계, 또는 외부 기술 경계 중 하나가 있을 때만 만든다.
- 패턴 도입이 50줄 해결책을 200줄 구조로 만들면 되돌려 단순화한다.
- 패턴을 쓰는 경우 이름보다 계약을 문서화한다: 책임, 입력/출력, 금지된 의존 방향, 검증 방법.

## 9. Shared Skill 컨벤션

- 공용 스킬 원본은 `skills/<skill-name>/`에 둔다.
- Codex/Claude별 user skill 경로는 원본으로 향하는 symlink 또는 installer로 연결한다. 적용 대상 repo의 `.claude/skills`는 기본 설치에서 만들지 않는다.
- skill 이름은 lowercase/digit/hyphen만 사용한다.
- 각 skill은 `SKILL.md`를 반드시 포함하고, Codex UI metadata가 필요하면 `agents/openai.yaml`을 둔다.
- host별 설치 대상이 다르면 `install.json`에 `{ "hosts": ["codex"] }` 또는 `{ "hosts": ["claude"] }`처럼 명시한다. 생략하면 모든 host에 설치하는 기존 동작을 유지한다.
- 스킬 안에는 README, 설치 가이드, changelog 같은 보조 문서를 만들지 않는다.
- 검증은 repo-owned `python3 scripts/validate-skill.py skills/<skill-name>`로 수행한다. Codex/Claude host-managed system skill 사본의 `quick_validate.py`는 upstream 상태와 로컬 Python 패키지 설치에 따라 달라질 수 있으므로 필수 검증 경로로 쓰지 않는다.

설치 adapter 규칙:

- `internal/adapter/install.InstallNative`가 현재 host-neutral 설치 engine이고 `port.HostInstaller`가 concrete host write의 SOLID 경계다. 새 use case는 `internal/application/<capability>`에 두되 설치 경로는 검증된 계약을 보존하며 점진 이동한다.
- Codex/Claude adapter는 자기 host의 user/global 설정만 기본으로 쓴다. Codex는 `~/.codex/hooks.json`, Claude는 `~/.claude/settings.json`에 `SessionStart` 하나의 같은 context hook CLI만 등록한다. repo-local `.mcp.json`, `.claude/settings.json`, `.claude/skills`는 `--project-local` 같은 명시적 opt-in 없이는 만들지 않는다.
- 기본 symlink는 사용자 홈의 skill 경로에서 중앙 `skills/<name>`을 참조하거나 installer-owned command shim(`~/.local/bin/agent-harness`, `~/.local/bin/ah`)을 연결할 때만 사용한다.
- adapter 설치 계약을 바꾸면 `internal/adapter/install_contract_matrix_test.go`와 `internal/adapter/testdata/native_install_contract_matrix.golden.json`을 함께 갱신해 user/global 기본 설치와 explicit project-local opt-in의 차이를 보존한다.

self-augment/self-verify 교정 가드레일 (v1 S5/S6 승계):

- 모든 self-augment/self-verify 교정 후보의 `VerifyWith`는 **외부 검증 메커니즘을 최소 1개 명시**해야 한다 — 실행 가능한 도구 신호(`go test`/`go build`/lint/golden/contract/smoke/coverage 또는 CLI 명령), 또는 문서·거버넌스 후보(`doc_artifact`)의 경우 구체적 산출물(ADR 엔트리·README 섹션·checklist·matrix·transcript). 모델 자기비판("inspection으로 확인했다", "읽어보니 맞다" 등)은 주관 축(문서 가독성)에 한해 advisory이며 **correctness 게이트로 절대 사용 금지**다.
- 후보는 `VerificationKind`(`tool_signal`/`doc_artifact`)로 **명시 분류**하고, `qualitycatalog.VerifyWithGrounded`가 종류에 맞는 외부 메커니즘 명시를 강제한다(`internal/domain/qualitycatalog`·`cmd/harness/selfworkflow/augmentcatalog` 테스트).
- 본 규약은 **카탈로그 위생**(후보가 메커니즘을 *명명*하는지)을 강제한다. 메커니즘이 실제 존재·통과하는지의 *실행* 게이팅은 `agent-harness self-verify`/CI가 담당한다.
- 근거: intrinsic self-correction은 외부 신호 없이 추론을 악화시킨다(Huang/Kamoi, CRITIC). v1 S5("measured gap or no EDIT")·S6(정직성 단서)를 문서 규약에서 Go-test 강제 불변식으로 격상한 것이다.

---
