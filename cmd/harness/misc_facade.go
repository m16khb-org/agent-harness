package main

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"agent-harness/cmd/harness/apidoc"
	"agent-harness/cmd/harness/contractcli"
	"agent-harness/cmd/harness/pathutil"
	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/core"
)

type CompatibilityContract = contractcli.CompatibilityContract

var (
	errAPIDocReviewGateFailed     = apidoc.ErrReviewGateFailed
	errAPIDocStaticGateFailed     = apidoc.ErrStaticGateFailed
	errSelfVerificationGateFailed = selfworkflow.ErrSelfVerificationGateFailed
)

func configureContractCLI() {
	contractcli.MCPTools = mcpTools
}

func runContract(args []string) error {
	configureContractCLI()
	return contractcli.Run(args)
}

func compatibilityContract() CompatibilityContract {
	configureContractCLI()
	return contractcli.BuildCompatibilityContract()
}

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, errAPIDocReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, errAPIDocStaticGateFailed)
}

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, errSelfVerificationGateFailed)
}

func printJSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func readHarnessFile(parts ...string) (string, error) {
	return pathutil.ReadHarnessFile(harnessRoot(), parts...)
}

func harnessRoot() string {
	return pathutil.HarnessRoot(filepath.Join("skills", skillName, "SKILL.md"))
}

func findUp(start, marker string) (string, bool) {
	return pathutil.FindUp(start, marker)
}

func resolveTarget(arg string) string {
	return pathutil.ResolveTarget(arg)
}

func exists(path string) bool {
	return pathutil.Exists(path)
}

func splitLines(s string) []string {
	return pathutil.SplitLines(s)
}

func splitCSV(s string) []string {
	return pathutil.SplitCSV(s)
}

func containsString(items []string, want string) bool {
	return pathutil.ContainsString(items, want)
}

func stateDoctorHasIssueCode(issues []core.StateDoctorIssue, want string) bool {
	return pathutil.StateDoctorHasIssueCode(issues, want)
}
