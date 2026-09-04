package issueopsapp

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"issueops/cmd/issueops/apidoc"
	"issueops/cmd/issueops/contractcli"
	"issueops/cmd/issueops/pathutil"
	"issueops/cmd/issueops/selfworkflow"
	statecontract "issueops/internal/contract/state"
)

type CompatibilityContract = contractcli.CompatibilityContract

type (
	apiDocStaticViolation = apidoc.StaticViolation
)

var (
	errAPIDocReviewGateFailed     = apidoc.ErrReviewGateFailed
	errAPIDocStaticGateFailed     = apidoc.ErrStaticGateFailed
	errSelfVerificationGateFailed = selfworkflow.ErrSelfVerificationGateFailed
)

func configureContractCLI() {
	contractcli.MCPTools = mcpTools
	contractcli.ConfigureConformance(contractcli.ConformanceDependencies{Root: issueOpsRoot, RunProcess: runToolConformanceLive})
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
	return pathutil.ReadHarnessFile(issueOpsRoot(), parts...)
}

func issueOpsRoot() string {
	return pathutil.IssueOpsRoot(filepath.Join("skills", skillName, "SKILL.md"))
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

func stateDoctorHasIssueCode(issues []statecontract.StateDoctorIssue, want string) bool {
	return pathutil.StateDoctorHasIssueCode(issues, want)
}

func runAPIDoc(args []string) error {
	return apidoc.Run(args)
}

func checkNestControllerStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestControllerStatic(file, text)
}

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestDTOStatic(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt, evidence string) string {
	return apidoc.BuildReviewPrompt(files, diff, extraPrompt, evidence)
}

func apiDocReviewEvidence(repo string, files []string) string {
	return apidoc.Evidence(repo, files)
}

func apiDocReviewSchema() map[string]any {
	return apidoc.ReviewSchema()
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	return apidoc.NormalizeFiles(repo, files)
}

func isAPIDocCandidate(file string) bool {
	return apidoc.IsCandidate(file)
}
