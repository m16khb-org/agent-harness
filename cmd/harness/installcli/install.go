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
	"agent-harness/internal/port"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write project-local .mcp.json/.claude settings and project skill links; default is user/global only")
	dryRun := fs.Bool("dry-run", false, "plan files and links without writing them")
	pathMode := fs.String("path-mode", "auto", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	adoptCommandFile := fs.Bool("adopt-command-file", false, "replace an existing managed agent-harness command file with rollback protection")
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
	req.AdoptCommandFile = *adoptCommandFile
	candidatePath, err := nativeInstallCandidatePath(req.BinPath, req.DryRun, deps.ExecutablePath)
	if err != nil {
		return err
	}
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
	readback := deps.ActivationReadback(req)
	activationService := activationapp.NewService(deps.ActivationBackend, readback)
	return executeInstall(req, candidatePath, *pathMode, activationStep, *jsonOut, activationService, activationRequest)
}

func executeInstall(req port.NativeInstallRequest, candidatePath, pathMode, activationStep string, jsonOut bool, activationService *activationapp.Service, activationRequest activationcontract.Request) error {
	if activationStep == "abort" {
		activationRequest.TransitionID = os.Getenv("HARNESS_NATIVE_ACTIVATION_TRANSITION_ID")
		aborted, abortErr := activationService.Abort(context.Background(), activationRequest)
		if abortErr != nil {
			return fmt.Errorf("abort native activation: %w", abortErr)
		}
		if jsonOut {
			return printJSON(aborted)
		}
		fmt.Printf("native activation transition %s aborted\n", aborted.TransitionID)
		return nil
	}
	preflight := port.NativeInstallResult{OK: true, Root: req.Root, Home: req.Home, CodexHome: req.CodexHome, BinPath: req.BinPath}
	pathTransaction, err := prepareInstallPathPlanForCandidate(&preflight, req, candidatePath, pathMode)
	if err != nil {
		return err
	}
	if req.DryRun {
		result, installErr := deps.InstallNative(req)
		result.Links = append(preflight.Links, result.Links...)
		result.Files = append(preflight.Files, result.Files...)
		result.Messages = append(preflight.Messages, result.Messages...)
		result.CommandPath = preflight.CommandPath
		return outputInstallResult(result, installErr, jsonOut)
	}
	hostPlanReq := req
	hostPlanReq.DryRun = true
	hostPlan, hostPlanErr := deps.InstallNative(hostPlanReq)
	if hostPlanErr != nil || !hostPlan.OK {
		if hostPlanErr == nil {
			hostPlanErr = fmt.Errorf("native installer dry-run preflight reported ok=false")
		}
		return finishHostPreflightFailure(&hostPlan, hostPlanErr, preflight, activationStep, jsonOut)
	}
	if hostPlanErr = planShellPath(&hostPlan, hostPlanReq, pathMode); hostPlanErr != nil {
		return finishHostPreflightFailure(&hostPlan, hostPlanErr, preflight, activationStep, jsonOut)
	}
	hostTransaction, hostTransactionErr := prepareInstallHostTransaction(hostPlan)
	if hostTransactionErr != nil {
		return finishHostPreflightFailure(&hostPlan, hostTransactionErr, preflight, activationStep, jsonOut)
	}
	done, err := prepareActivationTransition(activationService, &activationRequest, activationStep, jsonOut)
	if err != nil || done {
		return err
	}
	return applyAndSealInstall(req, pathMode, activationStep, jsonOut, activationService, activationRequest, preflight, pathTransaction, hostTransaction)
}

func finishHostPreflightFailure(result *port.NativeInstallResult, cause error, preflight port.NativeInstallResult, step string, jsonOut bool) error {
	result.OK = false
	result.Links = append(preflight.Links, result.Links...)
	result.CommandPath = preflight.CommandPath
	if step == "seal" {
		result.TransitionID = os.Getenv("HARNESS_NATIVE_ACTIVATION_TRANSITION_ID")
		result.AbortRequired = true
		if result.CommandPath != nil {
			result.CommandPath.AbortRequired = true
		}
	}
	return outputInstallResult(*result, cause, jsonOut)
}

