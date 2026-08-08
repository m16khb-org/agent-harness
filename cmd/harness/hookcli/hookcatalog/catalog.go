package hookcatalog

import (
	hookfailurecontract "agent-harness/internal/contract/hookfailure"
	hookmetricscontract "agent-harness/internal/contract/hookmetrics"
	workercontract "agent-harness/internal/contract/worker"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookadapter "agent-harness/internal/adapter/hook"
	hookprompt "agent-harness/internal/adapter/hookprompt"
	coreinstall "agent-harness/internal/adapter/install"
	lifecycle "agent-harness/internal/adapter/lifecycle"
)

type Config struct {
	// 정체된 worker job 탐지는 composition root가 주입한다.
	MaybeDetectStuckWorkerJobs func(minInterval time.Duration) (workercontract.WorkerListResult, bool, error)
	// prune 연산은 composition root가 주입한다.
	PruneHookFailureLog func(maxAge time.Duration) (hookfailurecontract.HookFailurePruneResult, error)
	PruneHookMetricsLog func(maxAge time.Duration) (hookmetricscontract.HookMetricsPruneResult, error)
	ResolveTarget       func(string) string
	PrintJSON           func(any) error
	RuntimeDiagnostic   func() (coreinstall.NativeRuntimeDiagnostic, error)
}

func RunPostCompact(args []string, config Config) error {
	fs := flag.NewFlagSet("hook post-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = config.ResolveTarget("")
	}
	result := lifecycle.BuildLifecyclePostCompactReminder(parsedRepo)
	if *jsonOut {
		return config.PrintJSON(result)
	}
	cat := hookprompt.BuildProjectDocCatalogContext(parsedRepo)

	// Codex post-compact uses systemMessage only.
	if hostOf(hostFlag) == "codex" {
		context := strings.TrimSpace(result.AdditionalContext)
		if context == "" && cat.ShouldInject {
			context = strings.TrimSpace(cat.UserView)
		}
		if context == "" {
			return config.PrintJSON(map[string]any{})
		}
		return config.PrintJSON(map[string]any{"systemMessage": context})
	}

	// Non-Codex post-compact: default host is Claude for catalog hooks.
	ho := resolveCatalogHost(hostFlag)
	if cat.ShouldInject {
		return config.PrintJSON(ho.FormatContext("PostCompact", joinContext(result.AdditionalContext, cat.Compact), cat.UserView))
	}

	// No catalog injection: just the reminder context.
	return config.PrintJSON(ho.FormatContext("PostCompact", result.AdditionalContext, ""))
}

func RunSessionStart(args []string, config Config) error {
	fs := flag.NewFlagSet("hook session-start", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	if config.RuntimeDiagnostic != nil {
		diagnostic, err := config.RuntimeDiagnostic()
		if reason, blocked := coreinstall.NativeRuntimeDiagnosticMessage(diagnostic, err); blocked {
			if *jsonOut {
				return config.PrintJSON(diagnostic)
			}
			ho := resolveCatalogHost(hostFlag)
			return config.PrintJSON(ho.FormatContext("SessionStart", reason, reason))
		}
	}
	// Best-effort rotation of the hook-failure log (quality program Q2 /
	// audit P1): the JSONL grew without bound because pruning required a
	// manual command. Session start is the natural low-frequency hook for it.
	// prune은 best-effort 유지보수다. 주입되지 않았으면 hook 자체를 실패시키지 않고
	// 건너뛴다.
	if config.PruneHookFailureLog != nil {
		_, _ = config.PruneHookFailureLog(720 * time.Hour)
	}
	if config.PruneHookMetricsLog != nil {
		_, _ = config.PruneHookMetricsLog(720 * time.Hour)
	}
	// Best-effort self-heal of crashed worker jobs (A2/W1): mark dead-PID running
	// jobs failed. Amortized to at most once per 6h via a stat-only sentinel
	// because the detector is an unbounded full-dir scan and the worker dir has
	// no TTL — running it every session start would grow the hot path unbounded.
	// Best-effort store maintenance: WAL checkpoint truncate + sidecar
	// permission repair on known sqlite state roots. Amortized to at most
	// once per 24h via a stat-only sentinel — maintenance is cheap (ms) but
	// unnecessary on every session start.
	_, _, _ = MaybeMaintainStateStores(24 * time.Hour)
	if config.MaybeDetectStuckWorkerJobs != nil {
		_, _, _ = config.MaybeDetectStuckWorkerJobs(6 * time.Hour)
	}
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = config.ResolveTarget("")
	}
	cat := hookprompt.BuildProjectDocCatalogContext(parsedRepo)
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	// Default host for catalog hooks is Claude (not Codex).
	ho := resolveCatalogHost(hostFlag)
	if hookinput.SourceFromHookInput(stdin) == "compact" {
		return config.PrintJSON(ho.FormatContext("SessionStart", "", ""))
	}
	if !cat.ShouldInject {
		return config.PrintJSON(ho.FormatContext("SessionStart", "", ""))
	}
	return config.PrintJSON(ho.FormatContext("SessionStart", cat.Compact, cat.UserView))
}

// resolveCatalogHost returns the hook output adapter for catalog hooks.
// Catalog hooks (SessionStart, PostCompact) default to Claude, not Codex.
func resolveCatalogHost(hostFlag *string) hookadapter.HostHookOutput {
	switch hostOf(hostFlag) {
	case "codex":
		return hookadapter.CodexHookOutput{}
	default:
		return hookadapter.ClaudeHookOutput{}
	}
}

// joinContext concatenates a prefix context with a catalog context.
func joinContext(prefix, catalog string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return catalog
	}
	return prefix + "\n" + catalog
}

func hostOf(hostFlag *string) string {
	return strings.ToLower(strings.TrimSpace(*hostFlag))
}
