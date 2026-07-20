# Pioneer Skills 2026-07-20 전수 감사 개선 로드맵 (P0–P4)

## TL;DR
> **Summary**: 2026-07-20 전수 감사(스킬 11종 4,200줄 + references/scripts/testdata 전량 정독, 실행 계약 실측 검증)에서 도출된 결함을 P0(안전·치명) → P4(인프라 재발 방지) 순으로 수정한다. 6월 품질 프로그램의 성과(가드레일 수정 안착, harness/orca/git 계약 stale 0건)는 유지하고, 이번에 드러난 실행 계층 결함·측정 드리프트·커버리지 갭을 닫는다.
> **Deliverables**: P0 안전 수정 3건, P1 정확성 수정 10건, P2 references 분리 패스(6개 스킬), P3 사용성 패스(경량 모드·라우팅 양방향화·한국어 출력 가이드), P4 인프라 5건(측정 편입·핀 확대·검증 도구 2종·stop-hook 재접지)
> **Effort**: Large (Wave 0–5, provider-native child 12개; 초기 3–5 세션 추정은 폐기)
> **Parallel**: YES — prerequisites가 없는 #51/#52/#62만 `[p]`; #59/#60은 #51 수락 후 같은 wave에 동시 dispatch하는 `[s]`
> **Critical Path**: #62 → child dispatch; #51/#52 → #53 → #54 → #55 → #61; #54 → #56 → #57/#58

## Context

### Original Request
사용자 선택지 2: "P0~P4 전체 로드맵을 `.agent-harness/plans/` 계획 문서로 먼저 기록해 우선순위를 확정한 뒤 수정에 착수한다."

### 근거 (감사 출처)
- 4개 병렬 서브에이전트 전수 감사 (2026-07-20, main @ 9ec412a): 그룹1 berners-lee/brooks/codd, 그룹2 dijkstra/engelbart/hopper, 그룹3 karpathy/shannon/torvalds, 그룹4 turing/von-neumann. 모든 결함은 file:line 증거 + 실행 재현(macOS, git 2.50.1, `grep`→ugrep 7.5.0 별칭 호스트) 기반.
- 인프라 층 직접 조사: rubric v2·홀드아웃 스위트(`.agent-harness/evidence/pioneer-skills-quality/`), 통합 벤치마크(`internal/core/issueops/benchmark/`), 라우팅(`internal/core/hookprompt/rules.go`), 회귀 핀(`internal/core/skillcontract/`), 배포 경로(`~/.claude/skills` 심링크).
- 선행 이력: `.agent-harness/plans/pioneer-skills-quality-improvement.md` (6월 사이클, 완료), `.agent-harness/operations/pioneer-v2-regrade-2026-06-16.md` (v2 홀드아웃 평균 4.78/5).

### 검증된 회귀 없음 (수정 불필요, 유지 대상)
- 6월 수정 전건 안착: karpathy CoT 프라이버시(다중 인코딩), shannon untracked 포함, torvalds rebase 확인 사다리(`:105-111`), turing proportionate mode(`:149-162`), von-neumann decline-to-plan routing record(`:131-141`), 가짜 CLI 제거.
- harness/orca/git 실행 계약 stale 0건 (전수 실측). `agents/openai.yaml` 4-key shape 일관 (brooks 1건 제외).

---

## P0 — 안전·치명 (즉시, 상호 독립 3건)

### P0-1. engelbart: 실제 내부 회의록·실명 git 노출 제거 [CRITICAL]
- 증거: `skills/engelbart/testdata/ai_devops_onboarding_transcript.txt`(353줄, 추적 중) — 클라우드 마이그레이션 계획·파트너 정책·벤더 비용·실명 직원 4명 원문. 처리본(554줄)도 추적 중. `.gitignore:31`은 같은 성격의 `background.local.md`를 보호하는데 픽스처가 누출.
- 수정: 가공 인명·가상 서비스·가상 ID로 완전 합성한 회의록으로 픽스처 6종 전면 교체 → `scripts/engelbart_quality_rubric.py:44-46`의 기대 문자열 재유도 → `scripts/engelbart_skill_contract_test.py`(14 케이스) 녹색 확인.
- 검증: `python3 scripts/engelbart_skill_contract_test.py` exit 0, `python3 scripts/engelbart_quality_rubric.py` 통과, `git grep`으로 실명 4건 0 hit.
- ⚠️ **사용자 확인 지점**: 워킹트리 교체(`git rm` + 신규 커밋)만으로는 이력에 원문이 남는다. 이력 정리(filter-repo 등)는 공유 저장소 force-push를 수반하는 비가역 작업이므로 **별도 명시 승인 없이 실행하지 않는다**. 기본 실행 범위는 워킹트리 교체까지.

