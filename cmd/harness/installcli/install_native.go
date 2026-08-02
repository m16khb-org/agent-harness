package installcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/adapter/installutil"
	mcpadapter "agent-harness/internal/adapter/mcp"
	activationapp "agent-harness/internal/application/nativeactivation"
	activationcontract "agent-harness/internal/contract/nativeactivation"
	"agent-harness/internal/core"
	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
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
	activationRequest := activationcontract.Request{StateRoot: stateDir, HarnessRoot: req.Root, TargetBinary: req.BinPath}
	activationService := activationapp.NewService(coreActivationBackend{}, hostActivationReadback{request: req})
	if !req.DryRun {
		if _, err := activationService.Begin(context.Background(), activationRequest); err != nil {
			return fmt.Errorf("begin native activation: %w", err)
		}
	}
	result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
	if pathErr := applyInstallPathPlan(&result, req, *pathMode); pathErr != nil {
		result.OK = false
		err = errors.Join(err, pathErr)
	}
	if !req.DryRun && err == nil && result.OK {
		sealed, activationErr := activationService.Seal(context.Background(), activationRequest)
		if activationErr != nil {
			result.OK = false
			err = errors.Join(err, activationErr)
		} else if !sealed.OK || !sealed.Sealed || sealed.Receipt == nil {
			result.OK = false
			err = errors.Join(err, fmt.Errorf("native activation receipt was not sealed"))
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

type coreActivationBackend struct{}

func (coreActivationBackend) Begin(_ context.Context, request activationport.BeginRequest) (activationport.Result, error) {
	result, err := core.BeginLegacyResetActivation(request.StateRoot, core.LegacyResetActivationBeginRequest{
		TargetSchema: 1, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
	})
	return mapActivationResult(result), err
}

func (coreActivationBackend) Seal(_ context.Context, request activationport.SealRequest) (activationport.Result, error) {
	evidence := make([]port.NativeActivationEvidence, 0, len(request.Evidence))
	for _, item := range request.Evidence {
		evidence = append(evidence, port.NativeActivationEvidence{
			Host: item.Host, Surface: item.Surface, Path: item.Path, SemanticSHA256: item.SemanticSHA256,
			SHA256: item.SHA256, Mode: item.Mode, Size: item.Size, Device: item.Device, Inode: item.Inode,
		})
	}
	result, err := core.SealLegacyResetActivation(request.StateRoot, core.LegacyResetActivationSealRequest{
		TargetSchema: 1, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
		CatalogSHA256: request.CatalogSHA256, Evidence: evidence,
	})
	if err != nil {
		return activationport.Result{}, fmt.Errorf("seal native activation: %w", err)
	}
	return mapActivationResult(result), nil
}

type hostActivationReadback struct{ request port.NativeInstallRequest }

func (readback hostActivationReadback) Verify(_ context.Context, harnessRoot, targetBinary string) (activationport.Readback, error) {
	if readback.request.Root != harnessRoot || readback.request.BinPath != targetBinary {
		return activationport.Readback{}, fmt.Errorf("native activation readback target changed")
	}
	codexEvidence, err := codexadapter.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	claudeEvidence, err := claudeadapter.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	tools := mcpadapter.IssueOpsBasicTools()
	if len(tools) != 1 || tools[0].Name != "issueops_execution" {
		return activationport.Readback{}, fmt.Errorf("IssueOps v1 MCP activation catalog must contain exactly issueops_execution")
	}
	catalogSHA, err := installutil.SemanticSHA256(tools)
	if err != nil {
		return activationport.Readback{}, err
	}
	evidence := append(codexEvidence, claudeEvidence...)
	result := make([]activationport.Evidence, 0, len(evidence))
	for _, item := range evidence {
		result = append(result, activationport.Evidence{
			Host: item.Host, Surface: item.Surface, Path: item.Path, SemanticSHA256: item.SemanticSHA256,
			SHA256: item.SHA256, Mode: item.Mode, Size: item.Size, Device: item.Device, Inode: item.Inode,
		})
	}
	return activationport.Readback{CatalogSHA256: catalogSHA, Evidence: result}, nil
}

func mapActivationResult(result core.LegacyResetActivationResult) activationport.Result {
	return activationport.Result{
		StateRoot: result.StateRoot, HarnessRoot: result.HarnessRoot, TargetBinary: result.TargetBinary,
		BinarySHA256: result.BinarySHA256, Pending: result.Pending, Sealed: result.Sealed, UpdatedAt: result.UpdatedAt,
	}
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
