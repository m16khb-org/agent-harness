# 이슈 #106 — released 비-done 사이클 정리 탈출구: cleanup abandon

이슈: https://github.com/m16khb-org/issueops/issues/106

## 문제

폐기된 비-done 사이클 레코드(released lease, RemoteArtifact 없음)는 prune(done 전용, `issueops_prune.go:65-85`)과 cleanup finish(artifact 필수, `feedback_cleanup.go:277-279`) 어느 경로로도 정리할 수 없어 doctor `operational_inventory_unknown` error를 영구 유발한다. 실측: io-ff5300b4aa0b — phase=problem, orca pending intent(worktree_create) + external_operation_ambiguous failure, holder 부재 released lease, worktree·로컬 브랜치 디스크 부재, RemoteArtifact nil. **reconcile은 이 pending을 설계상 영구 유지한다**(`execution_orca_intent.go:135-136`: InvocationState != notInvoked + authoritative zero → retry 거부 — 실측 confirm 거부 확인). 주의: execution reconcile의 --preview는 orca를 조회하지 않는 순수 함수다(reconcile 소스 86-89행) — preview 출력을 잔여물 부재 증거로 인용하지 말 것(design-review A-2 정정).

## 설계 (design-review 라운드 1 revise 반영판)

`issueops cleanup abandon --id ID --reason TEXT (--preview | --apply --confirm --fingerprint SHA256)` 신설 — 로컬 레코드 수명 종료 전용 경로. **원격(이슈 본문·PR·브랜치) 무접촉.**

### core: CleanupAbandon (`internal/core/issueops/issueops_cleanup_abandon.go` 신규)

게이트(전부 fail-closed, missing 나열):
1. `reason_required`: --reason 공백 거부. 제어문자 금지·길이 상한(usage에 명시 — lease 가드의 exact-command 파싱과 충돌하는 문자 제약 안내).
2. `phase_not_done`: phase == done 거부(finish/prune 경로 전용).
3. `lease_terminal` (**allowlist**): `Execution == nil` 또는 `Lease.Status == released`만 통과. `active`/`claimable`/`revoking` 전부 거부(design-review F5 — revoking은 fenced holder 보유 상태, `model/execution.go:191-198`). LeaseStatus 4값 전수 테이블 테스트로 고정.
4. `no_remote_artifact`: RemoteArtifact != nil 거부(reflect→finish가 정답).
5. `pending_intent_safe`: `Execution.Pending != nil`이면 원칙 거부하되, 다음 전부 충족 시에만 통과(design-review 라운드 2 확정안):
   - `Pending.Kind == worktree_create` (로컬 orca mutation 한정 — remote PR 계열 kind는 무조건 거부 + `execution reconcile` 안내: 원격 고아 PR 위험). 이 게이트는 `validateExternalOrcaIntentPayload`(execution_orca_intent.go:363-367)의 stage 불변식(worktree 단계에서 Prepared·Launch·ClaimTokenSHA256·TerminalPTYID·TaskID 전부 공백 강제)에 의존한다 — worktree_create 밖으로 넓히면 claim token·prompt·packet·terminal·task 잔여물이 함께 열린다(design-review C-1·C-2, 신규 파일 주석으로 고정).
   - 기록된 workspace root 디스크 부재(게이트 6과 동일 검사 — worktree_create 산출물의 유일 위치)
   - **`InspectIntent` 게이트(design-review 라운드 2 차단 1)**: preview·apply 양쪽에서 sealed marker로 orca 인벤토리를 실조회해 `AuthoritativeZero == true && len(Candidates) == 0`을 필수 통과 조건으로 삼는다. provisioner는 `feedbackcleanup.Deps` 필드로 주입(orca.New()는 issueops.go:154에 기존재), 어댑터 부재·전송 실패·후보 1개 이상·비권위적 zero 전부 fail-closed 거부. 레코드의 Failure.Code는 5가지 상이한 애매성 경로에서 동일하게 기록되므로 레코드만으로는 "없음"을 증명할 수 없다(execution_orca_intent.go:113-157).
   - 통과 시 apply는 external_intent_v1에서 **`{Pending.OperationID} ∪ {Failure.OperationID}`(공백·중복 제거)** 행을 레코드와 **같은 sqlstore.Apply 원자 배치로 삭제**(design-review 라운드 2 차단 2). 소유자 가드: 행 payload의 `LifecycleID != record.ID`면 하드 에러(execution_state.go:154-157 lease 인덱스 규율 준용), 행 부재는 성공(멱등 — normalizeOrcaRemoveWorktreeErr 계약 동형). `deleteIssueOps` 자체는 불변(finish/prune 계약 무접촉), abandon 전용 원자 삭제 함수 신설.