### P0-2. shannon: 측정 shell 크래시·무성 거짓 0 수정 [CRITICAL]
- 증거: `skills/shannon/SKILL.md:203-205` Metric 4 — `grep -c … || echo 0`이 무매치 시(`grep -c`는 0 출력 + rc=1) `BOILER="0\n0"` → `$((TOTAL - BOILER))` 산술 크래시, 모든 POSIX 호스트 재현. `:82,86-87` Metric 1 — ugrep 별칭 호스트에서 패턴 에러를 `|| true`가 삼켜 비어있지 않은 diff에 SNR=0 보고. 부수: `:367-368` xargs `total` 행 누출, `:376-377` awk 빈 문자열, `:199` 비인용 for-loop.
- 수정: `:82,86,87,134,203` 전부 `command grep -E`로 통일. `|| true`/`|| echo 0` 전량 제거 후 `${VAR:-0}` 정규화. 선행 자체진단 추가(`printf '+a\n' | command grep -cE '^\+[^+]'` ≠ 1이면 측정 중단). `:367` `| grep -v ' total$'`, `:376` `END{print d+0}`, `:199` `-z` + `while read -d ''`.
- 검증: 수정된 각 명령 블록을 이 호스트(ugrep 별칭 + BSD grep)에서 복붙 실행, 빈 diff/무매치/공백 경로 3케이스 수동 재현.

### P0-3. torvalds: bisect 조용한 오명령 + clean -fd 사다리 부재 [HIGH]
- 증거: `skills/torvalds/references/bisect-protocol.md:34,42` — 비인용 `$TEST_CMD` 확장으로 다른 명령이 실행되나 bisect는 범인 커밋을 자신 있게 지목(오답·무오류). `SKILL.md:333` — `git clean -fd`는 reflog·백업 브랜치 무효(untracked)인데 확인 사다리(`:105-111`)가 rebase에만 존재.
- 수정: `$TEST_CMD` 관용구를 스크립트 파일 방식(`bisect-test.sh` + `git bisect run ./bisect-test.sh`)으로 교체. clean 사다리 추가: `git clean -nd` dry-run → 파일 목록 제시 → 기본 대안 `git stash push -u`(`:40`이 이미 선호 명시) → 거부 시에만 명시 확인.
- 검증: 인용 깨짐 재현 케이스로 전후 비교, 사다리 텍스트가 `:105-111`과 동일 6단계 구조인지 확인.

## P1 — 정확성 (파일:줄 수준 수정 10건)

