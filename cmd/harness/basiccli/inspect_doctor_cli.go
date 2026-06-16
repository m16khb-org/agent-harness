package basiccli

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

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
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	home, _ := os.UserHomeDir()
	result, err := core.HarnessDoctor(core.HarnessDoctorRequest{RepoRoot: *repo, HarnessRoot: deps.HarnessRoot(), Home: home, Version: deps.Version})
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
