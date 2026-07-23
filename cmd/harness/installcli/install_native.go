package installcli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/adapter/installutil"
	mcpadapter "agent-harness/internal/adapter/mcp"
	"agent-harness/internal/core"
	"agent-harness/internal/port"
)

func runInstall(args []string) error {
	return runInstallCommand("install", args)
}

func runInstallNative(args []string) error {
	return runInstallCommand("install-native", args)
}

func runInstallCommand(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write project-local .mcp.json/.claude settings and project skill links; default is user/global only")
	dryRun := fs.Bool("dry-run", false, "plan files and links without writing them")
	pathMode := fs.String("path-mode", "auto", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	home, err := installUserHomeDir()
	if err != nil {
		return err
	}
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	if *interactive {
		if err := validateInteractiveInstallInput(os.Stdin); err != nil {
			return err
		}
		choices, err := promptInstallChoices(*projectLocal, *dryRun, *pathMode, os.Stdin, os.Stderr)
		if err != nil {
			return err
		}
		*projectLocal = choices.ProjectLocal
		*dryRun = choices.DryRun
		*pathMode = choices.PathMode
	}
	if !validInstallPathMode(*pathMode) {
		return fmt.Errorf("invalid --path-mode %q: expected auto, manual, or skip", *pathMode)
	}
	root := deps.HarnessRoot()
	req := core.DefaultNativeInstallRequest(root, home, codexHome, filepath.Join(root, "bin", "agent-harness"))
	req.ProjectLocal = *projectLocal
	req.DryRun = *dryRun
	stateDir := filepath.Dir(core.IssueOpsStateRoot())
	if !req.DryRun {
		if _, err := core.BeginLegacyResetActivation(stateDir, core.LegacyResetActivationBeginRequest{
			TargetSchema: 1, HarnessRoot: req.Root, TargetBinary: req.BinPath,
		}); err != nil {
			return fmt.Errorf("begin native activation: %w", err)
		}
	}
	result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
	if pathErr := applyInstallPathPlan(&result, req, *pathMode); pathErr != nil {
		result.OK = false
		err = errors.Join(err, pathErr)
	}
	if !req.DryRun && err == nil && result.OK {
		activationErr := sealNativeActivation(stateDir, req)
		if activationErr != nil {
			result.OK = false
			err = errors.Join(err, activationErr)
		} else {
			result.Messages = append(result.Messages, "native activation receipt sealed after strict Codex/Claude MCP and hook readback")
		}
	}
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printInstallNativeResult(result)
	return err
}

func sealNativeActivation(stateDir string, req port.NativeInstallRequest) error {
	codexEvidence, err := codexadapter.VerifyActivation(req)
	if err != nil {
		return err
	}
	claudeEvidence, err := claudeadapter.VerifyActivation(req)
	if err != nil {
		return err
	}
	tools := mcpadapter.IssueOpsBasicTools()
	if len(tools) != 1 || tools[0].Name != "issueops_execution" {
		return fmt.Errorf("IssueOps v1 MCP activation catalog must contain exactly issueops_execution")
	}
	catalogSHA, err := installutil.SemanticSHA256(tools)
	if err != nil {
		return err
	}
	_, err = core.SealLegacyResetActivation(stateDir, core.LegacyResetActivationSealRequest{
		TargetSchema: 1, HarnessRoot: req.Root, TargetBinary: req.BinPath, CatalogSHA256: catalogSHA,
		Evidence: append(codexEvidence, claudeEvidence...),
	})
	if err != nil {
		return fmt.Errorf("seal native activation: %w", err)
	}
	return nil
}

func installUserHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine user home directory: %w", err)
	}
	if home == "" {
		return "", errors.New("determine user home directory: empty path")
	}
	return home, nil
}

func validInstallPathMode(mode string) bool {
	switch mode {
	case "auto", "manual", "skip":
		return true
	default:
		return false
	}
}