func nativeInstallCandidatePath(target string, dryRun bool, executable func() (string, error)) (string, error) {
	if executable == nil {
		return "", fmt.Errorf("native install executable inspector is unavailable")
	}
	candidate, err := executable()
	if err != nil {
		return "", fmt.Errorf("inspect native install candidate: %w", err)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("inspect native install candidate: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("inspect native install target: %w", err)
	}
	candidate = filepath.Clean(candidate)
	target = filepath.Clean(target)
	if !dryRun && (filepath.Dir(candidate) != filepath.Dir(target) ||
		(candidate != target && !strings.HasPrefix(filepath.Base(candidate), ".agent-harness.activate-"))) {
		return "", fmt.Errorf("native install candidate must be the canonical target or a same-directory staged binary")
	}
	return candidate, nil
}

func prepareActivationTransition(service *activationapp.Service, request *activationcontract.Request, step string, jsonOut bool) (bool, error) {
	if step == "seal" {
		request.TransitionID = os.Getenv("HARNESS_NATIVE_ACTIVATION_TRANSITION_ID")
		return false, nil
	}
	pending, beginErr := service.Begin(context.Background(), *request)
	if beginErr != nil {
		return false, fmt.Errorf("begin native activation: %w", beginErr)
	}
	request.TransitionID = pending.TransitionID
	if step == "begin" {
		if jsonOut {
			return true, printJSON(pending)
		}
		fmt.Printf("native activation candidate %s is pending as transition %s\n", pending.BinarySHA256, pending.TransitionID)
		return true, nil
	}
	return false, nil
}

func applyAndSealInstall(req port.NativeInstallRequest, pathMode, activationStep string, jsonOut bool, activationService *activationapp.Service, activationRequest activationcontract.Request, preflight port.NativeInstallResult, pathTransaction *installPathTransaction, hostTransaction *installHostTransaction) error {
	result := preflight
	result.TransitionID = activationRequest.TransitionID
	if applyErr := pathTransaction.apply(&result); applyErr != nil {
		return finishFailedInstall(&result, applyErr, pathTransaction, hostTransaction, activationService, activationRequest, activationStep, jsonOut)
	}
	installed, installErr := deps.InstallNative(req)
	installed.Links = append(result.Links, installed.Links...)
	installed.CommandPath = result.CommandPath
	installed.TransitionID = activationRequest.TransitionID
	result = installed
	if installErr == nil && result.OK {
		installErr = planShellPath(&result, req, pathMode)
	}
	if installErr != nil || !result.OK {
		return finishFailedInstall(&result, installErr, pathTransaction, hostTransaction, activationService, activationRequest, activationStep, jsonOut)
	}
	sealed, sealErr := activationService.Seal(context.Background(), activationRequest)
	if sealErr != nil || !sealed.OK || !sealed.Sealed || sealed.Receipt == nil {
		if sealErr == nil {
			sealErr = fmt.Errorf("native activation receipt was not sealed")
		}
		return finishFailedInstall(&result, sealErr, pathTransaction, hostTransaction, activationService, activationRequest, activationStep, jsonOut)
	}
	result.Committed = true
	result.Messages = append(result.Messages, "native activation receipt sealed after strict Codex/Claude/Omo MCP and lifecycle readback")
	if finalizeErr := pathTransaction.finalize(&result); finalizeErr != nil {
		result.Messages = append(result.Messages, "native activation is committed; command backup cleanup requires manual recovery: "+finalizeErr.Error())
		if result.CommandPath != nil {
			result.CommandPath.BackupRetained = true
		}
	}
	return outputInstallResult(result, nil, jsonOut)
}

func finishFailedInstall(result *port.NativeInstallResult, cause error, transaction *installPathTransaction, hostTransaction *installHostTransaction, service *activationapp.Service, request activationcontract.Request, step string, jsonOut bool) error {
	result.OK = false
	if cause == nil {
		cause = fmt.Errorf("native installer reported ok=false")
	}
	hostRollbackErr := hostTransaction.rollback()
	rollbackErr := transaction.rollback(result)
	if step == "seal" {
		result.AbortRequired = true
		if result.CommandPath != nil {
			result.CommandPath.AbortRequired = true
		}
	} else {
		_, abortErr := service.Abort(context.Background(), request)
		cause = errors.Join(cause, abortErr)
	}
	cause = errors.Join(cause, hostRollbackErr, rollbackErr)
	return outputInstallResult(*result, cause, jsonOut)
}

func outputInstallResult(result port.NativeInstallResult, err error, jsonOut bool) error {
	if jsonOut {
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
	case "", "begin", "seal", "abort":
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
