package installcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	activationapp "agent-harness/internal/application/nativeactivation"
	activationcontract "agent-harness/internal/contract/nativeactivation"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
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
	if deps.NativeInstallRequest == nil || deps.InstallNative == nil {
		return fmt.Errorf("native installer is not configured")
	}
	req := deps.NativeInstallRequest(root, home, codexHome, filepath.Join(root, "bin", "agent-harness"))
	req.ProjectLocal = *projectLocal
	req.DryRun = *dryRun
	stateDir := filepath.Dir(IssueOpsStateRoot())
	activationRequest := activationcontract.Request{StateRoot: stateDir, HarnessRoot: req.Root, TargetBinary: req.BinPath}
	if !req.DryRun && deps.ActivationBackend == nil {
		return fmt.Errorf("native activation backend is unavailable")
	}
	activationStep, err := nativeActivationStep(req.DryRun, os.Getenv("HARNESS_NATIVE_ACTIVATION_STEP"))
	if err != nil {
		return err
	}
	if deps.ActivationReadback == nil {
		return fmt.Errorf("native activation readback is not configured")
	}
	activationService := activationapp.NewService(deps.ActivationBackend, deps.ActivationReadback(req))
	if !req.DryRun && activationStep != "seal" {
		pending, err := activationService.Begin(context.Background(), activationRequest)
		if err != nil {
			return fmt.Errorf("begin native activation: %w", err)
		}
		if activationStep == "begin" {
			if *jsonOut {
				return printJSON(pending)
			}
			fmt.Printf("native activation candidate %s is pending\n", pending.BinarySHA256)
			return nil
		}
	}
	result, err := deps.InstallNative(req)
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

func nativeActivationStep(dryRun bool, raw string) (string, error) {
	step := strings.TrimSpace(raw)
	if dryRun && step != "" {
		return "", fmt.Errorf("native activation step is not valid during dry-run")
	}
	switch step {
	case "", "begin", "seal":
		return step, nil
	default:
		return "", fmt.Errorf("invalid native activation step %q", step)
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
