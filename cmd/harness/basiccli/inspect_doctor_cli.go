package basiccli

import (
	doctorcontract "agent-harness/internal/contract/doctor"
	"agent-harness/internal/domain/operationalhealth"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type doctorRepeatedFlag []string

func (values *doctorRepeatedFlag) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *doctorRepeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	repo := fs.String("repo", "", "target repo/workspace")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" && fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	info := deps.InspectHarness(*repo)
	if *jsonOut {
		return printJSON(info)
	}
	fmt.Printf("agent-harness root: %s\n", info.HarnessRoot)
	fmt.Printf("target repo: %s\n", info.TargetRepo)
	fmt.Printf("skills: %d\n", len(info.Skills))
	for _, s := range info.Skills {
		fmt.Printf("- %s (%s)\n", s.Name, s.Path)
	}
	fmt.Printf("codex skill installed: %v\n", info.Integration.CodexSkillInstalled)
	fmt.Printf("claude skill installed: %v\n", info.Integration.ClaudeSkillInstalled)
	fmt.Printf("project Claude MCP config: %v\n", info.Integration.ProjectClaudeMCPConfig)
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	sealed := fs.Bool("sealed", false, "use the sealed audit profile: unowned live terminals and orchestration message history count as residue")
	var preserveCycles doctorRepeatedFlag
	var preserveTerminals doctorRepeatedFlag
	fs.Var(&preserveCycles, "preserve-cycle", "preserve one exact IssueOps cycle for this invocation (repeatable)")
	fs.Var(&preserveTerminals, "preserve-terminal", "preserve one exact terminal handle for this invocation (repeatable)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: agent-harness doctor [--repo PATH] [--sealed] [--preserve-cycle ID]... [--preserve-terminal HANDLE]... [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	cycleIDs, err := normalizeDoctorPreserve(preserveCycles, "--preserve-cycle")
	if err != nil {
		return err
	}
	terminalHandles, err := normalizeDoctorPreserve(preserveTerminals, "--preserve-terminal")
	if err != nil {
		return err
	}
	root, err := NormalizeRepoRoot(*repo)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	snapshot := deps.CollectOperationalHealth(context.Background(), root)
	home, _ := os.UserHomeDir()
	daemon := deps.CheckDaemonStatus()
	result, err := harnessDoctor(doctorcontract.HarnessDoctorRequest{
		RepoRoot:            root,
		HarnessRoot:         deps.HarnessRoot(),
		Home:                home,
		Version:             deps.Version,
		OperationalSnapshot: &snapshot,
		OperationalOptions: operationalhealth.Options{
			Now:                     now,
			Profile:                 doctorProfile(*sealed),
			PreserveCycleIDs:        cycleIDs,
			PreserveTerminalHandles: terminalHandles,
		},
		DaemonAdmission: doctorcontract.HarnessDoctorDaemonAdmission{
			ActiveConnections: daemon.ActiveConnections,
			MaxConnections:    daemon.MaxConnections,
			Accepting:         daemon.Accepting,
			Draining:          daemon.Draining,
		},
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Healthy {
		fmt.Printf("agent-harness doctor healthy: %s\n", result.RepoRoot)
		return nil
	}
	fmt.Printf("agent-harness doctor found %d issues for %s\n", len(result.Issues), result.RepoRoot)
	for _, issue := range result.Issues {
		fmt.Printf("%s %s %s\n", issue.Severity, issue.Code, issue.Summary)
		if issue.Fix != nil && issue.Fix.Command != "" {
			fmt.Printf("  fix: %s\n", issue.Fix.Command)
		}
	}
	return nil
}

func normalizeDoctorPreserve(values []string, flagName string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%s requires a non-empty value", flagName)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

// doctorProfile은 CLI 기본값을 interactive profile로 매핑한다. 실제 개발자
// 머신에서는 사용자가 연 Orca 탭과 orchestration 메시지 이력이 정상이기
// 때문이다. sealed audit 호출자는 엄격한 residue 계약을 다시 선택한다.
func doctorProfile(sealed bool) string {
	if sealed {
		return operationalhealth.ProfileSealed
	}
	return operationalhealth.ProfileInteractive
}
