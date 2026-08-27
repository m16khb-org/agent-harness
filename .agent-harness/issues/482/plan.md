# 482 — Orca 봉인 아티팩트 디렉터리를 이슈 폴더 아래로

이슈: https://github.com/m16khb/agent-harness/issues/482
브랜치: `482-sealed-artifact-dir` (base: `origin/main`)

## 결함

봉인 아티팩트 디렉터리가 상수 `IssueOpsArtifactDir`/`completionArtifactDir`(`.agent-harness/artifact`)로 다섯 곳(`issueops_artifact_stage.go:140,185`, `execution_owner_context.go:359`, `execution_resume.go:146,159`, `issueops_completion_remote.go:157`)에서 각각 재계산된다. 레코드에는 `PlanPath`가 절대 경로로 저장되는데 읽는 쪽은 저장값을 보지 않으므로, 디렉터리를 바꾸는 순간 저장 경로와 계산 경로가 어긋나고 completion은 조용히 빈 본문을 낸다.

## 설계 (v2 — brooks `revise` 반영)

v1의 "저장 경로 우선 4규칙 해석기"는 사용자가 `link-plan`으로 `.agent-harness/issues/<n>/plan.md`(봉인 아님)를 연결한 지배적 실사용 경로에서 규칙 1이 매치되지 않아 디스크 존재 여부(규칙 2)가 실질 결정자가 되고, 쓰기/읽기 시점 판정이 어긋날 수 있었다. 이슈가 요구한 대로 **레코드 필드 하나**로 결정한다.

- `internal/contract/issueops/execution.go` `Workspace`에 `ArtifactDir string \`json:"artifact_dir,omitempty"\`` (워크트리 상대, slash). lease/preparation 계약의 Workspace는 receipt(출력 DTO)라 건드리지 않는다.
- 순수 해석기 `sealedArtifactDir(record) string`: `Workspace.ArtifactDir`가 있으면 그것, 없으면 `IssueOpsArtifactDir`. 파일시스템 탐색·PlanPath 파싱 없음.
- 채우는 곳: Workspace를 구성하는 4곳(`execution_prepare_bridge.go:75,119`, `execution_orca_intent.go:153,235` — `workspaceFromReceipt` 경유)에서 linked issue 번호 `n`이 있으면 `.agent-harness/issues/<n>/artifact`, 없으면 빈 값(=legacy).
- 읽는 곳 5곳(`issueops_artifact_stage.go:140,185`, `execution_owner_context.go:359`, `execution_resume.go:146,159`, `issueops_completion_remote.go:157`)을 해석기로 치환, `completionArtifactDir` 삭제.
- completion: `MissingArtifacts`는 고정 3종이 아니라 **plan만** 기준(봉인 manifest는 레코드에 없고 spec/turing-loop는 선택). `plan.md`가 없을 때 `IssueProviderCompletionSection.MissingArtifacts=["plan"]`, 렌더러가 "봉인 아티팩트 없음: plan" 한 줄.
- 기존 레코드(필드 없음)는 legacy 경로 그대로. 옛 바이너리는 필드를 무시하고 legacy를 읽으므로 "열린 Orca 사이클 없을 때 배포".

## 작업

| # | 작업 | 파일 | 검증 |
|---|---|---|---|
| T1 | 필드 + `sealedArtifactDir`/`sealedArtifactPath` 해석기 + 단위 테스트 | contract, issueops_artifact_stage.go | `go test ./internal/adapter/issueops -run SealedArtifact` |
| T2 | `workspaceFromReceipt`와 bridge:75가 `artifact_dir`를 채움(issue 번호 기반); 5개 읽기 치환; `FromSlash(IssueOpsArtifactDir)` 직접 사용 0건 | 위 파일들 | 기존 owner packet/orca intent/resume/replace/mode-switch 테스트의 경로 단언을 해석기 기준으로 갱신 |
| T3 | completion `MissingArtifacts`(plan 기준) + 렌더 한 줄 | port/provider.go, completion remote, 렌더러 | 렌더 테스트/골든 |
| T4 | `.gitignore` `issues/*/artifact/`; CONVENTIONS 레이아웃 표 `artifact/` 행; #480 "범위 밖" 문장 정정 | 문서 | docs checker |
| T5 | 골든(workspace 스키마에 artifact_dir)·배터리 | | AGENTS.md §9 |

## 비목표

봉인 아티팩트 git 추적, #480 레이아웃 변경, lease/preparation 계약 변경.

## 롤백

단일 커밋 revert; 옛 바이너리는 legacy 경로를 읽으므로 새 경로에 봉인된 열린 사이클이 없을 때 배포한다.
