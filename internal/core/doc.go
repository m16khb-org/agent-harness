// Package core는 harness domain layer이자 cmd/ command와 MCP layer가 의존하는
// 유일한 안정 public surface다. 호출자는 core의 internal subpackage
// (core/issueops, core/lifecycle, core/worker, ...)가 아니라 core를 import하므로,
// 모든 호출자를 고치지 않고도 internal package layout을 바꿀 수 있다.
//
// # Facade 규칙(*_facade.go)
//
// *_facade.go 파일(issueops, workflow, utility, policy, project_doc,
// state_trace, draft_wiki, issueops_remote)이 이 public surface다. 의도적으로
// 얇게 유지하며 다음만 둘 수 있다.
//
//  1. subpackage type을 public surface로 다시 내보내는 type alias. 예:
//     `type WorkerJob = worker.WorkerJob`.
//  2. boundary를 넘는 type conversion. 예: core alias request를 위임 전
//     subpackage 자체 type으로 변환한다.
//  3. 여러 subpackage를 하나의 result로 묶는 composition. 예:
//     SummarizeHookFailureStats는 failure log와 hook metric을 합친다.
//  4. domain boundary를 지키도록 호출을 변환하거나 guard하는 boundary enforcement.
//     예: concrete adapter를 import하는 대신 호출자가 준 port.IssueProvider를
//     해결한다.
//
// 순수한 한 줄 위임(`func F(x) T { return sub.F(x) }`)은 허용되며 필요하다. 이는
// 우연한 barrel overhead가 아니라 public surface를 안정적으로 유지하고 internal
// package layout과 분리하는 장치다. facade audit은 모든 exported facade symbol에
// 실제 호출자가 있음을 확인했다. cmd/가 core의 internal 구조에 결합되므로 delegate를
// cmd/의 직접 subpackage import로 "평탄화"하지 않는다.
//
// cmd/harness는 stable harness domain API가 아니라 특정 subpackage mechanism을
// 검증하는 cmd-local tooling, diagnostics, test에서만 core subpackage를 직접
// import할 수 있다. State와 IssueOps lifecycle access는 cross-command contract인
// record를 다루므로 core facade 뒤에 둔다. internal-only inspection helper를
// 숨기려 facade wrapper를 추가하지 않는다. 직접 import를 tool에 국한하거나, 실제
// external caller가 필요할 때 behavior를 승격한다.
//
// facade에 두면 안 되는 것은 새 domain logic이다. facade function이
// conversion/composition/enforcement를 넘어서 커지면, logic을 소유 subpackage로
// 옮기고 facade는 얇게 유지한다.
package core