6. `worktree_absent`: 기록 워크트리 경로(Execution.Workspace.Root ∪ record.WorktreePath, 불일치 거부 C2-F7 준용)가 디스크에 존재하면 거부. **로컬 검사 한정**(원격 브랜치 잔여는 비범위 — design-review F8).
7. `branch_absent`: 로컬 브랜치 ref 존재 시 거부.
8. `no_children`: `ChildCycles` 비공 또는 미종결 `child` 타입 IssueLinks 존재 시 거부(design-review F6 — 자식 고아 방지, finish의 `child_tasks_closed` 대응물).

### fingerprint / apply

- **신규** `cleanupAbandonInventory` 구조체(id, repo, branch, worktree root/present, branch OID, phase, lease status, pending operation id) + 신규 fingerprint 함수 — finish 패턴 차용이지 재사용 아님(design-review F12 정정). apply 직전 재계산 일치(TOCTOU).
- apply: fingerprint 일치 → **원격 무접촉** — 레코드(+해당 시 external intent 행) 원자 삭제. 사유·시각은 `--json` 결과에 echo.
- preview `--json`에 **삭제 대상 레코드 전문 포함**(design-review F7 완화책 — "삭제 전 보존" 불변식 C2-F6을 이 경로가 의도적으로 예외 처리함을 계획·usage에 명시, 운영자가 삭제 전 캡처 가능).
- ~~ReflectCleanupAudit 재사용~~ **삭제**(design-review F3 critical): 빈 completion payload가 열린 이슈에 가짜 "완료 기록" 섹션을 append하고(`issue_body_section.go:159-170`), `completion_reflected` 게이트(`feedback_cleanup.go:302`)가 마커 존재만 보므로 미래 사이클의 파괴적 finish가 영구 개방된다. port에 대체 표면 없음(닫힌 섹션 집합, 코멘트 API 부재 — design-review F4). 감사 채널은 로컬 결과 JSON뿐.

### CLI/parity (design-review F10 — 계획 열거 정확 판정)

- `feedbackcleanup.RunCleanup` "abandon" 케이스 + help 문자열(`feedback_cleanup.go:79`, canonical 파생 비교).
- commandparse `cleanup abandon` spec: values(--id, --reason, --fingerprint), booleans(--preview, --apply, --confirm, --json).
- usage 2원문(adapter `usage.go` + `issueOpsUsageText()`) byte 동일 라인 추가.
- golden `-update` 재생성. dispatch registry·mcp_tools.golden 무영향(실측 확인됨).

## TDD 순서

1. RED: 게이트 8종 각각 거부(lease 4값 전수 테이블 포함) + pending kind별 허용/거부 + fingerprint 불일치 거부 + 성공 경로(레코드·intent 행 원자 삭제, preview 레코드 전문 포함) 단위 테스트.
2. GREEN: core 구현 → CLI 배선 → usage/spec/golden 정합.
3. 회귀: issueops·issueopscli·issueopsapp·전체 green.

## AC 매핑

- AC-01: 머지 후 io-ff5300b4aa0b 실환경 abandon → doctor healthy 실증(**resource kind=cycle 분기 A 실측 확인** — 레코드 삭제로 finding 소멸. 별도 사용자 승인 후 실행).
- AC-02: 게이트 테스트가 활성/claimable/revoking lease·artifact 보유·자식 보유·remote-kind pending 삭제 불가를 고정.
- AC-03: 전체 green + parity + golden.

## 비범위

- prune 자격 확장, reset-legacy drain 확장, orca task/worktree registry 잔여 정리(#99), 원격 브랜치 잔여 검사(design-review F8 — doctor `non_main_branch_residue`가 별도 추적).

## 역할 분담

- 계획·design-review 리뷰: Fable 5(메인). 구현안: Opus 5 서브에이전트(적용은 holder).