| # | 스킬 | 증거 | 수정 |
|---|------|------|------|
| P1-1 | berners-lee | `SKILL.md:88,253,306-313` 서브에이전트 패턴 번호 4중 3 오류(정본 `SUB_AGENT_PATTERNS.md:33-46`과 불일치, brooks `:26`의 정확한 인용과 모순, 잘못된 execution-decision slug 기록 유발) | 번호 대신 slug 문자열(`high-volume-exploration`, `devils-advocate-review`, `parallel-independent-research`, `cross-verification-consensus`)로 전면 교체 |
| P1-2 | brooks | 판정 기록 명령 부재 — 기록은 fail-closed implement 게이트(`issueops_readiness.go:143`)인데 109줄에 명령 없음 | `## IssueOps Integration` 섹션 추가: `agent-harness issueops devils-advocate review --id … --verdict … --finding … --json` + MCP 등가 명시. **별건**: 이 명령이 `internal/adapter/cli/usage.go`에서도 누락 — CLI 문서 갭 수정 |
| P1-3 | karpathy | `SKILL.md:417` "Shannon이 프롬프트 품질 측정" — 허위(shannon은 코드 지표만) | 행 삭제 또는 "Shannon은 코드 품질 측정; 프롬프트 변경이 생성 코드를 바꿀 때 산출물에 한해 적용"으로 재기술. `:418` issueops 주장 하향 |
| P1-4 | turing | `SKILL.md:24-131` 108줄(20%) supervised-handoff 산문이 `issueops/references/orca-handoff.md`와 중복 + `:129,131` 자기 지시 위반 | ~5줄 링크 stub로 교체("active `execution_handoff` 시 orca-handoff.md 로드; Turing은 evidence/receipt 의무만 보유") |
| P1-5 | turing | `references/evidence-contract.md` 고아+stale(참조 0건, `:3`/`:74`가 proportionate mode와 충돌) | 삭제, 또는 유지 시 SKILL.md 링크 + proportionate 예외 + 파일명 규약 통일(`:81` vs `SKILL.md:173`) |
| P1-6 | torvalds | `references/rebase-protocol.md:64` "90일 후 git gc가 정리" — 거짓(백업 브랜치는 ref, gc는 절대 prune 안 함) | "삭제 전까지 영구 유지: `git branch -D $BACKUP`"으로 정정 + 정리 지침 추가 |
| P1-7 | turing | `SKILL.md:190-195` QA 채널 도구명 — "agent-browser" 실존 안 함, `xdotool` macOS MISS | ch3 → `mcp__claude-in-chrome__*` 명시, ch4 → macOS `osascript`/`xdotool`은 Linux-only 표기 (`:235-242`의 host-availability 정직성 패턴 동일 적용) |
| P1-8 | von-neumann | `SKILL.md:172-181` pseudo-API `task(subagent_type="explore")` — 어느 host에도 없음 | 같은 파일 `:98`/`:151`의 host-neutral 문구 재사용으로 교체 |
| P1-9 | hopper | `SKILL.md:42,108` 전략 개수 3 표기(실제 4) + `:252-254` 미등록 self-augment 후보 ID `hopper-debug` + `:313-323` Strategy D 라우팅 행 부재 | 개수 4로 정정, 실존 후보 ID로 교체, golden/snapshot 행 추가 |
| P1-10 | dijkstra | `SKILL.md:198-223` 펜스 파손(203 열림, 208 닫힘, 212-222 비펜스, 223 고아 펜스) — scaling test 해석 키 훼손 | 펜스 재구성 |

## P2 — 토큰 경제: references/ 분리 일괄 패스 (스킬별 병렬 가능)

원칙: codd(83줄 본문 + `references/deep-database-review.md`, 중복 0)가 정본 패턴. 활성화 시점 자료(게이트·계약·단계 흐름·NEVER/ALWAYS)만 본문에 남기고 조회형 자료(표·패턴 라이브러리·템플릿)는 분리. **주의: `internal/core/skillcontract/skill_contract_test.go`가 assert하는 문구는 본문에 반드시 보존**(예: berners-lee 4개 문구 `:66-73`).

| 스킬 | 현재 | 분리 대상 | 목표 |
|------|------|-----------|------|
| dijkstra | 569줄, ref 0 | `:242-268,276-305,486-510` 조회표 ~110줄 → `references/complexity-tables.md`; `:57-128` 구조적 프로그래밍 에세이 71줄(자기 정체성 `:13`과 모순) → 이동 또는 삭제 | ≤420줄 |
| turing | 538줄 | P1-4 반출(−105줄) + P1-5 | ~420줄 |
| von-neumann | 494줄 | `:234-245` inline 삭제 → `references/clearance-checklist.md` 링크(고아 해소+ref의 추가 가치 회수); plan template `:336-438` ~100줄 → `references/plan-template.md` | ~380줄 |
| karpathy | 465줄, ref 0 | `:296-405` 패턴 110줄, `:151-172`, `:256-292` → references | ≤200줄 상주 |
| shannon | 457줄, ref 0 | `:58-209` 지표 상세 ~150줄 → `references/metrics.md` (공식·임계값·단계 흐름만 본문 유지) | ~300줄 |
| berners-lee | 374줄 | `:259-302` 템플릿 45줄 인라인 삭제(레퍼런스가 정본, Confidence 필드 드리프트 이미 발생) + `:107-244` fetch 사다리 146줄 → `references/fetch-resilience.md` (skillcontract 4문구는 stub에 보존) | ~190줄 |
| hopper | 375줄, ref 0 | `:262-308` 언어별 표 47줄 → `references/cross-language-debugging.md` + `:283-293` 중복 삭제 | ≤320줄 |
| engelbart | 382줄 | 4×/5× 반복 품질 규칙 ~170줄 → references 통합 | ≤240줄 |

검증: 분리 후 `go test ./internal/core/skillcontract/ -count=1` 녹색 + `scripts/validate-skill.py` 전 스킬 통과.

## P3 — 사용성 패스

