package basiccli

import (
	preflightcontract "agent-harness/internal/contract/preflight"
	"context"
	"encoding/json"
	"os"

	"agent-harness/cmd/harness/daemoncli"
	inspect "agent-harness/internal/contract/inspect"
	"agent-harness/internal/domain/operationalhealth"
)

// Deps는 basic CLI 명령들이 의존하는, 호스트가 제공하는 구현을 담는다.
// composition root가 Configure로 실제 구현을 주입하며, 단독 실행과 테스트는
// 기본값으로 대체한다.
type Deps struct {
	// GitPreflight는 composition root가 주입한다. git 실행은 CLI의 일이 아니다.
	GitPreflight             func(target, harnessRoot string) preflightcontract.PreflightResult
	HarnessRoot              func() string
	ResolveTarget            func(string) string
	Version                  string
	InspectHarness           func(string) inspect.InspectInfo
	CheckDaemonStatus        func() daemoncli.Status
	CollectOperationalHealth func(context.Context, string) operationalhealth.Snapshot
}

// deps는 현재 구성된 의존성을 담는다. package-private이며 Configure/Reset을
// 통해서만 변경되므로, import 순서에 민감한 init() 부수효과가 아니라 명시적으로
// 와이어링된다.
var deps = defaultDeps()

// Configure는 호스트가 제공하는 의존성을 설치한다. composition root가 시작 시
// 한 번 호출하며, 테스트는 fake로 호출한 뒤 t.Cleanup에서 Reset으로 복원한다.
func Configure(d Deps) { deps = d }

// Reset은 단독 실행 기본값을 복원한다. 테스트는 주입한 fake가 테스트 간에 새지
// 않도록 이를 defer한다.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{
		HarnessRoot:       defaultHarnessRoot,
		ResolveTarget:     defaultResolveTarget,
		Version:           "dev",
		InspectHarness:    func(string) inspect.InspectInfo { return inspect.InspectInfo{} },
		CheckDaemonStatus: daemoncli.CheckDaemonStatus,
		CollectOperationalHealth: func(_ context.Context, repo string) operationalhealth.Snapshot {
			return operationalhealth.Snapshot{
				RepoRoot: repo,
				InventoryProblems: []operationalhealth.InventoryProblem{{
					Source: "doctor", Code: "operational_collector_unconfigured", Detail: "operational inventory collector is not configured",
				}},
			}
		},
	}
}

func defaultHarnessRoot() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func defaultResolveTarget(target string) string {
	if target != "" {
		return target
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
