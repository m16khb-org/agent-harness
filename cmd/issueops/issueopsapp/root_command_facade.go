package issueopsapp

import (
	"errors"
	policycontract "issueops/internal/contract/policy"
	"os"
	"sync"

	"issueops/cmd/issueops/channelcli"
	"issueops/cmd/issueops/gatescli"
	"issueops/cmd/issueops/rootcmd"
	guard "issueops/internal/adapter/guard"
	cliadapter "issueops/internal/domain/cli"
)

var dependencyWiring sync.Once

func RunRootCommand(args []string) int {
	wireDependencies()
	return rootCommand().Run(args)
}

// wireDependencies injects the harness implementations into the leaf CLI
// packages. The composition root calls this explicitly at startup instead of
// relying on package init() side effects, so dependency wiring is ordered,
// visible, and not import-order sensitive.
func wireDependencies() {
	dependencyWiring.Do(func() {
		wireBasicCLIDeps()
		wireHostCLIDeps()
		wirePolicyCLIDeps()
		configureMCPCLI()
	})
}

func rootCommand() rootcmd.Command {
	command := rootcmd.Command{
		Version: version,
		Usage:   usage,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Runners: map[string]rootcmd.Runner{
			"inspect":       runInspect,
			"preflight":     runPreflight,
			"system-status": runStatus,
			"doctor":        runDoctor,
			"docs":          runDocs,
			"policy":        runPolicy,
			"verify-work":   runVerifyWork,
			"trace":         runTrace,
			"guard":         runGuard,
			"quality":       runQuality,
			"self-verify":   runSelfVerify,
			"self-augment":  runSelfAugment,
			"contract":      runContract,
			"state":         runState,
			"api-doc":       runAPIDoc,
			"hook":          runHook,
			"project":       runProject,
			"install":       runInstall,
			"update":        runUpdate,
			"bootstrap":     runBootstrap,
			"worker":        runWorker,
			"loop":          runLoop,
			"gates":         runGates,
			"channel":       runChannel,
			"web-fetch":     runWebFetch,
			"daemon":        runDaemon,
			"mcp":           runMCPCommand,
		},
		ErrorExitCode: rootSubcommandErrorExitCode,
	}
	for _, lifecycle := range cliadapter.LifecycleCommands() {
		name := lifecycle.Name
		command.Runners[name] = func(args []string) error { return runIssueOps(append([]string{name}, args...)) }
	}
	return command
}

func rootSubcommandErrorExitCode(name string, err error) int {
	switch name {
	case "policy":
		if policycontract.IsPolicyDenied(err) {
			return 3
		}
	case "guard":
		if guard.IsGuardBlocked(err) {
			return 3
		}
	case "gates":
		if _, ok := errors.AsType[gatescli.UsageError](err); ok {
			return 2
		}
	case "channel":
		if _, ok := errors.AsType[channelcli.TimedOutError](err); ok {
			return 1
		}
	}
	return 1
}