### P3-A. 경량 모드 이식 (5개 스킬)
정본 패턴: karpathy lifespan gate(`:59-67`, `skill_contract_test.go:34-35`로 핀) + turing proportionate mode.
- brooks: `## Proportionality` — 단일 모듈 5단계 미만 계획은 Gate 1·3·4만 + 판정/최대 결함/더 작은 계획 반환.
- codd: `## Proportional Mode` — 단일 쿼리/인덱스 질문은 SURVEY(행수만)→INDEX→VERIFY, 다중 테이블/스키마 변경 시 7단계 전체.
- shannon: 소형 diff quick 모드(SNR+대형파일만) vs PR 게이트 4지표 풀 모드.
- von-neumann: Phase-0 Trivial/Standard에서 wave 선택화 + F1-F4를 단일 체크로 축약(`:334,:367,:429-434`). non-negotiable(References/Acceptance/QA)은 유지.
- berners-lee: `:334-337` 불릿에 묻힌 Quick-lookup을 Phase 0 직후 1급 섹션으로 승격.

### P3-B. 라우팅 양방향화 + superpowers 경계 선언
- 검증된 양방향 쌍은 torvalds↔atomic-commit-push 유일. 주장된 모든 관계 엣지가 양쪽 파일에 존재하도록 1회 정합 패스: dijkstra에 shannon 행 추가(`shannon:13,303,406,442`가 이미 4× 역참조), codd에 관계 표 신설(dijkstra/hopper/berners-lee/von-neumann 경계), hopper에 `superpowers:systematic-debugging` 경계 행, turing `:522-534`에 superpowers:verification-before-completion + issueops 소유권 2행, von-neumann에 superpowers:writing-plans/brainstorming + EnterPlanMode 경계.
- repo 자기모순 해소: `issueops/SKILL.md:62`(brainstorming) vs `:257`(von-neumann) — 한쪽 양보 결정 필요. ⚠️ **사용자 확인 지점**(워크플로 정책 결정).
- torvalds `:305-309` 관계 행 4건이 자기 경계(`:15,314-315`)와 모순 — 정리. `:303`의 preflight 위임 주장은 미구현(`git_preflight.py`에 신호 0) — 산문 삭제 또는 구현 결정.
- berners-lee 관계 표에 외부 플러그인 `deep-research` 경계 행 추가.

### P3-C. 한국어 로케일
- 전 스킬 공통 출력 규칙 추가: "산문·발견은 사용자 언어를 따르되, 증거 블록 라벨 키는 영어 유지(`hasStructuredClause`가 영어 부분문자열 매칭 — `issueops_pioneer_checks.go:89-98`)." 이 제약은 현재 미문서화 상태로, 완전 한국어 증거 블록은 조용히 채점 실패.
- turing·von-neumann frontmatter description에 한국어 트리거 추가(현재 0; stability-audit의 "전수조사/안정성 점검" 패턴 준용). 본문 번역은 하지 않음.
- IssueOps feedback `--body` 예시 3건(karpathy `:464`, shannon `:456`, torvalds `:368`)을 한국어로 — repo 관례(한국어 remote issue/PR)와 정합.

## P4 — 인프라 재발 방지

- **P4-A. 측정 편입·재측정**: brooks·engelbart를 격리 rubric과 `issueOpsPioneerSkillEvidenceComplete`(`issueops_pioneer_checks.go` — brooks는 현재 `default: return false`)에 편입. 6/16 이후 대량 변경 스킬(codd −1,000줄, turing +155줄)과 P0–P3 수정분 재측정 → `quality-dashboard.md` 갱신. ⚠️ 홀드아웃 fresh-context 재실행은 토큰 비용이 있어 **사용자 opt-in**(대시보드 기존 정책과 동일).
- **P4-B. skillcontract 핀 확대**: brooks(서브에이전트 강제·regress 시맨틱·P1-2 반영 후 기록 명령), codd(라이브 DDL 금지·EXPLAIN-실행 경고), torvalds(확인 사다리·clean 사다리), dijkstra(Gate 0), turing(proportionate mode), berners-lee(패턴 slug). 근거: 이번 감사에서 핀된 계약은 지켜졌고 핀 안 된 상호참조가 썩었다.
- **P4-C. `scripts/validate-skill.py`에 description 길이 검사 추가**: brooks 1,020~1,026자로 한도(~1,024) 대비 여유 4자 — 편집 한 번에 조용히 잘림. brooks description 자체도 ~700자로 축소(전기 서술은 `<identity>`에 이미 존재). brooks `agents/openai.yaml:4` default_prompt 411자 → 형제 스타일 한 문장.
- **P4-D. `scripts/verify-skill-shell.sh` 신설**: SKILL.md fenced bash 블록 추출 → macOS/BSD + ugrep 별칭 환경에서 dry-run. 이 도구 하나면 S1–S5·T1을 기계적으로 전부 잡았음. 계열 규칙 명문화: `command grep -E` 사용, `||`를 에러 억제로 금지, 머지 전 1회 실행.
- **P4-E. 고아 references lint**: SKILL.md가 링크하지 않는 `references/*` 파일을 결함으로 검출(이번 감사에서 turing·von-neumann 각 1건 — repo 비용+미로드+drift의 최악 조합).
- **P4-F. engelbart stop-hook 재접지**: `cmd/harness/hookcli/hook_stop.go:171-189` — 산문 substring 매칭이라 engelbart를 *논의*만 해도 발화(이번 감사에서 실제 발화, 회의록 날조 압박). 실제 canvas-create tool-call 증거로 게이트하고, 부재 시 사용자에게 표면화.
- **P4-G. 미세 항목**: shannon `ai-slop-clean` 핸드오프 재기술(스킬 아님 — `issueops/references/ai-slop-clean.md` 단계) + `shannon-latest` state 루프 폐쇄(Phase 0 기록→Phase 1 판독). karpathy에 `command -v` 가드(`:196-202`)와 하네스 도구(`skill_manifest`/`contract_check`) 연결. 아티팩트 경로 정책 명문화(버전 프롬프트=추적, 측정 증거=무시). `~/.claude/skills/clova-meeting-minutes` dangling 심링크 제거.

## 실행·검증 공통 계약
- 각 단계는 issueops 사이클로 진행, 커밋은 atomic-commit-push 계약 준수.
- 전 단계 공통 검증: `go build ./cmd/harness && go test ./... -count=1`, `scripts/validate-skill.py` 전 스킬, `python3 scripts/engelbart_skill_contract_test.py`(engelbart 접촉 시).
- 승인 경계 요약: ① P0-1 git 이력 정리(비가역) — 명시 승인 필수, ② P3-B issueops 라우팅 자기모순 해소(정책 결정), ③ P4-A 홀드아웃 fresh-context 재실행(토큰 비용 opt-in). 그 외는 가역적 파일 수정.

## IssueOps 실행 분해 보정 (Brooks review, 2026-07-20)

초기 child 분해의 #53(P2+P3)과 #54(P4 전체)는 독립 검증·rollback 경계를 충분히 보존하지 못해 Brooks 판정 `revise`를 받았다. 원래 P0–P4 항목은 유지하되 실행 단위를 다음처럼 보정한다.

- Wave 0: #62가 세션별 Codex model/reasoning을 sealed handoff context와 안전한 Orca launch command에 추가한다. 이 기능 없이는 자기 자신을 요구 모델로 handoff할 수 없어 parent coordinator가 TDD bootstrap한다.
- Wave 1: #51 P0, #52 P1 병렬. Torvalds 소유권은 #51=`SKILL.md`+`bisect-protocol.md`, #52=`rebase-protocol.md`; parent가 통합 안전 계약을 검증한다.
- Wave 2: #53 P2 reference extraction만 수행한다.
- Wave 3: #54 P3-A/C, #59 Shannon shell pilot, #60 Engelbart stop-hook를 독립 실행한다.
- Wave 4: #55 P3-B의 검증된 edge만 #54 수락 후 실행하고, #56 Brooks/Engelbart evidence 계약·fixture를 실행한다.
- Wave 5: #58 metadata/orphan-reference 정적 검증은 #56 수락 후, #61 P4-G source 후속은 #55 수락 후 실행한다. #57은 #56의 확정 계약을 소비해 deterministic benchmark·pins·offline dashboard를 구현한다.

P4-D는 Shannon의 실제 회귀 fixture pilot부터 시작하고 재사용 가치가 입증되기 전에는 범용 fenced-bash extractor를 만들지 않는다. Fresh-context holdout은 실행하지 않으며 isolated score는 `not remeasured`로 남긴다. 모든 격리 child는 Orca coordinator가 worktree와 Codex 세션을 만들고 `gpt-5.6-terra`, `model_reasoning_effort=high`로 supervised handoff한다.
